// TroubaStage :desktopApp — the thin desktop (Windows, Linux, macOS) entrypoint (WIN02, DESK01), the analog of :androidApp.
// Compose for Desktop application{} + a per-user MSI via nativeDistributions. Holds no logic of its own:
// a skeleton window (title + build line) now; WIN03 puts the Stage app in it. Depends only on :shared (I14).
import org.jetbrains.compose.desktop.application.dsl.TargetFormat

// WIN08 — the SWT WebView2 code (src/windows/kotlin) and its dependency are added ONLY on a Windows host build.
// Linux/macOS build the same module with neither, keep "Coming soon.", and their app images hold no SWT jar.
val isWindowsHost: Boolean = System.getProperty("os.name").startsWith("Windows")

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
    // WIN08 — the SWT WebView2 implementation (and its META-INF/services registration) compile only on Windows.
    // On Linux/macOS these dirs are never added, nothing imports SWT, and ServiceLoader finds no provider, so
    // StudioHost keeps rendering "Coming soon." (the provider is resolved at runtime, not referenced at compile time).
    if (isWindowsHost) {
        sourceSets.getByName("main").kotlin.srcDir("src/windows/kotlin")
    }
}
if (isWindowsHost) {
    sourceSets.getByName("main").resources.srcDir("src/windows/resources")
}

dependencies {
    implementation(project(":shared"))
    // Compose for Desktop runtime (skiko) for this OS — on macOS for BOTH Mac architectures (DESK01 D3): the universal
    // app's two builds then carry the same app/ folder, and Skiko picks its own runtime by os.arch at launch.
    if (System.getProperty("os.name").startsWith("Mac")) {
        implementation(compose.desktop.macos_arm64)
        implementation(compose.desktop.macos_x64)
    } else {
        implementation(compose.desktop.currentOs)
    }
    implementation(compose.material3)         // currentOs bundles Material 2; the app uses Material 3
    implementation(libs.jna)                   // WIN03: SetThreadExecutionState, GetTickCount64 (already :shared's)
    implementation(libs.jna.platform)
    implementation(libs.zxing.core)            // WIN09: the --win09-qr smoke decodes the window's QR (the screen is :shared's)
    // WIN08 — WebView2 via SWT's Browser(SWT.EDGE), added ONLY on a Windows host build so the Linux/macOS app
    // images carry no SWT jar (the CI jobs assert that). isTransitive = false: the SWT win32 POM declares a
    // self-dependency on `org.eclipse.swt.${osgi.platform}` with an unresolved property, which breaks resolution;
    // SWT win32 is self-contained (its own JNI), so no real transitive deps are dropped. `-PswtVersion=3.133.0`
    // overrides the exact pin only for the B3 ordering sabotage (3.133.0 races the page); default is the catalog's.
    if (isWindowsHost) {
        val swtOverride = findProperty("swtVersion") as String?
        if (swtOverride != null) {
            implementation("org.eclipse.platform:org.eclipse.swt.win32.win32.x86_64:$swtOverride") { isTransitive = false }
        } else {
            implementation(libs.swt.win32) { isTransitive = false }
        }
    }
    testImplementation(kotlin("test"))
    // The real desktop tooltip over Stage (DesktopTooltipUiTest); :shared uses the same library, opted in the same way.
    @OptIn(org.jetbrains.compose.ExperimentalComposeLibrary::class)
    testImplementation(compose.uiTest)
}

// WIN02 D3 — the MSI version MUST increase on every build or Windows will not upgrade in place. CI passes
// -PdesktopVersion=1.0.<run_number> (monotonic per repo, mirrors :androidApp's versionCode); a local build
// stays 1.0.0. MSI ProductVersion fields cap at 255.255.65535, which 1.0.<run_number> clears for a long time.
val desktopVersion: String = (findProperty("desktopVersion") as String?) ?: "1.0.0"

// DESK01 — the brand mark as the window's icon (Main.kt's appIcon), from the same file the Linux packages use.
tasks.processResources {
    from("icons") { include("troubastage.png") }
}

// WIN03 — tests that build the desktop platform open a Storage(): point its data root at the build dir, never a profile.
// WIN07 — and give Skiko its own unpack dir under this build dir, never the shared ~/.skiko: this task and
// :shared:desktopTest run at once under the configuration cache, and on Windows the loser of the unpack race does an
// ATOMIC_MOVE onto a DLL the winner already loaded, which Windows denies (see :shared's build for the full note). One
// dir per task is enough while each runs a single fork (maxParallelForks == 1). exceptionFormat = FULL carries the path.
tasks.withType<Test>().configureEach {
    environment("TROUBASTAGE_HOME", layout.buildDirectory.dir("test-home").get().asFile.path)
    systemProperty("skiko.data.path", layout.buildDirectory.dir("skiko").get().asFile.path)
    // WIN08 — SWT extracts its native DLLs to swt.library.path; pin it under this build dir, never the shared
    // ~/.swt (WIN07's Skiko lesson: concurrent test JVMs racing one per-user unpack dir reddens Windows).
    if (isWindowsHost) systemProperty("swt.library.path", layout.buildDirectory.dir("swt-natives").get().asFile.path)
    testLogging { exceptionFormat = org.gradle.api.tasks.testing.logging.TestExceptionFormat.FULL }
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
            // DESK01 D2 — the Linux app image (wrapped as an AppImage by packaging/linux/make-appimage.sh).
            linux {
                iconFile.set(project.file("icons/troubastage.png"))
            }
            // DESK01 D3 — each Mac build (arm64, x86_64); packaging/macos/make-universal.sh joins the two into one app.
            macOS {
                bundleID = "com.troubastack.troubastage"
                iconFile.set(project.file("icons/troubastage.icns"))
            }
        }
    }
}
