# T52 — Setlist reorder motion: FLIP animation + drag-hint flicker fix (studio)

**Priority:** normal (VLL field report 2026-07-17) · **Size:** studio S · **Area:**
`web/studio` (SetlistDetail). Presentation only — no API/model/testid change.

## What VLL reported

Dragging a song in the setlist "flashes blue" and is chunky; wants nicer motion —
items animating to their new position (and ideally repositioning live while dragging).

## Ruling (Fable, docs/handoff/reviews.md 2026-07-17)

**Build (a) FLIP + the flicker fix.** Animate every reorder path uniformly through the
one place the order changes — FLIP covers drag, ↑/↓, ★ and cross-group moves for free,
dependency-free, on every browser. **(b) View Transitions** rejected (Firefox silent
fallback). **(c) live-reposition-under-the-finger** deferred as phase-2 (the N4(b) of
this feature: real value, real risk of mutating order mid-drag) — decide after VLL
lives with (a).

## Design (as pinned)

1. **Flicker fix (REQUIRED).** The `.row.drag-over` inset-blue hint is toggled by
   `onDragOver`→true / `onDragLeave`→false; `dragleave` fires when the cursor crosses a
   child (grip, buttons, T50 cue chips) still inside the row → the hint blinks. Fix:
   only clear when the pointer actually leaves the row (`relatedTarget` not contained).
   Soften the hint with a short CSS transition.

2. **FLIP** (First-Last-Invert-Play). A small dependency-free hook in SetlistDetail:
   each row registers its element by `item.id`; a `useLayoutEffect` snapshots every
   tracked row's rect each commit, and for any row whose position moved, applies the
   inverse `translate()` then transitions it to zero. Because BOTH the running-order and
   bench rows register into the ONE map, a ★ cross-group move animates the row across
   (not just the collapse/expand around it). ≤220ms; `prefers-reduced-motion: reduce`
   skips the transforms (instant, as today).

3. **No API/model/testid change.** The reorder endpoints, bench/on-call model, and all
   `data-testid`s are untouched; this is pure presentation over the existing reload.

## Acceptance

- Reorder still LANDS correctly (existing e2e: setlist-dnd, encore-bench — order/
  numbering asserted; motion itself is not mechanically testable).
- Drag hint no longer flickers when the cursor crosses row children.
- FLIP animates drag-drop, ↑/↓, and ★ cross-group moves; reduced-motion is instant.
- Screen capture at the gate.

## Out of scope (phase-2)

Live reposition-under-the-finger while dragging (option c) — revisit if VLL still wants
it after (a).
