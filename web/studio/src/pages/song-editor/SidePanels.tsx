/**
 * Editor side panels (T10 extraction — moved verbatim from SongEditor.tsx): the
 * Layers panel (visibility toggles, lock, active/focus) and the per-layer
 * Annotation list. Behavior + data-testids unchanged.
 */
import { useMemo, useState } from "react";
import type { AnnotationLayer, AnnotationObject, Role } from "../../api";
import { objectLabel } from "../../editor";
import { Avatar } from "../../components/Avatar";
import { isEditableLayer, type LayerVisibility } from "./helpers";
import { AudienceTag } from "../../components/AudienceTag";

export function LayersPanel({
  layers,
  visible,
  myUserId,
  myRole,
  activeLayerId,
  focusedLayerId,
  onToggle,
  onFocus,
  canToggleAccess,
  onSetAccess,
  canDelete,
  onDelete,
}: {
  layers: AnnotationLayer[];
  visible: LayerVisibility;
  myUserId: string | null;
  myRole: Role | null;
  activeLayerId: string | null;
  focusedLayerId: string | null;
  onToggle: (id: string) => void;
  onFocus: (id: string) => void;
  // Whether the viewer may flip THIS layer's lock (shared-zone owner/admin only).
  canToggleAccess: (l: AnnotationLayer) => boolean;
  onSetAccess: (id: string, access: "rw" | "ro") => void;
  // T83: whether the viewer may delete THIS layer (mirrors server write-rights); onDelete opens the
  // tiered-confirmation flow (soft when empty, type-DELETE when it holds annotations / is mandatory).
  canDelete: (l: AnnotationLayer) => boolean;
  onDelete: (l: AnnotationLayer) => void;
}) {
  return (
    <aside className="layers-panel" data-testid="layers-panel">
      <h2>Layers</h2>
      {layers.length === 0 ? (
        <p className="muted" data-testid="layers-empty">
          No annotation layers.
        </p>
      ) : (
        <ul className="list layers-list">
          {layers.map((l) => {
            // T56: the audience chip — 👤 Mine for my personal layer, 👥 Band for
            // shared/conductor (conductor labelled). Another member's personal layer
            // (locked, not mine) keeps a neutral zone label — it is neither Band nor
            // my Mine.
            const isMineLayer =
              l.zone === "personal" && myUserId != null && l.ownerId === myUserId;
            const audienceEl = isMineLayer ? (
              <AudienceTag audience="mine" />
            ) : l.zone === "shared" || l.zone === "conductor" ? (
              <AudienceTag audience="band" note={l.zone === "conductor" ? "conductor" : undefined} />
            ) : (
              <span className="pill">{l.zone}</span>
            );
            // Non-editable = a layer I may not write (RO, or someone else's
            // personal layer). Shown with a lock; never the active draw target.
            const locked = !isEditableLayer(l, myUserId, myRole);
            const isActive = l.id === activeLayerId;
            const isFocused = l.id === focusedLayerId;
            return (
              <li
                key={l.id}
                data-testid="layer-item"
                className={`layer-item${isActive ? " active-layer" : ""}${
                  isFocused ? " focused-layer" : ""
                }${locked ? " locked" : ""}`}
              >
                <div className="layer-row-wrap">
                  <input
                    type="checkbox"
                    data-testid="layer-toggle"
                    aria-label={`Show ${l.name}`}
                    checked={!!visible[l.id]}
                    disabled={l.mandatory}
                    onChange={() => onToggle(l.id)}
                  />
                  {/* Click focuses the layer: scopes the annotation list to it,
                      and (if editable) makes it the active draw layer. */}
                  <button
                    type="button"
                    data-testid="layer-row"
                    className="layer-row"
                    aria-pressed={isFocused}
                    title="Show this layer's annotations"
                    onClick={() => onFocus(l.id)}
                  >
                    <span data-testid="layer-owner" title={l.ownerId === myUserId ? "Your layer" : "Another member's layer"}>
                      <Avatar user={{ displayName: l.name, avatarKind: "neutral" }} size={20} />
                    </span>
                    <span className="layer-name">{l.name}</span>
                  </button>
                </div>
                {audienceEl}
                {l.mandatory && <span className="pill mandatory-pill">required</span>}
                {/* The `drawing` (active) and `viewing` (focused) pills are the ONLY
                    per-row content that changes when focus/active moves between layers.
                    If they were mounted conditionally, focusing a layer would change
                    that row's width and — at narrow widths with a wide font — tip its
                    pills onto an extra wrapped line, shifting the panel height (and the
                    viewer top when the sidebar stacks above it). That is the T13 CI-only
                    ~27px RO/RW shift. So they are ALWAYS mounted with visibility toggled
                    (space reserved), mirroring the toolbar/annotation-hint fix (772be41):
                    every row's footprint is now independent of which layer is
                    active/focused, so focus never moves the layout. */}
                <span
                  className={`pill active-pill${isActive ? "" : " layer-pill-off"}`}
                  data-testid="layer-active"
                  aria-hidden={!isActive}
                >
                  drawing
                </span>
                <span
                  className={`pill focused-pill${isFocused ? "" : " layer-pill-off"}`}
                  data-testid="layer-focused"
                  aria-hidden={!isFocused}
                >
                  viewing
                </span>
                {locked && (
                  <span
                    className="pill lock-pill"
                    data-testid="layer-lock"
                    title="Read-only — you can't draw on this layer"
                    aria-label="Read-only layer"
                  >
                    🔒 locked
                  </span>
                )}
                {/* #4: lock/unlock toggle for shared-zone layers I own or admin.
                    locked(ro) = others view-only; unlocked(rw) = others can edit. */}
                {canToggleAccess(l) && (
                  <button
                    type="button"
                    data-testid="layer-access-toggle"
                    className={`layer-access-btn${l.access === "ro" ? " is-locked" : ""}`}
                    aria-pressed={l.access === "ro"}
                    title={
                      l.access === "ro"
                        ? "Locked — others can only view. Click to unlock (allow edits)."
                        : "Unlocked — others can edit. Click to lock (view-only)."
                    }
                    aria-label={l.access === "ro" ? "Unlock layer" : "Lock layer"}
                    onClick={() => onSetAccess(l.id, l.access === "ro" ? "rw" : "ro")}
                  >
                    {l.access === "ro" ? "🔒" : "🔓"}
                  </button>
                )}
                {/* T83: delete this layer — permission-gated (mirrors server write-rights). Opens the
                    tiered confirm: soft when empty, type-DELETE when it holds annotations / is mandatory. */}
                {canDelete(l) && (
                  <button
                    type="button"
                    data-testid="layer-delete"
                    className="layer-delete-btn"
                    title="Delete this layer"
                    aria-label="Delete layer"
                    onClick={() => onDelete(l)}
                  >
                    🗑
                  </button>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </aside>
  );
}

// ===========================================================================
// Annotation list — objects on the current editing (active) layer
// ===========================================================================

/** A list of the objects on the FOCUSED layer (the layer the user clicked in
 *  the Layers panel — editable or locked). Each row shows the type + a short
 *  label; clicking selects + highlights that object (and the caller scrolls it
 *  into view). When the focused layer is locked, an inline hint explains that
 *  drawing is disabled while you can still browse/select its annotations. */
export function AnnotationList({
  objects,
  focusedLayerId,
  focusedLayer,
  focusLocked,
  selectedUuids,
  onSelect,
}: {
  objects: AnnotationObject[];
  focusedLayerId: string | null;
  focusedLayer: AnnotationLayer | null;
  focusLocked: boolean;
  selectedUuids: string[];
  onSelect: (uuid: string) => void;
}) {
  const items = useMemo(
    () => objects.filter((o) => o.layerId === focusedLayerId),
    [objects, focusedLayerId],
  );
  const selected = new Set(selectedUuids);
  return (
    <aside className="annotation-list-panel" data-testid="annotation-list">
      <h2 data-testid="annotation-list-title">
        Annotations{focusedLayer ? ` · ${focusedLayer.name}` : ""}
      </h2>
      {/* Always mounted so the panel's height (and, when the layout stacks the
          sidebar above the viewer at narrow widths, the viewer's top offset) is
          identical whether or not a locked layer is focused — only visibility
          flips, the line's space is always reserved. */}
      <p
        className={`muted annotation-list-locked-hint${focusLocked ? "" : " annotation-list-locked-off"}`}
        data-testid="annotation-list-locked"
        aria-hidden={!focusLocked}
      >
        read-only layer — pick an editable layer to draw
      </p>
      {!focusedLayer ? (
        <p className="muted" data-testid="annotation-list-empty">
          No layer selected — pick a layer to see its annotations.
        </p>
      ) : items.length === 0 ? (
        <p className="muted" data-testid="annotation-list-empty">
          No annotations on this layer.
        </p>
      ) : (
        <ul className="list annotation-items">
          {items.map((o) => (
            <li key={o.uuid}>
              <button
                type="button"
                data-testid="annotation-item"
                className={`annotation-item${selected.has(o.uuid) ? " selected" : ""}`}
                aria-pressed={selected.has(o.uuid)}
                onClick={() => onSelect(o.uuid)}
              >
                <span className={`pill ann-type ann-type-${o.type}`}>{o.type}</span>
                <span className="ann-label">{objectLabel(o)}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </aside>
  );
}


// T83 — tiered confirmation for deleting a layer. Soft when the layer is empty; a GitHub-style
// type-DELETE gate when it holds annotations OR is mandatory (deleting a mandatory layer changes what
// every viewer sees, so the tier tracks CONSEQUENCE, not just object count). The copy says "can't be
// undone in the editor" — NOT "permanently deleted": the revision history still holds it (§3).
export function DeleteLayerDialog({
  layer,
  objectCount,
  onConfirm,
  onCancel,
}: {
  layer: AnnotationLayer;
  objectCount: number;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const hard = objectCount > 0 || layer.mandatory;
  const [typed, setTyped] = useState("");
  const armed = !hard || typed.trim().toUpperCase() === "DELETE";
  return (
    <div
      className="modal-backdrop"
      data-testid="delete-layer-dialog"
      role="dialog"
      aria-modal="true"
      onKeyDown={(e) => {
        if (e.key === "Escape") onCancel();
      }}
    >
      <div className="modal card">
        <h3>Delete “{layer.name || "layer"}”?</h3>
        {hard ? (
          <>
            <p data-testid="delete-layer-warning">
              {objectCount > 0
                ? `This layer holds ${objectCount} annotation${objectCount === 1 ? "" : "s"} across the file — they will be deleted with it.`
                : "This is a mandatory layer — deleting it changes what everyone sees."}
            </p>
            <p className="muted">
              This can’t be undone in the editor. Type <strong>DELETE</strong> to confirm.
            </p>
            <input
              data-testid="delete-layer-input"
              className="delete-layer-input"
              value={typed}
              placeholder="DELETE"
              autoFocus
              onChange={(e) => setTyped(e.target.value)}
            />
          </>
        ) : (
          <p data-testid="delete-layer-warning" className="muted">
            This layer is empty. It can’t be undone in the editor.
          </p>
        )}
        <div className="inline-form">
          <button
            type="button"
            className="danger btn-sm"
            data-testid="delete-layer-confirm"
            disabled={!armed}
            onClick={onConfirm}
          >
            Delete layer
          </button>
          <button type="button" className="ghost-btn btn-sm" data-testid="delete-layer-cancel" onClick={onCancel}>
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
