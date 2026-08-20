# T83 — Delete a layer: tiered confirmation, and a cascade that doesn't punch a hole in I5

**Priority:** high (VLL 2026-08-20: delete a layer, soft confirm when empty, type-`DELETE` when it
holds annotations) · **Size:** M — studio affordance + a **core** change · **Area:**
`web/studio` (Layers drawer) + `core/internal/engine` **and** `core/internal/store`.

**Numbering:** the lane filed this as a provisional `T82`; T82 is the My-files stability spec, so
this is **T83** and the draft file is removed.

## Verified state (I checked all of it)

- **No client affordance to delete a layer** — the toolbar `Delete` is object/selection delete.
- **`KindLayerDelete` does not cascade.** `engine.go:229` and — note the path, it is **`store/fold.go:71`**,
  not `engine/fold.go` — both remove the layer record and its `layerOrder` slot and **touch no
  objects**. Nothing cascades at the app or httpapi layer either (grep-confirmed).
- **Orphans would ship.** `snapshot()` emits every object in `se.order` unconditionally, so an
  object whose layer is gone is still serialised, carrying a `LayerID` that resolves to nothing.
  Every layer-driven consumer — visibility, P205 identity filtering, the bake compositor — then has
  an object it cannot classify.

So the affordance VLL is asking for would, today, activate a latent correctness bug. Fixing the
cascade is the task; the dialog is the easy half.

## Rulings

### 1. Cascade — and it must **tombstone**, not silently drop

Deleting a layer deletes its objects, in **one mutation / one revision** (not "N object deletes then
a layer delete", which can half-fail and makes undo N+1 steps). Blocking contradicts what VLL asked
for; reassign-first is a chore nobody wants mid-rehearsal.

**The part the draft missed:** the cascade must write **tombstones** for those objects, exactly as a
normal object delete does. I5 says delete is a terminal tombstone and that a mutation arriving for a
dead object is rejected with `deleted-remotely` so the peer rolls back. If the cascade merely drops
objects from the fold, a concurrent peer editing one of them sends a mutation for a UUID the fold no
longer knows — and an unknown UUID looks like a *create*. That silently **resurrects an object into a
deleted layer**, which is precisely the failure I5 exists to prevent. Cascade-by-forgetting turns
layer-delete into a hole in the invariant; cascade-by-tombstone does not.

### 2. Both folds, or you get "works until reload"

`engine.go` (live, in-memory) and `store/fold.go` (replay) are two implementations of the same fold.
A cascade added to one and not the other means live state and replayed state diverge — the bug
appears only after a restart or a fresh client, which is the worst way to find it. **Required test:**
after a layer delete, a snapshot replayed from the store equals the engine's snapshot (layers,
objects, tombstones, order).

### 3. Restorability — no in-app restore, and don't lie about it

`KindRestore` revives an **object** UUID (I5's only revive); there is no layer-restore, and adding
one is scope creep. So: **the delete is not undoable in the editor**, and the type-`DELETE` gate is
the guard.

But the revision history still holds it — `SnapshotAt(songID, revision)` returns any past revision.
So the copy must not promise erasure. **"This can't be undone in the editor"** is true;
*"permanently deleted"* / *"cannot be recovered"* is **false** and must not be written. If someone
later wants real erasure, that is a different feature with privacy implications.

### 4. Permissions — mirror edit rights, don't invent a matrix

Whoever may **write** a layer may delete it: personal → its owner; **conductor-zone → the conductor
ROLE** (not merely the band admin — the zone is governed by role, per the existing rule); shared →
band admin. An `access: "ro"` layer is not deletable by someone who cannot write it, and the server
enforces this — the drawer hiding a button is not enforcement.

**Mandatory layers escalate the tier.** Deleting one changes what *every* viewer sees, not just your
own content, so a mandatory layer always gets the **hard** confirm even when empty. The tier should
track **consequence**, not only object count — that is the principle behind VLL's ask, generalised.

### 5. Count scope

The hard confirm names the number of objects on **that layer, across all pages of its file**. Layers
are per-file and objects carry `layerId`, so this is unambiguous — but state it in the dialog, since
"3 annotations" while the user is looking at page 1 otherwise reads as wrong.

### 6. Last-layer / active-layer edges

**No special case.** Deleting the last layer is allowed — the drawer already has `new-layer` to make
another, so the user is never stuck. If the deleted layer was active, activate the next one in
`layerOrder` (or none, if that was the last). Blocking the last delete would be a rule we would have
to explain forever.

## Acceptance criteria

- **Red-first orphan guard:** a core test that deletes a non-empty layer and asserts **no object
  survives referencing the dead layer id** — failing against today's code. Plus the replay-equality
  test from §2.
- Tombstones: after a cascade, a mutation arriving for one of those objects is rejected
  (`deleted-remotely`) rather than re-creating it. Assert this directly — it is the I5 hole.
- One revision per layer-delete (not 1+N).
- Studio: a permission-gated "Delete layer" in the Layers drawer. **Empty → soft confirm.
  Non-empty → type-`DELETE`**, dialog naming the object count. **Mandatory → hard confirm even when
  empty** (§4).
- Server-side permission test: an RO/foreign layer delete is refused by the API, not merely hidden.
- Realtime: a second client sees the layer and its objects vanish.
- Copy check: no string promises permanent/unrecoverable deletion (§3).
- Existing layer testids stay attached to equivalent elements (T78/T80 guard); run the
  **dangling-testid sweep** and the **full** `make e2e` — it runs on isolated ports since T81.
- `tsc -b studio`; `gofmt -l core`, `go vet`, `make test` green.

## Out of scope

- Layer restore/undo, bulk layer ops, merging layers, per-object reassignment.
- Real erasure from revision history (a different feature, with privacy implications).
- T82 (My-files stability), filed the same day.
