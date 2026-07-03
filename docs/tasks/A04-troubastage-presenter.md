# A04 — TroubaStage presenter: resilient read-only compositor + pager

**Priority:** A-track 4 (after A03) — **the centerpiece of the app track** · **Size:** L · **Area:** `app/shared` stage/, `app/androidApp`

## Context

TroubaStage is what musicians look at **on stage, mid-performance**. The architecture
gives it exactly one job (invariant I12): composite pre-baked images (page raster +
transparent layer overlays, in z-order) and page through them. All intelligence happened
at bake time on the server; the presenter is deliberately dumb, offline, and
self-contained.

Read first: `stage/Presenter.kt` (the scaffold's doc comments are the spec),
`docs/design/05-distribution.md`, `docs/ARCHITECTURE.md` I12/I13, and A02's
model/loader (`bundle/` package) which this task consumes.

## The two non-negotiables

**1. It must never crash — and never dead-end.** A crash or a stuck screen during a live
performance is the single worst failure this product can have. Every failure mode
degrades to "this page shows a neutral placeholder, everything else keeps working":

- Corrupt/missing page image ⇒ placeholder card ("Page unavailable") — pager, song jump,
  and every other page keep working. (A02's loader already flags these as `BundleIssue`s;
  decode failures at render time get the same treatment.)
- Corrupt overlay ⇒ composite **without** that overlay (raster still shows).
- Empty bundle / zero pages ⇒ calm empty state with a way back, not an exception.
- Out-of-range navigation ⇒ clamp, never throw. All state transitions are total.
- Image decode wrapped in try/catch at every call site; OOM-avoidance by decoding
  downsampled to the display size (`inSampleSize` on Android), an LRU of ~5 decoded
  pages, and preloading only current±1. Never decode the whole bundle.
- Rule of thumb for the whole `stage/` package: **exceptions are for programmer errors
  only; every input/environment problem is a state the UI can render.**

**2. It is read-only.** The presenter *performs*; it never writes. It must contain **no**
mutation of bundles, no annotation model, no access-control logic, and **no network** —
at performance time it depends on nothing server-side (I12). The only state the user can
change is **reading behavior** (below), held in a view-model, at most persisted as local
preferences. There is no path from this package to `SyncClient`, `Updates`, or any HTTP
client — enforce with imports (see acceptance).

## Controls — minimal by design, nothing fancy

- **Navigation:** tap right/left third of the screen = next/previous page; horizontal
  swipe does the same; a thin bottom bar with `‹ ›`, a "page 3/12" indicator, and a song
  picker (jump to the first page of each `BakedSong`; label = song index/name from the
  bundle). That's all — no thumbnails, no search.
- **Display:** fit mode toggle (fit-page ↔ fit-width; fit-width scrolls vertically),
  and keep-screen-on while Stage is open (Android: `FLAG_KEEP_SCREEN_ON` from the
  activity — one line in `androidApp`, not a new seam).
- **Immersive while performing:** hide the system bars while Stage is open
  (Android: `WindowInsetsControllerCompat.hide(systemBars())` with
  `BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE`, so an edge swipe reveals them transiently).
  Scope to the Stage screen's lifecycle exactly like keep-screen-on: bars restored on
  exit. Note the boundary: this hides *chrome*; it cannot and must not try to suppress
  other apps' notifications (WhatsApp heads-up banners etc.) — silencing those is
  **Do-Not-Disturb, which stays the user's responsibility** (no DND permission, no
  notification-listener; explicitly out of scope).
- **Layering:** an overlay panel (small "Layers" button) listing the bundle's layers with
  visibility checkboxes. `mandatory: true` layers are shown as locked/on (I12 — the
  viewer cannot hide them); `roleTag` sets the *default* visibility only. Toggling a
  layer only changes what is composited — it cannot modify the bundle.
- **Local role (no login!):** Stage runs fully offline with **no account and no auth
  gate** — but `roleTag` defaults need to know "who is reading". Solve it with a local,
  non-authenticated **"My role" preference** in Stage (a plain text/choice picker,
  suggestions harvested from the roleTags present in loaded bundles). Default-visibility
  rule: empty `roleTag` ⇒ visible by default; non-empty ⇒ visible by default only when it
  equals the local role; `mandatory` always wins. This is a *reading preference*, not
  identity — no biometrics, no PIN, no account linkage. It only seeds defaults; manual
  layer toggles still override per the previous bullet.
