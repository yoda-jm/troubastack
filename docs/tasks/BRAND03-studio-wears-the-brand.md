# BRAND03 — TroubaStudio wears the brand

**Status:** **CODE LANDED** — 4 commit(s), `79581c74`…`2b1a8f8b` (last 2026-09-04). This line previously said "spec, not started"; it is a SECONDARY copy of a fact the review gate owns (`docs/handoff/reviews.md`), and it rotted. Corrected 2026-09-04 from the git history. **Not re-verified against this spec's own done-when** — "code landed" is what was checked, not "every criterion met".
**Asked by:** VLL — "utiliser le nouveau branding pour chaque partie […] le code couleur
c'est sûr mais aussi un petit lien vers la page dans chacun, les icônes."

## The finding that shapes this task

Studio's accent today is `--brand: #4f46e5`, an indigo that appears nowhere in the brand
palette. So the editor contradicts its own mark. The obvious fix — drop the brand's layer-2
pink in — **is an accessibility regression**, measured against Studio's real grounds:

| ink | on `--surface` #fffdfa | on `--bg` #f7f4ee | body text (4.5)? |
|---|---|---|---|
| `#4f46e5` — today's indigo | 6.19 | 5.73 | passes |
| `#E13198` — brand layer 2 | **4.06** | **3.76** | **fails both** |
| `#D62A8A` — the site's paper ink | 4.54 | **4.20** | fails on `--bg` |

Even the value the project page settled on does not hold on Studio's second ground. This is
the same trap as the brand's ACCENT table, which claimed one set of accents held on every
ground; it did not. **Do not reuse a number measured against a different surface.**

## The token plan — three roles, not one

A single accent cannot serve body text, large text and icons at once. Split it, and let the
role carry the constraint:

| token | light theme | dark theme | may be used for |
|---|---|---|---|
| `--brand` | `#D11E87` (4.87 / 4.50) | `#E23B9D` (4.50 / 4.88) | anything, **including body text** |
| `--brand-mark` | `#E13198` | `#E13198` | ≥18.66px text, icons, borders, non-text UI — clears 3:1 on all four grounds (3.76 / 4.06 / 4.29 / 4.65) |
| `--brand-tint` | derive from `--brand-mark` via `color-mix`, do not hand-pick | idem | fills, chips, hovers |

`--brand-mark` **is the mark's own colour, unmodified** — that is the point: the pixel next
to the icon matches the icon. `--brand` is that same hue and saturation, darkened (light) or
lightened (dark) only until it clears 4.5:1. Hue preserved at 324.9°.

**Re-measure before landing.** These numbers are computed against the four ground colours in
`styles.css` today. If a ground moves, the ink moves with it.

## Work

1. **Tokens.** Replace the four `--brand*` values in both the `:root` and the
   `prefers-color-scheme: dark` block. Nothing else in `styles.css` changes.
2. **Favicon — currently missing entirely.** `web/studio/index.html` declares no icon at
   all, so browsers show a blank sheet. Add `troubastudio-minimal.svg` from
   `docs/brand/dist/` (already committed), plus a PNG fallback, and a `theme-color` per
   scheme. Vite must copy it; do not inline a second copy of the mark into the repo.
3. **`<title>` and the tab.** Keep `TroubaStudio`. Add nothing marketing-shaped: this is a
   tool people keep open for hours.
4. **The link back.** One entry — *About TroubaStudio* — in the existing settings/overflow
   menu, opening `https://yoda-jm.github.io/troubastack/` in a new tab, with the version
   string beside it. **Not** in the editor chrome: the canvas is a working surface and a
   promotional link there is a misuse of the user's attention.
5. **A drift guard.** `web/studio/src` currently holds **55 distinct hex literals**, most
   outside the token system — which is how `#4f46e5` outlived the brand in the first place.
   Add a check that fails when a raw hex appears outside `styles.css`, and **teeth-check it
   by adding the literal a human would actually paste** (`#4f46e5` in a component) and
   confirming the build goes red. A guard that cannot fail guards nothing.

## Explicitly out of scope

**Annotation colours are user data, not chrome.** The highlight, pen and cue colours a
member picks are content: they travel in the bundle, they identify who marked what, and they
are matched by tests and by the baked render. Restyling them to the brand palette would
change what is already on people's charts. Touch only application chrome.

Also out of scope: the editor canvas rendering, the icon-glyph palette (`IconGlyphPalette`,
`icon.tsx`) which is annotation vocabulary, and anything under `web/ink`.

## Done when

- Both themes measured again from the shipped `styles.css`, with the numbers written into
  the commit message — not copied from this file.
- A favicon appears in the tab in both themes.
- `#4f46e5` returns zero hits in `web/studio/src`.
- The drift guard has been shown to fail on a pasted literal, then pass.
- The About link opens the live page.
