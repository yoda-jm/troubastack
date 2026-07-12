# B10 — Seed a real text-chart lyrics file into the demo (not only PDFs)

**Priority:** normal (VLL 2026-07-12: "maybe the demo should have a text lyrics
file? not only pdfs") · **Size:** XS/S · **Area:** `core/cmd/seed` + the demo
bundle regen. **Pairs with:** the demo regen already queued after T36/T37 — do
it in the SAME regen pass, not a separate bundle.

## Context

The seeder creates only PDFs today (even "The Open Road" is a committed PDF
chart, verified in `core/cmd/seed/main.go`). So the seeded app + the committed
`demo-concert.tstage` never exercise the **T19 text-chart** type at all — the
one feature where a band member writes lyrics/chords in the chartpdf dialect and
the server renders them to a baked PDF. VLL wants the demo to show it. This also
gives T37 (lyrics import) a natural demo target and makes the "New text chart"
button in T36's panel visibly meaningful.

## Changes

1. **Seed one text chart** on an existing seeded song (candidate: **The Open
   Road**, the original demo song — a real lyrics+chords lead sheet as a TEXT
   chart reads perfectly, and keeps us clear of third-party lyrics in a
   committed artifact; write original/public-domain-safe words). Use the
   existing API the seeder already speaks: `POST /api/bands/{bandId}/songs/
   {songId}/text-charts` with chartpdf-dialect source (see `chartpdf` + the T19
   design for the dialect). It returns a normal file in the pool — no new
   plumbing.
2. **Keep the chart source committed** (like the existing `docs/demo-charts/`
   PDFs) so the seed is reproducible: a `docs/demo-charts/open-road-lyrics.chart`
   (or inline in the seeder if tiny) — the source of truth for the rendered
   file.
3. **Regen the demo bundle** in the same pass (B05 protocol): the text chart
   bakes through the real pipeline like any PDF (chartpdf → PDF → raster), so
   the new `demo-concert.tstage` gains a text-chart-origin part. Pixel-verify the
   baked page renders the lyrics (crop check), and that the app performs it.

## Outcome (2026-07-12, architect-implemented per VLL)

Done: the seeder now renders `docs/demo-charts/open-road-lyrics.chart` (original,
copyright-safe words in the chart dialect) into The Open Road's pool via
`POST /text-charts`. Verified live: it renders as a proper chords-over-lyrics
chart (title, orange section headers, blue chords over monospace lyrics) and
sits in the pool beside the lead-sheet PDF, openable in the T19 editor.

**Honest scoping note (T05 precedent):** the shared `.tstage` bake is
**one file per song by design** (B07 — the lowest-displayOrder viewable PDF),
and The Open Road's default is its **annotated lead sheet** (the demo's
purpose-built annotation showcase — keeping it default is correct). So the text
chart shows in the **seeded app** (VLL's actual ask: "the demo should have a
text lyrics file") but NOT in the shared committed bundle, where demoting the
annotated lead sheet would be worse. A member who curates the chart first in
their my-files gets it in their per-member bake — the sanctioned on-Stage path.
The demo bundle was regenerated anyway (B05 hygiene — pipeline re-verified end
to end with the new seed).

## Acceptance criteria

- `rm -rf core/troubadata && make demo`: the target song shows a text-chart file
  in its pool (distinct origin from the PDFs), openable in the T19 editor.
- The regenerated `demo-concert.tstage` includes the text-chart part; a page of
  it renders the lyrics (pixel-checked in the PR); the app performs it offline.
- `make fixtures` stays zero-diff OR the fixture change is intentional and noted
  (the synthetic fixtures don't need the chart — scope to the demo bundle).

## Out of scope

- Any chartpdf dialect change; new file types; importing real copyrighted
  lyrics into the committed artifact (write safe original words — the point is
  to demo the TYPE, not ship someone's song text).
