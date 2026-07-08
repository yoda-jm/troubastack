# Proposal — Canvas-first editor (VLL-requested; for arch review)

**Status:** proposal, awaiting arch review · **Raised by:** Web-Core (2026-07-08) ·
**Area:** `web/studio` — `SongEditor.tsx`, `Viewer.tsx` (+ Toolbar/SidePanels/WetCanvas) ·
**Relates to:** T17 (single-row toolbar redesign) and T15 (Viewer split) — both **attended-only**.

A mockup exists (self-contained HTML, shown to VLL, iterated 4×). This doc is the
reviewable description; the mockup is a visual aid for VLL only.

## Motivation (VLL)

The editor shows *everything, always*: a tall style toolbar, a separate zoom/file row,
an always-open right sidebar (Layers + Annotations), and the details/files section — so
the score itself gets a small slice of the screen. VLL wants a **canvas-first**,
full-viewport editor with **contextual** chrome and **wheel/zoom** behavior like modern
canvas tools. "Feels ok" on the 4th mockup iteration.

## Proposed design

1. **Full-viewport stage.** The score fills the viewport; chrome *floats over* it as
   glass bars rather than stacking above/beside it. The page fills the available
   **height** (portrait side-margins are inherent; not wasted vertical space).
2. **Floating top bar, centered + width-capped** (`min(1080px, 100vw−28px)`, centered) so
   ultra-wide screens don't get an edge-to-edge bar with an empty middle. Holds: back ·
   song title · the **tool cluster** (select/draw/line/rect/ellipse/text) · zoom % ·
   panel toggles (Layers/Annotations) · Details.
3. **Contextual toolbar.** The top bar shows only tools. Style options (color / Outline·
   Box·Highlight / width / target layer) appear **only when a draw tool is active**, as a
   floating bar under the top bar. A small **selection toolbar** (color, z-order,
   duplicate, delete) floats next to a **selected** object. Nothing irrelevant is on
   screen. Drives off existing `tool` + `selectedUuids` state.
4. **Layers/Annotations = a top-collapsing dropdown**, not an always-on sidebar. It hangs
   from the top-bar toggle, content-height, top-right; collapses upward (▲) when unused so
   it never sits over the score full-height. (Directly addresses VLL's "don't overlay the
   drawing" note.)
5. **Parts strip + status** in a floating bottom bar (also centered/capped): the file
   tabs (member my-files order) + "N objects · live".

## Interactions (the "how")

- **Plain wheel → scroll** the score (native `overflow:auto`; not intercepted) — needed
  for multi-page.
- **Ctrl/⌘ + wheel → in-app zoom toward the cursor.** Attach a **non-passive** wheel
  listener via a ref (`addEventListener("wheel", fn, {passive:false})`; React's `onWheel`
  is passive and can't be cancelled). When `e.ctrlKey || e.metaKey`: `preventDefault()`
  (this is what suppresses the browser's own ctrl+wheel page-zoom), then scale and keep
  the point under the pointer fixed (`scrollLeft += cursorX*(newScale/oldScale−1)`; same
  Y). Trackpad **pinch** arrives as a wheel event with `ctrlKey===true`, so the same
  branch covers it. Drag = pan.

## Responsive (tested in the mockup)

- **Desktop / tablet (target for annotating):** centered/capped bars; score fills
  height (desktop) / width (tablet); dropdown panel. Clean.
- **Phone:** top bar stays **one compact row** (icon-only toggles, small tools); the
  contextual bar and panel drop below it (no overlap). Usable but dense — the editor is
  **desktop/tablet-first**; the phone's real home is the **TroubaStage app** (performing),
  so phone editing is "works, not primary."

## Staged implementation (each = its own reviewed commit, verify per step)

1. **Scroll + Ctrl/⌘-wheel zoom-to-cursor** — self-contained, low-risk, high-value.
2. **Contextual toolbar** — style row only when drawing; floating selection toolbar.
3. **Fullscreen layout** — float the chrome, centered/capped bars, top-collapsing panel.

## Invariants to preserve (why this is attended)

`Viewer.tsx` carries the load-bearing editor invariants — please confirm the plan
respects them:
- **No re-raster on annotation edit** (the hidden `pdf-render-count` probe; overlay-only
  repaint path). Moving the toolbar/panels must not touch the raster effect.
- **No-reflow toolbar** (T05's reserved-slot mechanism) — the contextual approach *changes*
  this model (show/hide instead of reserve); the "zero-shift" concern T17 raised becomes
  "does show/hide cause layout shift of the canvas?" The floating chrome is `position:
  absolute` over the canvas, so the canvas box shouldn't shift — but T17 asked for a
  **zero-shift e2e spec FIRST**; I'll write it.
- **Render-timing** (the cancel-guard + one-shot settle from the flip fix) stays.
- **All `data-testid`s** the editor/viewer e2e drive by (tool-*, layer, annotation-item,
  zoom-*, file-tab, pdf-page, pdf-render-count, edit-canvas, …) preserved or specs updated.

## Questions for arch

1. Direction OK, or prefer a variant (e.g., a left vertical tool rail instead of a top
   bar)?
2. Does this **supersede T17** (and pair with T15), and should it be specced as such?
3. Any invariant above I'm underweighting before stage 1 (scroll + wheel-zoom)?

Not implementing until reviewed. Mockup URL is in the VLL chat.
