/**
 * T91 — one reusable, in-app confirm/prompt to replace the studio's blocking native browser dialogs.
 * The native ones can be silently disabled by the browser's "prevent this page from creating
 * additional dialogs" affordance — after which every destructive action no-ops with no message (a
 * T30 "no silent ink" violation). This generalises T83's `DeleteLayerDialog`: themed,
 * context-carrying, and — critically — a real DOM overlay we control.
 *
 * Promise-based so a call site changes minimally: `if (!(await confirm({...}))) return;` mirrors the
 * old blocking `if (!confirm(...)) return;`. The dialog is PORTALLED to <body> and marked
 * `data-portal` so a panel's outside-click dismiss (T89) treats it as inside, not a stray click.
 * Escape and an outside click cancel; the destructive button is never the default focus.
 */
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";

export interface ConfirmOptions {
  title: string;
  body?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
  /** When set, the confirm button is disabled until the user types this word (T83's hard confirm). */
  requireType?: string;
}
export interface PromptOptions {
  title: string;
  label?: ReactNode;
  initial?: string;
  placeholder?: string;
  confirmLabel?: string;
}

interface DialogAPI {
  confirm: (opts: ConfirmOptions) => Promise<boolean>;
  prompt: (opts: PromptOptions) => Promise<string | null>;
}

type Request =
  | { kind: "confirm"; opts: ConfirmOptions; resolve: (v: boolean) => void }
  | { kind: "prompt"; opts: PromptOptions; resolve: (v: string | null) => void };

const DialogContext = createContext<DialogAPI | null>(null);

/** Access the in-app dialogs. Throws if used outside <DialogProvider> so a missing provider is loud. */
export function useDialogs(): DialogAPI {
  const ctx = useContext(DialogContext);
  if (!ctx) throw new Error("useDialogs must be used within <DialogProvider>");
  return ctx;
}

export function DialogProvider({ children }: { children: ReactNode }) {
  const [request, setRequest] = useState<Request | null>(null);

  const confirm = useCallback(
    (opts: ConfirmOptions) =>
      new Promise<boolean>((resolve) => setRequest({ kind: "confirm", opts, resolve })),
    [],
  );
  const prompt = useCallback(
    (opts: PromptOptions) =>
      new Promise<string | null>((resolve) => setRequest({ kind: "prompt", opts, resolve })),
    [],
  );
  const settle = useCallback((result: boolean | string | null) => {
    setRequest((r) => {
      if (r) (r.resolve as (v: boolean | string | null) => void)(result);
      return null;
    });
  }, []);

  return (
    <DialogContext.Provider value={{ confirm, prompt }}>
      {children}
      {request && <DialogView request={request} onSettle={settle} />}
    </DialogContext.Provider>
  );
}

function DialogView({
  request,
  onSettle,
}: {
  request: Request;
  onSettle: (result: boolean | string | null) => void;
}) {
  const cancelValue = request.kind === "prompt" ? null : false;
  const cancel = useCallback(() => onSettle(cancelValue), [onSettle, cancelValue]);

  const backdropRef = useRef<HTMLDivElement>(null);
  // T101: did the current press START on the backdrop? A genuine outside-dismiss (mouse or touch)
  // begins with a pointerdown on the backdrop; the compatibility mousedown a browser synthesises after
  // a touch does not (it is not a pointer event, and the gesture's real pointerdown landed elsewhere,
  // before this backdrop existed). Gates the mousedown-dismiss below.
  const pressStartedOnBackdrop = useRef(false);
  const cancelBtnRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [typed, setTyped] = useState("");
  const [value, setValue] = useState(request.kind === "prompt" ? (request.opts.initial ?? "") : "");

  // Escape (document-level; the portal is not a descendant of any panel) + initial focus. The
  // destructive action is NEVER default-focused: focus the input if there is one, else Cancel.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") cancel();
    };
    document.addEventListener("keydown", onKey);
    (inputRef.current ?? cancelBtnRef.current)?.focus();
    return () => document.removeEventListener("keydown", onKey);
  }, [cancel]);

  const confirmDisabled =
    request.kind === "confirm" &&
    !!request.opts.requireType &&
    typed.trim().toUpperCase() !== request.opts.requireType.toUpperCase();

  const onConfirm = () => {
    if (request.kind === "prompt") {
      const v = value.trim();
      onSettle(v === "" ? null : v);
    } else {
      onSettle(true);
    }
  };

  return createPortal(
    <div
      className="modal-backdrop"
      data-portal="dialog"
      data-testid="app-dialog"
      role="dialog"
      aria-modal="true"
      aria-label={request.opts.title}
      ref={backdropRef}
      onPointerDown={(e) => {
        pressStartedOnBackdrop.current = e.target === backdropRef.current;
      }}
      onMouseDown={(e) => {
        // Dismiss on mousedown, but ONLY when this gesture's press began on the backdrop (T101).
        //
        // Why mousedown and not pointerdown: a sibling panel's outside-click (T89) evaluates this same
        // gesture; the backdrop is `[data-portal]`, so while it is mounted it shields the panel from
        // collapsing. Cancelling on pointerdown unmounts the backdrop early, and the trailing mousedown
        // then lands outside as a real outside-click that collapses the Details panel. Cancelling on
        // mousedown keeps the shield and the original timing.
        //
        // Why the `pressStartedOnBackdrop` gate: on a phone, WetCanvas opens the text prompt on
        // pointerdown (finger still down); after `touchend` the browser fires a compatibility mousedown
        // targeted at whatever is under the finger — the just-mounted backdrop, for a tap placed
        // off-centre. That compat mousedown has no preceding pointerdown on the backdrop, so it would
        // cancel the prompt with the very tap that opened it — the popup only flashed. Requiring a
        // real press-start on the backdrop ignores it while still honouring genuine mouse/touch dismissals.
        if (pressStartedOnBackdrop.current && e.target === backdropRef.current) cancel();
      }}
    >
      <div className="modal card">
        <h3>{request.opts.title}</h3>
        {request.kind === "confirm" ? (
          <>
            {request.opts.body != null && <div data-testid="app-dialog-body">{request.opts.body}</div>}
            {request.opts.requireType && (
              <>
                <p className="muted">
                  Type <strong>{request.opts.requireType}</strong> to confirm.
                </p>
                <input
                  data-testid="app-dialog-type"
                  className="delete-layer-input"
                  value={typed}
                  placeholder={request.opts.requireType}
                  autoFocus
                  onChange={(e) => setTyped(e.target.value)}
                />
              </>
            )}
          </>
        ) : (
          <>
            {request.opts.label != null && <label htmlFor="app-dialog-input">{request.opts.label}</label>}
            <input
              id="app-dialog-input"
              data-testid="app-dialog-input"
              ref={inputRef}
              className="delete-layer-input"
              value={value}
              placeholder={request.opts.placeholder}
              onChange={(e) => setValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") onConfirm();
              }}
            />
          </>
        )}
        <div className="inline-form">
          <button
            type="button"
            className={`${request.kind === "confirm" && request.opts.danger ? "danger" : "primary"} btn-sm`}
            data-testid="app-dialog-confirm"
            disabled={confirmDisabled}
            onClick={onConfirm}
          >
            {request.opts.confirmLabel ?? (request.kind === "prompt" ? "OK" : "Confirm")}
          </button>
          <button
            type="button"
            className="ghost-btn btn-sm"
            data-testid="app-dialog-cancel"
            ref={cancelBtnRef}
            onClick={cancel}
          >
            {request.kind === "confirm" ? (request.opts.cancelLabel ?? "Cancel") : "Cancel"}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
