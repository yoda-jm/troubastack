# B01 — web/bake: the headless overlay renderer (the real I8)

**Priority:** B-track 1 · **Size:** M/L · **Area:** `web/bake`, `web/ink`

## Context

`web/bake` has been a stub (`bake()` throws TODO) since the scaffold. It is the missing
half of invariant **I8**: baked images must come from *the same renderer* as the editor's
dry layer. Everything it needs already exists: `@troubastack/ink` is framework-free and
canvas-agnostic (`Ctx2D` accepts an OffscreenCanvas-style context), T07's registry
pre-populates all built-in draw functions at module load, and the output format is pinned
by `docs/design/08-bundle-container.md` + the Kotlin loader/presenter that already
consume it (A02–A05).

Scope discipline: bake renders **annotation overlays only**. PDF page rasters are core's
job (B02) — `docs/design/06-tech-stack.md` places the PDF rasterizer server-side, and
bake "composes against page rasters" it never produces.

## Changes

1. **Canvas backend**: add `@napi-rs/canvas` to `web/bake` (prebuilt binaries, no
   node-gyp). Its 2D context is Skia-based and structurally compatible with ink's `Ctx2D`;
   where TS nominal types disagree, adapt with one well-commented cast in bake, not by
   changing ink's types.
2. **The render function** (exported, unit-testable):
   `renderOverlays(doc, pageSizes, opts): PageOverlay[]` — for each page and each layer
   (z-ordered, per `Layer.order`), render that layer's live objects via
   `renderObjects` from `@troubastack/ink` onto a transparent canvas sized
   `opts.width × (width / pageAspect)`, and encode. `doc` is the annotations JSON exactly
   as `GET /api/bands/{b}/songs/{s}/annotations` returns it (same shape studio consumes —
   no new format). Emit **PNG** (WebP if `@napi-rs/canvas` encodes it cleanly — refs
   carry the real extension either way, consumers don't assume a codec).
3. **CLI** — `web/bake` gets a real build again (it was set to `--noEmit` while a stub;
   bring back emit via tsc or a tiny esbuild bundle, avoiding the old
   compiled-copy-of-ink trap — project references or bundling both solve it):
   `node dist/cli.js --in request.json --out <dir>` where `request.json` carries
   `{ doc, pages: [{index, width, height}], overlayWidth }` and the output dir receives
   `p<N>-<layerId>.png` files + an `index.json` manifest (`page → [{layerId, file,
   order, mandatory, roleTag, contentHash}]`, sha256-based hash). Core (B02) spawns this
   process; keep stdin/stdout clean (logs to stderr).
4. **The I8 parity test** (the one promised since the audit): a fixture annotations doc
   (freehand with pressure, line, rect+fill, ellipse, text, highlight) rendered two ways —
   by bake, and by the *studio dry path* in headless Chromium (a tiny Playwright script
   that loads ink and calls the same `renderObjects` on a browser canvas, dumping
   `toDataURL`). Compare per-pixel: identical code, two Skia builds ⇒ allow a small
   anti-aliasing tolerance (assert ≥99% of pixels within Δ≤3/255 per channel, and 100%
   agreement on which pixels are transparent). Wire it as `npm test` in `web/bake` and a
   CI step in the web job.
5. Text rendering caveat to handle explicitly: ink's text draw uses canvas fonts —
   register a bundled font file with `@napi-rs/canvas` (`GlobalFonts`) and have the
   parity script load the same font in the browser, else text pixels can't converge.
   Document the chosen font as part of the bake contract.

## Acceptance criteria

- `cd web/bake && npm ci --no-workspaces && npm run build && npm test` green (parity test
  included), and the CLI produces overlays for a fixture request that the **Kotlin loader
  accepts**: assemble the output + a dummy raster into a bundle dir and run the A02
  fixture test path against it (a small script or a documented manual check is fine).
- Overlays are genuinely transparent PNGs, one per layer per page, z-order preserved.
- No DOM/browser APIs at runtime (`node dist/cli.js` runs on a bare Node 24).
- Workspace typecheck still green for ink/studio/bake; `make e2e` untouched.

## Out of scope

- PDF rasterization, bundle.json/zip assembly, endpoints (all B02). Autobake (P201).

## As landed (`00a7da5`, reviewed post-hoc 2026-07-04)

Deviations accepted at review — recorded here per the "executor updates the task file"
rule (the flags originally lived only in the commit message):

- **Transparency clause realized as a tolerance, not the literal 100%.** Across two
  independent Skia builds, AA lands glyph/stroke edges on alpha 0 in one and 1..~130 in
  the other — the same reality that motivated this spec's own Δ≤3 allowance. As landed:
  ≥99.9% symmetric transparency agreement AND no disagreement with a fully-opaque side
  (α=255), which rejects misplaced content while allowing AA edges. Measured 99.9993% /
  99.9759% (worst opaque α=129).
- **Text contract:** ink gained `setTextFontFamily()` (default unchanged — studio is
  byte-identical); bake pins bundled Roboto Regular (`assets/Roboto-Regular.ttf`) under
  the private family `TroubaBakeText` plus `textRendering="geometricPrecision"`. Any
  renderer that must match bake's text pixels loads this file under this name.
- **Kotlin-loader criterion:** in-repo it's guarded by `bundle-crosscheck.test.mjs` (a JS
  mirror of `BundleLoader`'s contract); the review additionally ran the real Kotlin
  `BundleLoader` against a bake-assembled bundle — `Loaded`, zero issues. The live
  core→bake→loader path is wired in B02.
- **Install quirk:** bake's `npm run build` typechecks `web/ink` sources, so `web/ink`
  needs its own `npm ci --no-workspaces` first. CI's web job does this; a bare
  `cd web/bake && npm ci && npm run build` on a fresh clone fails in ink's
  `perfect-freehand` import.
