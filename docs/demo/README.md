# demo-concert.tstage — a real-music demo bundle (no server needed)

`demo-concert.tstage` (~531 KB, **11 pages**) is a **genuinely baked** concert bundle —
**Marie's PERSONAL bake** (B07) of the seeded band's *"Sat @ The Anchor"* setlist (four
songs: **Wonderwall**, **Hallelujah**, **Black Hole Sun**, and the original **The Open
Road** — a real lead sheet + guitar tab with purpose-built annotation layers, see
[`../demo-charts`](../demo-charts/)), flattened by the real server-side bake pipeline
(invariants I8/I11) per [`../design/08-bundle-container.md`](../design/08-bundle-container.md).
Install the app (root README → "The mobile app"), share/push this file to the device,
**Import**, and perform it fully offline.

It is the **per-member** variant so it showcases two personal features end-to-end:
**T50 song cues** (each `BakedSong` carries Marie's icon+color cues — Wonderwall → mic +
red electric guitar, Hallelujah → mic, Black Hole Sun → mic + tambourine, The Open Road →
acoustic + mic) and her **"my files"** pick (Wonderwall resolves to her *Vocals* part, not
the full *Score* — hence 11 pages, not the shared bake's 12). The shared band bake carries
no cues (they are personal by design), so demonstrating cues in an offline bundle requires
the personal bake. Each song still carries its annotation layers, demonstrating the
visibility rules the presenter enforces:

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

# 2. As marie (admin of "The Troubadours"), bake HER PARTS for "Sat @ The Anchor"
#    (scope=mine → the per-member variant that carries her T50 cues + my-files):
#    log in  → POST /api/bands/{band}/setlists/{setlist}/bake?scope=mine
#            → GET  /api/bands/{band}/concerts/{concert}/bundle  > docs/demo/demo-concert.tstage
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
> Regenerated 2026-07-17 (B12/T50): switched to **Marie's personal bake** (`scope=mine`)
> so the bundle carries her **song cues** — the whole point of shipping cues is to see
> them offline, and only the per-member bake carries them. Trade-off (flagged for the
> architect): this shows Marie's *Vocals* Wonderwall part instead of the shared *Score*,
> so it's her one-person view rather than the neutral band bake. If the shared-bake
> annotation-layer showcase is preferred as THE demo bundle, we ship two bundles (shared
> + a `-mine` variant) — say the word.
