# 12 — Annotation colour under the reading schemes

How a page that was **baked once** can be read under four different colour schemes, and what has to
happen to annotation ink so a colour that *means* something keeps meaning it.

Companion render: **`annotation-colour-matrix.svg`** — every user-accessible colour, four schemes,
before and after, each shown as the forms it is actually drawn in.

## 1. What the bake stores

The bake stores, per page:

- **one neutral raster** — black ink on white paper, no scheme variant;
- **one transparent PNG per layer** (`web/bake/src/render.ts`: *"a layer's objects on a page, as a
  transparent PNG"*).

There is **no per-scheme asset**, and there must never be one. Adding schemes to the bake would
multiply storage, force a re-bake on every palette change, and make the reading mode a server concern.

**Consequence worth knowing:** the Highlight preset's `blend: "multiply"`
(`web/studio/src/editor.ts`) is applied *inside the layer's own transparent canvas*. It therefore
**never multiplies with the page text** — only with other objects on the same layer. The highlighter
effect on the page comes from **opacity**, not from the blend. Model highlight legibility as plain
alpha compositing over the paper; multiply is a within-layer detail.

## 2. How the schemes work

A scheme is a 4×5 colour matrix applied **at draw time on the device**
(`StageColorMode.pageColorFilter()`), so switching costs one GPU pass, re-bakes nothing and stores
nothing.

| scheme | paper | printed ink | how |
|---|---|---|---|
| `NORMAL` | `#FFFFFF` | `#000000` | identity (`null` filter) |
| `WARM` | `#FFF5D1` | `#000000` | multiply only: `×1.00 / ×0.96 / ×0.82` |
| `NIGHT` | `#000000` | `#FFFFFF` | straight inversion |
| `AMBER` | `#000000` | `#FFBF73` | inversion + warm tint, folded into one matrix |

Two properties are load-bearing and must survive any change:

- **`WARM` has no offset term.** Since `0 × k = 0`, **black stays black** — it warms the *paper*
  without muddying the *ink*. Adding an offset to "improve" it would grey out every black note.
- **`AMBER` is one matrix, not two passes.** `G' = 191.25 − 0.75·G` already encodes invert-then-warm.

## 3. Why the naive path is wrong

`StageScreen.kt` passes the **same `colorFilter`** to the page raster *and* to every overlay
(lines 1087-1089, 1157-1159). Page pixels and annotation ink are not the same kind of thing:

- **printed text and handwriting are ink** — they should invert with the paper;
- **a red cue is a code** — red means the conductor's mandatory mark, and it must stay red.

Inverting everything scrambles the code in exactly the two schemes used on a dark stage, i.e. during
the performance. Measured on the real palette: Red `#e11d48` reads teal in Night; Emerald `#059669`
reads pink; Amber `#f59e0b` reads blue.

Worse, it is **self-contradictory**: cue glyphs are drawn as live UI (`parseCueColor`,
`StageScreen.kt:721` and `:992`) and are never filtered. In Night the same cue is a red glyph and teal
ink **on one screen**.

## 4. The rule

**Gate on chroma, not on "is it an overlay".**

**1 — Achromatic (`C* < 20` in CIE Lab): it is ink.** Apply the page matrix — it inverts with the
paper — then, if the result falls under 4.5:1 against the paper, lift its lightness until it clears.

> **Use Lab chroma, not HLS saturation.** The palette's own near-black `#111827` (Tailwind gray-900)
> is a *desaturated navy*: HLS saturation **0.39**, which an `S < 0.20` gate would classify as a colour
> code and refuse to invert — black handwriting would stay dark on a black page. Its Lab chroma is
> **11.5**, correctly ink. The threshold is robust rather than tuned: across the whole palette the next
> lowest chroma is Teal at **35.3**, so `C* = 20` sits in an empty band from 11.5 to 35.3.

**2 — Chromatic (`C* ≥ 20`): it is a code. Preserve hue and saturation; remap lightness only.**
On light grounds, leave it untouched — it was authored for white paper. On dark grounds, solve for the
lightness closest to the original satisfying **≥ 4.5:1 against the paper** *and* **ΔE ≥ 25 against the
printed ink**, so a mark never dissolves into the text it sits on.

**3 — Highlight fills are not text.** A highlighter is *meant* to be low-contrast against paper; it
works because the text reads *through* it. Judge it on **printed-text-through-band** contrast. On a
dark ground the band lightens *toward* the ink instead of away from it, so **reduce alpha from 0.55 to
0.30 on dark grounds** — that takes an amber highlight in `AMBER` from 3.34 to 7.06 without touching
its hue. Rotating the hue would score worse (6.15) *and* destroy the colour code.

**4 — Legibility depends on the form, not only the colour.** The same hue can be right for one form and
wrong for another, so the palette offered must be per **colour × form**:

- amber is an excellent highlighter (15.0:1 read-through) and a poor pen (1.79:1 on white);
- near-black is an excellent pen (21:1) and a mediocre highlight fill (4.41:1) — it *darkens* what it
  covers.

## 5. What the rule fixes, and what it does not

**Fixed — every dark-scheme cell passes** for all eleven user-accessible colours: strokes ≥ 4.5:1
against the paper, printed text through a highlight ≥ 7.3:1, and every colour keeps its identity.

**Not fixed, and deliberately so — the light schemes.** Clause 2 never touches light grounds, so the
weaknesses there are pre-existing palette choices, not filter behaviour:

| stroke on white paper | contrast |
|---|---|
| **Amber `#f59e0b`** | **2.1:1** — below the 3:1 non-text threshold |
| Amber-dark `#d97706`, Green `#16a34a`, Orange `#ea580c`, Teal `#0d9488`, Emerald `#059669` | 3.0–3.8:1 |
| Red `#e11d48`, Blue `#2563eb`, Violet `#7c3aed`, Pink `#db2777`, Near-black `#111827` | ≥ 4.3:1 |

**No colour matrix can fix this** — it is a palette decision. The remedies are to darken amber for
stroke use, or to restrict colours to the forms where they hold up (clause 4).

## 6. Where this lives

| concern | file |
|---|---|
| the four matrices | `app/shared/…/stage/StageColorMode.kt` |
| filter applied to raster **and** overlays | `app/shared/…/stage/StageScreen.kt` 1087-1089, 1157-1159 |
| cue glyph colour (live UI, unfiltered) | `app/shared/…/stage/CueGlyph.kt` — `parseCueColor` |
| drawing palette | `web/studio/src/editor.ts` — `COLOR_SWATCHES` |
| cue palette | `web/studio/src/pages/song-editor/MyCuesEditor.tsx` — `CUE_PALETTE` |
| per-layer transparent overlay PNGs | `web/bake/src/render.ts` |

Implementation is tracked in **[A64](../tasks/A64-night-mode-inverts-annotation-colours.md)**. The cue
glyph must take the same transform as the ink — that half is a defect, not a preference.
