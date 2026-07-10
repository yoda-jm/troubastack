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

### Stage 2 — floating selection toolbar + z-order + duplicate + color
*(resequenced by the 2026-07-08 z-order arch decision — reviews.md)*
- **A floating selection toolbar** (`position:absolute` over the canvas, by the
  selected object — **no layout shift**): color · z-order · duplicate · delete. Drives
  off existing `selectedUuids`.
- **Duplicate** (client-only): `createObject` a copy on the active layer with a small
  offset.
- **Per-object z-order (proto + core + sync — resolved design, build this first):**
  - Proto: `int32 order = 11;` on `Object` (within-layer only; R7 governs layer/zone
    stacking and is untouched — orthogonal). Back-compat: default `0`.
  - Render: within a layer, sort objects by `order`, tiebreak `created_at` then `uuid`
    (so untouched docs keep today's insertion order). Layer-major ordering unchanged.
  - Mutation: a **distinct `reorder` kind** carrying the object + new `order`, gated
    exactly like move/resize (active editable layer, owner/RW), LWW via `version`.
    Expose only **bring-to-front** (`order = maxSibling+1`) and **send-to-back**
    (`order = minSibling−1`), computed client-side from the layer/page siblings. No
    arbitrary drag-reorder (int, not fractional — revisit only if arbitrary reorder is
    ever wanted). Concurrent equal-`order` bumps resolve by the created_at/uuid tiebreak.
  - Both repos (mem + file) persist `order`; the WS snapshot carries it.
- **The style-row auto-hide is NOT in stage 2** — mounting/unmounting the style block
  shifts the stacked layout, violating zero-shift. It moves to **stage 3** (floating
  chrome makes contextual show/hide zero-shift by construction). Do NOT accept a
  transient shift in the current layout.

### Stage 3 — fullscreen layout + contextual style-row auto-hide (after T15)

**WIP review 2026-07-10 (reviews.md) — the ruled landing sequence, in order:**
1. **DOM/CSS reshape to the artifact mockup** (the design is ground truth): ONE slim
   pill top bar (back · serif title · tool cluster · zoom% mono · Layers/Notes/Details
   pill-toggles); layer management moves INTO the tabbed glass drawer
   (Layers | Annotations, collapse ▲); the style row becomes the separate slide-in
   `.ctx` pill (only when a draw tool is active / object selected); a bottom pill bar
   (file-tab parts strip + "N objects · ● live"); wheel-hint pill. The ⓘ Details
   toggle MUST restore access to Details & files (metadata/upload/chart editor/preview/
   danger zone — currently clipped = regression), and the initial scroll position must
   land the page top BELOW the chrome.
2. **Helper-mechanics migration** against the final DOM (its own commit; assertions
   frozen — band math from the chrome bbox, scroll-into-view fractions).
3. **Two sanctioned spec updates** (their own commit, citing the 2026-07-10 ruling):
   `editor-layers` readouts activate a tool first (steps only); `editor-uxfix` #1+#2's
   stable-footprint assertion is retired/rewritten (it tested T05's mechanism; the
   invariant is the live zeroshift spec — T17 precedent). Freeze binds everything else.
4. **Panel-toggle zero-shift flips LIVE** and passes.
5. **Full editor suite green** (incl. the wheelzoom post-zoom-edit invariant spec) →
   land the stack. Nothing lands red; VLL previews from a branch build.
- Float the chrome as `position:absolute` glass bars over the canvas (centered,
  `min(1080px,100vw−28px)`); top-collapsing Layers/Annotations dropdown; floating
  bottom parts/status bar; responsive (desktop/tablet-first; phone one compact row).
- **The style row now auto-hides** (shown only when a draw tool is active) — zero-shift
  because the chrome floats over the canvas; prove it by flipping the `editor-zeroshift`
  panel-toggle assertion from `fixme` to a live `test()` (it must pass). The draw-tool
  no-shift half is already a live guard (landed `146d567`).
- **e2e draw-helper update is SANCTIONED (arch decision 2026-07-09, option a) — with an
  assertion-freeze boundary.** Fullscreen conflicts with the shared draw/click helpers'
  baked-in assumptions (they scroll/measure against the window top + assume the Layers
  panel always open). You MAY update the helper *mechanics*: measure the draw band
  against the **scroll container's** client rect (chrome-inset-aware), scroll targets
  into view when the card is short, and manage the now-toggleable panel's open/closed
  state per what each spec needs (open it for `editor-layers`, dismiss it for
  draw/pick). You may NOT touch any `expect(...)` — assertions are frozen; no check
  dropped, relaxed, or its tolerance widened. Land the helper change legibly (own commit
  if feasible); the reviewer diffs the specs to confirm assertion lines are unchanged
  and spot-verifies a couple behaviorally (pixels) in the new layout. All invariants
  below must still assert AND pass under the updated helpers.

### Stage 3 MOBILE STOPGAP (required in the reshape — 2026-07-10 mobile ruling)

Fullbleed removes the gutters that made touch scrolling possible today (the wet
canvas is `touch-action: none`, `styles.css:504`, and the viewport meta is
`user-scalable=no`) — without a stopgap the fullscreen editor is **unscrollable and
unzoomable on any touchscreen**, including inside the Android app's EditScreen
(A06 embeds this exact route). Stage 3 must therefore include:
- **Select mode = one finger scrolls:** set `touch-action: pan-x pan-y` on the wet
  canvas while the active tool is Select (tap still selects; draw tools keep
  `touch-action: none`). ~2 lines; restores phone/tablet scroll immediately.
- The mockup's phone breakpoint rules (<600px: compact single-row top bar, ctx +
  drawer as full-width sheets, full-width bottom bar, wheel-hint hidden) ship with
  the reshape CSS as designed.
