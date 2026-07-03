// TroubaShare :androidApp — the thin Android entrypoint (I15). Holds NO logic of its own beyond
// wiring the shared app in and (later) handing the three Android seam actuals to it. Everything it
// can share, it shares — via :shared. Depends only toward the contract (I14).
plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.compose.multiplatform)
    alias(libs.plugins.kotlin.compose.compiler)
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
}
