# A37 — Stage reading colour schemes (extend A10)

**Priority:** normal · **Size:** S/M · **Area:** `app/shared` — `StageColorMode.kt` (+ the one chrome
toggle in `StageScreen.kt`). Lane: Mobile.
**Approved to build** by Fable's ruling (`a2d90dd`, 2026-08-21) on
`docs/handoff/proposals/stage-reading-color-schemes.md`; this is that proposal written up as a numbered
task with her three rulings + two interactions folded in. **She reviews this task file before I build.**

## Context

A10 shipped exactly two Stage modes and a binary toggle: **Normal** (paper as baked) and **Night**
(RGB invert). VLL asked for more reading conditions. Musicians read under warm room light (glare/blue
fatigue), in a dim club (Night), and in a pit/blackout where even a white-on-black page is a small
floodlight that **costs dark-adapted vision** — the reason pit stand-lights, cockpit dials and
observatory torches are amber. Everything here reuses the A10 seam: one `ColorMatrix` per scheme in
`pageColorFilter()`, applied at draw time to the decoded raster **and** its overlays. No bake change,
no re-encode, bundle untouched (I12).

## Design (decided)

Grow `StageColorMode` from 2 entries to 4, each a draw-time `ColorMatrix` (Normal = `null`):

| scheme | paper → ink | transform | when |
|---|---|---|---|
| **Normal** | white → black | `null` | lit stage, daylight practice |
| **Warm** | cream → warm dark | diagonal tint, blacks stay black: `R×1.0 · G×0.96 · B×0.82` (white → ≈`#FFF5D1`) | long practice; glare / blue-light comfort |
| **Night** | near-black → light | RGB invert `−1` diag `+255` (today's NIGHT) | dark venue |
| **Amber night** | black → amber | invert then warm: `R'=255−R · G'=(255−G)×0.75 · B'=(255−B)×0.45` (black → ink ≈`#FFBF73`) | pit / blackout; preserves dark-adapted vision |

Each scheme needs its matching `pagePlaceholder()` tint (N9) so a page-turn never flashes the wrong
colour: cream for Warm, near-black for Night/Amber.

### Ruling 1 (Fable) — the cycle must never step from a dark scheme straight to white

A single chrome tap cycles the scheme. A wrap-around order (`… → Amber → Normal`) means one mistimed
tap in a pit blackout floods the player and everyone near them with a full-white page and destroys the
dark adaptation Amber night exists to protect. **Ping-pong instead of wrap:**

```
Normal → Warm → Night → Amber → Night → Warm → Normal → …
```

`next()` walks up to Amber then back down; it never jumps dark→white. (Implement as a
direction-aware step, not a modulo wrap.) The Parameters screen (A36) can still offer direct selection,
but the on-stage toggle must be safe by construction.

### Ruling 1b (Fable, 2026-08-23) — the ping-pong DIRECTION, all three edges settled

My earlier review left the cold-start case as "pick one and write it down". That was the wrong call:
it is a design decision with a right answer, and leaving it open blocked the lane. Ruling it now.

**The governing principle: the next tap must be predictable from what the performer can SEE.** On a
dark stage, mid-song, the only visible state is the scheme you are currently in. Any hidden state that
changes what the button does is a bug waiting for the worst possible moment.

1. **Cold start → direction resets to "up" (toward Amber). Do NOT persist the direction.** The
   *scheme* is a preference — where you are — and rightly persists. The *direction* is the momentary
   state of a walk — how you got here — and persisting it means your first tap after a restart depends
   on which way you happened to be walking at last night's gig. Invisible and unmemorable, therefore
   wrong. With the reset, the first tap after any restart is fully determined by the scheme on screen:
   it steps darker, unless you are already at the darkest, where rule 3 flips it.

2. **Direct selection from Parameters resets the direction to "up".** Same reason: picking a scheme
   directly is a fresh start, not a continuation of a walk. This is an acceptance criterion — set
   Amber, walk down to Night, pick Warm in Parameters, and the next on-stage tap must go to **Night**,
   not back to Normal.

3. **The endpoints flip rather than no-op.** A tap at Amber with direction "up", or at Normal with
   direction "down", reverses and steps. There is no state in which a tap does nothing.

**Keep the step pure.** `next(scheme, direction) -> (scheme, direction)` — a function of its arguments,
not a method mutating a hidden field. That is what makes all three rules above table-testable, and it
is the A34/T85 precedent: the beat's timeline is pure precisely so vectors can pin it. Table-test at
minimum: each scheme in each direction, both endpoints, and the reset paths from (1) and (2).

### Ruling 2 (Fable) — fix the amber-on-amber count, not the palette

The A34 border pulse (amber `#FFB02E` / aqua `#3EE0D4`) draws over the page and reads fine even on
Amber night (amber on near-black). **Leave the pulse alone.** The collision is the **A34 centre
count**: an amber numeral at 34% alpha on Amber-night's amber ink (`≈#FFBF73`) is illegible. Fix it in
**Amber night only** — tint the centre count with the off-beat (aqua) colour for every tier, or drop
its alpha further. **Do NOT swap the downbeat colour globally**: amber/aqua is a shared visual
contract with the studio beat, pinned across two runtimes; a scheme-local rendering choice must not
reach back into it.

### Interaction 1 (Fable) — the three-tier beat must be legible in all four schemes

A35 adds a tier-2 subdivision colour (a desaturated blue-grey at ~45%). On Amber-night's amber-ink
ground that is close to invisible — the tier carrying the metre's subdivisions would silently vanish
in the scheme most likely to be used in a pit. **Whoever lands second (A35 or A37) owns the check:**
the three-tier beat must be legible in all four schemes. If A37 lands after A35, this task verifies
tier-2 legibility per scheme and adjusts the *count/tier rendering* (never the shared contract) to fix
any that fail.

### Other constraints (Fable)

- **Resolve the persisted scheme before the first page raster and its placeholder are drawn** — a cold
  start straight into Night/Amber must not flash a white page first. (The scheme already persists via
  A10's `stage.colorMode`; the ordering is the requirement.)
- **Device-tune the tints at stage brightness before locking** — same rule as A34's amber/aqua. Do not
  lock values picked on a desktop display.
- The `ColorMatrix` values live in `pageColorFilter()`, **not** the A36 theme — they are performance
  decisions about a dark room, not brand decisions.

### The control

The existing one chrome button cycles per Ruling 1. Four schemes are also directly selectable in the
A36 Parameters screen (Stage → Colour mode) — extend that selector from Normal/Night to the four.

## Acceptance criteria

- Four `StageColorMode` entries, each a `ColorMatrix` in `pageColorFilter()` (Normal = `null`); matching
  `pagePlaceholder()` per scheme. No bake/bundle change (I12).
- `next()` ping-pongs (Normal↔Warm↔Night↔Amber↔…) and is unit-tested to **never** step from a dark
  scheme (Night/Amber) directly to Normal.
- Persisted scheme resolved before the first raster/placeholder — assert no white flash cold-starting
  into Night/Amber (or state how it's guaranteed by ordering).
- A34 centre count legible on Amber night (the Ruling-2 fix), with the shared amber/aqua contract
  untouched — assert the downbeat colour constant is unchanged.
- The A36 Parameters `Colour mode` selector offers all four.
- Device-verified at stage brightness: a page + a running beat in each of the four schemes; the
  tints tuned on-device, not desktop-picked.
- `:shared:check` + `:androidApp:assembleDebug` + iOS klibs green. No new dependencies.

## Out of scope

- Per-scheme *overlay* colour correction (keeping a red cue "true" on an inverted page) — A10 decided
  against it for v1; keep that.
- A user-defined custom tint / picker — ship the curated four first.
- The A35 three-tier colours themselves — those are A35's; A37 only owns the cross-scheme legibility
  check per Interaction 1 if it lands second.
