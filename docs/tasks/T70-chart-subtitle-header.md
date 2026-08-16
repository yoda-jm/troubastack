# T70 — Text-chart subtitle (artist) in the rendered header — made explicit, not guessed

**Priority:** normal (falls out of the 2026-08-16 local-band work: charts authored from
lyrics carry an artist line under the title) · **Size:** S · **Area:**
`core/internal/chartpdf`, the chart-dialect docs. **The implementation exists** uncommitted
in the primary worktree; this task lands it with a corrected rule, a regression test and
documentation.

## Context

`chartpdf` renders the text-chart dialect (T19) to a PDF. Today `# Title` is the only
header element. The uncommitted change adds an artist/subtitle line to the header and
tightens the top gap, so a chart reads like the demo lead sheets.

The mechanism matters more than the look: **this changes how charts users already have are
rendered.** The header is not new content — a line that used to render in the body is now
lifted out of it. If the rule guesses wrong, a lyric silently disappears from the chart a
musician is reading on stage. That is the failure mode to design against.

## The defect in the current rule (fix this, don't just test it)

`subtitleOf` finds `# Title`, then scans forward to the **first non-blank line, skipping
blanks**, and treats it as the subtitle when the following line is blank or a `##` section.
So this chart:

```
# My Song

Pack a little light for the road ahead,

## Verse 1
```

loses `Pack a little light…` from the body — it is promoted into the header as an "artist".
Skipping blanks is what breaks it: in this dialect a blank line after the title means the
body has started.

Verified against the current implementation (reviewer probe, 2026-08-16):

| chart | `subtitleOf` returns |
|---|---|
| `# My Song` / `The Artist` / `` / `## Verse 1` | `"The Artist"`, idx 1 — intended ✅ |
| `# My Song` / `` / `Pack a little light…` / `` / `## Verse 1` | **`"Pack a little light…"`, idx 2 — a body lyric, silently lifted into the header** ❌ |
| `# My Song` / `` / `## Verse 1` | `""`, idx -1 — correct (why the demo charts are unaffected) |

**Decided rule — adjacency:** a subtitle is the line at **exactly `titleIndex + 1`**, with
no blank line between it and the title, and only when it is not itself a section (`#`…) or a
chord row, and the line after it is blank / a `##` section / EOF. Anything else → no
subtitle, nothing moved. This still covers the intended authoring shape

```
# My Song
The Artist

## Verse 1
```

while making it impossible to swallow a body line that is separated from the title by a
blank. It is also the rule that is trivial to state in one sentence in the docs, which is
the real test of whether authors can predict it.

## Acceptance criteria

- `subtitleOf` requires adjacency as above. Table-driven unit tests covering: adjacent
  subtitle ✓; **blank line then a lyric → NO subtitle** (the regression case, red before
  the fix); adjacent line that is a chord row → none; adjacent `## Section` → none; title
  at EOF; no `#` title at all; subtitle followed by a chord row → none.
- **Body-preservation regression test (the guard that matters):** for every committed
  `docs/demo-charts/*.chart` fixture, render before/after and assert **no body line is
  lost** — i.e. every non-blank source line that is not the title or the recognized
  subtitle still appears in the extracted PDF text. This is a property, not a golden, so it
  keeps holding as fixtures change.
- The demo charts render **identically** to today apart from the intended top-gap tightening
  (they have a blank line after the title, so none gains a subtitle) — state this explicitly
  in the handoff with the extracted text of one before/after pair.
- Header geometry: with no subtitle the rule sits where it did; with one, the subtitle
  renders in the muted italic style and the rule and body start move down consistently
  (`header` returns the body top rather than callers hardcoding it).
- The subtitle is drawn through `tr(...)` like every other string — an artist name with an
  accent or an em-dash must not mojibake. (Same class as the B13 `sectionLabel` bug;
  `docs/demo/README.md` records that regression.)
- **Docs:** one paragraph in the chart-dialect documentation stating the rule in a sentence
  ("the line directly under `# Title`, with no blank between, is the artist") plus a
  two-chart example — one with a subtitle, one without.
- `gofmt -l core` clean; `go vet`; `make test` green; `chartpdf` tests pass.

## Out of scope

- Other metadata in the header (key/tempo/capo) — a separate ask if wanted; do not
  generalize the parser here.
- An explicit `artist:` metadata syntax (would be the alternative design; the adjacency rule
  keeps authoring plain and needs no dialect change).
- Restyling the demo charts or `mkcharts`.

## Sequencing

Fix the rule → unit tests (regression case red first) → body-preservation test over the
fixtures → docs. One commit.
