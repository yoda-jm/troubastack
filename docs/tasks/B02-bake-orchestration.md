# B02 — Core bake orchestration + Studio "Bake" button (I11)

**Priority:** B-track 2 (after B01) · **Size:** L · **Area:** `core/internal/bake`, `core/internal/httpapi`, `web/studio`, `proto`

## Context

Invariant I11: performable revisions come from an **explicit, manual bake** of a setlist.
`core/internal/bake` is a documented stub. B01 supplies the overlay renderer; this task
turns a setlist into a downloadable `.tstage` — which completes the product loop *even
before distribution exists*: bake → download → share to phone → Import (A05) → perform.

Read first: `docs/design/04-publish-pipeline.md`, `docs/design/08-bundle-container.md`,
`bake/doc.go`, and the A03 generator (`core/cmd/mkbundle`) whose manifest-writing you
should mirror (canonical JSON, 64-bit ints as strings).

**Two design decisions are made here, not left open:**

1. **Setlist overrides ride as manifest metadata, not burned into pixels.** Extend
   `bundle.proto`'s `BakedSong` with `string display_notes`, `string key`, `int32 tempo`
   (new field numbers — wire-compatible). Update the Kotlin mirror (`BundleModel.kt`,
   with its AUTHORITY comment) in the same PR; the presenter may *display* them in a
   later task, loaders must tolerate their absence (they already ignore unknowns).
2. **v1 bakes one file per song**: the song's default shared-pool file (the same one
   Studio opens by default), all its pages, all layers. Per-member my-files bakes are a
   product decision for later — note it in the code where the file is chosen.

## Changes

1. **PDF rasterization sidecar**: shell out to `pdftoppm` (poppler-utils) —
   `pdftoppm -png -r 150 file.pdf out-prefix`. Binary path via `TROUBA_PDFTOPPM`
   (default `pdftoppm`); a missing binary fails the bake with a clear message, never the
   server. Document the dependency in the README toolchain line and install
   `poppler-utils` in the CI go job (it's in ubuntu's apt).
2. **`bake.Baker` (real API at last)**: `Bake(ctx, setlistID, actor) (ConcertBundle
   meta, error)` — resolve the setlist's songs (use each song's current head annotation
   revision; record it as `source_revision` — if the store's pin plumbing already gives a
   pinned revision per setlist item, prefer the pin; investigate and say which in the
   PR), rasterize the default file's pages, run B01's CLI for per-layer overlays, hash
   everything, write the bundle dir per the container spec + zip to `.tstage`, store
   under the data dir (`bakes/<concertId>/<rev>/`), bump `concert_rev` monotonically per
   setlist. Write `bundle.json` via a small hand-written Go mirror of `ConcertBundle`
   (AUTHORITY comment, canonical JSON: lowerCamelCase, 64-bit as strings — crib from
   `mkbundle`).
3. **Endpoints** (`httpapi`): `POST /api/bands/{b}/setlists/{s}/bake` (admin-only — same
   gate pattern as T08's import) returning the bake metadata; `GET
   /api/bands/{b}/concerts` (member) listing baked concerts (this becomes B03's
   manifest); `GET /api/bands/{b}/concerts/{c}/bundle` (member) streaming the `.tstage`.
4. **Studio UI**: on the Setlist page, an admin-visible **Bake** button → calls the
   endpoint, shows progress/failure, then a "Download .tstage" link + bake history
   (rev, when, by whom). Keep it one card; no new routes.
5. **Wire `Baker` into the composition root for real** (T11 removed the fake injection;
   this adds the genuine one — coordinate if T11 hasn't landed yet).
6. **Tests**: Go — a bake against seeded fixtures produces a bundle whose `bundle.json`
   parses, whose blob refs all exist, and whose re-bake bumps `concert_rev`; endpoint
   auth (member 403 on bake, 200 on list/download). Skip the pdftoppm-dependent test
   gracefully when the binary is absent (`t.Skip`) but run it in CI. E2E — one Playwright
   spec: admin bakes, download link appears.

## Acceptance criteria

- `make demo` → Setlists → Bake → download the `.tstage` → **it imports and performs in
  the Android app** (emulator or device; screenshot in the PR). This is the loop-closing
  criterion — the hand-baked `docs/demo` bundle's days are numbered.
- The parity chain holds: baked overlay pixels come from `web/ink` via B01 (grep: core
  never renders strokes — I8's "no third copy").
- `buf lint` green after the proto change; Kotlin mirror updated + `:shared:check` green.
- `make test` green (incl. new bake tests); e2e green.

## Out of scope

- App-side downloading/offers (B03), autobake/live (P201), per-member my-files bakes,
  bake GC/retention (P202 owns pruning old bakes).
