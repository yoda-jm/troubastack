# A64 — Night/Amber invert annotation colours; a saturation-gated rule, measured

**Lane:** mobile. **Size:** M. **Status:** spec — rule proposed **and measured**, needs VLL's sign-off
on the thresholds. **After the gig.**
**Origin:** VLL asked *"how do the different colors work? if we bake sort of images how can we have
different colors?"*, then immediately found the hard part: *"un texte noir on veut l'inverser, peut-être
même le gris; pour le rouge, le vert c'est moins sûr; le orange c'est compliqué sur le amber. Essaye de
me trouver une règle et teste-la."* This spec is that rule and that test.

## How the colours work today (context — and this part is good)

The bake stores **one neutral raster per page** (black ink, white paper) plus the overlay PNGs, and
**no** per-scheme variant. The four schemes are 4×5 colour matrices applied at draw time
(`StageColorMode.pageColorFilter()`): switching costs nothing, re-bakes nothing, stores nothing.
`WARM` is purely multiplicative (`×1.00/×0.96/×0.82`), so since `0×k=0` **black stays black** — it warms
the paper without muddying the ink. `AMBER` folds inversion and warm tint into one matrix, so it is one
GPU pass. **None of this should be traded away.**

## The finding

`StageScreen.kt:1087-1089` and `1157-1159` pass the **same `colorFilter`** to the page raster **and to
every annotation overlay**. Annotation ink therefore inverts along with the paper, so a *semantic*
colour changes meaning with the reading light — in the two schemes used on a dark stage, i.e. **during
the performance**:

| | NORMAL | NIGHT today | AMBER today |
|---|---|---|---|
| conductor cue, red | `#E53935` | **`#1AC6CA`** cyan | **`#1A945B`** green |
| personal note, green | `#43A047` | **`#BC5FB8`** magenta | `#BC4753` dull red |
| stroke, orange | `#FB8C00` | **`#0473FF`** blue | `#045673`, contrast **2.6:1** |
| highlight, amber | `#FFB300` | `#004CFF`, contrast **3.5:1** | `#003973`, contrast **1.8:1** |

The last row is the worst: in AMBER an amber highlight sits at **1.8:1** against its paper — effectively
invisible. That is a measured defect, not a preference.

**And one half is objectively broken, whatever we decide about inversion:** cue glyphs are drawn as
**live UI** (`parseCueColor`, lines 721 and 992), which the filter never touches. In Night the *same
cue* is a red glyph and cyan ink on one screen. The app contradicting itself is not defensible.

## The rule

VLL's insight is that overlay ink is **not homogeneous**: handwriting is *ink* and must invert with the
page; a red cue is a *code* and must not. So the gate is **saturation**, not "is it an overlay".

**1 — Achromatic (HLS `S < 0.20`): it is ink. Apply the page matrix, then guarantee legibility.**
Black handwriting inverts exactly like printed text, which is what VLL asked for. Then enforce ≥4.5:1
against the paper — this is what rescues grey, his other instinct: grey in AMBER goes `#7F5F39`
**3.6:1 → `#946F42` 4.6:1**.

**2 — Chromatic (`S ≥ 0.20`): preserve hue and saturation; remap lightness only.**
On light grounds (NORMAL/WARM) leave it alone — the colour was authored for white paper. On dark
grounds, solve `L` for **contrast ≥ 4.5:1 vs paper** *and* **ΔE ≥ 25 vs the printed ink** (so the mark
never dissolves into the text it sits on), choosing the `L` closest to the original.

**3 — Highlight fills are not text and must not get a text threshold.**
A highlighter is *meant* to be low-contrast against paper — it works because the text reads *through*
it. The correct metric is **printed-text-vs-band** contrast. This is where VLL's "orange is complicated
on amber" is real: on a dark ground the band lightens *toward* the ink instead of away from it.

## Measured result

Rule 2 on dark grounds, every colour keeps its identity and every failure is fixed:

| calque | NIGHT today → rule | AMBER today → rule |
|---|---|---|
| cue red | `#1AC6CA` → `#E53834` (4.9:1) | `#1A945B` → `#E53834` (4.9:1) |
| note green | `#BC5FB8` → `#44A248` (6.5:1) | `#BC4753` 4.2 ✘ → `#44A248` (6.5:1) |
| stroke orange | `#0473FF` → `#FA8B00` (8.8:1) | `#045673` 2.6 ✘ → `#FA8B00` (8.8:1) |
| highlight amber | `#004CFF` 3.5 ✘ → `#FFB300` (11.7:1) | `#003973` 1.8 ✘ → `#FFB300` (11.7:1) |

**Five measured failures fixed, and red stays red in all four schemes.**

Rule 3, reading the printed text **through** the highlight band (composited over the real paper):

| | band | text/band contrast |
|---|---|---|
| NORMAL, α 0.55 | `#FFD573` | 15.04 OK |
| NIGHT, α 0.55 | `#8C6200` | 5.41 OK |
| **AMBER, α 0.55** | `#8C6200` | **3.34 — borderline** |
| **AMBER, α 0.30** | `#4C3600` | **7.06 OK** |
| AMBER, blue α 0.55 | `#10427F` | 6.15 OK, but the colour code is destroyed |

⇒ **On dark grounds, reduce highlight fill alpha from 0.55 to ~0.30.** It beats rotating the hue on
both counts: better contrast (7.06 vs 6.15) *and* amber stays amber. This is the whole of VLL's
"orange is complicated on amber", and it costs one scheme-dependent constant.

**Visual check:** `docs/design/annotation-colour-readability.svg`, generated from the real matrices —
four schemes × today/rule, with strokes and a highlight over printed text. Today's NIGHT column shows
the conductor's red circle as cyan and the personal note as magenta; the rule column keeps both.

## Open for VLL

- **Thresholds**: 4.5:1 (text) for marks, ΔE ≥ 25 vs printed ink, saturation gate 0.20. The gate is the
  one to sanity-check against real annotations — if people write in a desaturated brown, it will be
  treated as ink and inverted. Worth checking what saturations actually occur in the field.
- **Alpha 0.30 on dark grounds** — confirm it still *looks* like a highlight and not a smudge, on the
  device, in a dark room. The number says yes; the eye decides.

## Done when

- The rule is implemented as a per-overlay transform, and **the cue glyph uses the same transform** so
  glyph and ink can never disagree (this half is a defect, not a preference).
- On device, all four schemes: a red cue reads **red**, and its glyph matches its ink.
- The measured table above is reproduced by a unit test over the real matrices — thresholds asserted,
  not eyeballed. Include a **discriminating** vector: today's `#1AC6CA` must fail the test that
  `#E53834` passes, or the test guards nothing.
- `WARM`'s black-stays-black property is intact (assert black ink is still `#000000` in Warm).
- The bake is untouched — no per-scheme asset. This stays a device-side filter.
- `:shared:testDebugUnitTest` green; match the count.

## Sequencing

**After the concert**, with [A63](A63-the-parameters-chips-say-what-they-are.md). The schemes have
always behaved this way, so it is not a regression and does not earn the freeze exception that
[A62](A62-scroll-mode-back-lands-at-the-song-start.md) does.
