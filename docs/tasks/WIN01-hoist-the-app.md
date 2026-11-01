# WIN01 — Hoist the app out of `androidApp` so a second JVM platform can run it (Android: no behaviour change)

Owner: **mobile** (start now). Spec: Fable, 2026-11-01. Size: M/L. First task of the Windows track
(WIN01 → WIN03; WIN02 runs in parallel on web-core).

## Why (VLL, 2026-11-01)

> "how complicated is it to have a windows app similar to the android app and keeping most of the source shared?"
> then: "refactor en premier puis juste stage avec studio qui fait un 'bientôt'"

The refactor comes first, on its own, with no behaviour change, so that the Windows work (WIN03) is mostly
"fill in a platform", not "rewrite the screens".

## What is there today (read on `main` e881a9e6)

- `app/shared/src/commonMain` (≈10 200 lines) already holds the logic and several screens: `StageScreen`,
  `HomeScreen`, `SettingsScreen`, `NotePad`, `IdentityPicker`, the distribution/sync/bundle code.
- `app/androidApp/src/main/kotlin/com/troubastack/app` (22 files, ≈4 600 lines) holds the rest: the whole
  navigation (`App()` in `MainActivity.kt`, lines ~215–760), `ConcertsScreen` + `NotesTab` (same file),
  `ConcertListPane`, `ConnectScreen` (the Connect dialog), `JoinDialog`, `EditedConcertsScreen`,
  `HttpTransport` (ktor + OkHttp), `ArmedUploader`, `BakeRunner`, `NotesTree`, `StagePosition`,
  `AndroidImportFs`, plus the Android-bound pieces listed below.
- **Most of it is not Android code, only Android-located.** `MainActivity.kt` is 1 196 lines of which about 60
  touch an Android API (`LocalContext`, `Intent`/`Uri`, `SystemClock`, `android.text.format.DateFormat`,
  `rememberLauncherForActivityResult`, `BackHandler`, `findActivity()`). `HttpTransport`, `ArmedUploader`,
  `BakeRunner`, `NotesTree`, `StagePosition`, `ConcertListPane` import no Android API at all.
- Genuinely Android-bound (they stay in `androidApp`): `AndroidImageDecoder` and `AndroidRehearsalNotes`
  (`android.graphics`), `AppUpdater` + `AppUpdateController` (PackageInstaller), `MidiPedal` (BLE-MIDI),
  `NsdServerDiscovery` (NsdManager), `QrScanScreen` (CameraX), `QrPresentScreen`, `EditScreen` +
  `StudioBrowseScreen` (the Studio WebView flow), `StageHost` (keep-screen-on + immersive), and the
  Activity itself (volume keys, deep-link intents, resume).

## The decision this supersedes

`reviews.md` 2026-07-06 and the mobile handoff §13 decided **(c) DEFER the shared App()/nav hoist** "until a
second consumer", with the trigger "an iOS ManifestTransport". Windows is that second consumer, and it is a JVM
platform, so `HttpTransport` and the `java.io.File` code can be shared as they are. **The iOS hoist stays
deferred**: nothing in this task touches `iosMain` or `MainViewController.kt`.

## Design (decided — do not re-open without the gate)

**D1 — Where the hoisted code lives.** A new intermediate source set in `:shared`, shared by the JVM
platforms and NOT by iOS. Name it `jvmSharedMain` (tests: `jvmSharedTest`). `androidMain` depends on it now;
the Windows target's source set will depend on it when WIN02 adds that target. iOS compiles none of it.
Moving it on to `commonMain` (for iOS) is the still-deferred iOS hoist, not this task.

**D2 — Keep the package.** Moved files keep `package com.troubastack.app`, so a move is a pure rename in git
(reviewable as 100 % similarity) and no import elsewhere changes.

**D3 — One platform interface, nullable capabilities.** The hoisted `App()` receives everything
platform-specific through **one** interface (suggested name `AppPlatform`; you may refine the names, and say so
at the gate). A capability a platform does not have is `null`, and `null` **hides** the affordance. It does not
show a broken one. The capabilities, from the call sites above:

| Capability | Android implementation (unchanged behaviour) | Purpose |
|---|---|---|
| `storage` | `Storage(context)` | the existing seam |
| `imageDecoder(dir)` | `AndroidImageDecoder(File(dir))` | Stage page decode |
| `rehearsalNotes(notesDir)` | `AndroidRehearsalNotes` | A70 note bitmaps |
| `importFs` / `pickBundle()` | SAF `OpenDocument` + `copyToTemp` | import a `.tstage` |
| `discovery: ServerDiscovery?` | `NsdServerDiscovery` | B06 Connect prefill |
| `qrScanner: (@Composable …)?` | `QrScanScreen` | A53/A57 scan to join |
| `studio: StudioHost?` | `StudioBrowseScreen` → `EditScreen`, `QrPresentScreen` | everything that opens Studio |
| `appUpdate: …?` | `AppUpdateController` + `AppUpdateDialogs` | A88 |
| `pedal: …?` | `MidiPedal` + permission launcher | A76 BLE-MIDI |
| `stageWindow(content)` | `StageHost` | keep awake + immersive while performing |
| clocks | `SystemClock.elapsedRealtime`, `DateFormat.getTimeFormat`, `System.currentTimeMillis` | T147/A74 |
| `resumeTicks` | `MainActivity.resumeTick` | A31 Home re-probe on foreground |
| `pendingInviteLink` | `MainActivity.pendingLink` | A52 deep link |
| `volumeTurnRegistrar` | `LocalVolumeTurnRegistrar` → `stageVolumeTurn` | A09/A13 volume keys |
| `settingsWereReset` | `Storage.settingsWereReset` | A54 notice |

