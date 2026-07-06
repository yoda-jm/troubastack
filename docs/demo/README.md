# demo-concert.tstage — a real-music demo bundle (no server needed)

`demo-concert.tstage` (~545 KB) is a **genuinely baked** concert bundle — the seeded
band's *"Sat @ The Anchor"* setlist (four songs: **Wonderwall**, **Hallelujah**,
**Black Hole Sun**, and the original **The Open Road** — a real lead sheet + guitar tab
with purpose-built annotation layers, see [`../demo-charts`](../demo-charts/)), flattened
by the real server-side bake pipeline (invariants I8/I11)
per [`../design/08-bundle-container.md`](../design/08-bundle-container.md). Install the
app (root README → "The mobile app"), share/push this file to the device, **Import**,
and perform it fully offline.

Each song bakes its **default shared-pool part** (e.g. Wonderwall → *Wonderwall — Score*)
and carries ~3 annotation layers, demonstrating the layer visibility rules the presenter
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

# 2. As marie (admin of "The Troubadours"), bake "Sat @ The Anchor" and download it:
#    log in  → POST /api/bands/{band}/setlists/{setlist}/bake
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
