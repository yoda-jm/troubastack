# A03 — Fixture bundle generator + committed demo bundle

**Priority:** A-track 3 (after A02) · **Size:** S/M · **Area:** `core/cmd/mkbundle`, `app/shared` test resources

## Context

The real bundle producer (the server-side bake, invariants I8/I11) doesn't exist yet, and
the presenter track must not wait for it. This task builds a **dev-only generator** that
emits valid bundles per the container spec (`docs/design/08-bundle-container.md`, from
A02), so A04's presenter has real, decodable input on day one — plus "torture" variants
to prove the never-crash contract.

Renderer boundary note (I8): the one stroke renderer is `web/ink`, and Go must never
render *user strokes*. Fixture overlays are synthetic test patterns (rectangles, lines,
labels) — allowed, but say so in the tool's doc comment so nobody grows it into a bake.

## Changes

1. **Generator** — new Go tool `core/cmd/mkbundle` (stdlib only: `image`, `image/png`,
   `encoding/json`, `archive/zip`):
   - Flags: `-out <dir>`, `-songs N` (default 2), `-pages N` per song (default 3),
     `-zip` (also emit `<name>.tstage`).
   - Page rasters: PNG, ~800×1130 (A4-ish), white background, big page number + song
     name text is optional — a simple pattern (borders, page number drawn as blocks) is
     fine; the point is *visually distinct pages*, not typography.
   - Overlays: 1–2 transparent PNGs per page (distinct translucent colors/shapes), with
     `layerId`, `order`, one layer marked `mandatory: true`, another with a `roleTag`.
   - `bundle.json`: canonical proto3-JSON `ConcertBundle` exactly as A02's loader parses
     (same field names — copy them from the container spec, not from memory), stable
     fake ids, `concertRev: 1`, `bakedAt` from a `-seed`able clock so output is
     deterministic (byte-identical across runs with the same flags).
   - Torture variants behind `-torture <dir>`: (a) `missing-blob` — valid json, one
     raster file deleted; (b) `bad-json` — truncated `bundle.json`; (c) `empty` — zero
     songs; (d) `no-manifest` — blobs only.
2. **Committed fixtures** — run the tool and commit the output under
   `app/shared/src/commonTest/resources/fixtures/` (demo bundle + the four torture
   variants). Keep it small: total < ~500 KB (shrink page size/count if needed).
   Document the regen command at the top of a `fixtures/README.md`.
3. **Wire one end-to-end test**: an A02 loader test that reads the *committed* demo
   fixture (via the test-resources path) and asserts song/page/overlay counts — proving
   generator and loader agree on the container format.
4. Add a `make fixtures` target (regenerates into the fixtures dir) and mention it in the
   tool's doc comment.

## Acceptance criteria

- `go run ./cmd/mkbundle -out /tmp/demo` (from `core/`) produces a directory that A02's
  loader loads with zero issues; `-zip` produces a `.tstage` whose contents match.
- Same flags ⇒ byte-identical output (determinism — needed for the committed fixtures to
  stay stable in review diffs).
- `cd app && ./gradlew :shared:check` green, including the new fixture-based test.
- `make test` green (the new Go tool has at least one test: torture variants exist and
  differ from the valid bundle in the intended way).
- Committed fixture footprint < 500 KB.

## Out of scope

- WebP encoding (PNG is fine for fixtures; refs carry extensions, consumers don't assume
  a codec); anything resembling the real bake (no PDF input, no `web/ink`).