- Nothing else. No zoom gestures in v1 (fit modes cover it), no editing affordances of
  any kind, no settings beyond the above.

## Changes

1. `stage/` in commonMain, split for testability:
   - `StageState` + `StageViewModel` (plain class + `StateFlow`; no Android deps):
     current page, fit mode, per-layer visibility, per-page status
     (`Ready`/`Unavailable`), song boundaries. All transitions pure + clamped. This is
     where most unit tests live.
   - `PageCompositor`: given a decoded raster + visible decoded overlays in z-order,
     draws them into a Compose `Canvas`/`Image` stack. Decoding itself goes behind a
     small `ImageDecoder` interface (`decode(path, targetW, targetH): Result<ImageBitmap>`)
     with the Android actual implemented in `androidMain`… **no** — seams are capped at
     three (I15). Instead: use Compose Multiplatform's `ImageBitmap` decoding available
     from commonMain if your CMP version provides it; if it does not, inject the decoder
     function from `androidApp` (constructor parameter — dependency injection by hand,
     no new expect/actual, no framework).
   - `StageScreen` composable: the pager UI + controls above.
2. `androidApp`: a "Concerts" placeholder list containing the A03 demo fixture shipped in
   `assets/`, opening `StageScreen` (copy asset → cache dir on first open so the loader's
   file interface can read it; this stopgap is replaced by real import in A05).
3. Unit tests (commonTest) — the resilience contract, mechanically:
   - navigation clamps at both ends; song jump lands on the right page; empty bundle ⇒
     empty state; every torture fixture from A03 loads into a usable state (bad page ⇒
     `Unavailable`, rest `Ready`); mandatory layer cannot be toggled off; layer toggle
     changes only the visible-set.
4. Manual verification on an emulator: page through the demo bundle, toggle layers,
   switch fit mode, jump songs — **in airplane mode**. Screenshot(s) in the PR.

## Acceptance criteria

- `cd app && ./gradlew :shared:check :androidApp:assembleDebug` green; all new unit
  tests pass.
- **No-network gate:** `grep -rn "ktor\|okhttp\|HttpClient\|java.net" app/shared/src/commonMain/kotlin/com/troubashare/shared/stage/`
  returns nothing, and `stage/` has no import of `sync/` or `distribution/`.
- **No-write gate:** `stage/` never writes to disk (reads only); layer toggles and fit
  mode live in the view-model.
- Torture fixtures: opening each one on the emulator neither crashes nor dead-ends —
  `bad-json`/`no-manifest` show the friendly failure screen with a back action;
  `missing-blob` performs normally with one placeholder page.
- Demo bundle pages render composited (raster + overlays), and toggling a non-mandatory
  layer visibly adds/removes its overlay.
- **No-login gate:** Stage (and the concerts entry point) is reachable from app launch
  without any account, session, or unlock; unit test for the roleTag default-visibility
  rule (empty ⇒ on, mismatched ⇒ off, matching ⇒ on, mandatory ⇒ always on).
- **Screen never sleeps mid-performance:** with the device screen timeout set to its
  minimum (emulator: `adb shell settings put system screen_off_timeout 15000`), the
  screen stays on indefinitely while Stage is open and the timeout behaves normally
  again after leaving Stage (flag must be cleared on exit — scope it to the Stage
  screen's lifecycle, not the whole app).
- **Immersive:** system bars hidden while Stage is open; an edge swipe shows them
  transiently and they auto-hide again; they are fully restored on every exit path
  (back navigation, song picker back-out, app switch and return).

## Out of scope

- Import/downloads (A05), updates & freeze UI (I13 — later, with the distribution task),
  zoom gestures, annotations of any kind, iOS.
- Do-Not-Disturb / notification suppression: user's responsibility (the app runs on
  tablets too — no calls, but WhatsApp-class banners exist everywhere; we hide our own
  chrome, never touch other apps' notifications).
