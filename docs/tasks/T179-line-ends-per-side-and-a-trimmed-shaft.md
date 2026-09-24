# T179 — The shaft must stop where the terminator starts, and each end gets its own shape

**Lane:** web-core · **Status:** specced, not started · **Origin:** VLL, 2026-09-24, on T177 as shipped.
Supersedes T177's `ends` model; the dash work and the geometry constants stand.

## 1. The defect: the shaft runs into the head

`web/ink/src/index.ts` strokes the line **tip to tip** (`moveTo(ax,ay) … lineTo(bx,by)`, `:354`) and then fills
the heads over it (`:369`). VLL: *"le trait doit s'arrêter au début du terminateur."* Three visible
consequences, and the third is new with T177:

1. **Opacity.** `style.opacity` applies to annotations, so a shaft under a filled head shows through it as a
   darker spine.
2. **Cap.** A round cap at the tip can poke past the triangle's point and blunt it.
3. **Dash.** A dashed shaft can land a gap *inside* the head — a triangle with a hole in it. This is the
   combination he was trying when he found it.

**Fix:** trim the shaft to the head's **base** (`cx, cy` in `arrowHeads`, already computed) on any end that
carries a terminator, and leave it at the endpoint on an end that does not. The geometry to do it exists;
nothing new has to be measured.

## 2. ⟨D1⟩ One shape per end, and "none" is a shape

T177 models this as `Ends { Head, Side: start | end | both }` — one shape applied to one or both ends.
**Replace it with two independent ends**: a shape for the start, a shape for the end, each of which may be
*none*.

That is smaller *and* more expressive: the `Side` axis disappears entirely (an end with no terminator is an
end whose shape is none), and a line can carry a different terminator at each end, which the old model
cannot express at all.

**VLL's reasoning, which is the better argument and belongs in the code comment:** *"la façon naturelle de
dessiner un trait c'est de terminer vers là où sera la flèche"* — you draw **toward** the thing you are
pointing at, so the common case is a terminator on the end you finished at, and the rare case is reversing
the line. A UI asking "which end?" makes the reader answer a question the drawing gesture already answered.

## 3. ⟨D2⟩ The shape set: named, closed, extensible by one row

Start with **none · arrow · circle · square**, exactly as T177's dash set is named and closed rather than
parameterised. VLL: *"une forme pour chaque côté (flèche, rond, carré, …)"* — the "…" is the point: adding a
diamond later must be one entry in one table, not a new field.

**T177's refusal rule carries over unchanged and is the important half:** an **unrecognised** shape draws
**nothing**, never a substituted arrow. A wrong mark on a chart reads as an instruction nobody wrote.

## 4. ⟨D3⟩ Geometry, per shape, in stroke widths

`ARROW_HEAD_LEN_W = 3.2` / `ARROW_HEAD_HALF_W = 1.5` stay for the arrow. Each new shape needs its own
**length along the shaft** — because that length is what the shaft is trimmed by — and its own cross size,
both in stroke widths so they survive the editor/bake scale difference T177 already tests at 4 px and 12 px.

A circle and a square are symmetric about the endpoint in a way an arrow is not: decide, and write down,
whether the endpoint is the shape's **centre** or its **far edge**. They look different at a line's end and
only one of them keeps the drawn length honest.

## 5. Degenerate cases — T177's answers, re-checked per end

The budget split changes: with two *different* shapes the available length is no longer halved evenly.

- A line shorter than the sum of its two terminators: each keeps its shape and shrinks, and **the shaft may
  become zero-length** — which is legitimate and must not throw.
- A zero-length line still draws no terminator (no direction) and keeps its cap: the dot the user tapped.

## 6. Acceptance

- A dashed line with an arrow at one end: **no dash gap inside the head**, and the shaft visibly stops at
  its base.
- The same line at 40 % opacity: **no spine visible** through the head.
- An arrow at one end and a circle at the other, rendered identically in Studio and in a real bake.
- An unrecognised shape name draws the shaft and nothing else — asserted, not assumed.
- Every object saved before this task renders byte-for-byte as it does today (the T177 method: both workers
  on the pre-change fixture, with the positive control that the same worker differs on a changed one).
- The `Style` mirrors carry the two ends, and the guards walk **into** the nested value.
