/**
 * Editor toolbar (T10 extraction — moved verbatim from SongEditor.tsx): tool
 * palette (registry-driven), style controls with contextual visibility, shape
 * presets, and the layer picker. Behavior + data-testids unchanged.
 */
import { type ReactNode, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { AnnotationLayer, AnnotationObject, AnnotationStyle } from "../../api";
import { type Tool, type PresetId, COLOR_SWATCHES, applyPreset, matchPreset, isNonDraw } from "../../editor";
import { WIDTH_STOPS, nearestStopIndex, widthToMm } from "../../strokeWidth";
import { FONT_STOPS, FONT_LABELS, nearestFontStopIndex } from "../../fontSize";
import { descriptorFor, toolsInOrder } from "../../annotations/registry";
import { AudienceTag, audienceForZone } from "../../components/AudienceTag";

// Select = a dashed marquee rectangle (T66) — the conventional rubber-band affordance,
// matching the already-dashed marquee (.selection-box) + selected bbox.
const SELECT_ICON = (
  <svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true">
    <rect
      x="2.5"
      y="3.5"
      width="11"
      height="9"
      rx="1"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.3"
      strokeDasharray="2 1.6"
    />
  </svg>
);

// Move/pan (T65): the standard 4-way arrow "move" glyph.
const MOVE_ICON = (
  <svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true">
    <path
      d="M8 1l2.4 2.4-1.4 1.4V7h2.2l-1.4-1.4L12 3.2 14.8 6 12 8.8l-1.2-1.2L12 6.2H9v3l1.4-1.4L12 9.2 9.6 11.6 8 13l-1.6-1.4L5 9.2l1.6 1.6L7 9.4V6.4H4l1.6 1.4L4 9.2 1.2 6.4 4 3.6l1.6 1.6L4 6.4h3V3.4L5.6 4.8 8 1z"
      fill="currentColor"
    />
  </svg>
);

// P206: a jump mark is a PAIR of matching landmarks — two dots joined by a dashed tie.
const JUMP_ICON = (
  <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.4">
    <circle cx="4" cy="12" r="2" />
    <circle cx="12" cy="4" r="2" />
    <path d="M5.4 10.6l5.2-5.2" strokeDasharray="1.7 1.7" strokeLinecap="round" />
  </svg>
);

type ToolButton = { tool: Tool; label: string; testid: string; icon: ReactNode };
const TOOLS: ToolButton[] = [
  // T66: Move is first — the editor opens in pan mode; Select follows, then the draw tools.
  { tool: "move", label: "Move", testid: "tool-move", icon: MOVE_ICON },
  { tool: "select", label: "Select", testid: "tool-select", icon: SELECT_ICON },
  ...toolsInOrder().map((t) => ({
    tool: t.id as Tool,
    label: t.label,
    testid: `tool-${t.id}`,
    icon: t.icon,
  })),
  // P206: not a registry type — it PLACES icon objects (a pair) via the Viewer's two-step jump flow.
  { tool: "jump", label: "Jump mark", testid: "tool-jump", icon: JUMP_ICON },
];

// The shape-style presets shown as one-click buttons (#5).
// Presets as an icon trio (T33): the word labels cost ~120px of bar width; the glyph
// + a `title`/`aria-label` carries the same affordance in the slim one-row ctx bar.
const PRESET_BUTTONS: { id: PresetId; label: string; title: string; testid: string }[] = [
  { id: "outline", label: "▢", title: "Outline", testid: "preset-outline" },
  { id: "box", label: "■", title: "Box", testid: "preset-box" },
  { id: "highlight", label: "▨", title: "Highlight", testid: "preset-highlight" },
];

/**
 * The ⋯ overflow popover (T33). Rare manual style combos — Fill, Border, Blend — plus
 * the hex readout live here so the ctx bar stays one slim row; presets cover the common
 * cases. Anchored panel (the VersionChip pattern), closes on outside-click / Esc. The
 * `style-fill` / `style-stroke` / `style-blend` / `style-color-value` testids moved
 * here unchanged. Shape-only controls are gated by `showShape` (hidden for text/none),
 * mirroring the inline slots' reserve-then-hide.
 */
function StyleMore({
  style,
  onStyle,
  disabled,
  showShape,
}: {
  style: AnnotationStyle;
  onStyle: (s: AnnotationStyle) => void;
  disabled: boolean;
  showShape: boolean;
}) {
  const [open, setOpen] = useState(false);
  // The popover is `position: fixed` with JS-measured coords: the ctx bar's
  // `.style-controls` is an overflow-x scroll container, which clips an absolutely
  // positioned child dropping below it — fixed escapes that clipping.
  const [coords, setCoords] = useState<{ top: number; right: number } | null>(null);
  const ref = useRef<HTMLSpanElement | null>(null);
  const btnRef = useRef<HTMLButtonElement | null>(null);
  useEffect(() => {
    if (!open) {
      setCoords(null);
      return;
    }
    const place = () => {
      const b = btnRef.current?.getBoundingClientRect();
      if (b) setCoords({ top: b.bottom + 6, right: window.innerWidth - b.right });
    };
    place();
    const onDown = (e: MouseEvent) => {
      const wrap = ref.current;
      if (!wrap) return;
      // Close only when the click lands OUTSIDE the whole style bar AND outside the
      // (now fixed-positioned) popover — so tweaking a preset/slider, which updates the
      // popover's fill/border/blend live, keeps it open; a canvas click dismisses it.
      const boundary = wrap.closest(".ctx-bar") ?? wrap;
      const target = e.target as Node;
      if (boundary.contains(target)) return;
      if ((target as Element).closest?.(".style-popover")) return;
      setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    window.addEventListener("resize", place);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("resize", place);
    };
  }, [open]);

  return (
    <span className="style-more-wrap" ref={ref}>
      <button
        type="button"
        className="style-more-btn"
        data-testid="style-more"
        aria-label="More style options"
        aria-expanded={open}
        title="Fill, border, blend, hex"
        disabled={disabled}
        ref={btnRef}
        onClick={() => setOpen((o) => !o)}
      >
        ⋯
      </button>
      {open && coords && createPortal(
        <div
          className="style-popover"
          data-testid="style-popover"
          role="group"
          aria-label="More style options"
          style={{ top: coords.top, right: coords.right }}
        >
          {showShape && (
            <>
              <label className="style-field shape-toggle">
                <input
                  type="checkbox"
                  data-testid="style-fill"
                  checked={style.fill ?? false}
                  disabled={disabled}
                  onChange={(e) => onStyle({ ...style, fill: e.target.checked })}
                />
                <span>Fill</span>
              </label>
              <label className="style-field shape-toggle">
                <input
                  type="checkbox"
                  data-testid="style-stroke"
                  checked={style.stroke ?? true}
                  disabled={disabled}
                  onChange={(e) => onStyle({ ...style, stroke: e.target.checked })}
                />
                <span>Border</span>
              </label>
              <label className="style-field">
                <span>Blend</span>
                <select
                  data-testid="style-blend"
                  value={style.blend ?? "normal"}
                  disabled={disabled}
                  onChange={(e) =>
                    onStyle({ ...style, blend: e.target.value as "normal" | "multiply" })
                  }
                >
                  <option value="normal">Normal</option>
                  <option value="multiply">Multiply</option>
                </select>
              </label>
            </>
          )}
          <label className="style-field">
            <span>Hex</span>
            <span className="style-value" data-testid="style-color-value">
              {style.color.toUpperCase()}
            </span>
          </label>
        </div>,
        document.body,
      )}
    </span>
  );
}

// useScrollFade toggles .of-start/.of-end on a horizontal scroll strip so the CSS edge fade
// shows ONLY when it actually overflows/can scroll in that direction (T65 Part C). No deps.
function useScrollFade<T extends HTMLElement>(dep?: unknown) {
  const ref = useRef<T>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const update = () => {
      el.classList.toggle("of-start", el.scrollLeft > 1);
      el.classList.toggle("of-end", el.scrollLeft + el.clientWidth < el.scrollWidth - 1);
    };
    update();
    el.addEventListener("scroll", update, { passive: true });
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => {
      el.removeEventListener("scroll", update);
      ro.disconnect();
    };
    // T156 ⟨A⟩ hardening (Fable): the ResizeObserver only fires on a BOX change, so a strip sitting exactly
    // at max-width that GAINS a control (a tool/selection switch, or the ⟨B⟩ preview changing size) grows
    // its content without growing its box — no observer callback, no .of-* class, and it overflows while
    // still pass-through glass (the original bug in an untested state). Re-running the effect when the
    // control-set signature changes recomputes the fade (and thus pointer-events) for exactly that case.
  }, [dep]);
  return ref;
}