- `backdrop-filter` blur ×3 bars over a large canvas is a GPU cost on low-end
  Android WebViews — provide a reduced/solid fallback at the phone breakpoint (or
  `@supports`/media-query gate).

### Stage 4 — touch gesture grammar (NEW; spec'd 2026-07-10, after stage 3)

The idiomatic canvas-tool grammar (Procreate/GoodNotes/tldraw/Figma-mobile
conventions), implemented on pointer events (which already drive the wet canvas):
1. **Two fingers ALWAYS navigate**, in every tool: two-finger drag pans/scrolls;
   **pinch zooms toward the gesture midpoint**. Feed the SAME live-CSS-transform +
   commit-one-raster-on-settle pipeline as stage 1's wheel zoom (gesture end =
   settle). A fast pinch = ONE raster — the stage-1 invariant applies verbatim.
2. **One finger is tool-modal:** Select → scroll/pan (the stopgap, kept); tap
   selects; drag on a selected object moves it. Draw tool → one finger draws.
3. **Second finger during a one-finger draw CANCELS the stroke** and becomes
   navigation (the GoodNotes/Procreate idiom — prevents accidental marks; never
   commit a half-stroke on gesture escalation).
4. **Pen vs finger (`pointerType === "pen"`):** with a draw tool armed, pen draws
   and a FINGER still navigates (palm-rejection idiom). This is deliberately the
   A07 tablet-stylus test surface — it makes the web wet path evaluable for the
   stylus spike without native ink.
5. Keep `user-scalable=no` (in-app zoom owns pinch; honored in WebViews); on iOS
   Safari, `preventDefault` on the two-finger touchmove inside the canvas blocks
   residual page-zoom (same non-passive pattern as stage 1).
6. e2e: Playwright `touchscreen` taps + CDP `Input.synthesizeTapGesture`/pinch
   where drivable; the raster-count invariant (one raster per settled pinch) is the
   assertable core, mirroring `editor-wheelzoom`.

**Apps note:** the Android app's EditScreen (WebViewHost/A06) inherits all of this —
stage 4 is a hard prerequisite for calling the in-app editor mobile-usable (with
`user-scalable=no` honored in the WebView, there is NO zoom at all until it lands).
Native Stage is untouched (it renders baked rasters natively). iOS Studio embedding,
when it arrives, inherits the same grammar via the existing WKWebView seam.

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