`Back` handling: the hoisted code needs a back abstraction that is not `androidx.activity.compose.BackHandler`.
Choose the mechanism (Compose Multiplatform's common back handler, or a host-provided one). The property that
matters: **every Back behaviour on Android is identical**, including A79 (Back is consumed in Stage; only ✕
leaves; an open drawer closes first, through the shared, tested `stageBackAction`).

**D4 — What stays in `androidApp`.** `MainActivity` becomes entrypoint glue (theme holder, `setContent`, intents,
resume, volume keys) plus the Android implementation of the interface, and the Android-only files listed
above. Any `@Composable` left in `androidApp` must be Android-bound (it imports an Android/CameraX/WebView API).

## Changes

1. Create `jvmSharedMain`/`jvmSharedTest` in `app/shared/build.gradle.kts`, with `androidMain` depending on
   it. Move the ktor/OkHttp/serialization dependencies the moved code needs into it.
2. Move the Android-free files (D2, pure renames).
3. Extract `App()`, `ConcertsScreen`, `NotesTab` and their helpers from `MainActivity.kt` into
   `jvmSharedMain`, replacing each Android call with the interface (D3).
4. Move the `androidApp` unit tests that cover moved code into `jvmSharedTest`
   (`AndroidPureFunctionsTest`, `NotesBulkOutcomeTest`, `NotesTreeTest`, `StagePositionRoundTripTest`, and
   whatever else follows its code). `AppUpdaterParseTest` stays with `AppUpdater`.
5. Update `docs/ARCHITECTURE.md` I15's text for the new layout (shared JVM code; the platform interface is DI,
   not a fourth seam, like iOS's `ImageDecoder`/`BundleFiles`).

**Commits:** at least two, in this order: the pure moves first, then the de-Androidification. **Each commit
builds and passes the tests on its own** (bisectability). Present them together; they land together.

## Guard until WIN02 exists

Until a second JVM target compiles `jvmSharedMain`, the compiler will accept Android imports in it (its only
compilation is Android's). Hold the line by hand, and say the result at the gate:

```
grep -rnE '^import (android\.|androidx\.(activity|core|camera|security))' app/shared/src/jvmSharedMain
```
→ empty. Once WIN02's desktop target lands, the compiler enforces it.

## Coordination with WIN02 (web-core, in parallel)

WIN02 adds the desktop JVM target, its actuals and `:desktopApp` to the same `app/shared/build.gradle.kts`. Whoever
lands **second** wires `desktopMain` to depend on `jvmSharedMain` and makes the desktop target compile it (stub
whatever the desktop side does not provide yet — it is WIN03's job to fill it). Tell the other lane when you
land.

## Acceptance criteria

1. `cd app && ./gradlew :shared:check :androidApp:test :androidApp:assembleDebug :shared:compileKotlinIosArm64`
   is green, and CI's `android` job is green on the landed SHA.
2. **No test lost.** Count the `<testcase>` elements across the `TEST-*.xml` of `:shared` and `:androidApp` at
   `main` and at your head. After ≥ before; give both numbers.
3. `git show -M --stat` on the move commit shows renames. Files that did not need to change are 100 %.
4. The guard grep above is empty.
5. Every `@Composable` remaining in `androidApp` is Android-bound (list them with the API each one needs).
6. **Android unchanged, checked on the emulator** (`Pixel_Tablet_Portrait`, demo concert injected; see
   the mobile handoff). Screenshots at `main` and at your head, same AVD and data, of: Home, the concert list
   (Stage tab and Notes tab), Stage (page mode, chrome shown, drawer open), Settings, the Connect dialog,
   TroubaStudio browse. Pixel-identical, or each difference explained (the clock).
   And these journeys, each one line in the submission:
   - Home → Perform → open a concert → swipe turns, a key turns, ✕ leaves.
   - In Stage, system Back does not leave; with the drawer open, Back closes only the drawer (A79).
   - Note mode: draw, Clear page, Exit.
   - TroubaStudio → open a band → Studio loads; ⋮ Reload keeps the page (A93).
   - Import a `.tstage` through the picker.
   - Kill the process in Stage (`adb shell am kill` after Home) and relaunch: the position restores (A46).
7. Line counts of `androidApp` and `jvmSharedMain` before and after, in the submission.

## Out of scope

- Any behaviour or string change on Android. If something looks wrong while you move it, file it; do not fix it here.
- The desktop target, its actuals and packaging (WIN02); the Windows behaviour (WIN03).
- iOS (`MainViewController.kt`, `iosMain`): untouched.
