# Brand assets

One mark, **four products**. Everything here is **generated from bricks** — never
hand-edit anything in `dist/`, change a brick or the recipe and rebuild.

```
python3 docs/brand/build.py          # SVGs
python3 docs/brand/build.py --png    # + PNG ladder (needs rsvg-convert)
python3 docs/brand/sheet.py          # the family sheet, from the same bricks
python3 docs/brand/tools/make-placement.py   # the placement editor page
```

## The bricks (`src/`)

Each file is a bare SVG fragment — no `<svg>` wrapper, no `viewBox`, and it
leans on the gradients in `_defs`. That is what makes them composable, and it
also means **nothing can display one on its own**. SVG offers no way round that
from the other side: it has no include directive, XML external entities are
disabled by renderers, external `<use href="other.svg#id">` is blocked by many
of them, and the Android VectorDrawable conversion needs one self-contained
file. So `build.py` concatenates them in a defined order — and also writes
`dist/bricks/<name>.svg`, a **viewable standalone copy of every brick**, if you
want to look at one.

| Brick | What it is |
|---|---|
| `_defs.svg` | every colour in the family, as gradients. The only place paint is defined. |
| `_ink.svg` | the highlighter's gradients, keyed by `{{SFX}}` because SVG ids are document-global and the sheet holds many marks at once |
| `tile.svg` | the dark squircle ground (omitted for the Android adaptive foreground) |
| `layers.svg` | the three planes, from corners VLL placed himself in `src/plane-corners.json` |
| `stroke.svg` | ONE pass of the chisel marker; the highlighter is three instances of it |
| `staff-full` / `staff-compact` | the staff rules, 5 or 3, fanning and converging |
| `notes-full` / `notes-compact` | four croches sol-la-si-do, or two |
| `monogram.svg` | "TS" as paths — no font dependency |
| `chip-stack` / `chip-pencil` / `chip-play` / `chip-core` | the badge that names the product |

**Two things carry geometry rather than shapes.** `src/plane-corners.json` holds
the plane corners as an INPUT, not an output, which is why it lives in `src/`.
And the marker stroke's maths lives in `build.py` — `arc()`, `band()` and the
`STROKES` table — because the numbers ARE the design: endpoints, how much the
arc bows, half width, corner radius. A brick holding only the resulting path
data could not be re-aimed.

**Draw order is load-bearing and it is not the obvious one.** The staff rules
are drawn **after** the highlighter, not under it. The reference's swipe is
nearly opaque yet the rules cross it unbroken, so they are printed on top —
which is also what a real highlighter does: the ink goes over the paper and the
print shows through.

## The recipes

Four marks x three levels of detail.

| Mark | Chip | MINIMAL stroke |
|---|---|---|
| `troubastack` | whirl, grey ground — the sum of the three | #FEE36A + #E13198 + #2A8FE9 |
| `troubastudio` | pencil, layer 2 pink | #E13198 |
| `troubastage` | play, layer 1 yellow | #FCCC55 |
| `troubacore` | CPU circuit, layer 3 blue | #2A8FE9 |

- **full** — everything. 512px and up (renders 1024, 512).
- **compact** — 3 rules, 2 notes, heavier strokes. 192 and 96px.
- **minimal** — layer stack + ONE stroke across the full width. 48, 32, 16px.

The PNG filename carries the size but not the variant, so **no two variants may
share a size**: 192 was listed under both `full` and `compact`, and the compact
render silently overwrote the full one — 32 renders, 28 files. `--png` now
refuses to write the same path twice.

At MINIMAL there is no chip, so **the stroke's colour is the only thing telling
the marks apart** — TroubaStack's runs all three layer colours in equal bands,
because that mark is the sum of the others. That stroke uses a non-uniform
scale: stretching it uniformly to span the tile thickened it into a bar.

## Palette

Six colours, and every one of them is **derived from the paint the artwork
actually uses** — never restated. Three layers, the tile, the shared
highlighter gold, and one neutral for TroubaStack. Each mark takes its layer's
colour exactly rather than a near-neighbour, which is what collapsed the panel
from eleven swatches to six.

`sheet.py` refuses to build if a swatch names a colour nothing draws. That check
exists because the panel drifted twice: once silently, and once naming three
layer colours that no longer appeared in the gradients at all — under a caption
promising the sheet could not drift.

## Android adaptive

`*-adaptive-background.svg` is the flat ground; `*-adaptive-foreground.svg` is
the artwork fitted to the launcher's 66/108 safe circle. The scale used to be a
hand-typed `0.66`, which is not the 66/108 ratio (0.611) and in any case says
nothing about where the ink actually ends: measured, the art reached 403 units
from centre against a 312.9-unit safe radius, so round-masked launchers clipped
the monogram's S in half.

What bounds the art is its **minimal enclosing circle**, not its extent from the
middle of the canvas — the planes sit high and the monogram low, so measuring
from the canvas centre charges for empty space on the opposite side. That circle
is centred 98 units *below* centre with radius 522, against 611 measured the
naive way. So the foreground moves the art's own circle onto the canvas centre
before scaling (`FG_ART_CENTRE`, `FG_ART_RADIUS`), which buys **17% more mark**
inside the identical safe circle. `--png` re-measures every foreground and fails
the build if any ink escapes. Convert to `VectorDrawable` at integration time.

**No SVG `<filter>` anywhere.** Filters do not survive that conversion, which is
why the chip shadows, the plane shadows and the highlighter's texture are all
built from plain shapes and gradients.

## Known gaps

- **Wordmarks use live `<text>`.** Outline the type before the website ships,
  or the lockup drifts per machine.
- No `.ico` bundle yet; generate from the 16/32/48 PNGs when the site needs one.
- The reference staff sits at 19.7 degrees against our 14. Changing it touches
  three bricks at once, since the staff, the highlighter and the monogram share
  the angle.

## Reference

The ChatGPT exploration plates the design was measured against are **not
committed** — they are external sources, not our artefacts. Drop them into
`reference/` (newest last) if a measurement has to be redone; `sheet.py` will
then also emit `family-sheet-vs-reference.png`, and skips it otherwise.

`BRIEF.md` is the original written spec, with dated corrections where the
measurements contradicted it. Most of the palette in it has since been
re-measured off the plates rather than read off a first impression, so read
this file for the shipped state.
