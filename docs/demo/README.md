# demo-concert bundle — the real-music demo bundle (no server needed)

One **genuinely baked** concert bundle of the seeded band's *"Sat @ The Anchor"* setlist
(three songs — all copyright-safe original / public-domain music: the original **The Open
Road**, the traditional **House of the Rising Sun**, and **Amazing Grace** — real lead
sheets, guitar tab and text charts with purpose-built annotation layers, see
[`../demo-charts`](../demo-charts/)), flattened by the real server-side bake pipeline
(invariants I8/I11) per [`../design/08-bundle-container.md`](../design/08-bundle-container.md).
Install the app (root README → "The mobile app"), share/push a file to the device,
**Import**, and perform it fully offline.

- **`demo-concert.tstage`** (~356 KB, **3 pages** — one default part per song: The Open
  Road → *Lead sheet*, House of the Rising Sun → *Guitar tab*, Amazing Grace → *Lead
  sheet*) — **PRIMARY: the band-wide bundle (P205)**: ONE artifact that serves the whole
  band. Each song bakes its **default shared-pool part** (the lowest-DisplayOrder file in
  its pool) and the bundle carries, per the P205 model:
  - the **band roster** (Marie/admin, Leo/conductor, Sasha/member) for view-time
    identity resolution;
  - **every layer, owner-tagged** — shared/conductor layers (owner `""`) plus each
    member's **personal** layers (owner = that member's id);
  - **every member's song cues** as `member_cues` (keyed by member id) — The Open Road
    carries all three members' cues (Marie: mic + red electric; Leo: acoustic; Sasha:
    bass), etc.

  Identity resolves at **view** time in the presenter (P205 Stage 3): a logged-in
  Connect session matching a roster member resolves automatically, otherwise a one-tap
  "Who are you?" picker (remembered per concert/device). The viewer then shows the
  shared/conductor layers **plus only your own** personal layers, and renders **your**
  cues from `member_cues`.

Each song carries its annotation layers, demonstrating the visibility rules the presenter
enforces:

- a **mandatory** conductor-cue layer — always composited, the viewer can't hide it;
- a **shared** markings layer — on by default, toggleable;
- a **per-part** layer tagged with a `roleTag` (e.g. `guitar`) — shown by default only
  for the matching viewer role;
- **personal** layers (owner-tagged) — shown only to their owner at view time.

## Provenance — produced by the real bake pipeline (P205 Stage 2)

This file is the exact output of the real pipeline (seed → `POST …/bake` → poppler
page rasters + `@troubastack/ink` overlays → `.tstage`), so its images are genuine
studio-parity renders **and** the packaging is the real thing. Reproduce from a clean
seed:

```sh
# 1. Fresh, deterministic seed (wipe the gitignored asset cache so the placeholder
#    PDFs regenerate from the current cmd/seed — the T16 assets-cache gotcha):
rm -rf core/troubadata core/cmd/seed/assets
make demo                     # boots a seeded core at :8080 (users marie/…/demo)
#    (or, without the SPA: run troubacore, then `cd core && go run ./cmd/seed`)

# 2. As marie (admin of "The Troubadours"), log in and bake the ONE band-wide bundle
#    for "Sat @ The Anchor":
#      POST /api/auth/login              {"username":"marie","password":"demo"}
#      POST /api/bands/{band}/setlists/{setlist}/bake
#        → GET /api/bands/{band}/concerts/{concert}/bundle        > docs/demo/demo-concert.tstage
```

**Reproducibility caveat:** the bytes are identical **modulo `bakedAt`** (the bake
timestamp) **and the server-assigned `concertId`/`songId`/member UUIDs** — all minted
fresh per seed run. Everything else (page rasters, overlays, hashes, structure) is
deterministic.

The charts render true em-dashes — *House of the Rising Sun — Guitar*, the conductor cue
*"rit. — watch me"* on The Open Road — proving the T16 seed-encoding fix in the shipped
artifact.

> Historical note: this bundle was originally **hand-baked** (a single *Wonderwall —
> Vocals* part, pre-T16, with an `â€"` mojibake title). B05 retired the hand-bake; the
> demo became the real-baked multi-song concert, per the architect's decision in
> `docs/tasks/B05-regenerate-demo-bundle.md`.
>
> Regenerated 2026-07-07 after the seed page-doubling fix (`a014d75`): a footer was
> tripping fpdf's auto page-break, spilling a blank page after every real page — the
> bundle now carries the intended 12 pages.
>
> Regenerated 2026-07-12 (B10): the seed added a **text chart** (T19) to The Open Road's
> pool; the baked bundle keeps its annotated lead sheet as the default part.
>
> Regenerated 2026-07-17 (B12/T50): two bundles — a shared band bake plus a `-mine`
> variant carrying Marie's `scope=mine` personal cues + my-files parts.
>
> Regenerated 2026-07-18 (**P205 Stage 2**): `demo-concert.tstage` becomes the **band-wide
> bake** — it carries the roster, every layer owner-tagged, and **all** members' cues in
> `member_cues`, so a single artifact serves the whole band and the presenter filters to
> the viewer's identity at view time. A temporary `demo-concert-mine.tstage` bridge was
> retained for pre-Stage-3a app builds.
>
> Cleaned up 2026-07-19 (**P205 Stage 3a landed** — A29/A30): the `-mine` bridge is
> **deleted**; the app now reads `member_cues` + filters by identity at view time (the
> shared view-resolution vectors run in both the Go and the app commonTest, so print ==
> screen), so ONE band-wide bundle is enough. `?scope=mine` stays in the API for one
> overlap release, then retires.
>
> Regenerated 2026-08-04 (**DEMO-VID Part A**): the seed's band retired the copyrighted
> song *titles* (Wonderwall/Hallelujah/Black Hole Sun were only metadata over synthetic
> placeholder PDFs) for real, copyright-safe music — The Open Road (original), House of
> the Rising Sun and Amazing Grace (public domain) — with genuine charts and the
> redesigned annotation showcase. Re-baked from that seed: 3 songs, 3 pages.
