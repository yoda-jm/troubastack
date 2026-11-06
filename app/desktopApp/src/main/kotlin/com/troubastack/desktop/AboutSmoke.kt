package com.troubastack.desktop

import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.WindowPosition
import androidx.compose.ui.window.application
import androidx.compose.ui.window.rememberWindowState
import com.troubastack.app.App
import com.troubastack.app.OPEN_SOURCE_LINE
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.ui.ThemePref
import com.troubastack.shared.ui.TroubaTheme
import java.io.File
import javax.swing.SwingUtilities

/**
 * WIN08 S2.8 + A101 — the About smoke (`--about-smoke <outDir>`), on each packaged desktop image. The app's own window and App(),
 * driven through the window's accessibility tree, as a person would go: the account chip → ⚙ Parameters → About, then
 * Licences. Each check prints a `S28-CHECK name: PASS|FAIL` or `A101-CHECK name: PASS|FAIL` line to about-smoke.log (in
 * [outDir]). Where the image ships SWT (Windows), S2.8's two, unchanged:
 *  - about-line-shows-version: the About line, read through the tree, names the version the shipped SWT-NOTICE.txt
 *    states, and that is the version of the SWT jar this app runs with;
 *  - licences-screen-shows-files: the SWT entry's screen shows the shipped notice and the "Eclipse Public License - v 2.0"
 *    text.
 * Elsewhere (Linux, macOS) A101's about-row-shows: About's row is "TroubaStage is built with open-source software." and
 * Licences opens. Then, everywhere, A101's:
 *  - about-ktor-entry: the Ktor entry shows its components (io.ktor:ktor-…) and the Apache License;
 *  - about-java-runtime-entry: the Java runtime entry shows the GPL with its "CLASSPATH" exception (the image's legal/).
 * Then opens every entry once and prints `A101-TIME largest-entry-open`. Writes about.png and licences*.png; exits 0 if
 * every check passed, else 1.
 */
internal fun runAboutSmoke(outDir: File) {
    outDir.mkdirs()
    val log = File(outDir, "about-smoke.log").apply { writeText("") }
    fun line(s: String) { println(s); runCatching { log.appendText(s + "\n") } }
    if (isLinux()) {
        // A101 — the fontconfig file this run uses (LinuxFontConfig), and a byte copy of it: the shipped one lives in the
        // AppImage's mount, gone once the app exits, so the CI checks the fonts through the copy. (System.getenv is
        // the environment at launch: Java reads it once, before main's setenv.)
        val f = LinuxFontConfig.inUse
        val copy = f?.let { runCatching { it.copyTo(File(outDir, "fontconfig-in-use.conf"), overwrite = true) }.getOrNull() }
        val sha = copy?.let { java.security.MessageDigest.getInstance("SHA-256").digest(it.readBytes()).joinToString("") { b -> "%02x".format(b) } }
        line("A101-FONTCONFIG in-use=${f?.path ?: "none"} how=[${LinuxFontConfig.how}] " +
            "FONTCONFIG_FILE-at-launch=${System.getenv("FONTCONFIG_FILE") ?: "unset"} copy=${copy?.name ?: "none"} sha256=${sha ?: "none"}")
    }
    Thread { try { Thread.sleep(150_000) } catch (_: InterruptedException) { return@Thread }
        line("about-smoke: BACKSTOP (>150s)"); Runtime.getRuntime().halt(1) }.apply { isDaemon = true; start() }
    val storage = Storage()
    application {
        val windowState = rememberWindowState(width = 1100.dp, height = 760.dp, position = WindowPosition(0.dp, 0.dp))
        val platform = remember { DesktopAppPlatform(storage, windowState) }
        Window(onCloseRequest = ::exitApplication, title = "TroubaStage", state = windowState, onKeyEvent = { windowKey(it, platform) }) {
            LaunchedEffect(Unit) {
                platform.awtWindow = window
                Thread { aboutFlow(window, outDir, ::line) }.apply { isDaemon = true; start() }
            }
            TroubaTheme(dark = false) { App(platform, themePref = ThemePref.LIGHT, onThemePref = {}) }
        }
    }
}

