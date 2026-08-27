# T120 — A content-keyed render cache for bake

**Priority:** normal · **Size:** M · **Area:** `core/internal/bake` (Web-Core lane).
VLL, 2026-08-27: *"can there be a render cache (pdf pages, layer pages) that is by default, that we can
force in the call and that is marked as dirty when a page pdf is changed or when an annotation changes a
layer?"* — plus: *"it is important to force not using the cache or cleaning it at least for testing purpose."*

## 1. Measured today

A bake runs two rendering stages, **neither cached**:

1. **poppler** `pdftoppm` per song PDF → page rasters (`render.go`, `popplerRasterizer`).
2. **one Node `web/bake` batch** for the whole bake → overlay PNGs (`nodeOverlayRenderer.RenderBatch`).

The only work-avoidance that exists is **T97's**: songs with no objects to draw never reach the overlay
renderer. Change one annotation on one page of one song and **every PDF in the setlist is re-rasterized
from scratch**.

**The hashing primitives already exist — but on the output side of the render:**

- Blobs are already content-addressed: `blobs/<sha256>`, dedup by `BlobHash` (`app/bandio.go`, `app/blob`).
- `bake.Sha256Hex` is computed on the **rendered** bytes: `PageImages{RasterHash: Sha256Hex(r)}`
  (`baker.go:612`), and the overlay manifest carries a per-overlay `ContentHash` (`render.go`).
- Those exist so the **app** can skip work (R10) — the Stage's `LruCache`/`decodeCached` is already keyed
  on `rasterRef + rasterHash`. The app half of this idea is effectively built.

So: we hash outputs so downstream can skip, and hash nothing on the way in, so the server always redoes it.

## 2. The design decision: key on content, do NOT mark dirty

The request said "marked as dirty when a page pdf changes". **Build the hash-keyed version instead.**

Dirty-marking is invalidation-by-notification: every code path that mutates a PDF or an annotation must
remember to invalidate. The path that forgets serves a **wrong pixel silently, forever** — a performer
reading a stale annotation on stage. Content-hash keying is self-invalidating: the key *is* the content, so
a changed input simply misses and no mutation path can forget to do anything.

**Cache keys:**

| entry | key |
|---|---|
| page raster | `sha256(pdf bytes)` + `dpi` + poppler version |
| page overlay | `sha256(stroke objects + layerId/order/roleTag + page dims)` + **ink version** |

**The ink version in the overlay key is not optional.** I8 makes `web/ink` the single authoritative
renderer and pixel parity with Studio the entire point of shelling out to it. If ink changes and the key
doesn't, every cached overlay keeps serving pixels from the *old* renderer — a silent, repo-wide parity
break, which is the exact failure I8 exists to prevent. Derive it from something that actually moves when
ink's output can move (its `package.json` version and/or a hash of the built `dist/`); state which you
chose and why.

## 3. What to build

**(a) The cache**, on by default, under a dir from a `TROUBA_*` env var (follow the existing
`TROUBA_PDFTOPPM` / `TROUBA_NODE` / `TROUBA_BAKE_CLI` convention).

**(b) Three controls, because "force" and "don't cache" are different things:**

- **`force`** on the bake call — skip reads, still **write** (next bake is warm). This is "I don't trust
  the cache right now".
- **`no-cache`** — neither read nor write. This is "I am measuring/testing and want a cold path".
- **purge** — a way to empty the cache dir (make target or `troubacore` subcommand).

**(c) Test isolation — the part that matters most.** e2e/CI must **not** share a cache across runs: either
default the test env to `no-cache`, or give each run its own dir. A warm cache making a test green is a
test passing for the wrong reason, and that is the single failure this repo's review standard exists to
catch. Say in the submission which you chose and show it holds.

## 4. Rules

- **Every input that can change a pixel must be in the key.** If you can name an input that isn't, the
  cache is wrong. Enumerate them in the submission.
- **Atomic writes.** Write to a temp file and `rename`. `baker.go` already has a known concurrency
  weakness around concurrent bakes of the same setlist; a cache adds shared mutable state to that path, and
  a torn cache entry is a corrupt page that persists. Two concurrent bakes hitting the same key must be
  safe.
- **File modes.** Cached rasters/overlays are user content. Dirs `0o700`, files `0o600`, matching what bake
  already does (`baker.go:259/304`) and what T107 fixed. Do not regress that.
- **Bound the growth** or provide the purge as the answer — say which. An unbounded cache on a LAN box
  that never gets cleaned is a slow disk-fill, not a feature.
- **No change to the bundle format or to `RasterHash`/`ContentHash` semantics.** This task adds a build
  cache; it must be invisible to the app and to R10.
- **Teeth-checks, reported per behaviour** — see acceptance.

## 5. Acceptance criteria

- A second identical bake reuses cached rasters and overlays; **measure and report cold vs warm
  wall-clock** for the same setlist.
- **A changed input misses.** Prove it in both directions: edit one annotation on one page → that page's
  overlay is re-rendered and *the others are not*; replace a song's PDF → that song's rasters re-render.
- **Byte-identical output.** A warm bake and a cold bake of the same inputs produce the same bundle —
  compare hashes, don't eyeball.
- **A stale-key mutation is caught:** drop one input from the key (e.g. dpi, or the ink version) and show a
  test reddens. A cache with no test proving it invalidates is the dangerous kind.
- `force` writes-but-doesn't-read; `no-cache` does neither; purge empties. Each demonstrated.
- Concurrent bakes of the same setlist stay correct with the cache on.
- Modes verified `0o700`/`0o600`. `gofmt -l core` clean; `go test ./...` green.

## 6. Out of scope

Studio's editor render path (not examined; a separate task if wanted). The app's `LruCache` — already
content-keyed, and its real open issue is **C5** (concurrent use against a documented single-thread
invariant), which is mobile-lane and unrelated. Changing the bundle format. Dirty-marking/invalidation
notification — deliberately rejected above.
