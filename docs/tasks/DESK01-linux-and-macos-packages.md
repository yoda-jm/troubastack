# DESK01 — Linux (AppImage) and macOS (unsigned DMG) packages, from the same CI matrix as Windows

Owner: **mobile** (moved from web-core on 2026-11-02: WIN03 and WIN04 have landed, the mobile lane is free and built the desktop app and its updater, while web-core is mid-T199). Starts now. Spec: Fable, 2026-11-01.
Size: M.

## Why (VLL, 2026-11-01, on the decisions page)

- Linux: "Oui, après WIN03".
- macOS: "DMG non signé", with the question "On peut faire un multiplateforme ?"

**The answer to "multiplateforme" is this task.** The desktop app is already one codebase (jvmSharedMain plus the
desktop target). Installers cannot be a single file: each OS needs its native package, and jpackage only builds
for the OS it runs on. So CI builds the three from the same code, in one matrix job: Windows → MSI (WIN02),
Linux → AppImage, macOS → DMG.

## Design

**D1 — The CI matrix.**
- The `windows` job becomes one `desktop` job matrixed over `windows-latest`, the **oldest supported** Ubuntu
  runner, and `macos-latest` (Apple silicon).
- Each OS packages, then runs the same launch smoke as WIN02, with the B5 rule: exit 0 only after the real window
  has drawn a frame. Linux runs it under Xvfb.
- Hard-gating, as `windows` is.
- Build on the oldest Ubuntu runner so that the AppImage runs on older distributions (glibc).

**D2 — Linux.**
- Compose's app image (the folder with Java bundled), wrapped by `appimagetool` into one
  `troubastage-linux-x86_64.AppImage`.
- Data root: the existing XDG branch (`${XDG_DATA_HOME:-~/.local/share}/troubastage`).
- Display sleep during Stage: the freedesktop inhibit over D-Bus where available; a documented no-op elsewhere,
  never a crash.
- Wayland runs through XWayland. Note it at the gate; don't fight it.

**D3 — macOS.**
- **One universal DMG** (VLL, 2026-11-01: "un binaire agnostique"): Apple silicon and Intel in the same app.
  jpackage builds only for the JDK it runs on, and Compose has no universal option, so:
  - build two app images, arm64 (native JDK) and x86_64 (an x86_64 JDK, under Rosetta on the arm64 runner, or an
    Intel runner if one is still offered);
  - merge them: every Mach-O file present in both trees (launcher, `libjvm` and the other runtime dylibs) goes
    through `lipo -create`; identical non-Mach-O files are kept once; any other difference fails the build;
  - put both Skiko runtimes on the classpath (`skiko-awt-runtime-macos-arm64` and `-x64`); Skiko picks its own by
    `os.arch`;
  - ad-hoc sign the merged `.app` (Apple silicon refuses fully unsigned code), then build the DMG.
  - CI asserts that `lipo -archs` lists `arm64 x86_64` for the launcher and `libjvm.dylib`, and that
    `codesign -dv` reports `Signature=adhoc`.
  - The smoke runs twice on the arm64 runner: natively, and under Rosetta (`arch -x86_64`). Both must draw a frame.
  - **Fallback**, if the merge proves fragile within the task's time box: two DMGs ("Apple silicon" and "Intel")
    from the same matrix, said so at the gate. A browser cannot reliably tell the two Macs apart, so the site then
    names both.
- Data root: `~/Library/Application Support/TroubaStage` (a new `resolveDataRoot` branch, unit-tested like the
  others).
- macOS conventions, through the per-OS seam WIN03 left (not `if (mac)` scattered through screens):
  - Cmd instead of Ctrl;
  - Cmd+Q quits;
  - full screen is the native one (green button, Ctrl+Cmd+F) instead of F11;
  - display sleep held with `caffeinate -d -w <pid>` or IOKit;
  - the reserved non-learnable keys follow the platform.
- **Trackpad:** a two-finger swipe is a wheel stream with inertia. WIN03's one-gesture-one-page rule must hold. A
  second quick swipe during the first one's inertia is the case to test, and it is tuned on a real trackpad.
- Self-update stays a "new version available" link on macOS (no in-place replace).

**D4 — Release.** The AppImage goes on `latest` beside the APK and the MSI. The DMG is built and uploaded as a CI
artifact, and goes on `latest` **only once a person has tested it on a Mac**: VLL tests it himself (2026-11-01; he will say whether his Mac has an Apple or an Intel chip). A88/WIN04's asset
selection gets a case for each platform.

**D4 amended 2026-11-02 (VLL):** the DMG goes on `latest` and on the site like the MSI, before the human pass: see
[DESK02](DESK02-the-mac-app-on-latest-and-on-the-site.md). VLL's Mac pass is still owed.

**D5 — Docs and site** (VLL's rule, as in WIN06): the site's download section and `docs/USER-JOURNEY.md` gain a
Linux line, and a macOS line once the DMG is published.
- For macOS, describe the first launch: the "Open Anyway" path in System Settings > Privacy & Security, and the
  `xattr -cr` fallback only if a tester actually meets the "damaged" message.
- Labels are grepped from the shared strings.

## Acceptance criteria

1. The `desktop` matrix is green on all three OSes. Each run uploads its package and its smoke PNG.
2. Sabotages:
   - a throwing window draw → red on each OS;
   - a wrong data-root branch → its unit test red.
3. The AppImage runs on VLL's Linux machine: install-free launch, Stage with the demo concert, notes, full screen.
4. macOS: CI proves ad-hoc signing and the smoke. A human pass on a Mac is required before D4 publishes the DMG.
5. The docs and site lines from D5, with the label grep.

## Out of scope

Code signing and notarisation (VLL: wait for WIN04's screenshots); Intel Macs (only if a tester has one);
Flatpak/Snap/.deb/.rpm; embedded Studio.
