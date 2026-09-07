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

## ⟨D1⟩ VLL rules, 2026-09-08 — **wrap the line**, and warn in Studio

*"Ce serait top de passer à la ligne automatiquement avec le mot qui dépasse, ou alors juste le flagger en
rouge dans le rendu de Studio pour dire : attention."*

**This is better than what I proposed** (refuse, or fall back to one column), and it is better for a reason
I had dismissed too quickly. I argued that wrapping breaks chord-over-word alignment. That objection is
real but narrow: it applies only to a chord+lyric **pair**, and even there it is tractable. For a plain
text line it does not apply at all — and **the package already wraps to a width**:

```go
func footnoteLines(m *fpdf.Fpdf, tr func(string) string, text string, scale float64, colW float64) []string
```

used for footnote prose, and already column-aware since stage 2. So the mechanism exists; it was simply
never applied to the body.

**How to wrap a chord+lyric pair without losing the alignment.** The chord row is drawn in **Courier
(monospace) from the column's left edge**, so a character offset in that row *is* a fixed x offset. The
lyric is proportional. So: choose the wrap point as the last word boundary of the LYRIC that fits `colW` in
its own font, then split the CHORD row at that same **character index** and emit both continuations as a
new pair. The chords that belonged over the wrapped words travel with them, at the offset they already had
minus the characters left behind. Nothing about the authored spacing has to be re-interpreted.

**The red flag in Studio is worth having, and must NOT be the only remedy.** As an authoring aid it is
excellent — it tells the author, while they are editing, that this chart does not sit in two columns. But
the renderer must never print past the paper edge *regardless of whether anyone read the warning*: the PDF
is what reaches a music stand, and a warning that lives in the editor does not travel with it. So: wrap in
the renderer, warn in Studio.

**The coupling to design around, because it is the thing that will bite.** Wrapping changes the number of
drawn lines, which changes the body height, which feeds `fitsAt` and pagination. So the fit predicate must
measure the chart **after** wrapping at the candidate size — otherwise a chart that no longer overflows
sideways will overflow *downwards* instead, and the bug will look fixed while moving one axis over.

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
- **A wrapped chord+lyric pair keeps its chords over its words:** after wrapping, the chord run on the
  continuation line starts at the character offset the wrap left behind. Teeth: wrap the lyric without
  splitting the chord row and the assertion must fail.
- **Wrapping is measured by the fit predicate:** a chart whose lines wrap must not then overflow the page
  vertically. Teeth: measure before wrapping and the page count assertion must fail.
- The same chart in **one column** is unchanged — the fix must not shrink type that was fitting before.
- **Teeth:** a chart whose longest line already fits the column keeps the SAME size it gets today, so the
  new width constraint cannot quietly shrink every chart.
- The rule is drawn only when `cols > 1`, and the one-column golden is untouched.

## Done means

VLL puts `columns: 2` on a real chart and either gets two readable columns with a grey rule between them,
or a clear reason why that chart cannot have them. He never gets words printed past the edge of the paper.
