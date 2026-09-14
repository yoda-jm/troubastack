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

- A mark under a **mid-block** line is *not* unanchored. Its centre falls inside a box — but **which** box is
  narrower than this task first said. **CORRECTED 2026-09-15 (Fable):** `sortAnchors` orders the manifest by
  (page, **Y0**, X0) and `AnchorAt` **breaks on the first** containing box, so the **upper** run gets first
  refusal. A mark whose centre lies in the *overlap* of two runs is therefore already assigned to the line
  **above** — the right owner for an underline. I wrote "claimed by the line below" on the lane's word without
  opening `AnchorAt`, which decides it, and which I had read when filing T172.
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

## 3b. The real population, and why it may be empty

Given the break-on-first-match above, ownership goes wrong only in one band. With the upper run's box
`[Y0₁, Y1₁]` and the lower run's `[Y0₂, Y1₂]` overlapping (`Y0₂ < Y1₁`):

- centre in `[Y0₁, Y1₁]` → **upper wins** — correct for an underline, no bug;
- centre in `(Y1₁, Y1₂]` → only the lower box contains it → **lower wins**.

So the disputed population is exactly *"below the upper run's box bottom, inside the next run's box"*. The
deciding boundary is **`Y1` of the upper run** — not the overlap, and not the gap tier.

**And that may make the population empty**, which is the open question. The run box is **29–55 % taller than
its type** (web-core: lyric-only 6.0, paired 5.0, chord row 5.0, against 11 pt = 3.88 mm), so there is slack
below the glyphs. If a typical underline lands inside that slack it is inside `[Y0₁, Y1₁]` and already
correct. A mark below `Y1₁` is a visible distance under the words and arguably *is* nearer the next line.

**Measure before designing.** Two numbers settle it: where the glyph ink sits within the recorded box (how
much slack is below the baseline), and where a real underline's centre falls relative to `Y1₁`. Until then
this task has no established population — twice already in this family the first premise was wrong, and both
times it was a fixture or an inference standing in for a measurement.

## 4b. A fourth rotation — the chord row (web-core, 2026-09-15), restated

web-core added a shape worth having and I am recording it corrected, because their sentence reads backwards
and the corrected version is the more useful one.

They wrote: *a chord row above a lyric line … where the upper one is the referent, the opposite of the
underline.* But an underline's referent is **also** the run above it, so that is not an opposite. **The real
difficulty is ambiguity, not direction.** In a chord chart the gap below a lyric line is flanked by a
**lyric above** and a **chord row below**, and a mark drawn there has two honest readings:

- it underlines the lyric — referent **above**;
- it brackets or circles the chord change — referent **below**.

So a chord chart is not a case where the naive rule points the wrong way; it is a case where **no purely
geometric preference can be right for both**, which is the stronger reason to rotate a candidate rule
through it. Their instinct was right and it is why this rotation earns its place.

**A lead, not a prescription.** The manifest `AnchorAt` reads carries geometry and text, **no run kind** —
so a tie-break cannot ask "is this a chord row?" without new plumbing. But `chart.go` renders with
*different leading constants* for different relationships (`leadChord` for a chord-only line, `leadPair` for
a chord+lyric pair), so **the gap sizes may already separate a within-pair gap from a between-block one**.
Measure that on real renders before concluding the kind has to be plumbed through — the geometry may already
encode the structure. If it does not, adding a kind to the manifest is a bigger change than this task and
comes back through the gate.

## 4c. Measured: the geometry already encodes the structure (web-core, 2026-09-15)

The lead in §4b was measured rather than argued, and it holds. Gaps between consecutive runs, **in
run-heights**, and **identical at 11, 13 and 16 pt** — the unit does its job, so a threshold written once
holds at every type size:

| gap | run-heights |
|---|---:|
| chord → its own lyric | **−0.140** (boxes overlap) |
| lyric → lyric | **−0.117** (boxes overlap) |
| lyric → next chord row | **+0.060** |
| section header → first run | **+0.083** |
| last lyric → next section | **+0.467 … +0.760** |
| title → first section | **+0.500** |

**No run kind needs plumbing through the manifest.** A tie-break can be written in terms of *which gap the
mark sits in*, and structure is legible from leading alone. That removes the biggest cost this task looked
like it had.

### There are THREE tiers, not two — and two thresholds, not one

The data sorts into three bands separated by real cliffs, and the prose around it must not collapse them:

- **tier 1, overlap:** −0.140 … −0.117 — within a chord+lyric pair, and between consecutive lyrics
- *cliff of 0.177*
- **tier 2, inside a block:** +0.060 … +0.083 — between pairs, and after a section header
- *cliff of 0.384*
- **tier 3, block seam:** +0.467 … +0.760

So "tight" versus "loose" is **two different splits** depending on which cliff is meant, and a rule that says
*"if the gap is loose, prefer X"* behaves completely differently at a ~0 threshold than at a ~0.25 one. State
the tier, never the adjective.

### What it does to the chord case: the dispute is between two PAIRS

web-core's measurement says something structurally useful: a chord row overlaps the lyric **below** it
(−0.140) while the gap *above* it is positive (+0.060). **A chord row belongs to the lyric under it.**

So the disputed mark does not sit between "a lyric and a chord row" — it sits between **two complete
chord+lyric units**, at the seam of a block. The two honest readings become symmetric: an underline attached
to the **bottom of the pair above**, or a bracket attached to the **top of the pair below**. That is a better
frame for the tie-break than the one §4b left it in, and it is still a judgement — geometry says which gap
you are in, never which reading the musician meant.

## 5. Explicitly out of scope

- **No migration.** Existing anchors keep their owner unless a separate, chosen decision moves them — same
  boundary T172 held.
- **Do not widen the radius.** This is an ownership question inside the overlap, not a reach question; the
  3.0 run-heights radius decided in T172 is untouched.
