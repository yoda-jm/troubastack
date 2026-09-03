# BRAND09 — the Home should tell the two products apart, and a connected tile should look connected

**Lane:** mobile. **Size:** S–M. **Status:** spec, not started. **After the gig (2026-09-05)** — it is
app-binary, so the freeze covers it.
**Raised by:** VLL, 2026-09-03 — *"anything about having correct color in the home page that match the
branding nicely? (layer color Stage/Studio, their boxes, the grey colors of things that are activated
when connected…)"* — and by the mobile lane, which investigated it and asked for the missing task.

This implements a ruling already given at the gate; it does not reopen it.

## Two findings

**1. Every accent on Home is the same indigo.** Both product tiles use `colorScheme.primary`
(`#4F46E5`) — TroubaStage's heading, its border and the perform button, TroubaStudio's heading, and the
concert card's Resume button. The products do **not** wear distinct colours, on the one screen whose
job is to choose between them. Across all of `app/` the only brand accent is `#FF1769D1` in the
launcher drawable: **no brand accent reaches the UI.**

**2. A connected TroubaStudio tile still looks disabled.** Its background stays `surfaceVariant` —
the *same grey* as the disabled state and as "Nothing to update" / "Re-bake". So a user who **is**
connected cannot tell the tile became active. This is **not** [A55](A55-studio-tile-enablement.md),
which correctly greys it *when not connected* with a reason; this is the **connected** state failing to
look connected. Do not change A55's logic — it is right and VLL asked for it.

## The ruling this implements (already given, not up for renegotiation)

> **An accent that says "act here" is chrome and stays single. An accent that says "this is that
> product" is content and wears the product's colour.**

- The tile's **heading and mark** wear their product accent.
- **"Resume" is an action → it keeps the chrome accent** (indigo). Same for other act-here buttons.
- [A36](A36-app-theme-parity.md)'s "one brand hue" is **not superseded** — it was always about chrome,
  and it stays true there.

## ⚠ The measurement, against the app's real grounds

BRAND06's published figures (Stage 4.81, Studio 4.61) are **against pure white**, which the app does
not use. Measured against `TroubaTheme`'s actual grounds — light `background #F7F4EE` / `surface
#FFFDFA`, dark `background #100E16` / `surface #191722`:

| accent | on background | on surface |
|---|---|---|
| Stage light `#936B1F` | **4.38** ✘ small text | 4.74 ✔ |
| Studio light `#D62A8A` | **4.20** ✘ small text | 4.54 ✔ |
| Stage dark `#C8912A` | 6.88 ✔ | 6.35 ✔ |
| **Studio dark `#D62A8A`** | **4.16** ✘ | **3.84** ✘ small text on *both* |

**So the product accent may carry the mark, a border, an icon, or a heading** — large text is judged at
3:1 and all four clear it with room — **and must not carry small text on `--background`.** Studio's
accent is the strict case: it clears small text only on the light `surface`, nowhere in dark.

**Do not nudge the colour locally to pass** — that forks the accent and defeats having a brand table.
The two legitimate routes are: put accented text on `--surface`, or **give BRAND06 app-ground values**.

> **Dependency worth filing back:** BRAND06's accents were derived against a ground the product does
> not use, and this task is their first real consumer. Exactly the risk flagged when BRAND06 passed —
> *"a darker ground is the one thing that would break it"*. Raise it against BRAND06 rather than
> patching here.

## Work

1. **Add brand accents as theme-aware tokens**, resolving per product **and per ground** — the
   `ACCENT` table already has both columns. Hardcoding `Color(0xFFD62A8A)` at a call site throws away
   the light/dark adaptation `primary` gives for free today. Theme extension or a brand-colour object
   is the lane's call; no raw hex at a call site either way.
2. **Tile heading + mark wear the product accent**; borders and highlights may too.
3. **Act-here chrome stays indigo** — Resume, and any button whose meaning is "do this now".
4. **Make the connected state look connected.** The tile's background should read as active/branded
   when Connected, and the greyed state should be **derived from** that accent rather than being an
   unrelated `surfaceVariant`. Then grey ⇒ disabled is a reliable signal, which it is not today.
5. **Status dots** (`HomeScreen.kt:649-652`) are ad-hoc but online/offline is **semantic**, not
   identity — it is legitimate for them to stay outside the brand palette. Record that as a decision
   and just verify their contrast on both grounds.

## Do not

- **Do not put a product accent on small text over `--background`** (see the table).
- **Do not change A55's enablement logic**, the tile ordering, or the copy.
- **Do not let brand accents reach the Stage reading surfaces.** Those are a separate measured system
  — [12-annotation-colour](../design/12-annotation-colour.md), [A64](A64-night-mode-inverts-annotation-colours.md).

## Done when

- Stage and Studio are distinguishable at a glance, **in both themes**.
- A **connected** Studio tile is visibly different from a disabled one, and the disabled colour is
  derived from the enabled one.
- Every accent used for text measures **≥ 4.5:1** against the surface it actually sits on, and every
  accent used as heading/mark/border **≥ 3:1** — asserted in a unit test over the token table for
  **all four ground/theme combinations**, not eyeballed on one device. Include Studio-dark on
  `surface` (3.84) as a case that must be **rejected for small text**; a test that passes everything
  guards nothing.
- No raw brand hex at a call site.
- Device-checked in both themes; `:shared:testDebugUnitTest` green, count matched.

## Sequencing

**After the concert**, queued behind [A62](A62-scroll-mode-back-lands-at-the-song-start.md) (the freeze
exception), then [A63](A63-the-parameters-chips-say-what-they-are.md) and
[A64](A64-night-mode-inverts-annotation-colours.md).
