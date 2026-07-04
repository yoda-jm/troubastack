# iosApp — thin iOS entrypoint

The thin iOS shell (SwiftUI hosting Compose Multiplatform) that mounts the shared `:shared` module
(I15). Holds **no** logic beyond wiring the shared app in — the Concerts list + Stage UI come from
`:shared`'s `MainViewController()`, exported in the `Shared` static framework. The three iOS `actual`
seams (WKWebView, storage/Keychain, ink=TODO) live in `:shared/iosMain`, not here.

## Layout

- `project.yml` — [xcodegen](https://github.com/yonyz/XcodeGen) spec (committed instead of a
  `.xcodeproj`, which diffs cleanly). Run `xcodegen generate` to produce `iosApp.xcodeproj`
  (git-ignored; regenerated in CI).
- `Sources/iOSApp.swift` — `@main` SwiftUI `App`.
- `Sources/ContentView.swift` — a `UIViewControllerRepresentable` that mounts
  `MainViewControllerKt.MainViewController()`.

## Building (macOS only)

No Mac exists in the dev loop, so this is **built + proven in CI**, not locally
(`.github/workflows/ios.yml`, `workflow_dispatch` + weekly — never per-push; macOS runner minutes
bill 10×). Simulator builds are **unsigned** (`CODE_SIGNING_ALLOWED=NO`) — no Apple ID, no
provisioning. To build on a Mac by hand:

```sh
cd app
./gradlew :shared:linkDebugFrameworkIosSimulatorArm64   # link the Kotlin framework first
cd iosApp && xcodegen generate
xcodebuild -project iosApp.xcodeproj -scheme iosApp \
  -sdk iphonesimulator -configuration Debug CODE_SIGNING_ALLOWED=NO build
```

Physical devices / TestFlight / App Store are IOS03.
