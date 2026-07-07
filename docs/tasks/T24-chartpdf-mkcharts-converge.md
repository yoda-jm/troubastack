# T24 — Converge `cmd/mkcharts` onto the extracted `internal/chartpdf` renderer

**Priority:** low (cleanup) · **Size:** S/M · **Area:** `core/cmd/mkcharts`, `core/internal/chartpdf`, `docs/demo-charts`, seed annotation anchors · **Attended:** yes (regenerates pixel-verified artifacts)

## Context

T19 productized the text→PDF renderer as `core/internal/chartpdf` (a dialect parser
+ `Render`/`Title`). Its spec change #1 asked to "**move, don't copy; the seed
becomes its first consumer**". In practice T19 landed `chartpdf` as a **fresh**
renderer rather than a move, because `cmd/mkcharts` (the dev tool that generates the
committed demo charts) had **diverged in layout** from the productized renderer:

- `chartpdf.sectionLabel` returns `y+8`; `mkcharts.sectionLabel` returns `y+7.5`.
- `chartpdf.header` takes only a title and rules at y=30; `mkcharts.header` takes
  `title, sub, meta`, draws a subtitle + "key • tempo • meter" line, and rules at y=43.
- `mkcharts` also renders things the dialect has no grammar for: a guitar-tab page,
  a blank-staff placeholder, and a footer.

So `newDoc` and `chordLine` are byte-identical duplicates, but `sectionLabel`/`header`
genuinely differ. Converging fully means either (a) expressing the demo lead sheets as
dialect strings through `chartpdf.Render` — which **regenerates `docs/demo-charts/*.pdf`
and shifts the layout under the seed's hand-placed Open Road annotation anchors**
(`core/cmd/seed/annotations.go` → `buildOpenRoadAnnotations`), the pixel-verified demo —
or (b) reconciling the layout constants, same regeneration risk.

Deferred from T19 per VLL (2026-07-07): land the product feature, converge separately.

## Changes

1. Decide the target: either export the shared primitives from `chartpdf` for
   `mkcharts` to consume (keeping mkcharts' richer header/tab/blank-chart on top), or
   drive the demo lead-sheet bodies through `chartpdf.Render` and keep only the
   non-dialect artifacts (tab page, blank chart) in mkcharts.
2. Delete the duplicated helpers from `mkcharts`.
3. **Regenerate `docs/demo-charts/*.pdf`**, re-check the seed's Open Road annotation
   coordinates against the new layout, regenerate `docs/demo/demo-concert.tstage`, and
   **re-verify by pixels** (the annotations must still land on their staves/blocks).

## Acceptance criteria

- No duplicated renderer helpers between `internal/chartpdf` and `cmd/mkcharts`.
- `go build ./...` + the chartpdf golden test still green.
- Demo charts regenerated; Open Road annotations verified on their anchors by pixels
  (composite the overlay layers, as in the demo-regen gate of 2026-07-07).

## Out of scope

- Any change to the T19 product surface (`Render`/`Title`, endpoints, Studio editor) —
  those are the shipped contract; this is a dev-tooling/demo-artifact cleanup only.
