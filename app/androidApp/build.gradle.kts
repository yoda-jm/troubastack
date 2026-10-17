// TroubaStage :androidApp — the thin Android entrypoint (I15). Holds NO logic of its own beyond
// wiring the shared app in and (later) handing the three Android seam actuals to it. Everything it
// can share, it shares — via :shared. Depends only toward the contract (I14).
plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.compose.multiplatform)
    alias(libs.plugins.kotlin.compose.compiler)
    alias(libs.plugins.kotlin.serialization)  // @Serializable ktor DTOs in HttpTransport (B03)
}

kotlin {
    // Same LTS compile toolchain as :shared — JDK 21, not the installed 25 (Kotlin caps jvmTarget
    // at 24, so a 25 toolchain fails AGP's Java/Kotlin target-consistency check; see the catalog note).
    jvmToolchain(libs.versions.jdk.get().toInt())
}

// A84 — the commit this build comes from, as the server reports its own (`git describe --always --dirty`).
// Read through providers.exec, so the configuration cache re-runs git and a new commit is never served a cached
// hash; correct in CI's shallow clone (describe --always falls back to the short hash). No git, or not a
// checkout (a source tarball) → "unknown", never a build failure.
val gitDescribe: String = runCatching {
    providers.exec {
        commandLine("git", "describe", "--always", "--dirty")
        workingDir = rootDir
        isIgnoreExitValue = true
    }.standardOutput.asText.get().trim()
}.getOrDefault("").ifBlank { "unknown" }

// A88 §0 — ONE signing key for every build that may update another: CI decodes the TROUBASTAGE_DEBUG_KEYSTORE
// secret (the dev machine's debug keystore, Android's standard debug passwords) to a temp file and points this at
// it; local builds use that same file by default. Absent → AGP's own debug key, as before.
val stableDebugKeystore: String? = providers.environmentVariable("TROUBASTAGE_DEBUG_KEYSTORE_FILE").orNull
// A88 §3 — PackageInstaller refuses a LOWER versionCode: CI numbers its builds by run (monotonic per repo); a
// dev build stays 1, so a CI update always installs over it (and going back is `adb install -r -d`).
val ciVersionCode: Int? = providers.environmentVariable("GITHUB_RUN_NUMBER").orNull?.toIntOrNull()

android {
    namespace = "com.troubastack.app"
    compileSdk = libs.versions.android.compileSdk.get().toInt()

    defaultConfig {
        applicationId = "com.troubastack.app"
        minSdk = libs.versions.android.minSdk.get().toInt()
        targetSdk = libs.versions.android.targetSdk.get().toInt()
        versionCode = ciVersionCode ?: 1   // A88 §3 (A84 left it for this task)
        versionName = "0.1.0+$gitDescribe" // A84: `dumpsys package` answers which build this is
    }

    signingConfigs {
        getByName("debug") {
            if (stableDebugKeystore != null) {
                storeFile = file(stableDebugKeystore)
                storePassword = "android"
                keyAlias = "androiddebugkey"
                keyPassword = "android"
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
        }
    }
}

dependencies {
    implementation(project(":shared"))
    // Compose Multiplatform UI (resolves to androidx.compose on Android). Placeholder screen only.
    implementation(compose.runtime)
    implementation(compose.foundation)
    implementation(compose.material3)
    implementation(compose.ui)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.core.ktx)  // window insets control for immersive mode
    implementation(libs.kotlinx.coroutines.core)  // Connect/download run in coroutine scopes
    // B03 distribution transport — the ktor impl of :shared's ManifestTransport (app DI, not a seam).
    implementation(libs.ktor.client.core)
    implementation(libs.ktor.client.okhttp)
    implementation(libs.ktor.client.content.negotiation)
    implementation(libs.ktor.serialization.kotlinx.json)
    implementation(libs.kotlinx.serialization.json)
    // A53: QR invite scanner — CameraX preview/analysis + ZXing decode (offline, no Play Services).
    implementation(libs.androidx.camera.core)
    implementation(libs.androidx.camera.camera2)
    implementation(libs.androidx.camera.lifecycle)
    implementation(libs.androidx.camera.view)
    implementation(libs.zxing.core)
    // A47: :androidApp's FIRST unit tests. src/test runs on the JVM (no device) via :androidApp:test /
    // testDebugUnitTest — covers the Android-only pure functions (sessionCookieFor, safeSegment) that had
    // no way to run a test at all. Same kotlin-test the shared module uses.
    testImplementation(kotlin("test"))
}
