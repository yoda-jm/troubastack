package com.troubastack.desktop.studio

import com.troubastack.shared.distribution.reloadUrl
import org.eclipse.swt.SWT
import org.eclipse.swt.awt.SWT_AWT
import org.eclipse.swt.browser.Browser
import org.eclipse.swt.browser.BrowserFunction
import org.eclipse.swt.browser.LocationAdapter
import org.eclipse.swt.browser.LocationEvent
import org.eclipse.swt.browser.ProgressAdapter
import org.eclipse.swt.browser.ProgressEvent
import org.eclipse.swt.events.KeyAdapter
import org.eclipse.swt.events.KeyEvent
import org.eclipse.swt.layout.FillLayout
import org.eclipse.swt.widgets.Display
import org.eclipse.swt.widgets.Shell
import java.awt.Canvas
import java.io.File
import javax.swing.SwingUtilities

/**
 * WIN08 — the SWT/WebView2 side of the embedded Studio host, isolated from Compose on its own thread.
 *
 * Threading (design GO, binding B1):
 *  - A dedicated **daemon** thread owns the SWT [Display] and runs its `readAndDispatch` pump (WebView2 is COM-STA
 *    and needs OS-message pumping; `readAndDispatch` is that pump). Nothing touches SWT off this thread — callers
 *    use [exec] (`display.asyncExec`).
 *  - Callbacks that change Compose state hop to the EDT first (`SwingUtilities.invokeLater`).
 *  - [dispose] runs an ordered teardown and a BOUNDED join (~2 s), so a stuck SWT thread can never keep the JVM
 *    alive (WIN04's updater waits for the process to end). The EDT never blocks on this thread but that join.
 *
 * The Browser's native DLLs unpack to `swt.library.path` (set under the data root in Main), never `~/.swt`; the
 * WebView2 user-data folder is [edgeDataDir] under the data root, set before the Browser is created.
 */
