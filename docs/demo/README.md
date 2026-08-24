# demo-concert bundle — the real-music demo bundle (no server needed)

One **genuinely baked** concert bundle of the seeded band's *"Sat @ The Anchor"* setlist
(four songs — copyright-safe original / public-domain / freely-licensed music: the original
**The Open Road**, the traditional **House of the Rising Sun**, **Amazing Grace**, and
**Greensleeves** (a real Mutopia voice+guitar edition, CC-BY-SA) — real lead sheets, guitar
tab and text charts with purpose-built annotation layers, see
[`../demo-charts`](../demo-charts/)), flattened by the real server-side bake pipeline
(invariants I8/I11) per [`../design/08-bundle-container.md`](../design/08-bundle-container.md).
Install the app (root README → "The mobile app"), share/push a file to the device,
**Import**, and perform it fully offline.

- **`demo-concert.tstage`** (**6 pages** — one default part per song: The Open
  Road → *Lead sheet* (2pp), House of the Rising Sun → *Guitar tab*, Amazing Grace → *Lead
  sheet*, Greensleeves → *Voice + guitar* (2pp)) — **PRIMARY: the band-wide bundle (P205)**:
  ONE artifact that serves the whole
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
>
> Regenerated 2026-08-10 (**B13 — annotation showcase v2**): every chart's annotations are
> now **anchored** (positions derived from `docs/demo-charts/*.anchors.json`, never eyeballed)
> and genuinely hand-drawn, covering all seven object types + five `icon` stamps (incl.
> `warning`). The Open Road lead sheet gained a page 2 (intro riff), so the band-wide bundle's
> default parts total **6 pages** (Open Road *Lead sheet* 2pp, House *Guitar tab*, Amazing
> Grace *Lead sheet*, Greensleeves *Voice + guitar* 2pp). Re-baked from that seed; the baked
> pages carry the `icon` stamps (rendered by the Go/ink icon path, glyphs.json).
>
> Re-baked 2026-08-16 (**B13 encoding fix**): `mkcharts`' `sectionLabel` drew without the
> cp1252 translator, so B13's two new House-tab labels ("Chorus — arpeggio variation",
> "Outro: … — fermata") baked in as `â€"` mojibake — a T16-class regression that reached this
> artifact. Fixed at the source, charts regenerated and the bundle re-baked. Guarded by
> `TestAnchorTextMatchesPDF`: every recorded anchor must appear verbatim in its rendered PDF,
> so the manifest and the page can never silently disagree again.
>
> Re-baked 2026-08-21 (**T86 core half — tempo/key/metre reach the bundle**): the bake now writes the
> **effective** tempo and key (setlist override, else the song's base) instead of override-only, and
> carries the new `meter` field. So the bundle's songs finally ship their metadata — The Open Road
> `♩=92 · 4/4 · G`, House of the Rising Sun `♩=72 · 6/8 · Bm` (its setlist key override), Amazing
> Grace `♩=72 · 3/4 · G`, Greensleeves `♩=90 · 3/4 · Am`. This is what makes the **A34 visual beat
> appear on demo content** (it drives off the song's tempo, which used to bake to 0), and gives A35
> its metre. Structure otherwise unchanged: 4 songs, 6 default-part pages, roster, all layers, all
> members' cues.
>
> **Not re-baked 2026-08-23 (T76 — chart auto-fit): the bundle is unaffected, on purpose.** T76 makes
> the server-side text-chart renderer (`chartpdf`) auto-fit the font size. The invariant that makes
> this bundle immune is about *bake time*, not display order: **a bake serves the stored bytes of the
> chosen file; the only path that re-renders through `chartpdf` during a bake is the D1 transpose**
> (`baker.go:336-348`, reached when an item sets `TransposeChords`), **and no demo item sets
> `TransposeChords`** (nor `KeyOverride`) — `cmd/seed/main.go` sets neither. So nothing in this bundle
> is re-rendered at bake, and T76 cannot change it, *whatever the display order*. (Note: "not the
> default part" would **not** be enough — D1 swaps the default out for the lowest-DisplayOrder
> *generated* chart and re-renders it, so a transposing item can bake a text chart; ours simply never
> transposes.) The seed's real T19 text charts do now auto-fit when first rendered, but that is at
> `POST /text-charts` time, not bake time. Recorded so nobody re-bakes a compliant artifact — and so
> the day someone adds a transposed item to the demo, they know that is the line that changes it.
>
> Re-baked 2026-08-24 (**T95 Stage B — amazing-grace converges onto chartpdf**): Amazing Grace's lead
> sheet used to be a hand-drawn `mkcharts` PDF that duplicated what the productized `chartpdf` renderer
> can already draw. It is now the chart-dialect source `docs/demo-charts/amazing-grace.chart` (its
> attribution is a real `{footnote}` block, T95 Stage A), and `mkcharts` **regenerates
> `amazing-grace.pdf` by rendering that source through `chartpdf`** — so its stored bytes changed and
> the bundle IS re-baked. (This is a change to the stored file itself, not the T76 bake-time
> re-render the note above rules out — that invariant still holds; the demo still transposes nothing.)
> Its anchor manifest is regenerated from the same `chartpdf.RenderWithAnchors`, so the demo highlights
> land on the new render identically (`TestAnchorTextMatchesPDF` + the ink test guard both green).
> Structure unchanged: 4 songs, 6 default-part pages.