// Measure the on-screen page box so the preview means the SAME thing as the ink: a stroke's width is a
// fraction of page WIDTH, a text's fontSize a fraction of page HEIGHT (I3). The rendered `.pdf-page` element
// is what the ink draws onto, so its clientWidth/Height ARE those dimensions in CSS px at the current zoom.
function usePageBox(): { w: number; h: number } {
  const [box, setBox] = useState<{ w: number; h: number }>({ w: 0, h: 0 });
  useEffect(() => {
    let ro: ResizeObserver | null = null;
    let raf = 0;
    const attach = () => {
      const el = document.querySelector('[data-testid="pdf-page"]') as HTMLElement | null;
      if (!el) {
        raf = requestAnimationFrame(attach);
        return;
      }
      const read = () => setBox({ w: el.clientWidth, h: el.clientHeight });
      ro = new ResizeObserver(read);
      ro.observe(el);
      read();
    };
    attach();
    return () => {
      if (ro) ro.disconnect();
      if (raf) cancelAnimationFrame(raf);
    };
  }, []);
  return box;
}

// BottomSizePreview (VLL): a live size preview pinned at the BOTTOM of the viewport — shown the moment a
// size tool is selected and updated as the size changes, NO hover required (a hover ring is impossible on a
// phone). It shows the TRUE size — a dashed circle at the stroke's real diameter, or a text sample at the
// real font size — with room the toolbar chip never had (that was capped to the bar height). The generous
// max is a layout safety rail, well above any real width/size, so the whole range still shows true. Portaled
// to <body>: the ctx-bar has a translateX(-50%) transform, which would anchor a fixed child to the bar.
const HUD_MAX_CIRCLE = 140;
const HUD_MAX_TEXT = 96;
function BottomSizePreview({
  style,
  isText,
  show,
  ping,
  previewSize,
  jumpSize,
}: {
  style: AnnotationStyle;
  isText: boolean;
  show: boolean;
  ping: number;
  previewSize: number | null; // a font size being HOVERED in the custom dropdown (overrides the committed one)
  jumpSize: number | null; // P206: the jump landmark's size (bbox side, page-width fraction) — a box hint
}) {
  const { w, h } = usePageBox();
  const [visible, setVisible] = useState(false);
  const timer = useRef<number | null>(null);
  // Flash on tool-select, size change, or a size-control hover (`ping`), then fade. BUT while a dropdown
  // size is being hovered (`previewSize`), stay visible and track it — no fade — so you can scan the list
  // (VLL: "hover 36 → HUD shows TroubaStudio at 36 live"). Fades once the hover ends (previewSize → null).
  useEffect(() => {
    if (!show) {
      setVisible(false);
      return;
    }
    setVisible(true);
    if (timer.current) window.clearTimeout(timer.current);
    if (previewSize == null) {
      timer.current = window.setTimeout(() => setVisible(false), 1500);
    }
    return () => {
      if (timer.current) window.clearTimeout(timer.current);
    };
  }, [show, isText, style.width, style.fontSize, ping, previewSize, jumpSize]);
  if (!show) return null;
  let visual: ReactNode;
  let label: string;
  if (jumpSize != null) {
    // P206: the jump landmark size hint — a dashed square at the true size, with the mm.
    const d = Math.min(HUD_MAX_CIRCLE, Math.max(6, jumpSize * (w || 600)));
    visual = <span className="size-hud-box" style={{ width: `${d}px`, height: `${d}px` }} />;
    label = `${(jumpSize * 210).toFixed(0)} mm`;
  } else if (isText) {
    const font = previewSize ?? style.fontSize;
    const px = Math.min(HUD_MAX_TEXT, Math.max(8, font * (h || 850)));
    visual = (
      <span className="size-hud-text" style={{ fontSize: `${px}px` }}>
        TroubaStudio
      </span>
    );
    label = `${Math.round(font * 1000)}`;
  } else {
    const d = Math.min(HUD_MAX_CIRCLE, Math.max(2, style.width * (w || 600)));
    visual = <span className="size-hud-circle" style={{ width: `${d}px`, height: `${d}px` }} />;
    label = `${widthToMm(style.width).toFixed(2)} mm`;
  }
  return createPortal(
    <div className={`size-hud${visible ? " show" : ""}`} data-testid="style-size-preview" aria-hidden="true">
      {visual}
      <span className="size-hud-label">{label}</span>
    </div>,
    document.body,
  );
}

