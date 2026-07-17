# demo-concert bundles — real-music demo bundles (no server needed)

Two **genuinely baked** concert bundles of the seeded band's *"Sat @ The Anchor"* setlist
(four songs: **Wonderwall**, **Hallelujah**, **Black Hole Sun**, and the original **The
Open Road** — a real lead sheet + guitar tab with purpose-built annotation layers, see
[`../demo-charts`](../demo-charts/)), flattened by the real server-side bake pipeline
(invariants I8/I11) per [`../design/08-bundle-container.md`](../design/08-bundle-container.md).
Install the app (root README → "The mobile app"), share/push a file to the device,
**Import**, and perform it fully offline.

- **`demo-concert.tstage`** (~556 KB, **12 pages** — Wonderwall 3, Hallelujah 4, Black
  Hole Sun 3, The Open Road 2) — **PRIMARY**: the **shared band bake**. Each song bakes its
  **default shared-pool part** (e.g. Wonderwall → *Wonderwall — Score*), the neutral
  everyone-sees-the-same view. Best for showing the annotation-layer visibility rules.
- **`demo-concert-mine.tstage`** (~531 KB, **11 pages**) — VARIANT: **Marie's personal
  bake** (B07, `scope=mine`), showing the two per-member features end-to-end: **T50 song
  cues** (each `BakedSong` carries her icon+color cues — Wonderwall → mic + red electric
  guitar, Hallelujah → mic, Black Hole Sun → mic + tambourine, The Open Road → acoustic +
  mic) and her **"my files"** pick (Wonderwall resolves to her *Vocals* part, not the full
  *Score* — hence 11 pages). The shared bake carries no cues (they're personal by design),
  so seeing cues offline needs the personal bundle.

Each song carries its annotation layers, demonstrating the visibility rules the presenter
enforces:

- a **mandatory** conductor-cue layer — always composited, the viewer can't hide it;
- a **shared** markings layer — on by default, toggleable;
- a **per-part** layer tagged with a `roleTag` (e.g. `guitar`) — shown by default only
  for the matching viewer role.

## Provenance — produced by the real bake pipeline (B05)

This file is the exact output of the B02 pipeline (seed → `POST …/bake` → poppler page
rasters + `@troubastack/ink` overlays → `.tstage`), so its images are genuine studio-
parity renders **and** the packaging is the real thing. Reproduce it from a clean seed:

```sh
# 1. Fresh, deterministic seed (wipe the gitignored asset cache so the placeholder
#    PDFs regenerate from the current cmd/seed — the T16 assets-cache gotcha):
rm -rf core/troubadata core/cmd/seed/assets
make demo                     # boots a seeded core at :8080 (users marie/…/demo)
#    (or, without the SPA: run troubacore, then `cd core && go run ./cmd/seed`)

# 2. As marie (admin of "The Troubadours"), bake BOTH bundles for "Sat @ The Anchor"
#    and download each:
#    PRIMARY (shared band bake):
#      POST /api/bands/{band}/setlists/{setlist}/bake
#        → GET /api/bands/{band}/concerts/{concert}/bundle       > docs/demo/demo-concert.tstage
#    VARIANT (Marie's parts — carries her T50 cues + my-files):
#      POST /api/bands/{band}/setlists/{setlist}/bake?scope=mine
#        → GET /api/bands/{band}/concerts/{concert}~{user}/bundle > docs/demo/demo-concert-mine.tstage
```

**Reproducibility caveat:** the bytes are identical **modulo `bakedAt`** (the bake
timestamp) **and the server-assigned `concertId`/`songId` UUIDs** — both are minted fresh
per seed run. Everything else (page rasters, overlays, hashes, structure) is
deterministic.

The old em-dash mojibake is **gone**: the title raster now renders *Wonderwall — Score*
with a true `—` (the T16 seed-encoding fix proving itself in the shipped artifact).

> Historical note: this bundle was previously **hand-baked** (a single *Wonderwall —
> Vocals* part, pre-T16, with an `â€"` mojibake title). B05 retired the hand-bake — the
> demo is now the real-baked multi-song concert, per the architect's decision recorded in
> `docs/tasks/B05-regenerate-demo-bundle.md`.
>
> Regenerated 2026-07-07 after the seed page-doubling fix (`a014d75`): the placeholder
> generator's footer was tripping fpdf's auto page-break, spilling a blank page after
> every real page — the old bundle carried ~22 pages (blanks interleaved, some
> annotations sitting on the blanks). It now carries the intended 12.
>
> Regenerated 2026-07-12 (B10): the seed now adds a **text chart** (T19) to The Open
> Road's pool — a real chart-dialect lyrics file, so the *seeded app* shows the
> text-chart type, not only uploaded PDFs. The shared bundle here is unchanged in
> substance (the bake is one file per song; The Open Road keeps its annotated
> lead sheet as the default part — demoting it would drop the annotation showcase).
> The text chart is visible in the seeded app and rides a member's per-member bake.
>
> Regenerated 2026-07-17 (B12/T50): now **two bundles** (VLL's call) — the **shared band
> bake stays PRIMARY** (`demo-concert.tstage`, 12 pages, the neutral annotation-layer
> showcase), plus a **`-mine` variant** (`demo-concert-mine.tstage`, Marie's `scope=mine`
> personal bake, 11 pages) that carries her **T50 song cues** + her my-files parts — so
> the offline demo shows cues without changing what the primary bundle is.
