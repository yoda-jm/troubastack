# T51 — Icon stamp annotation tool (place tinted glyphs on the score)

**Priority:** normal (VLL 2026-07-17: "inserting icons on the scores (a new tool) can
be fun — icon+color; semi-transparent blue square around a verse + a blue shaker icon
in the corner = use the shaker in this section") · **Size:** M · **Area:** `web/ink`,
`core/internal/bake`, `web/studio` editor + a shared glyph-geometry asset. **The app
needs ZERO work** — annotations bake into the page overlay rasters, so stamped icons
ride the existing pixel path (contrast T50 cues, which are setlist METADATA).

## Context

VLL's workflow: mark a section with an existing semi-transparent shape, then stamp a
small tinted instrument icon (the T50 set) beside it as the "what to play here" cue
in PAGE SPACE. The object machinery is ready for a new kind on both sides:
- TS: `@troubastack/ink` uses a draw REGISTRY (`registerInkDraw(type, fn)`,
  `web/ink/src/index.ts:152` — "no switch anywhere"), bbox select/move/resize/
  duplicate/z-order all generic.
- Go: `core/internal/bake/annotations.go:104` has the parallel type switch to extend.

## Changes

1. **Shared glyph geometry (the one real prerequisite, shared with T50):** the 18
   glyphs as **pre-tessellated normalized polylines/polygons** (points in [0,1]²) in
   ONE generated JSON (e.g. `web/ink/glyphs.json`, embedded via go:embed for the
   baker) — generated from the T50 SVGs. Both renderers already stroke/fill
   polylines, so NO SVG parser anywhere; TS ink, Go bake, and T50's studio picker
   all consume the same file. One source of truth for what a `cuica` looks like.
2. **Ink object kind `icon`:** `{type:"icon", icon:"<glyph-id>", color, opacity,
   bbox}` — registered draw fn scales the glyph into the bbox, tinted `color`,
   honoring opacity. **Unknown-glyph-id → render the `note` fallback** (T50's
   contract). Verify and, if needed, guard: an UNKNOWN object TYPE (old studio
   receiving `icon` from a newer peer over I8 sync) must SKIP gracefully, never
   crash the render — same for the Go baker's default case (skip + log).
3. **Go bake:** `TypeIcon` case rendering the same geometry (fill/stroke polylines,
   tint, opacity) so the baked overlay pixel-matches the editor (I8's committed-
   render fidelity, bake-side).
4. **Editor tool:** toolbar "Icon" → glyph grid + the T50 color swatches; click
   places at a sensible default size, drag sizes; thereafter it's a normal object
   (select/move/resize/recolor/duplicate/delete, layer scoping, sync). Testids for
   e2e.

## Acceptance criteria

- e2e: place a blue `shaker` icon + a semi-transparent rect (existing tool) → reload
  → both persist; move/resize the icon; recolor; pixels light+dark at the gate
  (VLL's exact verse workflow as the spec scenario).
- Red-first bake guard: a bundle baked from a song with an icon object shows the
  glyph in the page overlay (pre-fix: missing); Go + TS render from the SAME
  glyphs.json (assert the file is the single source — no duplicated geometry).
- Graceful-skip tests both sides: unknown object type doesn't crash TS render or Go
  bake; unknown glyph id falls back to `note`.
- `tsc -b`, gofmt/vet/test, editor invariant suites (zeroshift etc.) untouched.

## Out of scope

- App-side vector rendering (baked pixels suffice); free-form icon upload; icon
  libraries beyond the 18 (additive later); animating/flashing stamps.

## Sequencing

With/after T50 (the glyph JSON is the shared prerequisite — whichever lands first
generates it; the other consumes).
