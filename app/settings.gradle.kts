// TroubaShare — Gradle settings (COMMENTED STUB).
//
// This is a STRUCTURAL scaffold, not a wired build (see README.md). The lines below
// outline the intended Kotlin/Compose Multiplatform module graph (I15: app is a thin
// shell). They are commented out deliberately so that `gradle` will not attempt — and
// fail — a real configuration. Uncomment + pin versions when the build is derived later.
//
// rootProject.name = "TroubaShare"
//
// pluginManagement {
//     repositories {
//         google()
//         gradlePluginPortal()
//         mavenCentral()
//     }
// }
//
// dependencyResolutionManagement {
//     repositories {
//         google()
//         mavenCentral()
//     }
// }
//
// // The module graph. Dependencies point only toward the contract (I14):
// //   :shared      → the "mobile library" — all shared Kotlin (commonMain) + the 3 actual seams
// //   :androidApp  → thin Android entrypoint; depends on :shared
// //   :iosApp      → thin iOS entrypoint (CMP iOS, derived later); depends on :shared
// // No client imports another client; :shared embeds the BUILT Studio SPA, never its source (I10/I14).
// include(":shared")
// include(":androidApp")
// // include(":iosApp")   // iOS-later: enable once the iOS actuals are filled in (I15)

// TODO(scaffold): replace this file with a real settings.gradle.kts when wiring the build.
