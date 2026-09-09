# T169 — A scanned band's bundle is twice the size it needs, and the waste is one channel

VLL: *"pourquoi le bake du concert est si gros ? c'est quoi qui prend de la place ? y'aurait possibilité de
le rendre plus petit (une compression très simple ?)"*

Everything below is **measured**, on his two real bundles. No estimates.

## Where the bytes are

A scanned band's bundle, latest revision:

```
158 page rasters   97.7 MB     ← 98% of it
 56 overlay PNGs    0.87 MB
 bundle.json        0.06 MB
```

Rasters are **1240×1754, 8-bit RGB PNG** — A4 at 150 dpi. The overlays are sparse RGBA and already cheap;
they are not the problem and must not be touched (they carry the annotation ink, which IS coloured).

## The finding: the pages are greyscale, and we store three channels of it

Scanned **all 158** pages, not a sample — max per-pixel channel spread, measured on thumbnails:

```
pages with real colour (spread > 32):  0 of 158
worst spreads across the whole bundle: 8, 6, 6, 6, 6, 5, 5, 5
```

At full resolution 6 of 10 sampled pages are **exactly** R=G=B; the rest deviate by at most 17/255, which is
chroma noise from the source scan's own JPEG, not content.

## What actually saves bytes — and what does not

Lossless, 12-page sample:

| | size | vs stored |
|---|---|---|
| as stored (RGB PNG) | 8.61 MB | — |
| **RGB, max zlib effort** | **8.63 MB** | **nothing — very slightly worse** |
| greyscale, max zlib | 4.51 MB | **1.91×** |
| greyscale + 256 palette | 4.84 MB | 1.78× |

**Turning the PNG compression up buys literally nothing.** The encoder is already doing its job; the entire
lossless win is dropping the two redundant channels. Anyone who reaches for "compress harder" first will
spend a day for 0%.

**≈100 MB → ≈52 MB, with no quality decision to make.**

## R1 — per-PAGE greyscale detection, and why it must be per page

Compute the max channel spread on a **thumbnail** (a coloured page is coloured at any scale — this scanned
158 pages in seconds). Below the threshold, encode 8-bit greyscale; above it, leave the page exactly as it is.

**It must be per page, not per bundle**, and his other band shows why. Generated charts (not scans) carry a
small ochre element near the page foot:

```
36 of 44 pages "coloured" — by 1671 pixels of #926b1f, 0.077% of the page, rows 1542–1580
```

So a generated-chart page is 99.92% grey and would be *correctly* excluded by the rule. That is the right
outcome and it costs nothing: that whole bundle is **5.5 MB**. **This task is a win for scanned bands only** —
do not chase the generated ones.

I tested the obvious alternatives for those pages and both fail: 256-colour palette was pixel-exact on only
**2 of 6** (antialiased text exceeds the palette), and greyscale would discard the ochre. Leave them alone.

## R2 — the consequence nobody will think of until it ships

The raster bytes are content-addressed, and Stage's cache key is `(imageRef, contentHash, size, scheme)`.
**Re-encoding changes every raster's hash**, so:

- every existing bundle's content hashes change on the next bake;
- **every device re-downloads every page once**, over whatever wifi it has;
- the bake's byte-reproducibility guarantee (`jumps.go` sorts hotspots so an identical re-bake is
  byte-identical) still holds *after* the change, but the goldens pinning current bytes will go red **by
  design** and must be re-blessed deliberately, not silently.

None of that is a reason not to do it. It is a reason to land it when he is **not** in a gig week, and to
say so in the note that ships it.

## Explicitly NOT in this task

**WebP.** Measured at 3.4× (q92) and 4.5× (q85) versus 1.9× for lossless greyscale — a real further win, and
a real trade: at q85 thin staff lines soften. That is a judgement about a score read on a stand at arm's
length, so it is **VLL's call, on his tablet, not a table in a spec**. Separate lot if he wants it.

Server-side storage is also out of scope but worth stating once: each revision is kept **twice** (the
expanded directory *and* its `.tstage` zip), and every revision is kept forever — his other band has 13. That
is a retention question, not a compression one.

## Sizing

Small and self-contained: one encode path in `web/bake`, a thumbnail probe, a threshold, and a golden
re-bless. **Not started.**
