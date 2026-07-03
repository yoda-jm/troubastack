# A01 — Wire the Gradle build (KMP compiles, CI gates it)

**Priority:** A-track 1 (gates every other A-task) · **Size:** M · **Area:** `app/`

## Context

`app/` is a *structural scaffold*: the directory layout and the three `expect/actual`
seams (invariant I15) exist, but `app/settings.gradle.kts` and `app/build.gradle.kts` are
**fully commented out on purpose** ("Do not run gradle yet"), there is no Gradle wrapper,
and every function body is `TODO()`. The commented stubs contain the intended module
graph and dependency notes — treat them as the spec and replace them with a real build.

Intended graph (from the stubs):
- `:shared` — KMP library; `commonMain` (presenter, sync, distribution, seam `expect`s)
  plus `androidMain`/`iosMain` seam `actual`s. Compose Multiplatform for shared UI.
- `:androidApp` — thin Android entrypoint depending on `:shared`.
- `:iosApp` — **excluded for now** (iOS-later; all iOS actuals are `TODO` stubs).

## Changes

1. Write real `app/settings.gradle.kts` (rootProject `TroubaShare`, pluginManagement +
   dependencyResolutionManagement repos as outlined, `include(":shared", ":androidApp")`,
   iosApp left commented with the iOS-later note).
2. Add a version catalog `app/gradle/libs.versions.toml`. Use **current stable** versions
   of: Kotlin Multiplatform, Compose Multiplatform, Android Gradle Plugin, coroutines,
   kotlinx-serialization. The repo toolchain doc says JDK 25+; if the current AGP does not
   yet accept a JDK 25 toolchain, pin the Kotlin/Java toolchain to the newest LTS it does
   accept and leave a comment — do not fight the toolchain.
3. Write `app/build.gradle.kts` + `app/shared/build.gradle.kts` + a new
   `app/androidApp/` module (`build.gradle.kts`, `AndroidManifest.xml`, and a minimal
   `MainActivity` showing a Compose placeholder screen — e.g. the TroubaShare name and a
   "Stage" button that does nothing yet). `:shared`: `androidTarget()` only for now;
   keep the `sourceSets` seam comments from the stub (they encode I15).
4. Commit the Gradle wrapper (`gradle/wrapper/*`, `gradlew`, `gradlew.bat`) so builds are
   reproducible. Pin a wrapper version compatible with your chosen AGP.
5. The existing Kotlin stubs must compile **unmodified where possible**: `TODO()` bodies
   compile fine. If a signature is genuinely uncompilable, fix it minimally and note it in
   the PR. Do **not** add any platform-specific file beyond the existing six seam actuals
   (I15 — that's the review test for this whole track).
6. Update the root `Makefile` `app` target (currently `cd app && ./gradlew build`) to the
   real entry: `cd app && ./gradlew :shared:check :androidApp:assembleDebug`, and remove
   the "deferred" wording for `app` in the help text.
7. Add an `android` job to `.github/workflows/ci.yml`: checkout, `actions/setup-java`
   (Temurin, the version you pinned), then the same gradlew command. GitHub's
   `ubuntu-latest` runners have the Android SDK preinstalled; add
   `ANDROID_HOME` handling only if the build actually fails without it. Cache Gradle via
   `gradle/actions/setup-gradle`.

## Acceptance criteria

- `cd app && ./gradlew :shared:check :androidApp:assembleDebug` succeeds locally from a
  clean checkout (wrapper included, no global tool assumptions beyond a JDK).
- The `android` CI job is green on GitHub.
- `app/shared/src/{android,ios}Main` still contain **only** the six seam files (I15).
- The debug APK installs and shows the placeholder screen on an emulator (screenshot in
  the PR if you have an emulator; otherwise state that only assemble was verified).

## Out of scope

- Implementing any seam or any `TODO()` body; iOS target; proto codegen; app store signing.
