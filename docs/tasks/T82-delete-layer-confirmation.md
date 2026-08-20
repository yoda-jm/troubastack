# T82 (DRAFT for Fable to own/rule) — Delete a layer, with tiered confirmation

**Priority:** normal · **Size:** M (core + studio) · **Area:** `web/studio` (new UI) +
`core/internal` (cascade semantics). VLL request 2026-08-20: *"a draft for deleting a layer, with a
soft confirmation if it's empty and a hard confirmation if it contains things (type DELETE, like an
inbox)."* VLL also confirmed live: *"I don't see how to delete a layer — the delete seems to delete an
annotation from a layer."*

## Current state (grounded)

- **No client affordance to delete a layer.** The Layers drawer manages active/visibility/access and
  "+ New layer"; the toolbar's `Delete` (`sel-delete`, `canDeleteSelection`, `onDelete`) deletes the
  **selected objects**, not the layer. The Viewer comment at 1174 lists "Delete" for the layers tab,
  but the wired action is object-delete.
- **Core supports `KindLayerDelete`** end-to-end (`domain.go:80`, `engine.go:229`, `fold.go:71`,
  `sync/mapping.go`, `sync/apply.go`) — but its fold **removes only the layer record + its
  `layerOrder` slot. It does NOT touch the objects on that layer.** So a layer-delete today would
  leave its annotations **orphaned** (objects referencing a deleted `layerId`). This is the decision
  the feature has to make, not a detail.
- Revive is constrained by **I5: `KindRestore` is the ONLY revive** — relevant to whether a deleted
  layer (and its objects) can come back.

## What VLL asked for (the confirmation tiers)

- **Empty layer → soft confirm.** A light, one-step confirmation (a small "Delete this layer?"
  affirmation), because nothing is lost.
- **Non-empty layer → hard confirm.** A GitHub-style destructive gate: the user must **type `DELETE`**
  (or the layer's name) into a field before the button enables. The dialog states exactly what will be
  destroyed ("This layer has **N** annotations. They will be permanently deleted.").

## The design questions for you to rule (why this is a draft, not a spec)

1. **Cascade vs. block vs. reassign.** When a non-empty layer is deleted, do we (a) **cascade-delete
   its objects** in the same operation (needs a core change — `KindLayerDelete` fold must also drop
   objects whose `layerId` matches, and the sync mutation must carry/imply that); (b) **block** delete
   of a non-empty layer entirely (contradicts VLL's "hard confirm if it contains things"); or (c)
   **reassign** its objects to another layer first? (a) matches the request; it needs the correctness
   fix regardless, because the current orphan behaviour is a latent bug the moment any UI calls it.
2. **Restorability (I5).** Is a layer-delete restorable via `KindRestore` (layer + its objects come
   back together), or is the hard type-`DELETE` gate the deliberate point-of-no-return? A destructive
   action with a strong gate arguably should NOT be casually undoable, but I5 says restore is the only
   revive — your call on how they interact.
3. **Permissions.** Who may delete which layer? Mirror the existing layer-access rules — owner deletes
   own; conductor/admin deletes shared/others? An RO layer must not be deletable by a non-owner. State
   the matrix.
4. **What counts as "contains things."** Objects on that layer for **this file** only, or across the
   whole song? (Layers are per-song; objects are per-file+layer — confirm the count scope.)
5. **The active/last-layer edge.** Deleting the active layer (reassign active to another), and
   refusing to delete the last remaining layer if the model requires at least one.

## Straw-man acceptance criteria (for you to accept/rewrite)

- A "Delete layer" affordance in the Layers drawer, gated by permission.
- Empty layer: soft confirm → deletes; the layer disappears from the drawer + toggle set.
- Non-empty layer: a dialog naming the object count, with a type-`DELETE` field that enables the
  destructive button only on exact match; cancel destroys nothing.
- Core: `KindLayerDelete` no longer orphans — objects on the deleted layer are removed with it
  (or per your §1 ruling), with a Go test on the fold (layer + its objects gone; other layers intact).
- Realtime: a second client sees the layer (and its objects) vanish; RO-layer delete by a non-owner is
  rejected (there is already ws coverage for RO-layer rejects to mirror).
- Testids for the new affordances; e2e for both tiers (soft delete of an empty layer; hard type-DELETE
  of a non-empty one) + a reject path. Keep existing layer testids attached.
- `tsc -b studio`, `make e2e` (full), `gofmt`/`vet`/`make test` green.

## Out of scope (proposed)

- Bulk layer operations; merging layers; per-object reassignment UI (unless you pick §1(c)).
- The My-files instability (separate design request, same day).
