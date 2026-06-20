// TroubaShare :shared — Gradle build (COMMENTED STUB).
//
// STRUCTURAL scaffold only (see README.md). This outlines the intended KMP/CMP setup for
// the shared "mobile library" module. Everything is commented so `gradle` cannot try a real
// build that would fail (no plugins resolved, no versions pinned). Derive the real build later.
//
// Toolchains (docs/design/06-tech-stack.md): JDK 25+, Kotlin Multiplatform, Compose Multiplatform.
//
// plugins {
//     // kotlin("multiplatform")                 // Kotlin Multiplatform
//     // id("org.jetbrains.compose")             // Compose Multiplatform (shared UI / navigation)
//     // id("com.android.library")               // Android target of :shared
//     // TODO(scaffold): pin plugin versions in settings.gradle.kts / version catalog
// }
//
// kotlin {
//     // androidTarget()                          // Android NOW
//     // iosArm64(); iosSimulatorArm64()          // iOS LATER — just fill in the actuals (I15)
//
//     sourceSets {
//         // commonMain.dependencies {
//         //     // THE SHARED MOBILE LIBRARY (I15): presenter, downloader, sync, navigation.
//         //     // implementation(compose.runtime); implementation(compose.foundation)
//         //     // implementation(libs.ktor.client.core)        // sync client over WebSocket (I6)
//         //     // implementation(libs.kotlinx.coroutines.core)
//         //     // generated proto types are consumed from src/commonMain/kotlin/gen (I1)
//         // }
//         //
//         // androidMain.dependencies {
//         //     // SEAM ACTUALS ONLY (I15) — nothing here that could be shared:
//         //     // implementation(libs.androidx.webkit)         // seam 1: WebView host (I10)
//         //     // implementation(libs.androidx.ink)            // seam 2: low-latency wet overlay (I9/I8)
//         //     // implementation(libs.androidx.security.crypto)// seam 3: storage / secure prefs
//         // }
//         //
//         // iosMain.dependencies {
//         //     // SEAM ACTUALS ONLY (I15): WKWebView (1), PencilKit/Metal (2), FileManager/Keychain (3).
//         //     // These are platform APIs reached via cinterop; no extra shared logic.
//         // }
//     }
// }
//
// android {
//     // namespace = "com.troubashare.shared"
//     // compileSdk = TODO(); defaultConfig { minSdk = TODO() }
// }

// TODO(scaffold): replace with a real build.gradle.kts when wiring the build. Do NOT run gradle yet.
