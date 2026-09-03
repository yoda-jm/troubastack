# BRAND09 — the app Home should tell the two products apart by colour

**Lane:** mobile. **Size:** S–M. **Status:** spec, not started. **After the gig (2026-09-05).**
**Raised by:** VLL, 2026-09-03: *"anything about having correct color in the home page that match the
branding nicely? (layer color Stage/Studio, their boxes, the grey colors of things that are activated
when connected…)"*

## The finding: both products render in the same colour, and the app owns no brand accent at all

`HomeScreen.kt:444` paints the TroubaStage heading with `MaterialTheme.colorScheme.primary` — the
generic theme accent. The TroubaStudio tile alongside it takes the same theme colours. **So the two
products are visually identical on the one screen whose job is to offer a choice between them.**

Checked across the whole app: the **only** brand accent anywhere under `app/` is `#FF1769D1`, inside
`ic_launcher_foreground.xml` (BRAND05's launcher icon). **No brand accent reaches the UI.** The status
dot colours (`HomeScreen.kt:649-652`) are hand-picked Material greens and ambers with no relation to
the brand either.

Meanwhile the brand has defined a per-product, **per-ground** accent all along (`docs/brand/build.py`
`ACCENT`), because a single hex cannot serve both grounds.

## ⚠ The trap — and it is exactly the bug BRAND07 was filed for

Measured against the app's **actual** Material surfaces (dark `#1C1B1F`, light `#FFFBFE`) — not the
brand sheet's reference grounds, which give different figures:

| product | dark accent | vs dark surface | light accent | vs light surface |
|---|---|---|---|---|
| TroubaStage | `#C8912A` | **6.15** ✔ text | `#936B1F` | **4.69** ✔ text |
| TroubaCore | `#3E89EA` | **4.87** ✔ text | `#1769D1` | **5.15** ✔ text |
| TroubaStack | `#AEBAC6` | **8.68** ✔ text | `#5A6674` | **5.71** ✔ text |
| **TroubaStudio** | `#D62A8A` | **3.72** ✘ | `#D62A8A` | **4.49** ✘ |

**TroubaStudio's accent clears 3:1 but not 4.5:1 on either ground.** It is a legitimate colour for a
**graphic** element — a rail, a chip fill, an icon tint, a border — and **must not carry body text or
a label**. Painting the "TroubaStudio" heading with it reproduces BRAND07 precisely, one screen over.

So the rule for this task: **Stage may wear its accent as text; Studio may only wear its accent as
shape.** If that asymmetry looks odd, the answer is to give *both* tiles a shape-based treatment so
they are consistent — not to promote Studio's accent to text.

## Work

**1. Introduce the brand accents as theme-aware tokens, never raw hexes at call sites.**
The reason `primary` is used today is that it adapts to light/dark automatically. Hardcoding
`Color(0xFFD62A8A)` throws that away. Add a small brand-accent holder that resolves per product **and
per ground** (the `ACCENT` table already has both columns), and read it the way `MaterialTheme` is
read. See [A36](A36-app-theme-parity.md) for the existing theme structure.

**2. Give each product tile its own accent, as shape.** The tile's identity should read at a glance
without reading the words. Keep the text at `onSurface`; put the colour in the tile's own furniture.

**3. Leave the disabled state alone in *behaviour*, fix it in *derivation*.**
[A55](A55-studio-tile-enablement.md) already settled that the Studio tile is `enabled = false` with a
reason when not Connected — **do not touch that logic**; VLL asked for it explicitly and it is right.
What this task changes is that the greyed state should be **derived** from the accent token
(theme-aware disabled alpha), so a connected tile lights up *into its product colour* and a
disconnected one falls back to neutral. That is the "grey things that activate when connected" VLL
described: today the grey and the active colour are unrelated values.

**4. Status colours: decide, then apply consistently.** `StatusOnline*/StatusOffline*` are ad-hoc.
Online/offline is **semantic**, not brand identity, so it is legitimate for them to stay outside the
brand palette — but that should be a recorded decision, not an accident. Keep them semantic, and only
verify their contrast on both grounds.

## Do not

- **Do not colour text with Studio's accent** (see the trap above).
- **Do not change A55's enablement logic**, or the tile ordering, or copy.
- **Do not add a per-product colour to the Stage performance surfaces.** Stage's reading schemes are
  a separate, measured system ([12-annotation-colour](../design/12-annotation-colour.md),
  [A64](A64-night-mode-inverts-annotation-colours.md)); brand accents must not leak into the page
  rendering path.

## Done when

- Stage and Studio are **distinguishable at a glance** on Home, in both light and dark.
- Every accent used for text measures **≥ 4.5:1** against the surface it sits on, and every accent used
  as a graphic element **≥ 3:1** — asserted in a unit test over the token table, both grounds, not
  eyeballed on one device. Include Studio's `#D62A8A` as the case that must be **rejected** for text;
  a test that passes everything guards nothing.
- The disabled Studio tile visibly relates to its enabled colour, and A55's behaviour is unchanged
  (still `enabled = false` with the reason, not merely an alpha change).
- No raw brand hex at a call site — all accents come from the token holder.
- Checked on device in both themes; `:shared:testDebugUnitTest` green, count matched.

## Sequencing

**After the concert.** Nothing here is a defect on the stand — it is identity and polish, and the app
is frozen until 2026-09-05. Queue it behind [A62](A62-scroll-mode-back-lands-at-the-song-start.md)
(the freeze exception), [A63](A63-the-parameters-chips-say-what-they-are.md) and
[A64](A64-night-mode-inverts-annotation-colours.md).
