# T172 — Most marks have no anchor, and the reason is the centre test

**Lane:** web-core (Go `chartpdf` + the Studio draw path). **Size:** M. **Filed:** 2026-09-12 by the architect,
out of the 13 pt migration.

## Why this exists

T145 anchors a mark to the *words* it was drawn on, so a re-layout moves the mark with its text. The 13 pt
migration was the first time that machinery was load-bearing on real data, and it held for **3 of 18** marks
on generated charts. The other 15 kept their coordinates while the words moved under them. Nothing flagged
them, because nothing is wrong enough to flag: an unanchored mark is not broken, it is just unattached.

Measured on the real library at the time of the migration:

```
generated charts:  18 marks —  3 anchored, 15 not
scanned PDFs:     392 marks —  0 anchored (correctly: there is no text run to anchor to)
```

So "T145 handles the re-flow" is a true statement about a sixth of the marks that meet a re-flow.

## The cause is specific, and it is not "those marks aren't about text"

`chartpdf.AnchorAt` picks the run **containing the mark's centre**:

> *"It picks the run under the mark's centre … ok is false when the mark is over no text run (e.g.
> whitespace) — the caller then keeps the raw coordinates and flags the mark as un-anchorable, never
> guesses."*

Refusing to guess is right. But "centre inside a run's box" excludes marks that are unmistakably *about* a
line without sitting on top of it:

| mark | centre lands | anchored today |
|---|---|---|
| circle around a word | inside the word | ✅ |
| underline beneath a line | below the run box | ❌ |
| margin bracket beside a section | in the margin | ❌ |
| arrow pointing at a line | in whitespace | ❌ |
| highlight spanning two lines | in the gap between them | ❌ |

Four of those five are ordinary rehearsal marks. The gap is not that they have no relationship to the text —
it is that **the relationship is adjacency, and adjacency is not recorded**.

## What to build

**R1 — anchor by NEAREST run, not containing run**, with the relationship recorded rather than flattened:
the run, and the mark's offset from it (above / below / left / right, in the run's own em units so it
survives a size change). A mark 4 mm under a run is 4 mm under that run at any type size.

**R2 — a span, when the mark covers several runs.** A bracket or a long highlight relates to runs N..M, not
to one. Record the first and last; re-projection places it across their new extent.

**R3 — keep the refusal.** A bounded search radius, and beyond it still `ok == false`. A mark in the middle
of a blank half-page genuinely has no referent, and inventing one is worse than leaving coordinates. The
radius is a number to measure on real charts, not to pick here.

**R4 — the existing 15 are not migrated by this.** `cmd/migrate-anchors` exists and re-anchors from a render;
whether to run it is a separate decision with the same shape as the 13 pt one — it moves marks, so it is
VLL's call with the count in front of him. **Do not fold a migration into this task.**

## Teeth

The vector that matters is an **underline**: a mark whose box sits entirely below its run. Today it anchors to
nothing; after R1 it must anchor to the line above it and re-project under that line at a different type size.
Teeth-check by reverting to the centre test and watching it go un-anchorable.

**Not started.**
