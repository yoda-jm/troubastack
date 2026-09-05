# T149 — In scroll mode, stop at the last glyph instead of scrolling through blank paper

**Lane:** first stage core (bake), then mobile. **Size:** S/M. **Status:** spec, 2026-09-05, from VLL.

## What VLL asked

*"en presentation (stage) de type scroll, peut etre qu'on peut s'arreter aux derniers glyphes au lieu
d'avoir une grande page blanche ? … c'est du rendering ou du baking ?"*

A chart that ends a third of the way down its last page makes the performer scroll through two thirds of
white paper before the next song. On a stand, mid-set, that is dead travel.

## It is BAKING, and specifically: the bake MEASURES, the app OBEYS

Two tempting answers are wrong, and both are worth stating so nobody re-proposes them.

**Not rendering.** The PDF page geometry *is* the coordinate system annotations are anchored to — a mark is
a fraction of a page. Shortening the last page rewrites every fraction on it, breaking existing marks and
fighting **T145** head-on. **T144**'s goldens pin that geometry precisely so it cannot move silently.

**Not cropping the raster either.** Same coordinate problem, plus a trap VLL's own data already contains:
one of his songs has a mark at **Y 0.328–0.424 on a page whose text ends at 0.051**. Cropping "to the last
glyph of text" would have **deleted a real annotation**.

**So:** the baker computes, per page, a `contentBottom` fraction = **max(ink bottom of the page raster, ink
bottom of every overlay on that page)** and writes it into `bundle.json`. Stage, **in `FitMode.SCROLL`
only**, stops drawing that page at `contentBottom`. No raster is altered, no coordinate changes meaning,
and a mark below the text keeps the page open far enough to show it.

Additive field, exactly like `bandId`/`bandName` (T143): **absent ⇒ full page**, so bundles already on a
device keep working unchanged.

## Required

- `contentBottom` per page in the bundle (proto + `bundle_gen` + the app's `BundleModel`).
- Computed from the **rasterised** page and its overlays — the same ink-extent scan, one source of truth.
- **Only the last page of a song is trimmed.** An intermediate page with a short tail keeps its full height:
  trimming mid-song would make the page metaphor lie and would fight two-up.
- **Leave a breathing margin** below the last glyph (a few percent) so the final line is not flush against
  the next song's title.
- `FIT_PAGE` and two-up are untouched.

## ⟨R1⟩ Red first

- A song whose last page is ~5% full is followed **immediately** by the next song in scroll — assert the
  composed scroll height, not a screenshot. Red today (full page height).
- **The mark-below-the-text case, which is the one that protects a musician:** a page whose text ends at
  0.05 with an overlay reaching 0.42 must still show the overlay. Build the fixture from the real shape
  measured on VLL's library. **Teeth-check by computing `contentBottom` from the text alone — the mark
  must disappear, and the test must go red.**
- An old bundle with no `contentBottom` renders the full page (the T143 "Unknown band" lesson: never drop
  or distort what predates the field).

## Out of scope

Trimming in paged/two-up modes, and reflowing songs to share a page. This removes blank travel; it does
not change layout.
