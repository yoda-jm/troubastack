# IOS02 — iosApp + simulator proof on GitHub's macOS runners (no Apple account)

**Priority:** iOS-track 2 (after IOS01) · **Size:** M/L · **Area:** `app/iosApp`, `.github/workflows`

## Context

GitHub Actions macOS runners have Xcode + simulators preinstalled; **simulator builds
need no signing and no Apple ID** (`CODE_SIGNING_ALLOWED=NO`). That makes a real,
runnable iOS TroubaShare provable in CI without any Mac in the loop. Cost note: macOS
runner minutes bill at 10× on private repos — the job must be `workflow_dispatch` (+ a
weekly cron at most), **not** per-push.

## Changes

1. **`app/iosApp`**: the thin iOS entrypoint per the Compose Multiplatform template —
   an Xcode project (committed; generate with xcodegen or commit the `.xcodeproj`,
   whichever diffs cleaner) whose single screen hosts the shared Compose UI
   (`MainViewController` from `:shared`), wiring the three iOS actuals in (Storage,
   WebViewHost via `UIKitView`; InkOverlay still TODO). Enable `include(":iosApp")`
   equivalents/framework export (`XCFramework` or embedAndSign-less direct framework —
   simulator only).
2. **Workflow** `.github/workflows/ios.yml` (`workflow_dispatch` + weekly):
   macos-latest → checkout → JDK → `./gradlew :shared:linkDebugFrameworkIosSimulatorArm64`
   → `xcodebuild -project app/iosApp/... -sdk iphonesimulator -configuration Debug
   CODE_SIGNING_ALLOWED=NO build` → `xcrun simctl` boot an iPhone, install the .app,
   **inject the demo bundle** (`xcrun simctl get_app_container ... data` + `cp` the
   unpacked `docs/demo` bundle into `files/bundles/wonderwall-demo/` — mirrors the
   Android `run-as` trick), launch, `simctl io screenshot` the concerts list and a
   Stage page, upload the .app + screenshots as artifacts.
3. **Smoke assertions, not just screenshots**: after launch, poll `simctl
   spawn ... log` (or a marker file the app writes on successful bundle load) so the job
   *fails* if Stage crashes — a screenshot of a crash dialog must not pass.
4. README: a short "iOS (simulator)" subsection under the mobile-app section — status
   (simulator-proven, device pending IOS03), artifact location.

## Acceptance criteria

- The dispatched workflow is green on GitHub: framework links, app builds unsigned,
  simulator boots, demo bundle performs, screenshots + .app uploaded as artifacts.
- Screenshot shows the same Wonderwall demo page the Android shots show (modulo
  platform chrome).
- No signing identities, provisioning profiles, or Apple IDs anywhere in the workflow.
- Ubuntu CI unaffected (the ios workflow is separate and manual).

## Out of scope

- Physical devices, TestFlight/App Store (IOS03); ink overlay; performance tuning;
  making the webview editor part of the smoke test (Stage-only is the v1 bar).