internal class SwtBrowserController(
    private val canvas: Canvas,
    private val loadUrl: String,
    private val origin: String,
    private val edgeDataDir: File,
    private val sessionCookie: String?,
    private val forceNoWebview: Boolean,
    private val onReady: () -> Unit,
    private val onEngineMissing: () -> Unit,
    private val onFullScreenKey: () -> Unit,
    private val log: (String) -> Unit,
) {
    @Volatile var browserType: String? = null; private set
    @Volatile var bridgeReady: Boolean = false; private set
    @Volatile var helloAnswered: Boolean = false; private set
    @Volatile var navVetoed: Boolean = false; private set
    @Volatile var started: Boolean = false; private set

    @Volatile private var display: Display? = null
    @Volatile private var shell: Shell? = null
    @Volatile private var browser: Browser? = null
    private val thread = Thread(::run, "win08-swt-display").apply { isDaemon = true }

    fun start() = thread.start()

    /** Run [block] on the SWT display thread (no-op once disposed). */
    private fun exec(block: () -> Unit) {
        val d = display ?: return
        if (!d.isDisposed) runCatching { d.asyncExec { if (shell?.isDisposed == false) block() } }
    }

    private fun onEdt(block: () -> Unit) = SwingUtilities.invokeLater(block)

    /** ⋮ Reload — A93: stay on the page you are on (or the entry if it is blank / a different origin). */
    fun reload() = exec {
        val b = browser ?: return@exec
        b.url = reloadUrl(b.url, loadUrl)
    }

    /** Smoke only: try to navigate off-origin; the LocationListener must veto it ([navVetoed] is the proof). */
    fun probeOffOriginNav(foreign: String) = exec { browser?.setUrl(foreign) }

    /** Ordered teardown + bounded join, from the screen's onDispose (every exitApplication path runs it). */
    fun dispose() {
        exec { shell?.takeIf { !it.isDisposed }?.dispose() }
        runCatching { thread.join(2000) }
    }

    private fun run() {
        var display: Display? = null
        try {
            // EdgeDataDir MUST be set before the first Browser(SWT.EDGE) creates the WebView2 environment.
            System.setProperty("org.eclipse.swt.browser.EdgeDataDir", edgeDataDir.also { it.mkdirs() }.absolutePath)
            display = Display()
            this.display = display
            started = true
            if (forceNoWebview) {
                // S1.6 CI path: behave as if WebView2 is absent, without creating a Shell/Browser.
                log("win08: forceNoWebview — reporting engine missing")
                onEdt(onEngineMissing)
                return
            }
            // Wait for the AWT canvas to be realized (SwingPanel adds + shows it) before SWT_AWT.new_Shell.
            val deadline = System.currentTimeMillis() + 10_000
            while (!canvas.isDisplayable && System.currentTimeMillis() < deadline) Thread.sleep(20)
            if (!canvas.isDisplayable) { log("win08: canvas never became displayable"); onEdt(onEngineMissing); return }

            val shell = SWT_AWT.new_Shell(display, canvas)
            this.shell = shell
            shell.layout = FillLayout()

            val browser = try {
                Browser(shell, SWT.EDGE)
            } catch (e: Throwable) {
                log("win08: Browser(SWT.EDGE) threw: ${e.message}")
                onEdt(onEngineMissing)
                runLoopUntilDisposed(display, shell)
                return
            }
            val type = runCatching { browser.browserType }.getOrNull()
            browserType = type
            log("win08: browserType=$type")
            if (type != "edge") {
                // Never fall back to IE: a non-edge backend is "no WebView2".
                browser.dispose()
                onEdt(onEngineMissing)
                runLoopUntilDisposed(display, shell)
                return
            }
            this.browser = browser

            // Bridge: a BrowserFunction whose bare-identifier name Studio's bridge.ts picks up as the shell
            // (window.TroubaStageShellReceive). SWT 3.134.0 registers it via AddScriptToExecuteOnDocumentCreated,
            // so it exists before Studio's bundle runs initShellBridge.
            object : BrowserFunction(browser, "TroubaStageShellReceive") {
                override fun function(arguments: Array<out Any?>?): Any? {
                    val json = arguments?.getOrNull(0) as? String ?: return null
                    if (json.contains("\"ready\"")) {
                        if (!bridgeReady) { bridgeReady = true; log("win08: bridge ready"); onEdt(onReady) }
                        else helloAnswered = true // a second ready = our hello was answered
                    }
                    return null
                }
            }

            // Origin lock: veto any navigation off the server's origin.
            browser.addLocationListener(object : LocationAdapter() {
                override fun changing(event: LocationEvent) {
                    val to = event.location ?: return
                    if (to != "about:blank" && !sameOrigin(to, origin)) {
                        event.doit = false
                        navVetoed = true
                        log("win08: vetoed navigation to $to")
                    }
                }
            })

            // F11 forwarded to the app's full screen (S1.7); nothing else consumed, so Esc still reaches Studio.
            browser.addKeyListener(object : KeyAdapter() {
                override fun keyPressed(e: KeyEvent) {
                    if (e.keyCode == SWT.F11) { e.doit = false; onEdt(onFullScreenKey) }
                }
            })

            // After the first load, send a hello over the native→web channel; a ready back proves both ways.
            browser.addProgressListener(object : ProgressAdapter() {
                override fun completed(event: ProgressEvent) {
                    runCatching { browser.execute("window.__troubastageShell&&window.__troubastageShell.deliver(JSON.stringify({type:'hello'}))") }
                }
            })

            // Seed the session cookie (Browser exists ⇒ the WebView2 cookie manager is up), then load — signed in.
            sessionCookie?.let { c -> runCatching { Browser.setCookie(c, origin) }.onSuccess { ok -> log("win08: setCookie=$ok") } }
            browser.setUrl(loadUrl)

            runLoopUntilDisposed(display, shell)
        } catch (e: Throwable) {
            log("win08: SWT thread failed: ${e.message}")
            onEdt(onEngineMissing)
        } finally {
            runCatching { display?.takeIf { !it.isDisposed }?.dispose() }
        }
    }

    private fun runLoopUntilDisposed(display: Display, shell: Shell) {
        while (!shell.isDisposed) { if (!display.readAndDispatch()) display.sleep() }
    }
}

/** Scheme + host + port of [url], lowercased (default ports normalised), or "" when it cannot be parsed. */
internal fun originOfUrl(url: String): String = runCatching {
    val u = java.net.URI(url)
    val scheme = u.scheme?.lowercase() ?: return ""
    val host = u.host?.lowercase() ?: return ""
    val port = when {
        u.port != -1 -> u.port
        scheme == "http" -> 80
        scheme == "https" -> 443
        else -> -1
    }
    "$scheme://$host:$port"
}.getOrDefault("")

internal fun sameOrigin(url: String, origin: String): Boolean = originOfUrl(url) == origin
