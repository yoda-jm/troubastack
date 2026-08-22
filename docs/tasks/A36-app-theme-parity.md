# A36 — Give the app the website's colours instead of Material's default purple

**Priority:** normal · **Size:** M · **Area:** `app/shared` (a new theme in `commonMain`),
`app/androidApp`, `app/shared/src/iosMain`. Lane: Mobile.

VLL, 2026-08-21: *"the color scheme in the app could be the same as the website could be nice"*.

## Why the app looks like a different product

There is **no app theme at all**. Both entrypoints wrap the app in a bare `MaterialTheme`:

- `app/androidApp/src/main/kotlin/com/troubashare/app/MainActivity.kt:92` — `setContent { MaterialTheme { App() } }`
- `app/shared/src/iosMain/kotlin/com/troubashare/shared/MainViewController.kt:63` — `MaterialTheme { App() }`

With no `colorScheme` argument, Material 3 falls back to its **baseline palette** — the stock
lavender/purple every unstyled Compose app ships with. Meanwhile the studio has a deliberate identity:
a warm paper ground with an indigo brand (`web/studio/src/styles.css`). Home, Concerts, Connect,
Import and the settings sheet all render in Material's purple, so the app reads as unrelated software.

## The fix

Add **`TroubaTheme`** in `app/shared/src/commonMain/.../ui/` (a `lightColorScheme`/`darkColorScheme`
pair plus a composable wrapper), and use it at **both** entrypoints in place of bare `MaterialTheme`.

Map the studio tokens onto the Material 3 roles. These are the studio's real values — copy them
exactly, do not re-pick by eye:

| M3 role | light (studio `:root`) | dark (studio `@media prefers-color-scheme: dark`) |
|---|---|---|
| `primary` | `#4f46e5` `--brand` | `#a5b4fc` |
| `onPrimary` | `#ffffff` | `#201d33` |
| `primaryContainer` | `#efedfc` `--brand-tint` | `#201d33` |
| `onPrimaryContainer` | `#35309a` `--brand-ink` | `#c3ccff` |
| `background` | `#f7f4ee` `--bg` | `#100e16` |
| `onBackground` | `#201d29` `--fg` | `#efeaf6` |
| `surface` | `#fffdfa` `--surface` | `#191722` |
| `onSurface` | `#201d29` | `#efeaf6` |
| `surfaceVariant` | `#f2eee6` `--surface-2` | `#211e2c` |
| `onSurfaceVariant` | `#6d6979` `--muted` | `#9c96ac` |
| `outlineVariant` | `#e7e1d8` `--border` | `#2b2836` |
| `outline` | `#d8d1c5` `--border-strong` | `#3a3648` |
| `error` | `#b42318` `--error-fg` | `#fca5a5` |
| `errorContainer` | `#fbeceb` `--error-bg` | `#2a1414` |

Light/dark follows the system setting (`isSystemInDarkTheme()`), matching how the studio follows
`prefers-color-scheme`.

**No dynamic colour / Material You.** It would repaint the app from the user's wallpaper and defeat
the entire point of this task. If it is enabled anywhere, remove it.

## The guard that matters most: Stage must not change

Stage is a **deliberately dark performance surface** with its own palette, and A34's beat colours were
tuned and approved on-device by VLL over several iterations. The theme must not disturb either:

- Stage's own colours (`--stage-bg`-equivalents, the dark page well, the chrome pills) stay as they
  are — they are performance decisions about a dark room, not brand decisions.
- **A34's amber `#FFB02E` / aqua `#3EE0D4` beat colours are fixed.** They are the shared visual
  contract with the studio beat and are about to gain a third tier in A35. Do not route them through
  the theme.
- Anywhere Stage reads `MaterialTheme.colorScheme.*` today (e.g. `beatTint`'s `primary`/`outline`
  fallbacks in `StageBeat.kt`, the `MetaStrip` surface), check the result on a dark page and adjust
  the *Stage* code if the new palette reads worse — do not bend the palette to suit Stage.

While you are there: `StageBeatControl` hardcodes `accent = Color(0xE6198060)`, a teal-green that
matches neither palette. That is drift of exactly the kind this task exists to stop — either fold it
into the theme or leave it deliberately and say why in a comment.

## Acceptance criteria

- Both entrypoints use `TroubaTheme`; a grep for a bare `MaterialTheme {` wrapping `App()` returns
  nothing.
- **Before/after screenshots of a Stage page (page + chrome + a running beat) that are visually
  identical.** This is the regression proof for the guard above — attach both.
- Device screenshots of **Home** and **Concerts** in light *and* dark showing the warm-paper/indigo
  identity, not purple.
- No hardcoded colour literals introduced outside the theme file and Stage's documented performance
  palette. If you must add one, comment why.
- `:shared:check` green; `:androidApp:assembleDebug` green; iOS klibs still compile
  (`:shared:compileKotlinIosArm64`) — the theme lives in `commonMain`, so both consume it.
- No new dependencies.

## What actually shipped (2026-08-22, landed `090361d` — GO `0600166`)

The spec said "colour only"; VLL drove it further during on-device review, so the landed branch does
more than the palette swap. Recording it here so the next reader (A38 builds on this same Home) isn't
misled by a stale out-of-scope list:

- **`TroubaTheme`** — the light/dark M3 schemes from the studio tokens, at both entrypoints. As specced.
- **Concert mode (Stage) + the WebView are excluded** — each wrapped in
  `MaterialTheme(colorScheme = lightColorScheme())` (the M3 baseline Stage was built under), so the
  brand palette can't re-tint Stage. No file under `stage/` changed (VLL: "concert mode ok as-is;
  don't touch the webview").
- **Home restyle** — warm-paper cards with an indigo outline/heading/accent instead of lavender
  `primaryContainer` fills (VLL: "still feels like before / the webview feels more ochre").
- **A Parameters screen** (`⚙` on Home, `ui/SettingsScreen.kt`) — VLL: "missing a parameters native
  content, e.g. to set dark/light theme". Sections: Appearance→Theme (System/Light/Dark, persisted
  under `app.theme`, drives the theme live) and Stage→Reading/Colour mode, which write the SAME keys
  Stage's ⚙ writes (VLL: keep them in concert mode too — both editors, one stored default).
- **`ThemePref`** (SYSTEM/LIGHT/DARK) held above `TroubaTheme` at both entrypoints; iOS honours the
  same key (no Parameters UI there yet — Stage-only v1).

## Out of scope

- **Typography and shape.** The studio pairs a serif display with a system sans; a bigger call, a
  separate task if VLL wants it.
- Update policy in the Parameters screen — it's per-concert today (Manage), not a global toggle; a
  global default would be a new pref (flagged, not built).
- The studio side — it is the source of truth here and does not move.
- The connection status row / Connect modal — that is **A38**, landing on this same Home.

## Note for whoever does this

If a shared token file ever becomes worthwhile, this is the second place the studio's palette would
be duplicated by hand, and hand-copied palettes drift (see the `0xE6198060` accent above). The repo
already has the pattern for this: `glyphs.json`, the P205 vectors and `beat-phase.vectors.json` are
each one definition generated or pinned across runtimes. Generating a Kotlin palette from the CSS
tokens is **not** in scope here, but if a third consumer appears, propose it at the gate rather than
copying the table a third time.