private fun aboutFlow(frame: java.awt.Window, outDir: File, line: (String) -> Unit) {
    val results = LinkedHashMap<String, Boolean>()
    fun mark(prefix: String, name: String, ok: Boolean, detail: String) {
        results[name] = ok; line("$prefix-CHECK $name: ${if (ok) "PASS" else "FAIL"} — $detail")
    }
    fun capture(name: String) = runCatching {
        val b = frame.bounds
        javax.imageio.ImageIO.write(java.awt.Robot().createScreenCapture(java.awt.Rectangle(b.x, b.y, b.width, b.height)), "png", File(outDir, "$name.png"))
    }
    fun pressWithin(ms: Long, name: (String) -> Boolean): String? {
        val end = System.currentTimeMillis() + ms
        while (System.currentTimeMillis() < end) { pressAccessible(frame, name)?.let { return it }; Thread.sleep(200) }
        return null
    }
    fun back() = pressWithin(4_000) { it == "‹  Back" }.also { Thread.sleep(800) }
    try {
        Thread.sleep(2_500)
        // Home → the account chip (no server on a fresh data root: "Guest") → its sheet's ⚙ Parameters.
        val chip = pressWithin(15_000) { it.trim().endsWith("Guest") }
        val params = pressWithin(10_000) { it.endsWith("Parameters") && it.startsWith("⚙") }
        line("about-smoke: chip=$chip parameters=$params")
        Thread.sleep(1_500)
        // Scroll to the About section for the picture (the tree holds every section either way).
        runCatching {
            val b = frame.bounds
            java.awt.Robot().apply { mouseMove(b.x + b.width / 2, b.y + b.height / 2); repeat(40) { mouseWheel(3); delay(15) } }
        }
        Thread.sleep(1_000)
        capture("about")

        val dir = licencesDir()
        val swtNotice = dir?.let { File(it, SWT_NOTICE) }?.takeIf { it.isFile }
        if (swtNotice != null) {
            // WIN08 S2.8 (Windows) — unchanged: the SWT line names the shipped notice's version = the SWT jar's; Licences →
            // SWT shows the notice and the EPL.
            val noticeVersion = swtNoticeVersion(swtNotice.readText())
            // The SWT jar this app runs with: its file name carries the Maven version; the image adds a content hash
            // (…x86_64-3.134.0-<hex>.jar, leading zeros dropped), so the version is the part before it.
            val jarVersion = runCatching {
                val path = Class.forName("org.eclipse.swt.SWT").protectionDomain.codeSource.location.path
                Regex("""org\.eclipse\.swt\.win32\.win32\.x86_64-(\d+(?:\.\d+)*)(?:-[0-9a-f]{20,32})?\.jar$""").find(path)?.groupValues?.get(1)
            }.getOrNull()
            val aboutLine = accessibleNames(frame).firstOrNull { it.startsWith("Studio in this app is shown with SWT") }
            mark("S28", "about-line-shows-version",
                aboutLine != null && noticeVersion != null && noticeVersion == jarVersion && aboutLine.contains("SWT $noticeVersion,"),
                "line=[$aboutLine] notice=$noticeVersion jar=$jarVersion dir=$dir")
            val licences = pressWithin(8_000) { it == "Licences" }
            Thread.sleep(1_200)
            val entry = noticeVersion?.let { v -> pressWithin(8_000) { it == "SWT $v" } }
            Thread.sleep(1_500)
            capture("licences")
            val names = accessibleNames(frame)
            val showsNotice = names.any { it.contains("Bundle-Version:") && it.contains("source code of this exact version") }
            val showsEpl = names.any { it.startsWith("Eclipse Public License - v 2.0") }
            mark("S28", "licences-screen-shows-files", licences != null && entry != null && showsNotice && showsEpl,
                "licences=$licences entry=$entry notice=$showsNotice epl=$showsEpl titles=${names.filter { it.length < 60 }.take(12)}")
            back()
        } else {
            // A101 (Linux, macOS) — About's row for the open-source notices, and its Licences button.
            val row = accessibleNames(frame).firstOrNull { it == OPEN_SOURCE_LINE }
            val licences = pressWithin(8_000) { it == "Licences" }
            mark("A101", "about-row-shows", row != null && licences != null, "row=[$row] licences=$licences")
            Thread.sleep(1_200)
        }
        capture("licences-list")
        // A101 — two generated entries open with their texts: Ktor (its components and licence), the Java runtime (the
        // image's own legal/, read in place).
        val ktor = pressWithin(8_000) { it == "Ktor" }
        Thread.sleep(1_500)
        capture("licences-ktor")
        val ktorNames = accessibleNames(frame)
        mark("A101", "about-ktor-entry", ktor != null && ktorNames.any { it.contains("io.ktor:ktor-") } && ktorNames.any { it.contains("Apache License") },
            "pressed=$ktor components=${ktorNames.any { it.contains("io.ktor:ktor-") }} apache=${ktorNames.any { it.contains("Apache License") }}")
        back()
        val jre = pressWithin(8_000) { it.startsWith("Java runtime (OpenJDK ") }
        Thread.sleep(1_500)
        capture("licences-java-runtime")
        val jreNames = accessibleNames(frame)
        val gpl = jreNames.any { it.contains("GNU General Public License") }
        val cpe = jreNames.any { it.contains("\"CLASSPATH\" EXCEPTION") }   // OpenJDK's LICENSE: "CLASSPATH" EXCEPTION TO THE GPL
        // Windows' jlink writes one-line "Please see ..\java.base\LICENSE" pointers instead of symlinks: counted on disk
        // here, and none may show as a document of its own.
        val pointers = runCatching {
            File(System.getProperty("java.home"), "legal").walkTopDown().count { f ->
                f.isFile && f.length() < 512 && f.readText().startsWith("Please see ")
            }
        }.getOrDefault(-1)
        val pointerDocs = jreNames.count { it.startsWith("Please see ") }
        mark("A101", "about-java-runtime-entry", jre != null && gpl && cpe && pointerDocs == 0,
            "pressed=$jre gpl=$gpl classpathException=$cpe docs=${jreNames.count { it.contains("/") && it.length < 80 }} " +
                "pointers=$pointers pointer-documents=$pointerDocs")
        back()
        // How long each entry takes to open (LicencesScreen measures press → the first frame after it composed).
        val list = accessibleActionNames(frame).filter { it != "‹  Back" }
        for (name in list) {
            val before = com.troubastack.shared.ui.LicenceOpenTiming.count
            if (pressWithin(4_000) { it == name } == null) continue
            val end = System.currentTimeMillis() + 6_000
            while (com.troubastack.shared.ui.LicenceOpenTiming.count == before && System.currentTimeMillis() < end) Thread.sleep(50)
            back()
        }
        val slowest = com.troubastack.shared.ui.LicenceOpenTiming.slowest
        line("A101-TIME largest-entry-open name=${slowest?.first} ms=${slowest?.second} (opened ${com.troubastack.shared.ui.LicenceOpenTiming.count} entries: $list)")
    } catch (e: Throwable) {
        mark("A101", "flow", false, "threw: $e")
    }
    val ok = results.isNotEmpty() && results.values.all { it }
    line("about-smoke: ${results.count { it.value }}/${results.size} passed")
    Runtime.getRuntime().halt(if (ok) 0 else 1)
}