// SizeSelect: a CUSTOM text-size dropdown (VLL) — a native <select>'s option list is OS-rendered, so you
// can't preview a size by hovering an item. Here each option is a real element, so onMouseEnter drives the
// bottom HUD live (onPreview) before you commit (onClick). Button + a portaled listbox (the ctx-bar's
// translateX(-50%) transform + overflow would otherwise mis-place / clip an in-bar popup). Keyboard:
// ↑/↓ move + preview the active option, Enter commits, Esc closes; outside-click closes.
function SizeSelect({
  value,
  disabled,
  tabbable,
  onChange,
  onPreview,
}: {
  value: number;
  disabled: boolean;
  tabbable: boolean;
  onChange: (v: number) => void;
  onPreview: (v: number | null) => void;
}) {
  const [open, setOpen] = useState(false);
  const [coords, setCoords] = useState<{ left: number; top: number } | null>(null);
  const [active, setActive] = useState(0);
  const btnRef = useRef<HTMLButtonElement | null>(null);
  const idx = nearestFontStopIndex(value);

  useEffect(() => {
    if (!open) {
      setCoords(null);
      onPreview(null); // closing clears any hover-preview → HUD reverts to the committed size
      return;
    }
    setActive(nearestFontStopIndex(value));
    const place = () => {
      const b = btnRef.current?.getBoundingClientRect();
      if (b) setCoords({ left: b.left, top: b.bottom + 4 });
    };
    place();
    const onDown = (e: MouseEvent) => {
      const t = e.target as Element;
      if (btnRef.current?.contains(t) || t.closest?.(".size-select-list")) return;
      setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    window.addEventListener("resize", place);
    window.addEventListener("scroll", place, true);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", place, true);
    };
  }, [open, value, onPreview]);

  const commit = (i: number) => {
    onChange(FONT_STOPS[i]);
    onPreview(null);
    setOpen(false);
    btnRef.current?.focus();
  };

  return (
    <>
      <button
        ref={btnRef}
        type="button"
        className="size-select"
        data-testid="style-font"
        aria-label="Text size"
        title="Text size"
        aria-haspopup="listbox"
        aria-expanded={open}
        disabled={disabled}
        tabIndex={tabbable ? undefined : -1}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown" || e.key === "ArrowUp") {
            e.preventDefault();
            if (!open) {
              setOpen(true);
              return;
            }
            const next = Math.max(0, Math.min(active + (e.key === "ArrowDown" ? 1 : -1), FONT_STOPS.length - 1));
            setActive(next);
            onPreview(FONT_STOPS[next]);
          } else if (open && (e.key === "Enter" || e.key === " ")) {
            e.preventDefault();
            commit(active);
          }
        }}
      >
        {FONT_LABELS[idx]} <span aria-hidden="true">▾</span>
      </button>
      {open &&
        coords &&
        createPortal(
          <ul
            className="size-select-list"
            data-testid="style-font-list"
            role="listbox"
            aria-label="Text size"
            style={{ left: coords.left, top: coords.top }}
            onMouseLeave={() => onPreview(null)}
          >
            {FONT_STOPS.map((f, i) => (
              <li
                key={f}
                role="option"
                aria-selected={i === idx}
                data-testid="style-font-option"
                data-value={String(f)}
                className={`size-select-option${i === active ? " active" : ""}${i === idx ? " current" : ""}`}
                onMouseEnter={() => {
                  setActive(i);
                  onPreview(f);
                }}
                onClick={() => commit(i)}
              >
                {FONT_LABELS[i]}
              </li>
            ))}
          </ul>,
          document.body,
        )}
    </>
  );
}

