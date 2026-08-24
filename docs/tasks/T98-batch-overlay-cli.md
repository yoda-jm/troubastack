# T98 — Batch the overlay worker across songs: one node spawn per bake, not N

**Priority:** normal — the follow-up T97 §3.2 split out by ruling (2026-08-24). · **Size:** S/M ·
**Area:** `core/internal/bake`, `web/bake`. Lane: Web & Core.

## Why (measured in T97)

The overlay worker (`node` + `@napi-rs/canvas` Skia) costs **~577 ms of process startup per spawn**
(median of 5: bare `node -e ''` ~110 ms; overlay CLI cold start ~577 ms; `pdftoppm -h` ~12 ms). T97
stopped spawning it for songs with **nothing to draw**. But it still spawns **once per annotated
song**, and startup dominates the annotated case too (~0.7 s/song is mostly the Skia load, not the
draw). A heavily-annotated concert still pays an O(songs) node tax.

Fable's ruling (T97 verdict): batch it as its own increment, not folded into the one-guard change,
because it sits near the concurrency path.

## The change

Turn the per-song overlay spawn into **one `node` invocation for the whole bake**:

1. **`web/bake/src/cli.ts`**: accept a BATCH request `{ songs: [{ key, doc, pages, overlayWidth }] }`
   as well as the current single `{ doc, pages, overlayWidth }` (keep back-compat, or migrate the one
   caller). Render each song, namespace its outputs (`out/<key>/…`), and write a batch manifest
   `{ songs: [{ key, pages: [...] }] }`.
2. **`core/internal/bake/render.go`**: `OverlayRenderer.RenderBatch(ctx, []overlaySong) (map[key][]renderedOverlay, error)`
   — marshal one request, one spawn, parse the batch manifest, distribute per key.
3. **`core/internal/bake/baker.go`**: split `bakeSong` into a **stage** phase (raster + build the doc,
   collect the overlay request for songs with objects) and an **assemble** phase (write rasters +
   overlays). One `RenderBatch` call between them. **Keep the bake sequential** — do not add
   parallelism here (T97 §3.3); the point is O(1) spawns, and this path already has a known race
   (`TestBake_ConcurrentSameSetlist_distinctRevs`).

## Acceptance

- **Byte-identical bundles**: each song's overlays are the same whether rendered in a batch or alone
  (proven at the property level — the CLI runs the same `renderOverlays` per song; a per-song blob-hash
  comparison across single vs batch is the check).
- **One spawn**: a bake of an all-annotated N-song setlist spawns the overlay worker **once**
  (counting fake); a no-annotation bake still spawns **zero** (T97's guard preserved).
- **Re-measure** the T97 4-song bake (before/after) — the annotated-song node tax should collapse to
  a single startup.
- **`go test -race ./internal/bake/` green**, including `TestBake_ConcurrentSameSetlist_distinctRevs`,
  run `-count` a few times — the restructure must not surface or worsen that race.
- **CI gap (from the T97 verdict)**: `TestOverlayRenderer_EmptyDoc_ZeroOverlays` — and any new
  batch-vs-single parity test that needs the real CLI — must actually RUN in CI, not skip. The `go`
  job never builds the bake CLI; fix by building it in the `go` job or moving the case to the `web`
  job (which already runs the I8 bake-parity test). A skip that disables a load-bearing proof is worse
  than not writing it.
- `gofmt -l core` clean; `go test ./...` + the `web` bake tests green.

## Out of scope

- Parallelism / a persistent worker (T97 §3.3, later if ever).
- Progress reporting — **T96**.
