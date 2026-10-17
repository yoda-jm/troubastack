# A88 — The app updates itself from the GitHub `latest` release, and leaves development builds alone

**Lane:** mobile, plus one CI change · **Status:** specced 2026-10-17, not started.
**Origin:** VLL, 2026-10-17: *"can you spec auto update from the app? based on github artifacts? … especially
since we aren't using the google play store. There should be a date check and/or git/dev check so I am not
bothered with it while developing."*

## 0. The blocker to clear first: a stable signing key

Android installs an update only if it is signed with **the same key** as the installed app. Today:
- `app/androidApp/build.gradle.kts` has **no `signingConfigs`**, so every build uses the build machine's
  auto-generated debug key;
- CI (`ci.yml`, the `android` job feeding `release-apk`) runs on a fresh runner, so **every CI APK is signed with
  a new, throwaway key**. One CI APK cannot update another, nor the app on VLL's tablet;
- VLL's tablet runs builds signed with **the debug key of VLL's dev machine** (`~/.android/debug.keystore`). All
  installs so far came from there.

**⟨D1⟩ Recommended: make the dev machine's existing debug key the one signing key.**
- CI signs `assembleDebug` with it, read from a GitHub Actions **secret**: the keystore base64-encoded, plus
  its standard debug passwords.
- Local builds keep using the same file.
- Then CI APKs install over the tablet's current app **with no uninstall, so no loss of notes**. An uninstall
  wipes `files/notes/`. Never require one.
- **The secret's name is fixed: `TROUBASTAGE_DEBUG_KEYSTORE`**, holding the base64 of `~/.android/debug.keystore`.
  The passwords are Android's standard debug values (store `android`, alias `androiddebugkey`, key `android`),
  so no other secret is needed. The CI step decodes it to a temp file and passes it to a `debug` signingConfig.
  When the secret is absent (forks, or before VLL adds it), CI falls back to today's throwaway key, and the
  `release-apk` job says so in the release body.
- **VLL adds the secret himself** (repository Settings → Secrets → Actions). No lane or agent handles that
  keystore or its secret. The lane writes the CI step that reads it, and documents the exact secret names.
- **Rejected alternative:** a new key committed to the repo. Anyone could then sign an APK that installs over his
  app, and switching keys would force an uninstall.

## 1. What the app checks, and when

- **Source:** the public, unauthenticated GitHub API for the rolling prerelease that `release-apk` publishes:
  `GET https://api.github.com/repos/<owner>/<repo>/releases/tags/latest`.
  - Use the asset named `troubastage-debug.apk`.
  - Take the release's commit (the tag's target; the release body already names it) and its `published_at`.
  - Read the repository path from one constant. Do not hard-code it in several places.
- **Compare** with the installed build's identity from A84 (`BuildInfo.GIT`, `BuildInfo.BUILT_AT`):
  - same commit → up to date;
  - release `published_at` **later than** the installed `BUILT_AT` and a different commit → **update available**;
  - installed build newer than the release (a dev build ahead of main) → **nothing**.
- **⟨D2⟩ Development builds stay quiet.** `generateBuildInfo` (A84) gains `CHANNEL`: `"ci"` when the build runs in
  GitHub Actions (`CI=true` / `GITHUB_ACTIONS=true`), otherwise `"dev"`. Then:
  - a `"dev"` build **never checks on its own**. Parameters → About still offers **"Check for updates"** by hand;
  - a `-dirty` build is always `"dev"`;
  - a `"ci"` build checks automatically.
- **When:**
  - automatically at most **once a day**, on app start or on returning to Home;
  - **never in a concert** (Stage open), and never while a note is being drawn or sent;
  - offline, or on any API error or rate limit (60/h unauthenticated): nothing is shown, and the next try is
    the next day.
- **Remembered:** the last check time and the last release commit seen, per device (preferences).
  - "Not now" hides the banner until a **newer** release appears, so the same build never nags twice.

## 2. What the user sees

- **On Home:** a quiet banner, *"Update available, built <date>"*, with **Update** and **Not now**. No dialog, and
  nothing on Stage.
- **Parameters → About:**
  - the A84 line;
  - *"Latest: <commit> · built <date>"*, or *"Up to date"*, or *"Couldn't check"*;
  - a **Check for updates** button;
  - an **Automatic checks** switch: default on for `"ci"`, off for `"dev"`.
- **On Update:**
  1. Download with a progress indicator, to the app's cache. On mobile data, warn first: the APK is about 15 MB.
  2. **Verify before installing:**
     - the downloaded file's signing certificate equals the installed app's, through
       `PackageManager.getPackageArchiveInfo(…, GET_SIGNING_CERTIFICATES)`. On a mismatch, refuse with a plain
       message and never offer an uninstall;
     - the package name matches;
     - the size matches the asset's.
  3. Hand the file to **`PackageInstaller`**. Android asks the user to confirm; the install is never silent.
     - The first time, Android sends the user to "Install unknown apps" for TroubaStage. Explain that in one
       line before sending him there.
     - Needs `REQUEST_INSTALL_PACKAGES` in the manifest, plus a `FileProvider` (or a PackageInstaller session
       stream).
  4. After the update, the app restarts on Home, and About shows the new hash.
- **Never** delete app data, and never interrupt a concert.

## 3. versionCode

`PackageInstaller` refuses a **lower** `versionCode`.
- CI sets `versionCode = GITHUB_RUN_NUMBER` (monotonic per repository).
- Dev builds keep `versionCode = 1`. A CI update then always installs over a dev build.
- Going back from CI to a dev build stays an `adb install -r -d` (debuggable downgrade) on VLL's machine. Document
  that in the README.

A84 left `versionCode` deliberately untouched; this task is the decision it deferred.

## 4. Not in this task

- iOS. TestFlight is its own story.
- A Play Store or F-Droid listing.
- Delta updates, or background downloads without the user tapping Update.
- Updating while the app is closed.

## 5. Acceptance

- **Unit (pure):**
  - the decision function over (installed commit, built at, channel, release commit, published at, last seen,
    dismissed): up to date / available / dev-silent / installed-newer / dismissed-until-newer;
  - the once-a-day throttle;
  - the never-in-concert gate.
- **Teeth:** remove the dev-channel check, and the dev-silent case goes red.
- **CI:**
  - `aapt`/`apksigner verify --print-certs` on the `latest` APK shows the **same certificate digest** as the
    tablet's installed app (`adb shell dumpsys package` signatures, or `apksigner` on a locally built APK);
  - two consecutive CI APKs have the same digest;
  - `versionCode` equals the run number.
- **Device (VLL's tablet):**
  1. install a CI APK over the current app with `adb install -r`: **notes intact**, counting files before and after;
  2. then publish a newer CI build and let the app find it;
  3. Update → confirm → restart → About shows the new hash, and the notes are still intact;
  4. a dev build never shows the banner on its own.
- **Regression:** the CI set (unit, assemble, both iOS compiles) is green.
