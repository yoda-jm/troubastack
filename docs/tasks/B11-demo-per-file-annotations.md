# B11 — Demo per-file annotations: show each part carrying its OWN notes

**Priority:** normal (resolves VLL's "annotations aren't smart" 2026-07-13 — a demo
gap, not a product bug) · **Size:** S · **Area:** `core/cmd/seed` + demo regen.

## Context

Per-file-per-member annotation scoping WORKS (ruled 2026-07-13: layers bind to
`Layer.FileID`; the viewer filters by `selectedFileId`). But the **demo never
showcases it**: `cmd/seed` attaches every annotation layer to `firstPDFFileID(...)`
(`main.go:496`, helper `:547`) — the song's FIRST PDF only. So Wonderwall's cues /
markings / chords all sit on **Score**, and Vocals/Guitar/Bass/Lyrics get **zero**
annotations. Every song looks "annotated on one file," which read to VLL as the
feature being absent/dumb. The fix is demo data, not code.

## Changes

1. **Spread demo annotations across parts** so switching file tabs visibly shows
   each part's OWN notes. Concretely, for a couple of multi-file songs:
   - **Wonderwall** (Score/Vocals/Guitar/Bass): put the **section form** on Score,
     **breath/phrase marks** on Vocals, **capo + chord shapes** on Guitar, a
     **rhythm cue** on Bass — each a small layer on its OWN `fileId`.
   - Optionally one more song (Hallelujah / The Open Road) with a 2-part split so
     more than one example exists.
   The seed already builds layers per song; extend the annotation builder to target
   a chosen file index per layer (not always `firstPDFFileID`), so a layer can be
   bound to the Vocals/Guitar/… file. Keep it deterministic + coordinate-safe (the
   existing per-song annotation coords are tuned to a layout — pick marks that read
   on each part, or reuse the generic placement).
2. **Regen the demo bundle** in the same pass (B05 protocol): a per-member bake of a
   member who curates those parts, or note that the shared bake still shows file[0]
   (one file per song by design — B07/B10). The primary win is the SEEDED APP showing
   per-file annotations when you switch tabs; the bundle showcase rides a per-member
   bake. Pixel-verify at least one part's distinct marks.

## Acceptance criteria

- `rm -rf core/troubadata && make demo`: on a multi-file song, switching file tabs
  shows DIFFERENT annotations per part (not all on Score, not empty) — screenshot a
  couple of tabs in the PR.
- Deterministic seed (no random placement); `make test` + `gofmt` green.

## Out of scope

- The annotation MODEL (unchanged — it already scopes per file); T40's product-side
  "Notes for: <file>" clarity label (separate, VLL's call); real copyrighted content
  (original/safe marks only).
