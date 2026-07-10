# T34 — Touch editor stuck in navigation after a two-finger gesture (stale pointer entry)

**Priority:** HIGH (field: editing becomes impossible on touch until reload) ·
**Size:** XS/S · **Area:** `web/studio` (WetCanvas pointer state) + committed
reproducer · **Found:** VLL field report 2026-07-11 ("after removing the 2 fingers
and only using one after, I was stuck in the [nav] — edition impossible"),
root-caused + reproduced by the architect the same day.

## Root cause (proven by a two-variant reproducer)

`WetCanvas.onPointerDown` tracks touches in `pointersRef` and starts a two-finger
nav at `size >= 2`. Two compounding facts:

1. **Nav pointers are never captured** — every `setPointerCapture` call sits in
   the pan/stroke/move branches; the nav-start branch returns without capturing
   either finger. An uncaptured touch delivers its `pointerup` to the element
   under the finger — and the floating chrome's BUTTONS are `pointer-events:
   auto` islands above the canvas (the pills' glass passes through, the controls
   don't). A nav finger lifting over any button (or outside the window) means
   the canvas **never sees that `pointerup`**.
2. **`pointersRef` has no self-healing** — the missed entry stays forever. Every
   subsequent single-finger `pointerdown` makes `size >= 2` → instant nav →
   `cancelWetGesture()` → no stroke ever starts. Editing is dead until reload.

Verified both directions on main: a CLEAN two-finger nav (both lifts on-canvas)
followed by a single-finger stroke commits fine (control passes); the same flow
with one lift dispatched off-canvas fails persistently — the exact field
symptom.

## Changes

1. **Self-healing invariant (the load-bearing fix):** at the top of
   `onPointerDown`, if `e.isPrimary` is true, clear `pointersRef` (and reset
   `navRef` if somehow set). A primary pointer is BY SPEC the only active pointer
   of its type — any lingering same-type entries are stale by definition. This
   heals missed ups AND browser-swallowed `pointercancel`s of every flavor.
2. **Capture the nav pointers:** in the nav-start branch, `setPointerCapture`
   both ids (and any later extra touches that get ignored while navigating), so
   real devices deliver `pointerup`/`pointercancel` to the canvas regardless of
   where fingers lift. (Capture calls must stay throw-safe for exotic
   pointers — wrap or check `hasPointerCapture` on release as the code already
   does.)
3. **Committed reproducer** (`editor-touch-stucknav.spec.ts` or extend
   `editor-touch.spec.ts`): raw `PointerEvent` dispatch (touch type), with a
   `setPointerCapture`/`releasePointerCapture` try/catch shim via
   `addInitScript` (synthetic pointerIds can't be captured — without the shim
   the draw branch dies on `NotFoundError` and the spec lies). Sequence: F1
   down + F2 down on the canvas (nav) → F1 up on canvas → **F2 up dispatched on
   `document.body`** (the missed lift) → single-finger stroke → assert
   `object-count` increments. Must FAIL pre-fix (it does — verified) and pass
   post-fix. Keep the clean-lift control as a second test (it passes today;
   guards the fix against overcorrecting).

## Note for the attended device pass (NOT this task)

`penSeenRef` is sticky for the session: once ANY pointer reports `pen`, fingers
with a draw tool armed pan forever (the designed pen/finger split). If a device
ever misclassifies a touch as pen, finger editing silently stops — same felt
symptom, different mechanism. Worth checking on VLL's tablet during the A07
session; a possible refinement (decay/reset when no pen among active pointers
for N minutes) is a design call for VLL, out of scope here.

## Acceptance criteria

- The new reproducer fails on pre-fix main and passes post-fix; the clean-lift
  control passes on both.
- `editor-touch.spec.ts` (pinch → one raster) stays green — capture must not
  break the CDP-driven pinch path.
- Full editor suite green; `tsc -b studio` clean.

## Out of scope

- The pen/finger split design (above); wheel/mouse paths (unaffected — mouse
  pointers are always captured in their branches).
