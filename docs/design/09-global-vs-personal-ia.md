# Global vs Personal — legibility scheme (PROPOSAL for VLL review, 2026-07-17)

**Status:** analysis + recommendations, nothing built. VLL: "in Details it's not easy
to see what is global and what relates to me — in the whole app it's complicated."

## The diagnosis

The product has exactly two audiences for any piece of state:
- **Band** — others see it or feel its consequences (metadata, file pool, shared/
  conductor layers, setlists, live mode, bakes, delete).
- **Mine** — only you see or feel it (my files, my cues, personal layers, visibility
  toggles, auto-update, "bake my parts").

The illegibility has ONE root cause: **shared is unmarked and personal is only
sometimes marked** ("My files", "My cues" — but personal layers, toggles, and stamps
carry no marker at the moment of use). The UI also interleaves the two audiences by
FEATURE instead of grouping by AUDIENCE (the Details stack alternates
global/global/personal/personal/global).

The "ambiguous middle" resolves with one rule: **classify by who sees the EFFECT of a
change, not who authored the content.**
- Layer visibility toggles → *Mine* (nobody else sees your toggle), even though the
  layers are Band content.
- Conductor layers → *Band* (everyone sees them; authorship is restricted — that's a
  permissions note, not a third category).
- Annotations/stamps → inherit the ACTIVE LAYER's zone. The real gap: that zone is
  **invisible at draw time** — you can't tell whether the stamp you're placing is
  private or band-visible until you look at the drawer.

## The scheme (three parts, one vocabulary)

1. **Vocabulary — two words, everywhere.** "Band" and "Mine" (UI copy: "Shared with
   the band" / "Just for you"). One icon each: group-silhouette vs person-silhouette.
   The `personal · mine` chip that ALREADY exists on layers in the drawer becomes THE
   component — one style, reused on every personal surface. Band surfaces get the
   group chip only where confusion is real (no chip noise on obviously-shared pages
   like Members).

2. **Details panel — group by audience** (the concrete pain point):
   - **Shared with the band** 👥: Metadata · Files (pool)
   - **Just for you** 👤: My files · My cues
   - **Admin** (unchanged, last): danger zone.
   Two labeled group headers, personal group visually quieter/accented — no feature
   moves, just grouping + headers.

3. **Zone-at-draw-time** (the invisible-boundary fix): the editor's always-visible
   "Drawing on: <layer>" indicator gains the zone chip — "Drawing on: My notes 👤" vs
   "Drawing on: Cues 👥 (conductor)". Every stroke/stamp then declares its audience at
   the moment it matters. (The drawer already shows zone chips; this surfaces them at
   the point of action.)

## Priority order (worst offenders first)

1. **T54 — Details regroup** (studio, S): part 2 above. Highest confusion-per-effort.
2. **T55 — zone-at-draw-time chip** (studio, S): part 3. Closes the only place where
   personal/band is decided invisibly.
3. **Chip/vocabulary sweep** (studio+app, S, rides 1+2): "Bake my parts" → mine-chip;
   app settings sheet marks Auto-update + layer toggles "just for you"; bundle picker
   already distinguishes ("…(Marie's parts)") — align its wording to "Mine".

## Options for VLL

- **(A) The full scheme** (1+2+3, then the sweep) — recommended; three small tasks.
- **(B) Details regroup only** — fixes the reported pain, leaves draw-time invisible.
- **(C) Chips-only, no regrouping** — lightest touch, weakest fix (the interleaving
  stays).

Pick a direction (or amend); the lanes get task specs from it.
