# WIN04 — The Windows app updates itself from the `latest` release (A88's twin)

Owner: **mobile**. Starts after **WIN03** has landed: the MSI is on the release from WIN03 on. Spec: Fable,
2026-11-01. Size: S/M.

## Why

On Android, VLL takes every build through A88's self-update. Without the same thing on Windows, each new build
means finding the release page and reinstalling by hand, and the Windows app drifts behind the tablet.

## What is there

- Shared, tested: `buildinfo/AppUpdate.kt` holds `updateVerdict`, `shouldAutoCheck` (never in a concert, never
  while a note is busy, never on a "dev" build) and `latestLine`. `LatestRelease` carries `apkUrl`/`apkSize`.
- Android-only: `AppUpdater` (fetch the release JSON with no band-server cookie, download with progress, verify,
  hand over to PackageInstaller) and `AppUpdateController` (state, the Home banner, the Settings rows).
- From WIN02 and WIN03: a per-user MSI with a fixed `upgradeUuid` and an increasing version, published on `latest` as
  a stable asset name.

## Design (decided)

- **Reuse the shared decision code unchanged in meaning.** Generalise `LatestRelease` to "the asset for this
  platform", with the asset name supplied by the platform. Android's behaviour and tests stay identical.
- **The same surfaces as Android**: the Home banner (Update / Not now), the Settings rows (latest line, Check for
  updates, Update, the Automatic switch). Same words.
- **Downloading:** the same rules as `AppUpdater`. It never sends the band server's cookie to GitHub, shows
  progress, and verifies the size, plus the SHA-256 when the release JSON gives one.
- **Installing:** a per-user `msiexec` run of the downloaded file, then the app exits so the files can be
  replaced. The new build should **open again by itself** after the install. If that cannot be done reliably,
  say so and show "Updated — open TroubaStage again" instead. Never an uninstall; never during a concert (the
  shared rule already says so).
- An update that fails leaves the installed app working, with one plain-words line: the A88 pattern.

## Acceptance criteria

1. The shared tests still pass, with new cases for the platform-asset selection: a release with both assets
   gives each platform its own; a release with only the APK gives Windows "no update" rather than an error.
2. On Windows (CI runner or VLL's machine), going from build n to build n+1 through the app's Update button
   upgrades in place, keeps the data under `%LOCALAPPDATA%\TroubaStage` (concerts, notes, sign-in), and ends on
   n+1. Settings shows the new build line.
3. Report exactly what Windows shows during the install (UAC, SmartScreen, the msiexec UI), with screenshots.
   That is VLL's input on code signing.

## Out of scope

Code signing; channels other than `latest`; delta updates.
