# T168 — in two columns the text runs off the page; and a rule between the columns

**Surface:** `core/internal/chartpdf`. **Lane:** core / web-core. **Kind:** bug (severe) + a small addition.
**Number claimed** in the same push as this file.

VLL tried `columns: 2` on a real chart (2026-09-07): *"the text overflow on the right of the page, and I
might want the vertical line to separate them (something grey probably)."*

## The overflow is structural, not a rounding slip — measured

A two-column render of an invented chart whose lyric lines are longer than half a page:

| | |
|---|---|
| runs spilling **out of the left column** (past x = 0.476) | **26** |
| runs spilling **off the paper** (past x = 0.943) | **14** |
| worst right edge reached | **2.085** — more than twice the page width |

## Why

`columns: 2` narrows the column to `(right − leftMargin − gutter) / 2` and threads that column's **left
edge** into the body primitives. It threads **no width**:

```go
func chordLine(pdf *fpdf.Fpdf, tr func(string) string, left, y float64, chords, annot, lyric string, …)
func textLine (pdf *fpdf.Fpdf, tr func(string) string, left, y float64, line string, …)
```

A chart's lyric line is drawn as authored — it is never wrapped, because wrapping would break the
chord-over-word alignment that is the whole point of the dialect. In ONE column that is safe: the author
wrote the line to fit a page-wide column. In TWO it is not: the same line now has half the width and simply
keeps going — over the gutter, through the right column, and off the paper.

And auto-fit does not save it. `autoFitBodyPt` walks the type size down until `fitsAt(...)` is true, and
that predicate is about **height and pagination**. Nothing in it asks whether the longest line still fits
the column it is being drawn into.

## The design question, because the obvious fix fights the feature

Two columns exist to **trade freed width for a LARGER type size**. But a narrower column needs a *smaller*
type for the same line to fit. For a chart with long lines those two pull in opposite directions, and a
naive "shrink until the widest line fits" can land on a size **smaller than the one-column render** — the
directive would then make the chart worse while appearing to work.

**Recommendation, and the precedent is in this package already.** A tab stave that cannot fit is
**refused** (`ErrTabTooWide`) rather than clipped, because a silently clipped stave is a lie. Do the same
here, in three steps:

1. **Measure width in the fit predicate.** `fitsAt` must also require that every drawn line fits `colW` at
   the candidate size. That alone removes the overflow.
2. **Compare honestly.** If the best two-column size is not larger than the one-column size for that chart,
   the directive has nothing to offer it.
3. **Say so rather than degrade.** Either refuse with a message naming the chart (the tab precedent), or
   render one column and report why. **Do not silently render two columns of tiny type**, and above all do
   not render text off the paper.

Whether step 3 refuses or falls back is **VLL's call** — refusing is safer for a gig sheet, falling back is
kinder to an author experimenting. Ask before choosing.

## The column rule (VLL's second ask)

A hairline down the gutter, centred between the columns, in a **muted grey** — the same weight as the
existing header/section rules rather than a new visual language. It runs the height of the body area, not
the whole page: it separates *columns of content*, so it should start where the body starts and stop where
it ends. Absent in one-column mode, obviously.

## ⟨R1⟩ Red first

- **No drawn run's right edge exceeds its column's right edge**, for a chart whose lines are longer than
  half a page. **Red today: 26 runs escape the left column and 14 leave the paper (worst 2.085 × page
  width).** Assert on the anchor manifest — it carries every run's box, so this is measurable without
  rasterising.
- The same chart in **one column** is unchanged — the fix must not shrink type that was fitting before.
- **Teeth:** a chart whose longest line already fits the column keeps the SAME size it gets today, so the
  new width constraint cannot quietly shrink every chart.
- The rule is drawn only when `cols > 1`, and the one-column golden is untouched.

## Done means

VLL puts `columns: 2` on a real chart and either gets two readable columns with a grey rule between them,
or a clear reason why that chart cannot have them. He never gets words printed past the edge of the paper.
