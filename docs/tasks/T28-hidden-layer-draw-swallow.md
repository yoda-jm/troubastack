# T28 — Drawing on a hidden layer silently swallows the annotation (auto-reveal fix)

**Priority:** high (VLL field bug, 2026-07-10) · **Size:** XS/S · **Area:** `web/studio`
(draw commit path + local visibility state) · **Exists on `main`** (longstanding);
amplified by T27 stage 3 (drawer closed by default → no visibility cue).

## Context (mechanism, proven by reproducer)

VLL: a Highlight ("stabilo") drawn on the Open Road PDF displays while drawing, then
disappears at stroke end and stays gone after a tool change. Root cause, isolated
2026-07-10 (reviews.md): **the wet canvas renders the in-progress stroke regardless of
layer visibility (mid-stroke alpha 255), but the dry overlay filters to
`visibleLayers` — so a stroke committed to a HIDDEN active layer vanishes at commit**
(overlay alpha 0 immediately, after the echo, and after tool changes). The object is
real (synced, in the doc); it is just never painted. Reproduced identically on `main`
and the stage-3 branch; every other flow variant (fresh layer, auto-create, zoomed +
scrolled, echo, tool switch) is clean on both.

How users get there: unchecking the layer's visibility checkbox, or a role-tagged /
seeded layer that `defaultVisible` hides while it is (or becomes) the active draw
layer. On `main` the always-open sidebar at least shows the unchecked box; under the
stage-3 closed-by-default drawer there is NO cue — which is why it reads as pure data
loss.

## Fix (decided): auto-reveal on draw

When a stroke **begins** on the active layer and that layer is not in the local
`visible` set, **set it visible** (local, per-viewer state — no server/model change;
visibility was always a local view preference). The user's intent when drawing is
unambiguous: they want to see what they draw. This is the idiom in every serious
editor (Photoshop refuses hidden-layer edits; GoodNotes/Figma auto-reveal); the
auto-reveal variant is chosen over a blocking notice — zero friction, one state flip.

- Implementation point: the start-draw path (WetCanvas `onPointerDown` draw branch or
  the Viewer handler that resolves the active layer for a new object) — flip
  `visible[activeLayerId] = true` through the existing toggle path so the drawer
  checkbox and the dry overlay both follow.
- Optional stage-3 polish (non-blocking): the Layers pill-toggle shows a small dot
  when the ACTIVE layer is hidden — with auto-reveal this state is rare/transient.

## e2e reproducer (commit as `editor-hidden-layer-draw.spec.ts` — asserts the FIXED behavior; fails today)

Scaffold: register → band → song → upload `fixtures/sample.pdf` → reload →
editor-ready (`pdf-page` + `edit-canvas` visible, `conn-status` = live). Then:

1. Create a layer; **uncheck its `layer-toggle`** (on the stage-3 DOM: open the
   drawer via `sidebar-toggle` first, close it after).
2. `tool-rect` → `preset-highlight` → drag a rect across the page
   (e.g. (0.2,0.2)→(0.6,0.3)).
3. **Mid-stroke** (before `mouse.up`): sample the `edit-canvas` pixel at the rect
   centre — alpha MUST be > 0 (wet renders; this half already passes and pins the
   "visible while drawing" contract).
4. After `mouse.up` + `object-count` reaches 1:
   - the layer's `layer-toggle` is CHECKED again (auto-reveal happened);
   - the `.annotation-overlay` pixel at the rect centre has alpha > 0 **immediately
     (t0), after 2s (t1 — sync echo), and after switching to `tool-select` (t2)**.

(The investigation's throwaway version of this spec — wet=255, t0/t1/t2=0 on both
main and branch — is in reviews.md 2026-07-06→10 session notes; the committed spec
must assert the fixed behavior so it hard-gates the regression.)

## Acceptance criteria

- The new spec green on `main`; the full editor suite green (`tsc -b studio` clean).
- Behavior: drawing on a hidden active layer reveals the layer; the committed
  annotation is visibly painted with no flicker-and-vanish; mandatory layers are
  unaffected (they can't be hidden).
- The stage-3 branch picks the fix up on rebase (verify the spec passes there too —
  drawer-DOM mechanics via the fullscreen helpers).

## Out of scope

- Server/model changes (visibility stays a local view preference); the drawer-badge
  polish (optional follow-up); blocking-modal UX (rejected).
