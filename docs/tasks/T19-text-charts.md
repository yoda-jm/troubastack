# T19 — Text charts: write formatted song documents in Studio, bake them like PDFs

**Priority:** high product value · **Size:** M/L · **Area:** `core`, `web/studio`, `proto` (maybe)

## Context

Many songs never have a PDF — the band just wants typed lyrics/chords ("G  D / Pack a
little light…"). Today the only song content is uploaded PDFs. The Open Road seed work
proved the clean path: **core already renders text to PDF** (`core/cmd/seed/pdf.go` —
Helvetica/cp1252, em-dash-safe since T16, sections/styles). Productize that: a member
WRITES a chart in Studio, the server renders it to a PDF, and it enters the song's
shared pool as a generated file — after which **everything downstream already works**
(viewing, annotations, my-files ordering, bake, Stage) with zero changes.

**Design decisions (resolved):**
1. **Render-to-PDF at save time, downstream stays PDF-only.** The text source is
   stored + versioned server-side; each save regenerates the derived pool file
   (marked `generated: true`, same fileId — a new upload revision, not a new file).
   No new bundle/bake/Stage concepts — I8/I12 untouched.
2. **Format: a deliberately tiny chart dialect**, not full Markdown: `# Title`,
   `## Section` (Verse/Chorus…), blank-line paragraphs, `**bold**`, and **chord lines**
   (a line consisting of chord tokens renders monospace-bold above the following lyric
   line — the one piece of real chart intelligence). Document the dialect in the
   editor's help popover. Anything else renders literally. ASCII + latin-1 (cp1252
   renderer); reject other chars with a clear message rather than mojibake.
3. **Editor UX v1:** on the song page, "New text chart" → a two-pane card (plain
   `textarea` + rendered PDF preview via the existing file viewer, refresh on save).
   No realtime collab on the SOURCE (annotations stay the collab layer; the chart
   source is edited like a file — LWW on save with a "was changed by X, reload"
   guard).

## Changes

1. **Core**: extract the seed's PDF text renderer into an internal package
   (`internal/chartpdf` — move, don't copy; the seed becomes its first consumer);
   extend for the dialect above. Endpoints: `PUT/GET
   /api/bands/{b}/songs/{s}/files/{f}/chart-source` (member; store source, regenerate
   the derived PDF atomically) and `POST .../files:text-chart` to create one. Tests:
   dialect rendering (golden text-extraction via pdftotext), source round-trip,
   regeneration bumps the file revision.
2. **Studio**: the editor card + preview; `generated` files show a "text chart" badge
   and an Edit-source affordance instead of Replace-upload.
3. **e2e**: create a text chart → it appears in the pool → annotate over it → bake →
   the baked page contains the rendered text (assert via the bake's raster existing;
   pixel-level is covered by the pipeline's existing tests).

## Acceptance criteria

- Write a chart with title/sections/chords/bold in Studio → save → the pool shows the
  generated PDF; `pdftotext` on it extracts the content with correct punctuation
  (em-dash test included).
- Edit + save → same fileId, new revision; annotations on it survive (they're
  page-relative, unchanged pages keep them — document the "editing may shift layout
  under existing annotations" caveat honestly in the UI).
- Bake a setlist containing a text-chart song → performs on Stage like any PDF song.
- `make test` + e2e green; the seed still builds using the extracted package
  (`make fixtures`/demo unaffected or regenerated in the same PR).

## Out of scope

- Full Markdown, images, columns; realtime collab on chart source; transposition
  (chord-aware transpose is a fantastic FUTURE task the chord-line dialect enables —
  note it in the code); client-side rendering.

## As landed (`9058aa9`, reviewed 2026-07-07 — see reviews.md)

- **Renderer provenance:** `internal/chartpdf` is a fresh dialect renderer, not the
  spec'd "move" — the real text renderer had meanwhile diverged in `cmd/mkcharts`
  (the spec's `cmd/seed/pdf.go` attribution was itself imprecise; seed's pdf.go
  renders the placeholder *scores*). Deviation approved by VLL ("land now, defer");
  convergence is **T24** (attended — regenerates pixel-verified demo artifacts).
- **Preview pane (decision 3) deferred:** the landed editor is textarea-only with
  a format-help popover; the two-pane rendered preview is **T25**.
- **UI caveat missing (fix-forward assigned):** the acceptance criterion's honest
  "editing may shift layout under existing annotations" note is not in the editor —
  web-core adds it (XS) with the T22-gap fix-forward.
- **e2e narrower than spec'd, accepted:** `text-chart.spec.ts` covers create →
  badge → servable PDF → edit-in-place; the annotate→bake leg is covered by
  construction (a generated file is byte-identical to an uploaded pool PDF) and was
  verified live at the gate (real bake of a text-chart setlist; raster pixels show
  the rendered chart).
