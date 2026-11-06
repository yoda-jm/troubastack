package com.troubastack.desktop

import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.WindowPosition
import androidx.compose.ui.window.application
import androidx.compose.ui.window.rememberWindowState
import com.troubastack.app.App
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.ui.ThemePref
import com.troubastack.shared.ui.TroubaTheme
import java.io.File
import javax.swing.SwingUtilities

/**
 * WIN08 S2.8 — the About smoke (`--about-smoke <outDir>`), on the packaged Windows image. The app's own window and App(),
 * driven through the window's accessibility tree, as a person would go: the account chip → ⚙ Parameters → About, then
 * Licences → the SWT entry. Each check prints an `S28-CHECK name: PASS|FAIL` line to about-smoke.log (in [outDir]):
 *  - about-line-shows-version: the About line, read through the tree, names the version the shipped SWT-NOTICE.txt
 *    states, and that is the version of the SWT jar this app runs with;
 *  - licences-screen-shows-files: the SWT entry's screen shows the shipped notice and the "Eclipse Public License - v 2.0"
 *    text.
 * Writes about.png and licences.png, then exits 0 if both passed, else 1.
 */
internal fun runAboutSmoke(outDir: File) {
    outDir.mkdirs()
    val log = File(outDir, "about-smoke.log").apply { writeText("") }
    fun line(s: String) { println(s); runCatching { log.appendText(s + "\n") } }
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
    fun mark(name: String, ok: Boolean, detail: String) { results[name] = ok; line("S28-CHECK $name: ${if (ok) "PASS" else "FAIL"} — $detail") }
    fun capture(name: String) = runCatching {
        val b = frame.bounds
        javax.imageio.ImageIO.write(java.awt.Robot().createScreenCapture(java.awt.Rectangle(b.x, b.y, b.width, b.height)), "png", File(outDir, "$name.png"))
    }
    fun pressWithin(ms: Long, name: (String) -> Boolean): String? {
        val end = System.currentTimeMillis() + ms
        while (System.currentTimeMillis() < end) { pressAccessible(frame, name)?.let { return it }; Thread.sleep(200) }
        return null
    }
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
        val noticeVersion = dir?.let { File(it, SWT_NOTICE) }?.takeIf { it.isFile }?.readText()?.let(::swtNoticeVersion)
        // The SWT jar this app runs with: its file name carries the Maven version; the image adds a content hash
        // (…x86_64-3.134.0-<32 hex>.jar), so the version is the part before it.
        val jarVersion = runCatching {
            val path = Class.forName("org.eclipse.swt.SWT").protectionDomain.codeSource.location.path
            Regex("""org\.eclipse\.swt\.win32\.win32\.x86_64-(\d+(?:\.\d+)*)(?:-[0-9a-f]{32})?\.jar$""").find(path)?.groupValues?.get(1)
        }.getOrNull()
        val aboutLine = accessibleNames(frame).firstOrNull { it.startsWith("Studio in this app is shown with SWT") }
        mark("about-line-shows-version",
            aboutLine != null && noticeVersion != null && noticeVersion == jarVersion && aboutLine.contains("SWT $noticeVersion,"),
            "line=[$aboutLine] notice=$noticeVersion jar=$jarVersion dir=$dir")

        // Licences → the list of entries → the SWT entry's texts.
        val licences = pressWithin(8_000) { it == "Licences" }
        Thread.sleep(1_200)
        val entry = noticeVersion?.let { v -> pressWithin(8_000) { it == "SWT $v" } }
        Thread.sleep(1_500)
        capture("licences")
        val names = accessibleNames(frame)
        val showsNotice = names.any { it.contains("Bundle-Version:") && it.contains("source code of this exact version") }
        val showsEpl = names.any { it.startsWith("Eclipse Public License - v 2.0") }
        mark("licences-screen-shows-files", licences != null && entry != null && showsNotice && showsEpl,
            "licences=$licences entry=$entry notice=$showsNotice epl=$showsEpl titles=${names.filter { it.length < 60 }.take(12)}")
    } catch (e: Throwable) {
        mark("flow", false, "threw: $e")
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
