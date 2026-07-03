// TroubaShare — Gradle settings.
//
// The app is a THIN SHELL (I15). The module graph below points only toward the contract (I14):
//   :shared      → the "mobile library" — all shared Kotlin (commonMain) + the 3 Android actual seams
//   :androidApp  → thin Android entrypoint; depends on :shared
//   :iosApp      → thin iOS entrypoint (CMP iOS), enabled LATER once the iOS actuals are filled in
// No client imports another client; :shared embeds the BUILT Studio SPA, never its source (I10/I14).

pluginManagement {
    repositories {
        google()
        gradlePluginPortal()
        mavenCentral()
    }
}

plugins {
    // Auto-provisions the compile JDK (see jvmToolchain(21) in shared/build.gradle.kts) so a clean
    // checkout with only the launcher JDK installed can still build reproducibly — no global tool
    // assumption beyond a JDK. Downloads from Adoptium via the Foojay Disco API on first use.
    id("org.gradle.toolchains.foojay-resolver-convention") version "0.10.0"
}

dependencyResolutionManagement {
    // Fail if a subproject declares its own repositories — one place decides where artifacts come from.
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "TroubaShare"

include(":shared")
include(":androidApp")
// include(":iosApp")   // iOS-later: enable once the iOS seam actuals are real (currently TODO() stubs, I15)
