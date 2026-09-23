# T177 — A line style for lines, rectangles and ellipses; and ends for a straight line

**Lane:** web-core · **Status:** specced, not started · **Origin:** VLL, 2026-09-23: *"un style de trait pour
au moins straight lines, elipses and rectangle, peut être que pour straight line je veux aussi un style de
terminaison et les côtés de terminaison."*

## 1. What exists, so nobody rebuilds it

`domain.Style` carries `Color, Opacity, Width, FontSize, Fill, Stroke, Blend` — **no dash, no cap, no end
decoration**. The only arrow in the tree is `web/ink`'s dev-only `"arrow"` **object type** (T07), registered
solely when `localStorage.devArrow` is set. Studio's jump tie draws its dashes and arrowhead in **CSS**
(`.jump-segment`, `.jump-arrow`), which is chrome, not an annotation — none of it survives a bake.

So this is real renderer work, and it lands in `web/ink`: the **one** stroke renderer, consumed by Studio's
dry canvas *and* the bake worker. **The bake is what a musician sees on stage**, so "it looks right in
Studio" is not the acceptance.

## 2. ⟨D1⟩ It is a STYLE on the existing types, never a new object type

Do not add an `arrow` type. An arrowhead is a property of a line, not a different thing: as a type, every
line already drawn can never become an arrow, the palette grows by one entry per combination, and every
mirror learns a new case. As a style, an existing line gains an end and the tool count is unchanged.

The dev-only T07 `"arrow"` type is prior art to **read and then not follow**.

## 3. ⟨D2⟩ Two additions to `Style`, both absent-means-today

- **`dash`** — the line pattern, applying to a `line`, and to the **border** of a `rect` and an `ellipse`
  (i.e. wherever `Stroke` is on). A small closed set, named: solid, dashed, dotted. Not an arbitrary
  dash-array: a musician picks a look, and an open numeric array is a migration surface and a rendering
  divergence waiting to differ between Studio and the baker.
- **`ends`** — straight **line only**: the decoration and which side carries it. Two facts, so two fields or
  one small record: *what* (none, arrow) and *where* (start, end, both).

**Absent must mean exactly today's drawing** — solid, no decoration — following `Fill`/`Stroke`'s existing
`*bool` "nil = infer" precedent. And per T172's ruling, prefer **structural presence** (an optional nested
value) over a sentinel: an in-band "0 = none" is the shape that bit us when a flat underline turned out to
be legitimately zero-height.

## 4. ⟨D3⟩ Ends are geometry, and geometry has to be decided once

An arrowhead is drawn, not overlaid, so its size and shape must be pinned in the renderer and expressed in
the **same units as the stroke** — it scales with `Width`, not with the page. State it once in `web/ink` and
let every consumer inherit it, because the failure mode here is an arrow that is right in the editor and
wrong in the bake, which nobody sees until a gig.

**Decide, and write down:** how the head scales with width, whether it is filled or stroked, and what
happens at the degenerate end of the range — a zero-length line, and a line shorter than its own arrowhead.
That last one is the case a musician creates by accident with a tap.

## 5. ⟨D4⟩ The mirrors, because this field family has been dropped three times

`Style` is copied field-by-field through `app/bandio_v2.go`, `sync/mapping.go` and
`httpapi/annotations.go`. The reflection guards on those seams have already caught `Anchor`,
`PointsRenderHash` and stylus `Pressure` being silently lost. **Extend the guards to the new fields, and —
if `ends` is a nested value — make them walk INTO it**, exactly as T172 required: a mirror that copies the
pointer and forgets a member is the same bug one level deeper, and a top-level check reports green.

## 6. Acceptance

- A dashed rectangle, a dotted ellipse and a dashed line **render identically in Studio and in a real bake**
  — compared on the baked raster, not on the editor canvas.
- An arrow line with the head at the start, at the end, and at both.
- An object saved before this task renders byte-for-byte as it does today.
- A line shorter than its arrowhead, and a zero-length line, both render without an exception and without a
  head bigger than the line.
- A round trip through the realtime wire, a `.tband` export and a bake preserves every new field — a mirror
  sabotage names the field.
