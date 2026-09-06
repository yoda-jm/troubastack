# A69 — the song drawer and settings sheet stay white in a dark venue

**Lane:** mobile. **Size:** S. **Status:** spec — **proposed by the mobile lane, routed for review before
implementation** (VLL asked for the spec + gate on 2026-09-06). Not started.

**Raised by:** the mobile-lane burst review, 2026-09-06. VLL: after I flagged that the drawer/settings stay
bright in Night/Amber, "spec it and run it through the gate."

## The gap (verified)

Stage's reading schemes (A10/A37: NORMAL · WARM · NIGHT · AMBER) recolour the **page** with a draw-time
`pageColorFilter()` — NIGHT inverts to near-black paper, AMBER to black-paper/amber-ink for a blackout. But
the **Compose chrome does not follow the scheme.** The whole Stage is wrapped in
`MaterialTheme(colorScheme = lightColorScheme())` (`MainActivity.kt:697`, a deliberate pin of the approved
M3 baseline), so:

- the **song drawer** (`SongDrawerSheet` → `ModalDrawerSheet(drawerContainerColor = colorScheme.surface)`,
  `StageScreen.kt`) is a **bright-white panel**, and
- the **⚙ settings sheet** (`SettingsSheet`, a `ModalBottomSheet`) likewise,

**in every scheme, including NIGHT and AMBER.** So a musician who has darkened the page for a black stage
and then taps the drawer to jump a song — or opens settings mid-set — gets a **face-full of white light**.

**This is the exact failure A37 already legislated against, one surface short.** `StageColorMode.kt:13-16`
designed the ping-pong cycle so *"a mistimed on-stage tap in a pit blackout can't flood the player with a
full-white page."* The page can't flood white anymore; the drawer and the settings sheet still can. A69 is
finishing A37's own intent on the two chrome surfaces it never reached.

## The design: a scheme-aware surface for the two sheets, nothing more

A **pure** scheme→chrome-colour mapping, consumed only by the drawer and the settings sheet — NOT a global
theme swap. The M3 light baseline stays exactly as designed everywhere else; the deviation is scoped to two
opaque surfaces, and only where the scheme is dark.

```kotlin
// StageColorMode.kt — sibling to pageColorFilter()/pagePlaceholder(), same "performance decision, not
// brand" rationale. Pure, so it is unit-testable (the A34/A37 precedent for every Stage decision).
data class ChromeColors(val surface: Color, val onSurface: Color, val outline: Color)
fun StageColorMode.chromeColors(): ChromeColors = when (this) { … }
```

- **NIGHT / AMBER** → a **dark** surface. The palette already exists: `pagePlaceholder()` uses `#1A1A1A`
  (night) and `#1A1710` (amber, faintly warm); reuse that character so the drawer reads as the same world
  as the page it slid over. `onSurface` a light ink at **≥ 4.5:1** contrast; AMBER leans warm to preserve
  dark-adapted vision (don't drop a cool-white list onto an amber page).
- **WARM** → open (see below). **NORMAL** → the current light baseline, unchanged.

Drawer and settings then read `mode.chromeColors()` for their container / text / divider colours. Section
headers, the scrollbar (A60), the mode cycle + chrono controls in settings, and the running-order numbers
all need to pass contrast on the new ground — audit each, don't just swap the container.

**Bonus alignment — it also resolves a latent coupling.** The burst review noted `cueTint` computes the cue
glyph colour against the *scheme's* dark paper, while today's drawer is light — a mismatch that stays legible
only by luck of the current mid-tone palette. A dark Night/Amber drawer makes the cue tints sit on the
ground they were computed for, so the drawer cue chips become correct-by-construction, not
legible-by-coincidence.

## The tradeoff for the reviewer to weigh

The light M3 baseline (`MainActivity.kt:694` comment) was *"designed and approved against"* — so the
question is **scoped per-surface override vs. a fuller night MaterialTheme.** I recommend the scoped
override (two surfaces, dark schemes only): it fixes the blackout flood with no risk to the approved look
anywhere else, and it keeps the colour values as *performance* decisions living beside the other Stage
colour code, not as brand-theme changes. If Fable prefers a proper dark `ColorScheme` wrapper on the two
sheets instead, that is a fine alternative — same user outcome; I'll build whichever is ruled.

## Open for a ruling

- **WARM** is a comfort scheme, not a blackout one (`isDark == false`). Does the drawer follow it to a cream
  surface to match the page, or stay the light baseline? I lean cream-to-match, but it carries no
  night-vision risk, so it is the low-stakes half — NIGHT/AMBER are the part that matters.
- **The top bar / FABs / clock overlay** already draw on a translucent **dark** scrim (e.g. the clock at
  `StageScreen.kt:776`), so they don't flood — confirm during the work and leave them alone if so. Scope is
  the two opaque **sheets**.

## ⟨R1⟩ Red first

- `chromeColors()` returns a **dark** surface for NIGHT and for AMBER (luminance below a set bar), each with
  `onSurface` at **≥ 4.5:1** contrast against it; NORMAL returns the light baseline. Pure unit test in
  commonTest, the A37 `stageSchemeStep` precedent.
- **Teeth-check:** make `chromeColors()` return the light baseline for NIGHT too and confirm the contrast /
  darkness assertion **fails** — otherwise "the drawer goes dark at night" is untested.
- **Device-QA (owed, tablet):** in a dark room, cycle to NIGHT then AMBER, open the drawer and the settings
  sheet in each — no white flood, every control and the section headers legible, AMBER preserves the warm
  cast. This is the acceptance that a pure test cannot give.

## Out of scope

The page content (already handled by `pageColorFilter`), the on-stage scheme **cycle** itself (A37, done),
any brand-colour change, and the NORMAL/day appearance (must be pixel-unchanged). No new scheme is added.
