// TroubaStage — root Gradle build.
//
// The root project builds nothing itself; it only declares the plugins the modules apply so
// their versions resolve from one place (the version catalog, gradle/libs.versions.toml).
// Each `apply false` makes the plugin available to subprojects without applying it here.
plugins {
    alias(libs.plugins.android.application) apply false
    alias(libs.plugins.android.library) apply false
    alias(libs.plugins.kotlin.multiplatform) apply false
    alias(libs.plugins.kotlin.android) apply false
    alias(libs.plugins.kotlin.compose.compiler) apply false
    alias(libs.plugins.kotlin.serialization) apply false
    alias(libs.plugins.compose.multiplatform) apply false
}
