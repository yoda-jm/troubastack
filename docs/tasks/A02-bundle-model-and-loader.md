# A02 — Concert-bundle domain model + resilient loader (shared)

**Priority:** A-track 2 (after A01) · **Size:** M · **Area:** `app/shared` commonMain/commonTest, `docs/design/`

## Context

TroubaStage (the presenter) consumes **baked concert bundles**: per page, a PDF-page
raster image plus transparent per-layer overlay images (invariant I12 — the presenter is
a pure image compositor + pager). The bundle *messages* are defined in
`proto/troubastack/v1/bundle.proto` (`ConcertBundle`, `BakedSong`, `PageImages`,
`LayerImage`, `AvailableConcert(s)`) — read that file first; it is the authority.

Nothing produces bundles yet (the server-side bake is a stub), and no Kotlin proto
codegen exists. Per the repo's current policy (see T09/T12: proto is the source of truth,
clients carry hand-written mirrors until codegen is adopted), this task hand-writes the
Kotlin mirror **with a comment on each type pointing at bundle.proto** — never invent
fields that aren't in the proto.

This task also fixes what no doc defines yet: **the on-disk container format**. Bundles
need a concrete file layout before the presenter, the importer, and (later) the server
bake can agree.

## Changes

1. **Container spec** — write `docs/design/08-bundle-container.md` (~1 page):
   - A bundle is a directory: `bundle.json` + `blobs/<ref>` files.
   - `bundle.json` is the proto3 **canonical JSON** encoding of `ConcertBundle`
     (lowerCamelCase field names, e.g. `concertId`, `concertRev`, `bakedAt`), so a future
     `buf`/protobuf-generated encoder produces the same bytes.
   - Blob refs (`pageRasterRef`, `imageRef`) are opaque relative paths under `blobs/`
     (e.g. `blobs/p0-raster.png`). Production rasters will be WebP; refs carry the real
     extension and consumers must not assume a codec.
   - Transport form: the same directory zipped, extension `.tstage`, `bundle.json` at the
     zip root. (Import/unzip is A05's job — only *specify* it here.)
2. **Kotlin model** — `app/shared/src/commonMain/kotlin/com/troubashare/shared/bundle/`:
   data classes mirroring bundle.proto exactly (`ConcertBundle`, `BakedSong`,
   `PageImages`, `LayerImage`; also `AvailableConcert`/`AvailableConcerts` for later),
   annotated with kotlinx-serialization `@Serializable` (+ `@SerialName` where the JSON
   name differs from the Kotlin name). Add kotlinx-serialization-json to commonMain deps.
3. **Resilient loader** — `BundleLoader` in the same package. Its contract is the
   presenter's resilience foundation, so it is strict:
   - **Total function: it never throws.** Every failure is a value:
     ```kotlin
     sealed interface LoadResult {
       data class Loaded(val bundle: ConcertBundle, val issues: List<BundleIssue>) : LoadResult
       data class Failed(val reason: String) : LoadResult   // human-readable, no stack trace
     }
     ```
   - Filesystem access goes through a tiny `BundleFiles` interface (`exists(path)`,
     `readText(path)`, `sizeOf(path)`) passed in by the caller — keeps commonMain free of
     platform IO and makes the loader unit-testable with an in-memory fake. (The real
     implementation lands with A05 via the Storage seam; do not touch the seams now.)
   - Missing `bundle.json`, malformed JSON, or unknown-but-required structure ⇒ `Failed`
     with a message a musician could read ("bundle.json is missing", not a JSON path).
     Configure Json with `ignoreUnknownKeys = true` (forward compatibility).
   - A **missing/empty blob file referenced by a page ⇒ NOT Failed**: the bundle still
     loads, the page is flagged via `BundleIssue(page, ref, kind)` so the presenter can
     show a placeholder for just that page (I12 resilience: one bad page must never take
     down a performance).
   - Sort overlays by `order`; ignore (and flag) duplicate `layerId`s on the same page.
4. **Unit tests** (commonTest, using the in-memory fake — no real files needed):
   valid manifest round-trip; missing manifest; truncated JSON; unknown extra keys OK;
   missing blob ⇒ Loaded-with-issue; zero songs / zero pages ⇒ Loaded, empty, no crash;
   overlays out of order ⇒ sorted.
5. Replace the placeholder `Page`/`ConcertBundle` classes in
   `stage/Presenter.kt` with imports of the new model **only if** it keeps compiling
   trivially; otherwise leave Presenter.kt untouched and note it for A04 (which rewrites
   it anyway).

## Acceptance criteria

- `cd app && ./gradlew :shared:check` green, including the new tests.
- Grep gate: no `throw` reachable from `BundleLoader.load` for malformed *input* (defensive
  `require` on programmer error is fine — input problems must come back as values).
- `docs/design/08-bundle-container.md` exists and matches what the code parses
  (field names, layout, `.tstage`).
- Each model class carries a comment naming its proto message.

## Out of scope

- Image decoding, compositing, any UI (A04); real file IO / zip (A05); network.
