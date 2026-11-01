# WIN02 — A desktop JVM target, `:desktopApp`, and a Windows installer built in CI

Owner: **web-core** (start now, in parallel with WIN01). Spec: Fable, 2026-11-01. Size: M.
The Windows track is WIN01 → WIN03 (mobile) with this task beside WIN01; see WIN01 for the why.

## Goal

The plumbing for a Windows build of TroubaStage, proven end to end on a real Windows runner, before any
screen is ported: the shared code compiles and its tests run on a desktop JVM, a `:desktopApp` module opens a
window, and CI packages it as an `.msi` and proves the packaged app starts on Windows.

The window shows almost nothing yet. WIN03 (mobile) puts the app in it.

## What is there (read on `main` e881a9e6)

- `app/shared/build.gradle.kts`: Kotlin Multiplatform with `androidTarget()` and `iosArm64()`/`iosSimulatorArm64()`;
  Compose Multiplatform 1.9.0, Kotlin 2.2.20 (`app/gradle/libs.versions.toml`). Compose Multiplatform supports a
  desktop JVM target with these versions.
- Five `expect` declarations in `commonMain`, each needing a desktop `actual`:
  - `seams/Storage.kt`: `expect class Storage` (`bundlesDir`, `tempDir`, `notesDir`, `getSecret`, `putSecret`)
    and `expect fun unpackBundle`.
  - `seams/WebViewHost.kt`: `expect class WebViewHost`.
  - `seams/InkOverlay.kt`: `expect class InkOverlay` (`TODO` on Android and iOS too: A07 never landed).
  - `stage/OverlayTransform.kt`: `expect fun transformOverlayBitmap`.
  - `distribution/LocalTime.kt`: `expect fun localUtcOffsetSeconds`.
- The Android `Storage` actual already uses `java.util.zip` for `unpackBundle` with the zip-slip and size-cap
  guards; the iOS `transformOverlayBitmap` actual (`OverlayTransform.ios.kt`, 46 lines) already uses Skia, which is
  also what Compose renders with on desktop.
- CI (`.github/workflows/ci.yml`): the `android` job runs `./gradlew :shared:check :androidApp:test
  :androidApp:assembleDebug` on ubuntu with temurin 21, then cross-compiles the iOS klibs.

## Design (decided)

**D1 — Target and module.** `jvm("desktop")` in `:shared` (source sets `desktopMain`/`desktopTest`). A new
`:desktopApp` module is the entrypoint, the analog of `androidApp` and `iosApp`: Compose for Desktop
`application { }` with `nativeDistributions`.

**D2 — The actuals.**
- `Storage`: one data root.
  - Windows: `%LOCALAPPDATA%\TroubaStage`. **Superseded at the gate (B2):** that is the per-user install folder, so the data root is `%LOCALAPPDATA%\TroubaStageData`.
  - Elsewhere (Linux is where lanes develop and test): `${XDG_DATA_HOME:-~/.local/share}/troubastage`.
  - Both overridable by `TROUBASTAGE_HOME`, so tests and agents run isolated.
  - `bundles/`, `notes/`, `tmp/` under it.
- Secrets: one key/value file under the root, written atomically (temp + rename).
  - On Windows its content is protected with **DPAPI, current-user scope** (JNA's `Crypt32Util` is acceptable;
    say which dependency you add).
  - Elsewhere: the file is created `0600`.
  - A file that cannot be read or decrypted is healed to empty with the same "settings were reset" signal as
    Android's A54 open-or-heal, never a crash.
  - Every preference goes through this store (the Stage position is written on every page turn), so keep writes
    cheap.
- `unpackBundle`: the Android `java.util.zip` code is JVM code. Share it rather than copy it, if it moves
  cleanly (e.g. into WIN01's `jvmSharedMain` once that exists); otherwise copy it **with its guards and their
  tests**.
- `transformOverlayBitmap`: Skia, as on iOS (copy or share — your call; say which).
- `localUtcOffsetSeconds`: `java.time`.
- `WebViewHost`: no embedded browser in this track yet (WIN05 decides). An `actual` that is never constructed on
  desktop. If anything constructs it, fail loudly with a message that names WIN05.
- `InkOverlay`: `TODO`, as on the other two platforms.

**D3 — Packaging.**
- `.msi` via Compose's `nativeDistributions`.
- **Per-user install** (no admin rights, no UAC prompt).
- A fixed `upgradeUuid` committed in the build file, so a newer MSI upgrades in place.
- Start-menu entry.
- Product name **TroubaStage** (the Android app's label).
- The MSI version must increase on every build, or Windows will not upgrade: derive it from the CI run number
  (e.g. `1.0.<run_number>`). The commit and build time are stamped into the app separately, as Android's are
  (`buildinfo/AppBuild.kt`'s `buildLine`).
- App icon: a Windows `.ico` generated from the existing brand mark. Commit it with the command that made it.

**D4 — CI.** A new **hard-gating** `windows` job on `windows-latest` (temurin 21, `gradle/actions/setup-gradle`), running:
- the desktop tests;
- the MSI packaging task;
- a **smoke**: the packaged app is launched with a flag (e.g. `--smoke <png>`) that opens the real window,
  renders one frame to a PNG, and exits 0. A crash or a hang fails the job (put a timeout on it).

The MSI and the PNG are uploaded as artifacts. Timeout 30 min. The `android` job's `:shared:check` will now also
run the desktop tests on Linux. That is intended: it shows that both JVMs agree.

**D5 — Not on the release yet.** The `latest` release keeps only the APK in this task. The MSI goes on it in WIN03,
when the app in it is worth installing.

## The skeleton window

Title "TroubaStage", the build line (commit + UTC build time) and nothing else that a user could mistake for a
feature. It is replaced in WIN03.

## Acceptance criteria

1. On this Linux box: `cd app && ./gradlew :shared:desktopTest :desktopApp:run` (or the equivalent) opens the
   window. Run `:shared:check` and `:androidApp:test` as before; both green.
2. The `Storage` desktop actual has tests (run in `desktopTest`) for:
   - the root override;
   - the secret round trip;
   - the heal-on-corrupt path;
   - the zip-slip and size-cap refusals.

   The DPAPI path runs in the Windows job's tests, and the submission names the test that proves the file on
   disk is not plaintext.
3. The `windows` job is green on your branch's push: the MSI artifact exists, and the smoke PNG shows the window.
   Download both and attach the PNG to the gate entry.
4. **Upgrade in place is proven, not assumed.** In the Windows job, or by script on the runner: install MSI n,
   then MSI n+1 built with a higher version, and show that one product entry remains at the new version (or
   explain why this cannot be shown on a runner, and say what VLL must check by hand).
5. CI's other jobs are unchanged and green. `:shared:compileKotlinIosArm64` still compiles.
6. Sabotages, each shown to land:
   - make the smoke render throw → the job goes red;
   - break the zip-slip guard → its test goes red.

## Coordination with WIN01 (mobile, in parallel)

WIN01 creates `jvmSharedMain` (JVM code shared by Android and desktop) in the same build file. Whoever lands
**second** makes `desktopMain` depend on it and makes it compile for desktop. Stub whatever the desktop side does
not provide yet; WIN03 fills it. Tell the other lane when you land.

## Out of scope

Any app screen on desktop (WIN03); the MSI on the `latest` release (WIN03); self-update (WIN04); an embedded
browser (WIN05); code signing (VLL's call, it costs money; without it Windows SmartScreen warns on a browser
download); macOS/Linux packages.
