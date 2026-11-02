# DESK02 — the Mac app on `latest` and on the site

Owner: **web-core**. Spec: Fable, 2026-11-02, per VLL: "align with windows, publish the artifact in latest and add it
also to the pages". Size: S, in two stages that land separately.

This reverses DESK01 D4's "only once a person has tested it on a Mac". The MSI and the AppImage went on `latest`
with only CI's checks behind them, and the DMG now does the same. VLL's own pass on his Mac is still owed, and it
supplies the first-launch button names, which are left out until then.

## What is there (main, 52820353)

- `.github/workflows/ci.yml`:
  - `desktop-macos` builds one universal `TroubaStage.app` and launches it three ways. It then makes
    `troubastage-macos.dmg` and uploads it, with the launch PNGs, as the artifact `troubastage-macos-dmg`.
  - `release-apk` needs `[android, desktop]` only. It recreates `latest` with three assets: the APK, the MSI and
    the AppImage.
  - `release-apk` republishes only when `before..sha` touches `app/`.
- The app already picks `troubastage-macos.dmg` on macOS (`UPDATE_ASSET_DMG`). A release without that asset counts as
  no update.
- `web/site/build.sh` links the APK and the MSI, and fails the build if either link does not resolve. The AppImage is
  described in "Where packaging stands" but has no link.
- Docs:
  - `docs/USER-JOURNEY.md:57–58` says the Mac app "joins the release once it has been tested on a Mac";
  - `README.md:160` names the MSI only, not the AppImage;
  - the TroubaStage row of `docs/glossary.md` says "Windows and Linux".

## Stage 1 — the release (CI)

1. **Four assets, recreated together.**
   - Add `desktop-macos` to `release-apk`'s `needs`.
   - Fetch the DMG artifact. Like the MSI, expect exactly one `.dmg` or stop, and attach it as
     `troubastage-macos.dmg`.
   - The release notes gain a macOS line: one app for Apple silicon and Intel Macs, not signed yet, built from the
     same commit.
2. **The trade-off this brings back (b67486a3).** A red Mac leg now holds up the APK, the MSI and the AppImage.
   - The one Mac failure seen so far was the DMG step, on land run 41451900079 attempt 1. The re-run passed with
     nothing changed.
   - So a single transient failure of `hdiutil create` must not turn the leg red: retry it, a bounded few times,
     removing any partial file between tries. A failure on every try still turns the leg red.
   - Retry only `hdiutil create`. The launch checks get no retry.
3. **The landing commit republishes.** Today a commit that touches only `.github/` keeps the old release, because of
   the `app/` check. A change to the workflow that defines the release's contents must republish it. How is yours:
   for example, the check also matches `.github/workflows/ci.yml`. It still fails open.
4. **Sweep the old wording.** These all say the DMG waits for a human test on a Mac:
   - in `ci.yml`: the header comment above `release-apk` (~298), the `desktop-macos` comment (~864) and the DMG step
     name (~945);
   - `AppUpdate.kt:28`, `DesktopAppUpdate.kt:127`, `DesktopOsTest.kt:84` and `ReleaseParseTest.kt:62`.

   In `app/`, change only the comments.

**Acceptance, stage 1:**
- The PR run is green on every job, `desktop-macos` and `desktop (windows)` included.
- The retry, both ways: a stand-in `hdiutil` that fails once and then succeeds leaves the step green, and one that
  fails every time turns it red. Run the loop with a shell-function stub and show both outputs.
- After landing:
  - the land run's `release-apk` ran its publish step;
  - `latest` points at the landing commit and lists exactly four assets;
  - a ranged GET of the DMG returns 206;
  - the published DMG's sha256 equals that of the DMG in the same run's artifact.

## Stage 2 — the site and the docs

Stage 2 lands only once `latest` carries the DMG. Otherwise `build.sh`'s link check fails the Pages deploy.

1. **`build.sh`:**
   - add `DMG_URL` and `APPIMAGE_URL`, the stable asset names on `latest`;
   - give each one the same check that no `{{…}}` token survives;
   - include all four links in the check that they resolve.
2. **`index.html`:**
   - In the install note, after the Windows link: a Mac download (.dmg) and a Linux download (the AppImage), as
     plain links in the same register as "download the installer".
   - In "Where packaging stands", one sentence for the Mac:
     - one app for Apple silicon and Intel Macs;
     - not signed yet, so macOS may refuse to open it the first time; allow it under Privacy & Security in the
       system settings;
     - **Update** opens the new download in the browser, as on Linux.
   - Name the place only, not the buttons. This follows the SmartScreen rule (WIN06, amended 6b98d3fe): the button
     names come from VLL's first launch.
3. **`docs/USER-JOURNEY.md:57–58`** becomes the macOS line: the download, the first launch as above, and where the
   data lives. Take the data folder from `Storage.desktop.kt`, not from memory.
4. **`README.md:160`:** the rolling release also carries the AppImage and the DMG.
5. **`docs/glossary.md`:** the TroubaStage row says Windows, Linux and macOS.
6. Every UI label quoted in these lines is grepped from the shared strings (DESK01 D5).

**Acceptance, stage 2:**
- A sabotage run: a wrong DMG asset name fails `build.sh`, and restoring it passes.
- After landing:
  - the Pages deploy is green;
  - the live site carries both new hrefs;
  - a ranged GET of each returns 206.

## Out of scope

- Signing and notarisation.
- An in-place update on the Mac.
- The exact first-launch button names: a one-line follow-up after VLL's pass.
