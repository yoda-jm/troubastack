// TroubaShare :androidApp — the thin Android entrypoint (I15). Holds NO logic of its own beyond
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

android {
    namespace = "com.troubashare.app"
    compileSdk = libs.versions.android.compileSdk.get().toInt()

    defaultConfig {
        applicationId = "com.troubashare.app"
        minSdk = libs.versions.android.minSdk.get().toInt()
        targetSdk = libs.versions.android.targetSdk.get().toInt()
        versionCode = 1
        versionName = "0.1.0"
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
