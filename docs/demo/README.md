# demo-concert bundle — the real-music demo bundle (no server needed)

One **genuinely baked** concert bundle of the seeded band's *"Sat @ The Anchor"* setlist
(four songs: **Wonderwall**, **Hallelujah**, **Black Hole Sun**, and the original **The
Open Road** — a real lead sheet + guitar tab with purpose-built annotation layers, see
[`../demo-charts`](../demo-charts/)), flattened by the real server-side bake pipeline
(invariants I8/I11) per [`../design/08-bundle-container.md`](../design/08-bundle-container.md).
Install the app (root README → "The mobile app"), share/push the file to the device,
**Import**, and perform it fully offline.

- **`demo-concert.tstage`** (~556 KB, **12 pages** — Wonderwall 3, Hallelujah 4, Black
  Hole Sun 3, The Open Road 2) — **the band-wide bundle (P205)**: ONE artifact that
  serves the whole band. Each song bakes its **default shared-pool part** (e.g.
  Wonderwall → *Wonderwall — Score*) and the bundle carries, per the P205 model:
  - the **band roster** (Marie/admin, Leo/conductor, Sasha/member) for view-time
    identity resolution;
  - **every layer, owner-tagged** — shared/conductor layers (owner `""`) plus each
    member's **personal** layers (owner = that member's id);
  - **every member's song cues** as `member_cues` (keyed by member id) — Wonderwall
    carries all three members' cues, etc.

  Identity resolves at **view** time in the presenter (P205 Stage 3): a logged-in
  Connect session matching a roster member resolves automatically, otherwise a one-tap
  "Who are you?" picker (remembered per concert/device). The viewer then shows the
  shared/conductor layers **plus only your own** personal layers, and renders **your**
  cues from `member_cues`. Default visibility is captured at **bake** time (the studio
  bake dialog); this scripted bake leaves it absent, so the viewer computes defaults as
  before (`mandatory || role_tag` rules).

Each song carries its annotation layers, demonstrating the visibility rules the presenter
enforces:

- a **mandatory** conductor-cue layer — always composited, the viewer can't hide it;
- a **shared** markings layer — on by default, toggleable;
- a **per-part** layer tagged with a `roleTag` (e.g. `guitar`) — shown by default only
  for the matching viewer role;
- **personal** layers (owner-tagged) — shown only to their owner at view time.

## Provenance — produced by the real bake pipeline (P205 Stage 2)

This file is the exact output of the real pipeline (seed → `POST …/bake` → poppler page
rasters + `@troubastack/ink` overlays → `.tstage`), so its images are genuine studio-
parity renders **and** the packaging is the real thing. Reproduce it from a clean seed:

```sh
# 1. Fresh, deterministic seed (wipe the gitignored asset cache so the placeholder
#    PDFs regenerate from the current cmd/seed — the T16 assets-cache gotcha):
rm -rf core/troubadata core/cmd/seed/assets
make demo                     # boots a seeded core at :8080 (users marie/…/demo)
#    (or, without the SPA: run troubacore, then `cd core && go run ./cmd/seed`)

# 2. As marie (admin of "The Troubadours"), bake the ONE band-wide bundle for
#    "Sat @ The Anchor" and download it:
#      POST /api/auth/login              {"username":"marie","password":"demo"}
#      POST /api/bands/{band}/setlists/{setlist}/bake
#        → GET /api/bands/{band}/concerts/{concert}/bundle  > docs/demo/demo-concert.tstage
```

**Reproducibility caveat:** the bytes are identical **modulo `bakedAt`** (the bake
timestamp) **and the server-assigned `concertId`/`songId`/member UUIDs** — all minted
fresh per seed run. Everything else (page rasters, overlays, hashes, structure) is
deterministic.

The title raster renders *Wonderwall — Score* with a true `—` (the T16 seed-encoding fix
proving itself in the shipped artifact).

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
> Regenerated 2026-07-17 (B12/T50): briefly **two bundles** — a shared band bake plus a
> `-mine` variant carrying Marie's `scope=mine` personal cues + my-files parts.
>
> Regenerated 2026-07-18 (**P205 Stage 2**): back to **ONE bundle**, now the **band-wide
> bake** — it carries the roster, every layer owner-tagged, and **all** members' cues in
> `member_cues`, so a single artifact serves the whole band and the presenter filters to
> the viewer's identity at view time. The `-mine` variant is retired (identity now lives
> in the viewer, not in separate per-member bundles). `?scope=mine` stays available in the
> API until P205 Stage 3 ships in the app.
