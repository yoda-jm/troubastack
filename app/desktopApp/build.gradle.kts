// TroubaStage :desktopApp — the thin desktop (Windows) entrypoint (WIN02), the analog of :androidApp.
// Compose for Desktop application{} + a per-user MSI via nativeDistributions. Holds no logic of its own:
// a skeleton window (title + build line) now; WIN03 puts the Stage app in it. Depends only on :shared (I14).
import org.jetbrains.compose.desktop.application.dsl.TargetFormat

plugins {
    // No version: the Kotlin Gradle Plugin is already on the build classpath (the KMP modules bring it), so
    // :desktopApp applies kotlin("jvm") WITHOUT a version — a version request is refused as "already on the
    // classpath". Compose + its compiler plugin resolve by alias as in the sibling modules.
    kotlin("jvm")
    alias(libs.plugins.compose.multiplatform)
    alias(libs.plugins.kotlin.compose.compiler)
}

kotlin {
    // Same LTS compile toolchain as :shared / :androidApp — JDK 21 (see the catalog note).
    jvmToolchain(libs.versions.jdk.get().toInt())
}

dependencies {
    implementation(project(":shared"))
    implementation(compose.desktop.currentOs) // Compose for Desktop runtime (skiko) for this OS
    implementation(compose.material3)         // currentOs bundles Material 2; the app uses Material 3
    implementation(libs.jna)                   // WIN03: SetThreadExecutionState, GetTickCount64 (already :shared's)
    implementation(libs.jna.platform)
    testImplementation(kotlin("test"))
}

// WIN02 D3 — the MSI version MUST increase on every build or Windows will not upgrade in place. CI passes
// -PdesktopVersion=1.0.<run_number> (monotonic per repo, mirrors :androidApp's versionCode); a local build
// stays 1.0.0. MSI ProductVersion fields cap at 255.255.65535, which 1.0.<run_number> clears for a long time.
val desktopVersion: String = (findProperty("desktopVersion") as String?) ?: "1.0.0"

// WIN03 — tests that build the desktop platform open a Storage(): point its data root at the build dir, never a profile.
tasks.withType<Test>().configureEach {
    environment("TROUBASTAGE_HOME", layout.buildDirectory.dir("test-home").get().asFile.path)
}

compose.desktop {
    application {
        mainClass = "com.troubastack.desktop.MainKt"
        nativeDistributions {
            targetFormats(TargetFormat.Msi)
            // WIN03 — the JDK modules the packaged runtime needs beyond Compose's default: the first three are what
            // `./gradlew :desktopApp:suggestRuntimeModules` reports for the shared app (ktor, coroutines); jdk.crypto.ec
            // is loaded as a security provider, which jdeps cannot see, and HTTPS needs it (servers, GitHub for WIN04).
            modules("java.instrument", "java.management", "jdk.unsupported", "jdk.crypto.ec")
            packageName = "TroubaStage"        // the Android app's label
            packageVersion = desktopVersion
            description = "TroubaStage"
            vendor = "TroubaStack"
            windows {
                // Per-user install: no admin rights, no UAC prompt.
                perUserInstall = true
                // Fixed upgrade UUID: a newer MSI (higher packageVersion) upgrades in place, leaving ONE entry.
                // Generated once with `cat /proc/sys/kernel/random/uuid`; NEVER change it or upgrades break.
                upgradeUuid = "bbb66ebc-f242-48c2-8546-22a9975075eb"
                menuGroup = "TroubaStage"      // Start-menu entry
                shortcut = true                // desktop shortcut
                // No dirChooser: a per-user install must not let the user pick an admin-only folder.
                // The brand mark as a multi-size .ico (see icons/README.md for the exact build command).
                iconFile.set(project.file("icons/troubastage.ico"))
            }
        }
    }
}
