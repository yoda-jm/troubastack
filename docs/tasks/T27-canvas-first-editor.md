# T27 — Canvas-first editor (supersedes T17; pairs with T15)

**Priority:** high (VLL-requested) · **Size:** L (staged) · **Area:** `web/studio` —
`SongEditor.tsx`, `Viewer.tsx`, Toolbar/SidePanels/WetCanvas · **Attended:** yes
(editor render-timing + zero-shift; drive the e2e safety net each stage)

## Context

VLL asked for a full-viewport, canvas-first editor (contextual chrome, wheel-scroll +
Ctrl/⌘-wheel zoom-to-cursor, responsive), iterated a mockup 4× to "feels ok", and asked
that the direction be arch-reviewed before build. Proposal:
`docs/handoff/proposals/editor-canvas-first.md`. Arch review = this spec (verdict in
`reviews.md`, 2026-07-08). The proposal's design description stands; this file records
the **rulings** that shape the build.

## Rulings (the three questions)

1. **Direction: GO as proposed** — canvas-first, floating centered/width-capped top bar,
   contextual style/selection toolbars, top-collapsing Layers/Annotations dropdown,
   floating bottom parts/status bar. The **top bar stays** (do NOT build the left
   vertical tool-rail variant now — VLL validated the top-bar mockup; the rail is a
   fallback only if the floating bar proves cramped in practice, filed then, not
   speculatively).
2. **Supersedes T17; sequenced with T15.** T17 (collapse the style bar behind a
   disclosure) was the narrow fix for the same root problem (chrome eats the score);
   the contextual-chrome design solves it more completely → **T17 is CLOSED, superseded
   by T27**, and T17's hard requirement carries over: **the zero-shift e2e spec is
   written FIRST** (before stage 3). **T15** (split `Viewer.tsx` into pdf/overlay/sync
   hooks) **lands before stage 3** — the fullscreen-layout rework builds on the split,
   not the monolith. Stages 1–2 do not need T15.
3. **Invariants: the proposal's list is right; one addition is load-bearing for
   stage 1 — see below.**

## Staged implementation (each = its own reviewed commit; verify + e2e each step)

### Stage 1 — scroll + Ctrl/⌘-wheel zoom-to-cursor
- Plain wheel scrolls (native, not intercepted); Ctrl/⌘+wheel (and trackpad pinch =
  `ctrlKey` wheel) zooms toward the cursor via a **non-passive** `wheel` listener on a
  ref (`{passive:false}` + `preventDefault()` only on the ctrl/meta branch).
- **REQUIRED (the invariant the proposal underweights): decouple visual zoom from
  rasterization.** The PDF render pass is keyed on `scale` (`Viewer.tsx:831`), so a
  continuous wheel/pinch would fire one poppler render *per tick* and churn the
  flip-fix cancel-guard. Apply the live zoom cheaply (CSS transform on the canvas box)
  and **commit the crisp re-raster only on wheel-settle** (debounce, e.g. ~120ms idle),
  or otherwise coalesce scale changes. A fast pinch must produce ONE raster, not dozens.
- Zoom-to-cursor scroll math (`scrollLeft += cursorX*(newScale/oldScale−1)`) must be
  computed against the **scroll container's geometry synchronously** — not the
  async-decoded canvas (which hasn't re-rendered yet at the new scale).
- Clean up the listener on unmount / ref-node change.

### Stage 2 — contextual toolbar
- Top bar shows tools only; the style row (color/Outline·Box·Highlight/width/target
  layer) mounts **only when a draw tool is active**; a small selection toolbar
  (color/z-order/duplicate/delete) floats by a selected object. Drive off existing
  `tool` + `selectedUuids` state.

### Stage 3 — fullscreen layout (after T15)
- Float the chrome as `position:absolute` glass bars over the canvas (centered,
  `min(1080px,100vw−28px)`); top-collapsing Layers/Annotations dropdown; floating
  bottom parts/status bar; responsive (desktop/tablet-first; phone one compact row).

## Invariants to preserve (confirm each stage)

- **No re-raster on annotation EDIT** — the `pdf-render-count` probe / overlay-only
  repaint path stays. NB: zoom re-rastering is EXPECTED and fine; the invariant is
  specifically about edits. **Add a test:** after a zoom settles, an edit still does
  not bump the render count.
- **Zero-shift** — floating chrome is absolute-over-canvas, so the canvas box shouldn't
  move on show/hide; prove it with the T17-mandated zero-shift e2e (written before
  stage 3), covering contextual-toolbar and panel open/close.
- **Render-timing** — the flip-fix cancel-guard + one-shot settle stay intact; verify
  fast zoom doesn't defeat them.
- **All editor/viewer `data-testid`s** (tool-*, layer/active-layer, annotation-item,
  zoom-*, file-tab, pdf-page, pdf-render-count, edit-canvas, …) preserved or specs
  updated in the same commit.

## Acceptance criteria

- Each stage: `tsc -b studio` clean; the editor/viewer e2e (editor-noflicker, viewer,
  flows, box-render) green; the changed interaction has its own spec (stage 1: a
  wheel-zoom test asserting one raster per settled zoom + point-under-cursor stays;
  stage 3: the zero-shift spec).
- Attended per step (render-timing is environment-sensitive; the flip bug proved
  headless can't see everything — drive it in a real browser too).

## Out of scope

- The left tool-rail variant; phone-first editing (phone = the Stage app); changing the
  annotation model or the bake path.
