// TroubaStage :desktopApp — the thin desktop (Windows, Linux, macOS) entrypoint (WIN02, DESK01), the analog of :androidApp.
// Compose for Desktop application{} + a per-user MSI via nativeDistributions. Holds no logic of its own:
// a skeleton window (title + build line) now; WIN03 puts the Stage app in it. Depends only on :shared (I14).
import org.jetbrains.compose.desktop.application.dsl.TargetFormat
import java.util.jar.Manifest
import java.util.zip.ZipFile

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

// WIN08 S2.8 — SWT's licences, shipped in the Windows app image only (`resources/licences/`, read by Parameters → About).
// SWT is EPL-2.0 and carries third-party content with notices of its own (about.html + about_files/: libjpeg, PuTTY's
// zlib code, Microsoft's WebView2 loader under BSD). This copies those from the SWT jar the build RESOLVED (so they are
// that version's), adds the EPL-2.0 text, and writes SWT-NOTICE.txt naming the exact version and where its source is
// published (EPL-2.0 §3.2). Registered only on a Windows host — the same switch that adds SWT — and put under
// appResourcesRootDir's `windows/` folder, so the Linux and macOS images get none of it. WIN08_SABOTAGE
// licencewrongversion / licencenofile (a CI measurement run only) breaks it so the MSI check can be shown to red.
abstract class SwtLicences : DefaultTask() {
    @get:InputFiles abstract val swtJar: ConfigurableFileCollection
    @get:Input abstract val swtModule: Property<String>
    @get:Input abstract val swtVersion: Property<String>
    @get:InputFile abstract val eplText: RegularFileProperty
    @get:Input abstract val sabotage: Property<String>
    @get:OutputDirectory abstract val outDir: DirectoryProperty

    @TaskAction
    fun write() {
        val dir = outDir.get().asFile.resolve("windows/licences").apply { deleteRecursively(); mkdirs() }
        val jar = swtJar.singleFile
        ZipFile(jar).use { zip ->
            val manifest = zip.getInputStream(zip.getEntry("META-INF/MANIFEST.MF")).use { Manifest(it) }
            val bundleVersion = manifest.mainAttributes.getValue("Bundle-Version") ?: error("no Bundle-Version in $jar")
            // about.html and every about_files/ entry, byte for byte (the CI check compares them with the jar's).
            val copied = zip.entries().asSequence().filter { !it.isDirectory && (it.name == "about.html" || it.name.startsWith("about_files/")) }.toList()
            check(copied.any { it.name == "about.html" }) { "no about.html in $jar" }
            for (e in copied) dir.resolve(e.name).apply { parentFile.mkdirs() }.writeBytes(zip.getInputStream(e).use { it.readBytes() })
            val aboutHtml = dir.resolve("about.html").readText()
            // A101 §4 — the notice's third-party sentence is written here; fail if this SWT's about.html no longer
            // names each component it names, so an SWT bump can't leave the sentence stale.
            for ((_, named) in THIRD_PARTY) check(aboutHtml.contains(named)) {
                "$jar's about.html no longer names \"$named\", which SWT-NOTICE.txt's third-party sentence names: re-read it and update THIRD_PARTY"
            }
            dir.resolve("SWT-about.txt").writeText(htmlToText(aboutHtml))
            val wrong = sabotage.get() == "licencewrongversion"
            dir.resolve("SWT-NOTICE.txt").writeText(notice(
                if (wrong) "3.0.0" else swtVersion.get(), if (wrong) "3.0.0.v00000000-0000" else bundleVersion))
        }
        if (sabotage.get() != "licencenofile") eplText.get().asFile.copyTo(dir.resolve("EPL-2.0.txt"))
    }

    private fun notice(version: String, bundleVersion: String): String {
        val module = swtModule.get()
        val sources = "https://repo1.maven.org/maven2/org/eclipse/platform/$module/$version/$module-$version-sources.jar"
        return """
            |SWT in TroubaStage for Windows
            |
            |TroubaStage for Windows shows TroubaStudio with SWT, the Eclipse Standard Widget Toolkit:
            |  Component: org.eclipse.platform:$module
            |  Version: $version
            |  Bundle-Version: $bundleVersion
            |
            |SWT is made available under the Eclipse Public License 2.0. Its text is in EPL-2.0.txt, beside this file.
            |
            |The source code of this exact version of SWT is published at:
            |  $sources
            |and in the SWT project's repository:
            |  https://github.com/eclipse-platform/eclipse.platform.swt
            |
            |SWT itself includes third-party content: ${THIRD_PARTY.joinToString(", ", limit = THIRD_PARTY.size - 1, truncated = "") { it.first }}and ${THIRD_PARTY.last().first}.
            |SWT's own notice for it is about.html (also as plain text, SWT-about.txt), with the texts it refers to in
            |about_files/. They sit beside this file, copied unchanged from the SWT jar in this app.
            |""".trimMargin()
    }

