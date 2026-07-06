// TroubaShare :shared — the "mobile library" (I15). All device-agnostic behaviour lives here in
// commonMain (presenter I12, distribution I13, sync I6, the three expect seams); androidMain holds
// ONLY the three Android seam actuals. Android target NOW; iOS target LATER (fill in the actuals).
plugins {
    alias(libs.plugins.kotlin.multiplatform)
    alias(libs.plugins.android.library)
    alias(libs.plugins.compose.multiplatform)
    alias(libs.plugins.kotlin.compose.compiler)
    alias(libs.plugins.kotlin.serialization)
}

kotlin {
    // Compile/target toolchain. NOT the installed JDK 25: Kotlin 2.2.20 caps its JVM target at 24,
    // so a JDK 25 toolchain trips AGP's Kotlin(24)-vs-Java(25) target-consistency check. JDK 21 is
    // the newest LTS both agree on (see gradle/libs.versions.toml header). The Gradle daemon still
    // runs on the installed JDK 25; only compilation targets 21. Same toolchain in :androidApp.
    jvmToolchain(libs.versions.jdk.get().toInt())

    androidTarget()                         // Android NOW
    // iOS targets. The default hierarchy template creates the shared `iosMain`/`iosTest` source sets
    // over the two targets; the three seam actuals (I15) live in iosMain. On Linux we compile klibs
    // only (cross-compilation); linking the framework + running a simulator is IOS02's macOS job.
    listOf(iosArm64(), iosSimulatorArm64()).forEach { iosTarget ->
        // Export :shared as a framework the Xcode `iosApp` links + embeds (IOS02). `baseName` is the
        // Swift module name (`import Shared`); the entrypoint wrapper is `MainViewControllerKt`.
        // DYNAMIC (not static): Kotlin/Native resolves skiko's heavy transitive system-framework deps
        // (Metal, CoreText, …) into the framework binary at link time, so the app just embeds it —
        // a static framework would force the app to re-declare every one of those linker flags.
        iosTarget.binaries.framework {
            baseName = "Shared"
            isStatic = false
        }
    }

    sourceSets {
        commonMain.dependencies {
            // THE SHARED MOBILE LIBRARY (I15): presenter, downloader, sync, navigation.
            implementation(compose.runtime)
            implementation(compose.foundation)
            implementation(compose.material3)  // Stage UI controls (bottom bar, dialogs)
            implementation(compose.ui)
            implementation(libs.kotlinx.coroutines.core)     // sync client + downloader are coroutine-based (I6)
            implementation(libs.kotlinx.serialization.json)  // JSON on the Studio bridge / manifest diff (I1)
            // generated proto types are consumed from src/commonMain/kotlin/gen (I1, git-ignored)
        }

        commonTest.dependencies {
            implementation(kotlin("test"))  // shared unit tests (bundle loader, …); run on androidUnitTest
            implementation(libs.kotlinx.coroutines.test)  // runTest for the coroutine-based downloader (B03)
        }

        androidMain.dependencies {
            // SEAM ACTUALS ONLY (I15) — nothing here that could be shared:
            //   implementation(libs.androidx.webkit)          // seam 1: WebView host (I10)      [A06]
            //   implementation(libs.androidx.ink)             // seam 2: low-latency wet overlay (I9/I8) [A07]
            implementation(libs.androidx.security.crypto)      // seam 3: EncryptedSharedPreferences (B03)
        }

        // iosMain.dependencies {
        //     // SEAM ACTUALS ONLY (I15): WKWebView (1), PencilKit/Metal (2), FileManager/Keychain (3).
        //     // These are platform APIs reached via cinterop; no extra shared logic.
        // }
    }
}

android {
    namespace = "com.troubashare.shared"
    compileSdk = libs.versions.android.compileSdk.get().toInt()
    defaultConfig {
        minSdk = libs.versions.android.minSdk.get().toInt()
    }
}
