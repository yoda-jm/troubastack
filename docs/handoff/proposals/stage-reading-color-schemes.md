# Proposal — Stage reading color schemes (extend A10)

**Status:** proposal, awaiting arch review · **Raised by:** Mobile (2026-08-21) ·
**Area:** `app/shared` — `StageColorMode.kt` (+ the one chrome toggle in `StageScreen.kt`) ·
**Relates to:** A10 (stage night mode) — this is purely additive to it.

A self-contained visual prototype (the four schemes on a real-looking lead sheet, plus the exact
transforms) is published for VLL/Fable to react to:
<https://claude.ai/code/artifact/f5dd4e4b-c5d0-4d22-a03a-e5c24d56f769>. This doc is the reviewable
description; the artifact is the visual aid.

## Motivation (VLL floated "color scheme")

A10 shipped exactly two modes and a binary toggle: **Normal** (paper as baked, white) and **Night**
(a straight RGB invert). That covers "lit" and "pitch black" and nothing between. Musicians read in
more conditions than that:

- a **long practice** under warm room light, where a stark white raster glares and the blue content
  is tiring;
- a **dim club** (Night already serves this);
- an **orchestra pit / blackout**, where even a white-on-black page is a small floodlight that
  **costs the player their dark-adapted vision** — the exact reason pit stand-lights, cockpit dials
  and astronomers' torches are amber.

Everything here reuses the seam A10 already built: one `ColorMatrix` per scheme in
`pageColorFilter()`, applied at draw time to the decoded page raster **and** its overlays. No bake
change, no re-encode, the bundle is untouched (I12). It is the cheapest possible feature that answers
a real gigging need.

## Proposed design

Grow `StageColorMode` from 2 entries to 4, each a draw-time `ColorMatrix` (Normal = `null`, as today):

| scheme | paper → ink | transform | when |
|---|---|---|---|
| **Normal** | white → black | `null` (unchanged) | lit stage, daylight practice |
| **Warm** | cream → warm dark | diagonal tint, blacks stay black: `R×1.0 · G×0.96 · B×0.82` (white → ≈`#FFF5D1`) | long practice; glare & blue-light comfort |
| **Night** | near-black → light | RGB invert `−1` diag `+255` (**exactly today's NIGHT**) | dark venue |
| **Amber night** | black → amber | invert **then** warm: `R'=255−R · G'=(255−G)×0.75 · B'=(255−B)×0.45` (black → ink ≈`#FFBF73`) | pit / blackout; preserves dark-adapted vision |

- Each scheme needs its matching `pagePlaceholder()` tint (N9) so a page-turn never flashes the wrong
  colour mid-slide — cream for Warm, near-black for Night/Amber.
- `next()` cycles `Normal → Warm → Night → Amber → Normal` on the existing single chrome button. The
  persisted preference already tolerates any enum value (`parse()` falls back to `NORMAL`), so old
  saved prefs and old bundles keep working.
- The A34 beat frame + centre count draw **over** the filter, so amber/aqua stay true — with one
  caveat in Amber night (below).

## Open questions for review

1. **The control.** Four schemes on a cycle is up to 3 taps to return. Keep the cycle (proposed for
   v1, matches A10), or move to a small popup menu like ⚙?
2. **Which ship in v1.** All four, or a subset — Warm is the most "nice-to-have", Amber the most novel
   and most asked-for. My lean: ship all four; they're one matrix each.
3. **Amber-night beat colour.** On an amber page the A34 amber downbeat may not read against amber
   ink. Leave it (aqua off-beats still pop), or swap the downbeat to a deeper red **only** in Amber
   night?
4. **Exact tints.** The values above are tuned on screen; I'd device-tune them on the tablet before
   locking, the same way the A34 amber/aqua were.

## Out of scope

- Per-scheme *overlay* colour correction (keeping a red cue "true" on an inverted page) — A10 already
  decided against it for v1 and this keeps that decision.
- Any bake/bundle change. This is a pure local reading preference, like fit/role.
- A user-defined custom tint. Ship the curated set first; a picker is a later question.