    private companion object {
        /** The notice's third-party sentence (its words) and what this SWT's about.html must name for each. */
        val THIRD_PARTY = listOf(
            "the Independent JPEG Group's JPEG software" to "Independent JPEG Group",
            "code from PuTTY" to "PuTTY",
            "Microsoft's WebView2 loader" to "WebView2",
        )
    }

    /** about.html as readable text: block ends become line breaks, a link keeps its target ("text (url)" when the text
     *  is not the url itself — A101 §4), tags go, the few entities it uses are decoded. */
    private fun htmlToText(html: String): String = html
        .replace(Regex("(?is)<head.*?</head>"), "")
        .replace(Regex("(?is)<a\\s[^>]*?href=\"([^\"]+)\"[^>]*>(.*?)</a>")) { m ->
            val url = m.groupValues[1].trim()
            val text = m.groupValues[2].replace(Regex("<[^>]*>"), "").trim()
            if (text.isEmpty() || text == url) url else "$text ($url)"
        }
        .replace(Regex("(?i)<br\\s*/?>"), "\n")
        .replace(Regex("(?i)</(p|h[1-6]|li|div|tr|pre|ul|ol)>"), "\n\n")
        .replace(Regex("<[^>]*>"), "")
        .replace("&quot;", "\"").replace("&rsquo;", "\u2019").replace("&lsquo;", "\u2018").replace("&nbsp;", " ")
        .replace("&lt;", "<").replace("&gt;", ">").replace("&amp;", "&")
        .lines().joinToString("\n") { it.trimEnd() }
        .replace(Regex("\n{3,}"), "\n\n").trim() + "\n"
}

if (isWindowsHost) {
    val swtModuleName = "org.eclipse.swt.win32.win32.x86_64"
    val swtArtifacts = configurations.named("runtimeClasspath").flatMap { it.incoming.artifacts.resolvedArtifacts }
    fun swtOnly(all: Set<org.gradle.api.artifacts.result.ResolvedArtifactResult>) =
        all.filter { (it.id.componentIdentifier as? org.gradle.api.artifacts.component.ModuleComponentIdentifier)?.module == swtModuleName }
    tasks.register<SwtLicences>("swtLicences") {
        swtJar.from(swtArtifacts.map { all -> swtOnly(all).map { it.file } })
        swtModule.set(swtModuleName)
        swtVersion.set(swtArtifacts.map { all ->
            swtOnly(all).map { (it.id.componentIdentifier as org.gradle.api.artifacts.component.ModuleComponentIdentifier).version }.single()
        })
        eplText.set(layout.projectDirectory.file("licences/EPL-2.0.txt"))
        sabotage.set(providers.environmentVariable("WIN08_SABOTAGE").orElse(""))
        outDir.set(layout.buildDirectory.dir("swt-licences"))
    }
}

// A101 — the open-source notices of every component this image ships, generated from the resolved runtimeClasspath
// into `common/notices/` (every OS), read by About → Licences. The app-resources root is ONE folder, so a Sync task
// assembles it: these notices, plus S2.8's `windows/licences/` on a Windows host.
val desktopPlatform = when {
    isWindowsHost -> "windows"
    System.getProperty("os.name").startsWith("Mac") -> "macos"
    else -> "linux"
}
val openSourceNotices = troubastack.notices.NoticesSetup.register(
    project, "openSourceNotices", configurations.getByName("runtimeClasspath"), arrayOf("jar"), desktopPlatform,
    "common/notices", rootProject.file("licences"),
)
val appResources = tasks.register<Sync>("appResources") {
    from(openSourceNotices.flatMap { it.outputDir })
    if (isWindowsHost) from(tasks.named<SwtLicences>("swtLicences").flatMap { it.outDir })
    // A101 — Linux: the fontconfig file without bitmap fonts (LinuxFontConfig), in the Linux image only.
    from(layout.projectDirectory.dir("fonts")) { into("linux") }
    into(layout.buildDirectory.dir("app-resources"))
}

compose.desktop {
    application {
        mainClass = "com.troubastack.desktop.MainKt"
        nativeDistributions {
            targetFormats(TargetFormat.Msi)
            // WIN08 S2.8 + A101 — the assembled app-resources root (above): `common/` reaches every image, `windows/`
            // (SWT's licences) the Windows image only.
            appResourcesRootDir.set(layout.dir(appResources.map { it.destinationDir }))
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