/** Press the first actionable node whose accessible name matches [name] (its first action, a click). The node's class
 *  for the log, or null when none is exposed. */
private fun pressAccessible(frame: java.awt.Window, name: (String) -> Boolean): String? {
    var found: String? = null
    SwingUtilities.invokeAndWait {
        val ctx = findAccessible(frame, name, 0) ?: return@invokeAndWait
        if (ctx.accessibleAction?.doAccessibleAction(0) == true) found = ctx.accessibleName
    }
    return found
}

private fun findAccessible(node: javax.accessibility.Accessible, name: (String) -> Boolean, depth: Int): javax.accessibility.AccessibleContext? {
    val ctx = node.accessibleContext ?: return null
    if ((ctx.accessibleAction?.accessibleActionCount ?: 0) > 0 && ctx.accessibleName?.let(name) == true) return ctx
    if (depth > 80) return null
    for (i in 0 until ctx.accessibleChildrenCount) findAccessible(ctx.getAccessibleChild(i) ?: continue, name, depth + 1)?.let { return it }
    return null
}

/** The accessible names of the window's pressable nodes, in tree order (the Licences list's entries, …). */
private fun accessibleActionNames(frame: java.awt.Window): List<String> {
    val out = ArrayList<String>()
    fun walk(node: javax.accessibility.Accessible, depth: Int) {
        val ctx = node.accessibleContext ?: return
        if ((ctx.accessibleAction?.accessibleActionCount ?: 0) > 0) ctx.accessibleName?.takeIf { it.isNotBlank() }?.let { out += it }
        if (depth < 80) for (i in 0 until ctx.accessibleChildrenCount) ctx.getAccessibleChild(i)?.let { walk(it, depth + 1) }
    }
    SwingUtilities.invokeAndWait { walk(frame, 0) }
    return out
}

/** Every accessible name in the window, in tree order. */
private fun accessibleNames(frame: java.awt.Window): List<String> {
    val out = ArrayList<String>()
    fun walk(node: javax.accessibility.Accessible, depth: Int) {
        val ctx = node.accessibleContext ?: return
        ctx.accessibleName?.takeIf { it.isNotBlank() }?.let { out += it }
        if (depth < 80) for (i in 0 until ctx.accessibleChildrenCount) ctx.getAccessibleChild(i)?.let { walk(it, depth + 1) }
    }
    SwingUtilities.invokeAndWait { walk(frame, 0) }
    return out
}
