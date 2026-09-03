# A64 — Night/Amber invert annotation colours; a chroma-gated rule, measured

**Lane:** mobile — **dispatched 2026-09-03**. **Size:** M. **Status:** spec ready, rule **retained by
VLL** ("we will keep the new rule it is better") and verified against the real palette.
**Freeze lifted — takeable now.**
**FREEZE LIFTED — the concert was cancelled (VLL, 2026-09-03).** The pre-gig app freeze is over and this is takeable now. The sequencing note below is kept only where it still carries a real dependency; "after the gig" no longer applies to anything.


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
page; a red cue is a *code* and must not. So the gate is **how colourful the ink is**, not "is it an
overlay".

**1 — Achromatic (Lab chroma `C* < 20`): it is ink. Apply the page matrix, then guarantee legibility.**
Black handwriting inverts exactly like printed text, which is what VLL asked for. Then enforce ≥4.5:1
against the paper — this is what rescues grey, his other instinct: grey in AMBER goes `#7F5F39`
**3.6:1 → `#946F42` 4.6:1**.

> ⚠ **Use Lab chroma, NOT HLS saturation — this spec originally had it wrong.** Measured against the
> real palette, the app's own near-black swatch `#111827` (Tailwind gray-900) is a *desaturated navy*:
> HLS saturation **0.39**, so an `S < 0.20` gate classifies it as a colour code and **refuses to invert
> it** — black handwriting would stay dark on a black page, the exact opposite of the requirement. Its
> Lab chroma is **11.5**, correctly ink. HLS saturation is unstable at low lightness; chroma is not.
> The threshold is robust rather than tuned: across all eleven palette colours the next lowest chroma
> is Teal at **35.3**, so `C* = 20` sits in an empty band from 11.5 to 35.3.

**2 — Chromatic (`C* ≥ 20`): preserve hue and saturation; remap lightness only.**
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

## Clause 4 — the rule must be per **colour × form**, not per colour

Rendering each colour as the forms it is actually used in — stroke, highlight, circled word,
handwriting — showed the three clauses above are **not sufficient**. Legibility depends on the form as
much as the hue:

- **Amber is an excellent highlighter and a bad pen.** Read-through as a highlight: **15.0:1**. Used as
  a stroke on white paper: **1.79:1**.
- **Black is an excellent pen and a mediocre highlighter.** As a stroke: 21:1. As a highlight fill it
  *darkens* the text it covers: **4.41:1**.

**And the residual failures are on the LIGHT grounds, which clauses 1-3 deliberately never touch — so
they pre-date all of this:**

| stroke on light paper | NORMAL | WARM |
|---|---|---|
| orange | **2.37:1** | **2.26:1** |
| amber | **1.79:1** | **1.72:1** |

Both are under the 3:1 non-text threshold, in the **default** scheme — the one used in rehearsal. After
the rule, **every dark-scheme cell passes**; the weakest cell in the whole matrix is orange-on-white.

**This is a palette decision, not a filter decision, and no colour matrix can fix it.** For VLL:
darken orange for stroke use, or restrict colours to the forms where they are legible (amber →
highlight only, black/grey → never a highlight fill). Reference render:
`docs/design/annotation-colour-matrix.svg` (before | after, four schemes, four forms each).

## Verified against the real palette (not invented samples)

Re-measured over **every user-accessible colour** — `COLOR_SWATCHES` (`web/studio/src/editor.ts`, the
drawing swatches) plus `CUE_PALETTE` (`MyCuesEditor.tsx`), eleven distinct colours:

**On dark grounds the rule passes all eleven** — strokes ≥ 4.5:1 against the paper, printed text read
through a highlight ≥ 7.3:1, and every colour keeps its identity. Today, by contrast, Red `#e11d48`
reads teal in Night, Emerald `#059669` reads pink, and Amber `#f59e0b` reads blue.

**The only remaining failure is on light paper, and it is the palette's, not the filter's:**

| stroke on white | contrast |
|---|---|
| **Amber `#f59e0b`** | **2.1:1** — under the 3:1 non-text threshold |
| `#d97706`, `#16a34a`, `#ea580c`, `#0d9488`, `#059669` | 3.0–3.8:1 |
| `#e11d48`, `#2563eb`, `#7c3aed`, `#db2777`, `#111827` | ≥ 4.3:1 |

One more thing the code says, and it simplifies the model: the Highlight preset's `blend: "multiply"`
is applied **inside the layer's own transparent canvas** (`web/bake/src/render.ts` renders each layer
as a transparent PNG), so it never multiplies with the page text — only with objects on the same
layer. **Highlight legibility is plain alpha compositing**; do not model it as multiply.

Full transformation write-up: **[docs/design/12-annotation-colour.md](../design/12-annotation-colour.md)**.
Reference render: `docs/design/annotation-colour-matrix.svg` (all eleven colours × four schemes ×
before/after, each in four forms).

## Open for VLL

- **Thresholds**: 4.5:1 for marks, ΔE ≥ 25 vs printed ink. The chroma gate is now settled by evidence
  rather than taste (see the warning above), so it is no longer a question.
- ~~**Clause 4**~~ — **RULED by VLL, 2026-09-03: leave it as is.** Amber `#f59e0b` stays available as a
  stroke at 2.15:1 on white, and no darkened variant is added. The reasoning to respect if anyone
  reopens it: a second amber would put two near-identical yellows in one palette, and the colour is
  *chosen by a person who can see it on their own screen* — this is not text a reader must decode.
  **Do not "fix" this in implementation.** Clauses 1-3 are unaffected and still apply.
- **Alpha 0.30 on dark grounds** — confirm it still *looks* like a highlight and not a smudge, on the
  device, in a dark room. The number says yes; the eye decides.

## Done when

- The rule is implemented as a per-overlay transform, and **the cue glyph uses the same transform** so
  glyph and ink can never disagree (this half is a defect, not a preference).
- On device, all four schemes: a red cue reads **red**, and its glyph matches its ink.
- The measured tables are reproduced by a unit test over the real matrices **and the real palette**
  (`COLOR_SWATCHES` + `CUE_PALETTE`) — thresholds asserted, not eyeballed. Include a **discriminating**
  vector: today's Night rendering of `#e11d48` (teal) must FAIL the test that the rule's output passes,
  or the test guards nothing. Assert `#111827` classifies as **ink** — that is the case the first draft
  of this spec got wrong.
- `WARM`'s black-stays-black property is intact (assert black ink is still `#000000` in Warm).
- The bake is untouched — no per-scheme asset. This stays a device-side filter.
- `:shared:testDebugUnitTest` green; match the count.

## Sequencing

**After the concert**, with [A63](A63-the-parameters-chips-say-what-they-are.md). The schemes have
always behaved this way, so it is not a regression and does not earn the freeze exception that
[A62](A62-scroll-mode-back-lands-at-the-song-start.md) does.
