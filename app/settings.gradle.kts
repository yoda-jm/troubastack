// TroubaStage — Gradle settings.
//
// The app is a THIN SHELL (I15). The module graph below points only toward the contract (I14):
//   :shared      → the "mobile library" — all shared Kotlin (commonMain) + the Android & iOS seams
//   :androidApp  → thin Android entrypoint; depends on :shared
// The iOS entrypoint (app/iosApp) is an Xcode project, NOT a Gradle module: it links the `Shared`
// dynamic framework that :shared exports (IOS02). So there is no `:iosApp` Gradle subproject to
// include — Xcode consumes the framework Gradle links.
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

rootProject.name = "TroubaStage"

include(":shared")
include(":androidApp")
// No include(":iosApp") — iosApp is an Xcode project consuming the :shared framework (see above).