export function EditorToolbar({
  part,
  tool,
  onTool,
  style,
  onStyle,
  jumpSize,
  onJumpSize,
  jumpAwaitingDest,
  controlsLocked,
  multiSelected,
  selectedType,
  editableLayers,
  activeLayerId,
  activeLayer,
  onActiveLayer,
  onNewLayer,
  canDraw,
  drawLocked,
  canEditFocusedLayer,
  focusedLayerName,
  onEditLayer,
  showEditLayerHint,
  selectionCount,
  canDeleteSelection,
  onDelete,
}: {
  // Which slice of the toolbar to render into the T27 stage-3 fullscreen chrome:
  //   "tools"  → the compact tool cluster (floating top-bar pill)
  //   "style"  → the contextual style row (floating .ctx pill, shown when drawing/selected)
  //   "layers" → layer management: active layer, +New layer, Edit-this-layer, Delete
  //              (lives in the on-demand drawer, per the approved mockup)
  part: "tools" | "style" | "layers";
  tool: Tool;
  onTool: (t: Tool) => void;
  style: AnnotationStyle;
  onStyle: (s: AnnotationStyle) => void;
  // P206: the jump landmark's size (bbox side, page-width fraction) + its setter — shown in the ctx bar
  // when the jump tool is active.
  jumpSize: number;
  /** P206: true between the two clicks of a pair — the source is down and the destination is next. */
  jumpAwaitingDest: boolean;
  onJumpSize: (n: number) => void;
  // The selected object is on a locked layer → style controls reflect but are disabled.
  controlsLocked: boolean;
  // More than one object is selected (#4): style/restyle controls are disabled,
  // since one set of controls can't sanely restyle a heterogeneous selection.
  multiSelected: boolean;
  // The selected object's type (drives the tool/shape indicator), or null.
  selectedType: AnnotationObject["type"] | null;
  editableLayers: AnnotationLayer[];
  activeLayerId: string | null;
  activeLayer: AnnotationLayer | null;
  onActiveLayer: (id: string) => void;
  onNewLayer: () => void;
  canDraw: boolean;
  drawLocked: boolean;
  // The focused layer is editable but not active → offer "Edit this layer".
  canEditFocusedLayer: boolean;
  focusedLayerName: string | null;
  onEditLayer: () => void;
  // A non-active editable object is selected → show the inline "edit this layer" hint.
  showEditLayerHint: boolean;
  selectionCount: number;
  canDeleteSelection: boolean;
  onDelete: () => void;
}) {
  // One scroll-fade ref (T65 Part C); only one of the two scroll strips below renders per
  // instance (the component is called once per `part`), so it binds to whichever mounts.
  // The signature covers everything that changes the strip's CONTENT width without necessarily changing its
  // box: the tool/selection (which controls show) and the style values that resize the ⟨B⟩ size preview.
  const fadeRef = useScrollFade<HTMLDivElement>(
    `${part}|${tool}|${selectedType}|${multiSelected}|${style.width}|${style.fontSize}`,
  );
  // Bumped when a size control is hovered (desktop) → the bottom preview flashes so you can see the size
  // without changing it (VLL). Touch never fires mouseenter, so it stays a desktop-only cue.
  const [sizeHoverPing, setSizeHoverPing] = useState(0);
  const pingSize = () => setSizeHoverPing((n) => n + 1);
  // A font size being HOVERED in the custom size dropdown → the HUD previews it live before you commit.
  const [previewFont, setPreviewFont] = useState<number | null>(null);

  // The tool cluster (top-bar pill). Keeps `editor-toolbar`/`tool-palette` testids.
  const toolsEl = (
    <div className="editor-toolbar" data-testid="editor-toolbar">
      <div className="tool-palette" role="toolbar" aria-label="Annotation tools" ref={fadeRef}>
        {TOOLS.map((t) => (
          <button
            key={t.tool}
            type="button"
            data-testid={t.testid}
            className={`tool-btn tool-icon-btn${tool === t.tool ? " active" : ""}`}
            aria-pressed={tool === t.tool}
            aria-label={t.label}
            title={t.label}
            disabled={(!canDraw || drawLocked) && !isNonDraw(t.tool)}
            onClick={() => onTool(t.tool)}
          >
            {t.icon}
          </button>
        ))}
        {/* Locked hint lives in its OWN reserved slot (NOT inline among the tool
            buttons): it is ALWAYS mounted so its row never appears/disappears,
            and only its visibility flips with `drawLocked`. Mounting it inline
            (or display-toggling it) changed the palette's wrapped width/height
            and pushed the whole viewer down — same footprint-stability rule as
            the .style-slot-off control slots. */}
        <span
          className={`draw-locked-hint${drawLocked ? "" : " draw-hint-off"}`}
          data-testid="draw-locked-hint"
          role="status"
          aria-hidden={!drawLocked}
        >
          read-only layer — pick an editable layer to draw
        </span>
      </div>
    </div>
  );

  // The contextual style row (.ctx pill). Returns null in the neutral state.
  const styleEl = (() => {
      // ---- per-type control relevance (#1+#2) ----------------------------
      // The bar's FOOTPRINT never changes with selection: every control slot is
      // ALWAYS rendered; irrelevant slots are hidden via `visibility:hidden`
      // (the .style-slot-off modifier) so they reserve their space and the page
      // never reflows. Relevance only flips which slots are VISIBLE:
      //   - TEXT target  → color, opacity, SIZE (fontSize). Width + shape hidden.
      //   - SHAPE/draw   → color, opacity, WIDTH, border/fill/blend + presets.
      //                    Text size hidden.
      //   - nothing selected + Select tool → the neutral baseline: show all
      //     slots (stable, maximal footprint) so picking up a selection only
      //     ever HIDES slots, never adds them.
      // `disabled` greys + locks the inputs (locked single OR multi-selection).
      const disabled = controlsLocked || multiSelected;
      // Which style controls apply to the current target (selection, else the draw
      // tool) — read from the annotation registry (T07). Neutral baseline (Select
      // tool, nothing selected) shows every slot, so picking up a selection only
      // ever HIDES slots (never adds), preserving the stable footprint.
      // Neutral = a non-drawing tool (Select or Move — T66) with nothing selected.
      const neutral = selectedType == null && isNonDraw(tool);
      // Contextual toolbar (T27 stage 3): the style row appears only when a draw
      // tool is active or an object is selected — the neutral (Select/Move + nothing
      // selected) state shows just the tools, keeping the floating bar compact.
      // Multi-selection is NOT neutral: it shows the row (disabled) so the "N
      // selected" indicator + restyle-lock stay visible.
      if (neutral && !multiSelected) return null;
      const targetType =
        selectedType ??
        // P206: the jump tool places ICON objects, so it wears the icon's controls (colour/opacity).
        (tool === "jump" ? "icon" : !isNonDraw(tool) ? (tool as AnnotationObject["type"]) : null);
      const controls = targetType ? (descriptorFor(targetType)?.styleControls ?? []) : [];
      const showWidth = neutral || controls.includes("width");
      const showShape = neutral || controls.includes("shapePreset");
      const showFont = neutral || controls.includes("textSize");
      const slot = (on: boolean) => `style-field${on ? "" : " style-slot-off"}`;
      return (
      <div
        className={`style-controls${selectedType ? " editing-selection" : ""}${
          disabled ? " controls-locked" : ""
        }`}
        data-testid="style-controls"
        ref={fadeRef}
      >
        {/* Live size preview pinned at the bottom (VLL) — shown on tool-select, no hover; portals to body. */}
        <BottomSizePreview
          style={style}
          isText={showFont && !showWidth}
          show={showWidth || showFont || tool === "jump"}
          ping={sizeHoverPing}
          previewSize={showFont && !showWidth ? previewFont : null}
          jumpSize={tool === "jump" ? jumpSize : null}
        />
        {/* Shape/type indicator: the selection's type/count, else the draw tool. */}
        <span className="pill style-target" data-testid="style-target">
          {multiSelected
            ? `${selectionCount} selected`
            : selectedType
              ? `Editing: ${selectedType}`
              : `Draw: ${tool}`}
        </span>
        {/* T156 ⟨B⟩: a live size legend, among the first-visible items — see the size before you draw. */}
        <span className="swatches">
          {COLOR_SWATCHES.map((c) => (
            <button
              key={c}
              type="button"
              className={`swatch${style.color === c ? " active" : ""}`}
              style={{ background: c }}
              aria-label={`Color ${c}`}
              disabled={disabled}
              onClick={() => onStyle({ ...style, color: c })}
            />
          ))}
        </span>
        {/* T33: labels dropped to title/aria-label (the height cost); the numeric value
            sits INLINE to the right of each slider (CSS makes .style-field a row). */}
        <label className="style-field">
          <input
            type="color"
            data-testid="style-color"
            aria-label="Custom color"
            title={`Color ${style.color.toUpperCase()}`}
            value={style.color}
            disabled={disabled}
            onChange={(e) => onStyle({ ...style, color: e.target.value })}
          />
        </label>
        <label className="style-field">
          <input
            type="range"
            data-testid="style-opacity"
            aria-label="Opacity"
            title="Opacity"
            min={0.1}
            max={1}
            step={0.05}
            value={style.opacity}
            disabled={disabled}
            onChange={(e) => onStyle({ ...style, opacity: Number(e.target.value) })}
          />
          <span className="style-value" data-testid="style-opacity-value">
            {Math.round(style.opacity * 100)}%
          </span>
        </label>
        {/* WIDTH — stroke width. Relevant for shapes/strokes, not text. */}
        <label className={slot(showWidth)} aria-hidden={!showWidth} onMouseEnter={pingSize}>
          <input
            type="range"
            data-testid="style-width"
            aria-label="Stroke width"
            title="Stroke width"
            min={0}
            max={WIDTH_STOPS.length - 1}
            step={1}
            data-stops={WIDTH_STOPS.join(",")}
            value={nearestStopIndex(style.width)}
            disabled={disabled || !showWidth}
            tabIndex={showWidth ? undefined : -1}
            onChange={(e) => onStyle({ ...style, width: WIDTH_STOPS[Number(e.target.value)] })}
          />
          <span className="style-value" data-testid="style-width-value">
            {widthToMm(style.width).toFixed(2)} mm
          </span>
        </label>
        {/* P206 (VLL, 2026-09-09): which END the next click drops. The chain is source-first, and it used
            to be announced only by a notice that vanished — so an author returning to the canvas had no way
            to know whether the next click starts a pair or finishes one. Persistent, in the bar, and it
            names the end rather than the step number. */}
        {tool === "jump" && (
          <span
            className={`chip jump-step${jumpAwaitingDest ? " warn" : ""}`}
            data-testid="jump-step"
            title={jumpAwaitingDest ? "Next click places the destination" : "Next click places the source"}
          >
            {jumpAwaitingDest ? "next: destination →" : "next: source"}
          </span>
        )}
        {tool === "jump" && (
          <label className="style-field" onMouseEnter={pingSize}>
            <input
              type="range"
              data-testid="jump-size"
              aria-label="Jump mark size"
              title="Jump mark size"
              min={0.03}
              max={0.16}
              step={0.005}
              value={jumpSize}
              disabled={disabled}
              onChange={(e) => onJumpSize(Number(e.target.value))}
            />
            <span className="style-value" data-testid="jump-size-value">
              {(jumpSize * 210).toFixed(0)} mm
            </span>
          </label>
        )}
        {/* Shape style (#5): fill / border(stroke) / blend + presets. Relevant for
            shape/draw targets; hidden (space reserved) for text/none. */}
        {/* Shape presets as an icon trio (#4). Fill/Border/Blend + the hex readout moved
            into the ⋯ popover (#5) so this stays one slim row. */}
        <div
          className={`shape-style${showShape ? "" : " style-slot-off"}`}
          data-testid="shape-style"
          aria-hidden={!showShape}
        >
          <span className="preset-buttons" role="group" aria-label="Shape presets">
            {PRESET_BUTTONS.map((p) => {
              const active = matchPreset(style) === p.id;
              return (
                <button
                  key={p.id}
                  type="button"
                  data-testid={p.testid}
                  className={`preset-btn${active ? " active" : ""}`}
                  aria-pressed={active}
                  aria-label={p.title}
                  title={p.title}
                  disabled={disabled || !showShape}
                  tabIndex={showShape ? undefined : -1}
                  onClick={() => onStyle(applyPreset(style, p.id))}
                >
                  {p.label}
                </button>
              );
            })}
          </span>
        </div>
        {/* TEXT SIZE — relevant only for a text target; hidden (space reserved)
            for shapes/strokes. */}
        <label className={slot(showFont)} aria-hidden={!showFont} onMouseEnter={pingSize}>
          {/* Custom dropdown (VLL): each option is a real element, so hovering one previews that size in the
              bottom HUD live (onPreview) before you commit — a native <select>'s OS-rendered list can't.
              An off-ladder stored size shows as the nearest stop without being rewritten (fontSize.ts). */}
          <SizeSelect
            value={style.fontSize}
            disabled={disabled || !showFont}
            tabbable={showFont}
            onChange={(f) => onStyle({ ...style, fontSize: f })}
            onPreview={setPreviewFont}
          />
        </label>
        {/* ⋯ overflow: fill / border / blend / hex (#5). Always present (fixed
            footprint → no shift); shape-only controls gated inside by showShape. */}
        <StyleMore style={style} onStyle={onStyle} disabled={disabled} showShape={showShape} />
      </div>
      );
      })();

  // Layer management (drawer). Keeps active-layer / new-layer / edit-this-layer /
  // delete-object testids present + reachable (delete's primary UX is the selbar).
  const layersEl = (
      <div className="layer-controls">
        {/* Prominent, brand-colored chip: always shows where ink will land. */}
        <span
          className="pill active-layer-indicator"
          data-testid="active-layer-indicator"
          title="New annotations are drawn on this layer"
        >
          <span className="ali-label">
            Drawing on: {activeLayer ? activeLayer.name : "no editable layer — draw to create one"}
          </span>
          {activeLayer && (
            <AudienceTag
              audience={audienceForZone(activeLayer.zone)}
              note={activeLayer.zone === "conductor" ? "conductor" : undefined}
            />
          )}
        </span>
        <label className="style-field">
          <span>Active layer</span>
          <select
            data-testid="active-layer"
            value={activeLayerId ?? ""}
            disabled={editableLayers.length === 0}
            onChange={(e) => onActiveLayer(e.target.value)}
          >
            {editableLayers.length === 0 && <option value="">No editable layer</option>}
            {editableLayers.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          data-testid="new-layer"
          className="new-layer-btn"
          disabled={!canDraw}
          onClick={onNewLayer}
        >
          + New layer
        </button>
        {/* "Edit this layer": activates the focused (editable, non-active) layer
            so its objects become editable. The active layer is the ONLY edit
            target (Bug #2), changed explicitly here or via the selector. */}
        {canEditFocusedLayer && (
          <button
            type="button"
            data-testid="edit-this-layer"
            className="edit-layer-btn"
            onClick={onEditLayer}
            title="Make this layer the active edit target"
          >
            Edit this layer{focusedLayerName ? `: ${focusedLayerName}` : ""}
          </button>
        )}
        {showEditLayerHint && (
          <span
            className="edit-layer-hint"
            data-testid="edit-layer-hint"
            role="status"
          >
            Editing happens on the active layer — Edit this layer?
          </span>
        )}
        <button
          type="button"
          data-testid="delete-object"
          className="delete-object-btn"
          disabled={selectionCount === 0 || !canDeleteSelection}
          onClick={onDelete}
        >
          Delete{selectionCount > 1 ? ` (${selectionCount})` : ""}
        </button>
      </div>
  );

  if (part === "tools") return toolsEl;
  if (part === "style") return styleEl;
  return layersEl;
}

// ===========================================================================
// Selection toolbar (T27 stage 2) — a small floating bar by a selected object
// ===========================================================================

/** A compact floating toolbar shown next to the single, active-editable selection:
 *  colour · bring-to-front · send-to-back · duplicate · delete. It floats OVER the
 *  canvas (position handled by the caller) with its own pointer-events, and stops
 *  pointerdown from reaching the wet canvas underneath (which would start a marquee
 *  / clear the selection). Drives off the existing selection + object mutations —
 *  no new layout, no shift. */
export function SelectionToolbar({
  color,
  onColor,
  onBringToFront,
  onSendToBack,
  onDuplicate,
  onDelete,
  onSwapJump,
  jumpRelation,
}: {
  color: string;
  onColor: (c: string) => void;
  onBringToFront: () => void;
  onSendToBack: () => void;
  onDuplicate: () => void;
  onDelete: () => void;
  /** P206: present only when the selected mark is one end of a jump — swaps which end is the source. */
  onSwapJump?: () => void;
  /** P206 ⟨D4⟩: what this mark does ("Jumps to p.7"), and going there. Absent for a non-jump. */
  jumpRelation?: { label: string; disabledReason?: string; onGo: () => void };
}) {
  return (
    <div
      className="sel-toolbar"
      data-testid="sel-toolbar"
      role="toolbar"
      aria-label="Selected annotation"
      // Keep clicks/drags on the bar from reaching the wet canvas below.
      onPointerDown={(e) => e.stopPropagation()}
    >
      <label className="sel-color" title="Colour">
        <input
          type="color"
          data-testid="sel-color"
          value={color}
          onChange={(e) => onColor(e.target.value)}
          aria-label="Colour"
        />
      </label>
      {jumpRelation && (
        // ⟨D4⟩ R2: the LABEL IS THE BUTTON. VLL asked to "navigate to its counterpart by clicking
        // somewhere" — the somewhere a person tries is whatever names the other end, and after R1 this
        // sentence is the only thing that does in every case. A second icon in a bar already carrying six
        // controls would buy nothing and cost a slot.
        <button
          type="button"
          className="sel-jump-rel"
          data-testid="sel-jump-relation"
          disabled={jumpRelation.disabledReason != null}
          title={
            jumpRelation.disabledReason
              ? `Can't go there — ${jumpRelation.disabledReason}`
              : `${jumpRelation.label} — click to go there`
          }
          aria-label={
            jumpRelation.disabledReason
              ? `${jumpRelation.label}. Can't go there — ${jumpRelation.disabledReason}`
              : `${jumpRelation.label}. Go to the other end`
          }
          onClick={jumpRelation.onGo}
        >
          {jumpRelation.label}
        </button>
      )}
      {onSwapJump && (
        // P206 (VLL, 2026-09-09): getting the direction wrong used to mean deleting the pair (and since
        // "a jump deletes as ONE thing", that means both ends) and placing it again. One click now. It is
        // its own inverse — press it twice and you are back — so it records no undo entry.
        <button
          type="button"
          data-testid="sel-swap-jump"
          title="Swap jump direction"
          aria-label="Swap jump direction"
          onClick={onSwapJump}
        >
          <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
            <path d="M2.5 5.5h9M9 3l2.5 2.5L9 8" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" />
            <path d="M13.5 10.5h-9M7 8l-2.5 2.5L7 13" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>
      )}
      <button type="button" data-testid="sel-front" title="Bring to front" aria-label="Bring to front" onClick={onBringToFront}>
        <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
          <rect x="2" y="2" width="9" height="9" rx="1" fill="none" stroke="currentColor" strokeWidth="1.3" opacity="0.5" />
          <rect x="5" y="5" width="9" height="9" rx="1" fill="currentColor" />
        </svg>
      </button>
      <button type="button" data-testid="sel-back" title="Send to back" aria-label="Send to back" onClick={onSendToBack}>
        <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
          <rect x="5" y="5" width="9" height="9" rx="1" fill="none" stroke="currentColor" strokeWidth="1.3" opacity="0.5" />
          <rect x="2" y="2" width="9" height="9" rx="1" fill="currentColor" />
        </svg>
      </button>
      <button type="button" data-testid="sel-duplicate" title="Duplicate" aria-label="Duplicate" onClick={onDuplicate}>
        <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
          <rect x="2" y="2" width="9" height="9" rx="1" fill="none" stroke="currentColor" strokeWidth="1.3" />
          <rect x="5" y="5" width="9" height="9" rx="1" fill="none" stroke="currentColor" strokeWidth="1.3" />
        </svg>
      </button>
      <button type="button" data-testid="sel-delete" className="danger" title="Delete" aria-label="Delete" onClick={onDelete}>
        <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
          <path d="M3 4h10M6 4V3h4v1M5 4l.7 9h4.6L11 4" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>
    </div>
  );
}

// ===========================================================================
// Edit canvas — per-page pointer capture + wet-object rendering
// ===========================================================================

/** A page-relative point captured during a gesture. */
