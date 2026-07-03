# IOS01 — Enable iOS targets + fill the Storage/WebView actuals (Linux-verifiable)

**Priority:** iOS-track 1 · **Size:** M/L · **Area:** `app/shared`

## Reality check (read before estimating)

No Mac exists in this loop. What that means, honestly:

- **On Linux we can compile, not link**: recent Kotlin supports cross-compiling *klibs*
  for Apple targets on any host, so `iosArm64()`/`iosSimulatorArm64()` compilation
  (including the `iosMain` actuals) can be type-checked locally and in the ubuntu CI
  job. Linking a framework/app and running a simulator require macOS — that's IOS02's
  GitHub macOS runner. If the current Kotlin version refuses klib cross-compilation on
  Linux, scope this task's *verification* down to "compiles in IOS02's mac job" and say
  so — do not fight the toolchain.
- **No Apple credentials are needed** for anything in IOS01/IOS02 (simulator builds are
  unsigned). Physical devices / App Store are IOS03's decision stub.

## Changes

1. **Gradle**: uncomment `iosArm64(); iosSimulatorArm64()` in `shared/build.gradle.kts`;
   add the iOS source-set wiring the scaffold comments describe. Keep the androidApp
   path completely unaffected.
2. **Storage actual (iosMain)** — the meaty one:
   - `bundlesDir()`/`tempDir()`: `NSFileManager` Documents/Caches paths.
   - Secrets: Keychain (`SecItemAdd`/`CopyMatching` via the Security framework) — small
     wrapper, no third-party dependency.
   - `unpackBundle`: **there is no zip API in Kotlin/Native's stdlib or the Apple SDK.**
     Smallest honest path: a pure-Kotlin zip reader (parse the end-of-central-directory
     + central directory + local headers — the format is simple and our zips are our
     own) with DEFLATE via the platform `zlib` (`platform.zlib`, present on Apple
     targets; raw inflate with `inflateInit2(windowBits = -15)`). Port the zip-slip and
     size-cap guards from the Android actual verbatim. Budget ~200–300 lines + tests.
     If a maintained tiny KMP zip library exists and is genuinely lighter than this,
     justify it in the PR — no kitchen-sink deps.
3. **WebViewHost actual (iosMain)**: `WKWebView` + `WKScriptMessageHandler` (web→shell)
   + `evaluateJavaScript` (shell→web), mirroring the Android actual's surface (`view`
   exposed for Compose interop, `State` callbacks, same handshake JSON). Compose
   Multiplatform iOS embeds UIKit views via `UIKitView` — keep that glue in `iosApp`
   (IOS02), not in the seam.
4. **InkOverlay actual stays `TODO("iOS-later")`** — it is gated behind A07, which is
   gated behind the tablet spike. Do not touch.
5. **Shared-code portability sweep**: anything commonMain that leaned on JVM types must
   compile for K/N now (the A-track was written portable — verify, fix the stragglers).
   The zip logic moved into common? No — it stays per-platform behind the seam (Android
   keeps `java.util.zip`).
6. **CI**: add iOS klib compilation to the android job (`:shared:compileKotlinIosArm64`
   or the equivalent metadata task) if Linux cross-compilation works; else defer to IOS02.

## Acceptance criteria

- `./gradlew :shared:compileKotlinIosArm64 :shared:compileKotlinIosSimulatorArm64`
  succeeds on this Linux machine (or the documented fallback: green in IOS02's mac job).
- The Kotlin zip reader has commonTest-style unit tests runnable on Android/JVM too
  (parse + inflate + zip-slip rejection + size cap) — structure it so the *parser* is
  testable off-device even though it ships in iosMain (e.g. parser in commonMain
  internal, inflate behind a tiny expect).
- Android app completely unaffected: `:shared:check :androidApp:assembleDebug` green.
- I15 intact: still exactly three seams; iOS platform code only in the three iosMain
  actual files (+ their tests).

## Out of scope

- The `iosApp` entrypoint, xcodeproj, simulator runs (IOS02); devices/App Store (IOS03);
  InkOverlay (A07).
