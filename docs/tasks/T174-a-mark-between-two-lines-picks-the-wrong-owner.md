# T174 — A mark between two tight lyric lines anchors to the line below it

**Lane:** web-core · **Status:** specced, not started · **Origin:** web-core's fixture measurement during
T172, 2026-09-15, flagged rather than folded in — correctly.

## 1. The measurement that produced it

Building T172's type-size test, web-core replaced a hand-built manifest with **two real renders of one chart
source** (11 pt and 13 pt) and the test failed. Not a test bug:

```
consecutive lyric runs OVERLAP vertically by −0.12 run-heights
```

**There is no whitespace between the lines of a verse.** The synthetic fixture had invented a gap that real
charts do not have — which is also the reason this was invisible until someone measured instead of imagining.

## 2. What that implies, and what it does NOT

`AnchorAt` anchors to the run whose box contains the mark's **centre**. Because the boxes overlap:

- A mark under a **mid-block** line is *not* unanchored. Its centre falls inside the **next** line's box, so
  the existing centre test claims it — **for the line below**. It follows the wrong words through a reflow.
- A mark under a block's **last** line, or beside a section break, has no next line to fall into. That one is
  genuinely unanchored, and it is the case **T172 fixed**.

**T172's premise as I wrote it was over-general** (Fable): I said an underline fails because its centre lands
in whitespace below its run. That is true only at the bottom of a block. T172 remains correct for the
population that was measured; this task is the other half, and it was never in T172's scope.

## 3. The decision this needs

**Which run owns a mark whose centre falls in an overlap?** An underline belongs to the words **above** it —
that is what a musician drew. Today it goes to the line below.

This is a change to the **centre test itself**, which every existing anchor was created under. That makes it
a semantics change with a migration shadow, and it is exactly why it is not a T172 amendment: folding a
mark-mover into a mechanism change is how a migration nobody chose gets shipped (T172 R4).

## 4. The deciding observable — not the mechanism

**Reflow a chart so that two adjacent lyric lines separate, and see which line the underline tracks.**

That is the acceptance, and it is deliberately stated as a behaviour rather than a rule, because the
mechanism is the implementer's: a tie-break on overlap, a preference for the run whose box holds more of the
mark, or a rule on the mark's top edge rather than its centre. Each is defensible and each fails differently
on a strikethrough, a two-line highlight and a mark that genuinely straddles. **Rotate the candidate rule
through those three before choosing it.**

## 5. Explicitly out of scope

- **No migration.** Existing anchors keep their owner unless a separate, chosen decision moves them — same
  boundary T172 held.
- **Do not widen the radius.** This is an ownership question inside the overlap, not a reach question; the
  3.0 run-heights radius decided in T172 is untouched.
