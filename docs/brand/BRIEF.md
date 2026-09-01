# TroubaStack icon family — design brief + generator prompts

Source of truth for the art direction: `ChatGPT Image Sep 1, 2026, 10_51_09 AM.png`.
Colours below are **sampled from that file**, not invented.


> **📋 Correction, 2026-09-01 (second pass).** Several rulings in this brief were
> later reversed by VLL and the palette was re-measured. What ships now:
> **four** marks, not three — TroubaCore joined, and **every** mark carries a
> chip, including TroubaStack, which this brief says has none. Chips take their
> layer's hue: Stage yellow, Studio pink, Core blue with the CPU circuit, Stack
> a three-bladed whirl on grey because it is the sum of the others. The MINIMAL
> variant is the stack plus ONE stroke across the full width in the mark's
> colour, not the stack plus the highlighter. And the highlighter's colour
> depends on its ground: the same ink reads `#FEE963` on a light tile and
> `#B2A541` on a dark one, so the figures below are only right for the ground
> they were sampled on. Read `README.md` for the shipped state; this file
> records what was asked for first.

---

## 1. What we are making

**One mark, three states.** The artwork is identical across all three; only the badge changes.

| Mark | Badge | Used for |
|---|---|---|
| **TroubaStack** | *none* | the whole project — website favicon, GitHub, docs |
| **TroubaStudio** | white circular chip, pencil | the web app (browser tab / PWA) |
| **TroubaStage** | orange circular chip, play triangle | the Android/iOS app icon |

The two products are **never on screen together**, so the badge alone is enough to
tell them apart. That is a deliberate decision, not an oversight.

**The names are one word.** `TroubaStack`, `TroubaStudio`, `TroubaStage` — no space.
This matches the code everywhere (`HomeScreen.kt` on `origin/main`).

**The monogram is "TS" for all three** — Stack, Studio and Stage all reduce to TS.
That is why it belongs in the shared artwork rather than in the badge.

---

## 2. Fixed elements (identical in all three)

1. **Tile** — squircle (iOS/Android adaptive corner radius), flat dark ground.
2. **Layer stack** — three rounded rectangles in perspective, offset up-left to
   down-right, in order top→bottom: warm yellow, magenta, blue. This is the
   product's actual data model: overlay layers stacked over a page.
3. **Staff + highlighter** — a music staff running lower-left to upper-right with
   a few noteheads, and a **translucent yellow highlighter stroke swiped across
   it**. This is the single most important element: freehand highlight over real
   notation is what the product *is*. If anything gets simplified, this survives.
4. **"TS" monogram** — bottom-left, light, partly overlapped by the highlighter
   stroke so it reads as part of the artwork rather than a sticker.

## 3. Variable element

Bottom-right **circular chip**, ~26% of tile width, sitting proud of the artwork:

- **TroubaStudio** — off-white fill, dark pencil glyph.
- **TroubaStage** — orange gradient fill, white play triangle.
- **TroubaStack** — **no chip at all.** The umbrella mark is the bare artwork.

## 4. Palette (sampled)

> **Correction, 2026-09-01.** An earlier pass recorded the highlighter as
> `#FFF9B3`. That was sampled from the bright *core* of the swipe, not its
> body — re-sampling the swipe area gives `#B8A020`–`#C0A828`. The stroke is
> a dark gold with a hot centre, which is why the first vector pass came out
> washed out. Corrected here, in README.md and in build.py.

| Role | Hex |
|---|---|
| Tile ground | `#202C37` |
| Layer 1 (top) | `#EAAD55` |
| Layer 2 (mid) | `#D131A7` |
| Layer 3 (bottom) | `#1563C7` |
| Highlighter body | `#B8A020` (dark gold) |
| Highlighter hot core | `#F8F8C8` |
| Studio chip fill | `#FBF7F3` |
| Stage chip gradient | `#D77F42` → `#AB5A28` |
| Wordmark accent (second word) | `#8E4620` |
| Wordmark base (first word) | `#000000` |
| Tagline grey | `#A7ACB5` |

## 5. Geometry

- Square canvas. Artwork inside a **safe area of 80%** — nothing important within
  10% of any edge (Android adaptive icons crop to a circle).
- Layer stack occupies the upper-left ~55%; staff crosses the middle band
  diagonally; monogram bottom-left; chip bottom-right.
- No drop shadows under the tile itself. Depth comes from the layer offsets.

## 6. Size ladder — this is a hard requirement

