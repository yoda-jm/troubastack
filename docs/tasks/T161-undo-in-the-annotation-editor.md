# T161 — Undo in the annotation editor

**Surface:** TroubaStudio, the song editor. **Lane:** web-core. **Kind:** feature.
**Number claimed** in the same push as this file.

VLL asked for it directly. The gap was **exposed**, not created, by T155: before that, a completed draw
left the new shape selected, so pressing Delete removed a botched stroke. T155 correctly stopped a drawing
tool from leaving things selected — and that removed the only quick way to take a stroke back. There is
**no undo anywhere in Studio** today (`git grep -i undo` over `web/studio/src` returns nothing) and no
task specified one.

## The model already wants this

The annotation engine is event-sourced and was built with revert in mind:

- `KindDelete` is a **terminal tombstone**, `KindRestore` is *"the ONLY revive"* (I5).
- A snapshot's `Objects` *"includes tombstones (Deleted=true) **so revert is reconstructable**"*.

So the rule falls out of the model rather than being imposed on it:

> **Undo APPENDS the inverse mutation. It never rewrites, removes, or rolls back history.**

Every kind already has an inverse: `Create` → `Delete`; `Delete` → `Restore`; `Move`, `Resize`,
`SetStyle`, `SetText`, `Reorder` → the same kind carrying the previous value. Nothing new is needed on the
wire, and sync, layers and the bake keep working because an undo looks like any other edit.

## Three rules that make undo safe on a SHARED canvas

This is not a single-user document. Get these wrong and undo becomes a way to damage a bandmate's work.

1. **Undo undoes YOUR last action, not the last action.** The stack is per user and per song. A global
   "undo the most recent mutation" would let one member erase another's stroke with a keystroke.
2. **Refuse when the object moved on underneath you.** If the object has been mutated by anyone else since
   your action, do **not** apply the inverse — say so ("someone else changed this since") and drop that
   entry. Silently overwriting their change to honour your undo is the harm this rule exists to prevent.
3. **Re-check permission at undo time, not at record time.** The editor already refuses to commit onto a
   layer it may not write (`isEditableLayer`, T30). A layer can become read-only between the action and the
   undo; the same check must run again, with the same visible notice.

## Scope, stated so nobody assumes more

- **In-session, per song, bounded** (50 entries is plenty). Leaving the song clears it. It is not
  persisted, and it is **not** a document history — say that in the UI wording if any is needed.
- **Redo is out of scope** for this slice, but the stack must not preclude it: record the inverse *and*
  the forward value, so redo is a later addition and not a rewrite.
- Layer create/update/delete/reorder are out of scope; this is about marks on the page.

## The surface

A **visible control**, not only `Ctrl`/`Cmd`+`Z`. The editor is used on touch — T156 exists because the
style bar could not be reached on a phone — so a keyboard-only affordance would be no affordance at all
for half the cases. Put it where it is reachable at a narrow viewport, i.e. in the bar T156 just made
pannable, and disable it (visibly) when the stack is empty rather than hiding it, so it does not appear
and disappear under the thumb.

## ⟨R1⟩ Red first

- **The motivating case:** draw a stroke, press undo → the object count returns to what it was, the canvas
  shows the earlier state, and **the tool stays armed** (undo is not a mode change). Red today: there is
  no undo.
- **The shared-canvas guard:** your object is mutated by another author, then you undo your own earlier
  change to it → **nothing is written** and the user is told. **Teeth:** remove the version check and this
  test must go red, otherwise rule 2 is decoration.
- **Per-user:** with two authors' mutations interleaved, undo removes **your** last one and leaves theirs
  untouched — assert the other author's object is byte-identical afterwards.
- **Undo of a delete restores**, and the restored object keeps its points and style (the `Restore` path,
  not a re-create with a new id).
- **Bounded:** more than the limit of actions, then undo repeatedly → it stops at the limit without error
  and without corrupting the stack.
- **Permission:** the layer becomes read-only after the action → undo refuses with the T30 notice, and the
  stack entry is dropped rather than retried forever.

## Done means

The stroke VLL just botched can be taken back in one gesture, on a phone as well as a laptop; no undo can
touch another member's work; and the annotation history remains append-only, so the bake, the sync and
T145's anchors all keep the meaning they have today.