The same mark must be delivered in three levels of detail. **Test by rendering at
the stated pixel size and squinting; if an element disappears, it should not be
in that variant.**

| Variant | Target | Rules |
|---|---|---|
| **Full** | 512px+ (stores, docs, hero) | everything above |
| **Compact** | 96–192px (launcher) | staff reduced to 3 lines, max 3 noteheads, monogram kept, chip kept |
| **Minimal** | 16–48px (favicon, tab) | **layer stack + highlighter stroke only.** No staff lines, no noteheads, no monogram, no chip. Distinguish Studio/Stage — if needed at all — by the stroke colour, not by added detail. |

Thin hairlines and 1px staff rules must not appear in Compact or Minimal.

## 7. Wordmark lockup (for the website, not inside the app icon)

- `Trouba` in near-black + second word (`Stack` / `Studio` / `Stage`) in `#8E4620`,
  set solid with **no space** between the two words.
- Tagline in `#A7ACB5`, letterspaced small caps:
  - TroubaStudio — READ. HIGHLIGHT. ANNOTATE. CREATE.
  - TroubaStage — PRACTICE. PLAY. PERFORM.
  - TroubaStack — (umbrella; no tagline, or the project one-liner)
- **No text inside the app icon.** The wordmark is a separate asset.

## 8. Don'ts

- No space in the product names.
- No text or letterforms inside the icon other than the "TS" monogram.
- No realistic sheet-music engraving — the staff is a suggestion, not a score.
- No glossy bevels, no skeuomorphic paper curl, no drop shadow on the tile.
- Do not make the layer stack look like a stack of *files*; it is overlay layers.

## 9. Deliverables

- `troubastack.svg` / `troubastudio.svg` / `troubastage.svg` — Full
- Compact and Minimal variants of each
- PNG exports: 1024, 512, 192, 96, 48, 32, 16
- Android adaptive: separate foreground + background layers
- Wordmark lockups, horizontal, on light and dark grounds

---

## 10. Copy-paste prompts

### A. TroubaStage (app icon)

> A square app icon with rounded-squircle corners on a flat dark slate ground
> (#202C37). Upper-left: three rounded rectangles stacked in shallow perspective,
> offset diagonally, coloured top to bottom warm yellow #EAAD55, magenta #D131A7,
> blue #1563C7, flat with soft gradients and no outlines. Across the middle,
> running lower-left to upper-right, a simplified music staff with a few
> noteheads, and swiped over it a translucent pale-yellow highlighter stroke
> (body #B8A020, hot core #F8F8C8) with soft rounded ends — the highlighter is the
> hero element. Bottom-left, a light "TS" monogram in a clean geometric sans,
> partly overlapped by the highlighter stroke. Bottom-right, a circular chip with
> an orange gradient fill (#D77F42 to #AB5A28) containing a white play triangle.
> Keep all elements inside 80% of the canvas. Flat modern vector style, no drop
> shadows, no bevels, no text other than the TS monogram. Must stay legible at
> 96 pixels.

### B. TroubaStudio (web app icon)

> Identical to the above in every respect, except the bottom-right circular chip
> has an off-white fill (#FBF7F3) containing a dark pencil glyph instead of the
> orange play chip.

### C. TroubaStack (project / favicon)

> Identical to the above, but with **no circular chip at all** in the bottom-right
> — the artwork alone: dark squircle, three offset gradient layers, staff with
> highlighter swipe, TS monogram bottom-left. This is the umbrella mark for the
> whole project.

### D. Minimal variant (16–48px favicon) — generate separately for each

> An extremely simplified version of the same icon for use at 16 pixels: dark
> slate squircle (#202C37), three offset rounded rectangles in yellow #EAAD55,
> magenta #D131A7 and blue #1563C7, and one thick translucent pale-yellow
> highlighter stroke (#B8A020) swiped diagonally across them. No staff lines, no
> noteheads, no monogram, no chip, no text. Bold shapes only, readable at 16
> pixels.

---

## 11. Inside the app (separate from the icon work)

Echo the icon so the app and its icon feel like one thing — using motifs, not a
shrunken copy of the icon:

- **Layer chips** in the layers UI take the icon's three layer colours in order.
- **Highlighter tool** uses `#B8A020` as its default colour.
- **Empty states** (no concerts on device, no annotations yet) use the bare layer
  stack as the illustration.
- **Splash / Home header** may use the staff-with-highlighter band as a divider.

Do **not** put the full icon inside the app; a UI that repeats its own icon reads
as filler.
