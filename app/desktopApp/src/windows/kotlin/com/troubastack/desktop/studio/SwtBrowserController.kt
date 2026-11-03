package com.troubastack.desktop.studio

import com.troubastack.shared.distribution.reloadUrl
import com.sun.jna.platform.win32.WinDef.HWND
import org.eclipse.swt.SWT
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
    private val onStartupTimeout: () -> Unit,
    private val onFullScreenKey: () -> Unit,
    private val log: (String) -> Unit,
) {
    @Volatile var browserType: String? = null; private set
    @Volatile var bridgeReady: Boolean = false; private set
    @Volatile var helloAnswered: Boolean = false; private set
    @Volatile var navVetoed: Boolean = false; private set
    @Volatile var started: Boolean = false; private set
    @Volatile var engineMissingReported: Boolean = false; private set
    @Volatile var startupTimedOut: Boolean = false; private set
    @Volatile var docCookie: String? = null; private set   // what document.cookie sees (item 5 — must NOT hold the session)
    @Volatile private var disposed: Boolean = false
    // B2 DPI measurement (filled by measureDpi, read by the smoke).
    @Volatile var swtThreadDpiAwareness: Int = -1; private set
    @Volatile var canvasClient: String = "?"; private set
    @Volatile var shellClient: String = "?"; private set
    @Volatile var rectsMatch: Boolean = false; private set

    @Volatile private var display: Display? = null
    @Volatile private var shell: Shell? = null
    @Volatile private var browser: Browser? = null
    @Volatile private var canvasHwnd: HWND? = null
    // Daemon so a stuck SWT thread can never keep the JVM alive (B1) — the reliable close path is the window close.
    private val thread = Thread(::run, "win08-swt-display").apply { isDaemon = true }

    private fun reportEngineMissing() { engineMissingReported = true; onEdt { onEngineMissing() } }
    private fun reportStartupTimeout() { startupTimedOut = true; onEdt { onStartupTimeout() } }

    /** WIN08 sabotage knob (dormant unless WIN08_SABOTAGE is set — only the CI sabotage runs set it, never
     *  production). Each value breaks exactly one guarded behaviour so its smoke check can be shown to red. */
    private val sabotage: String = System.getenv("WIN08_SABOTAGE").orEmpty()

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

    /** Keep the reparented child Shell filling the AWT canvas, in PHYSICAL pixels (item 4): size from the canvas
     *  HWND's own client rect via SetWindowPos, so it is right even when AWT's scale differs from SWT's (e.g.
     *  -Dsun.java2d.uiScale=1.5). SWT's setBounds (SWT points) is only correct while both toolkits share a scale. */
    private fun fitShellToCanvas(shell: Shell) {
        if (shell.isDisposed) return
        val ch = canvasHwnd
        // "logicalfit" sabotage (item 4): use SWT's logical setBounds instead of the physical GetClientRect, so at
        // -Dsun.java2d.uiScale=1.5 the Shell comes out ~2/3 of the canvas and the DPI check reds.
        if (ch != null && sabotage != "logicalfit") Win08Native.fitChildToParent(Win08Native.hwnd(shell.handle), ch)
        else shell.setBounds(0, 0, maxOf(1, canvas.width), maxOf(1, canvas.height))
    }

    /** B2 — on the SWT thread, read this thread's DPI awareness and compare the embedded Shell's client rect to the
     *  canvas's (equal at 100 %). The process and EDT awareness are read by the smoke on their own threads. */
    fun measureDpi() {
        val d = display ?: return
        if (d.isDisposed) return
        runCatching {
            d.syncExec {
                swtThreadDpiAwareness = Win08Native.threadDpiAwareness()
                val ch = canvasHwnd
                val sh = shell?.takeIf { !it.isDisposed }?.let { Win08Native.hwnd(it.handle) }
                if (ch != null && sh != null) {
                    val (cw, chh) = Win08Native.clientSize(ch)
                    val (sw, shh) = Win08Native.clientSize(sh)
                    canvasClient = "${cw}x${chh}"
                    shellClient = "${sw}x${shh}"
                    rectsMatch = cw == sw && chh == shh && cw > 0 && chh > 0
                }
            }
        }
    }

    /** Ordered teardown + bounded join, from the screen's onDispose (every exitApplication path runs it). Sets
     *  [disposed] FIRST and wakes the display, so a dispose() that arrives before the Shell exists is honoured by
     *  the run() thread (which checks [disposed] at each step) rather than silently dropped. */
    fun dispose() {
        disposed = true
        val d = display
        runCatching { d?.takeIf { !it.isDisposed }?.asyncExec { shell?.takeIf { !it.isDisposed }?.dispose() } }
        runCatching { d?.wake() }
        runCatching { thread.join(2000) }
    }

    private fun run() {
        var display: Display? = null
        try {
            log("ctrl: run start (thread=${Thread.currentThread().name})")
            // EdgeDataDir MUST be set before the first Browser(SWT.EDGE) creates the WebView2 environment.
            System.setProperty("org.eclipse.swt.browser.EdgeDataDir", edgeDataDir.also { it.mkdirs() }.absolutePath)
            display = Display()
            this.display = display
            started = true
            log("ctrl: Display created")
            if (forceNoWebview) {
                // S1.6 CI path: behave as if WebView2 is absent, without creating a Shell/Browser.
                log("ctrl: forceNoWebview — reporting engine missing")
                reportEngineMissing()
                return
            }
            // Wait for the AWT canvas to be realized (SwingPanel adds + shows it) before SWT_AWT.new_Shell.
            val deadline = System.currentTimeMillis() + 10_000
            while (!canvas.isDisplayable && System.currentTimeMillis() < deadline) Thread.sleep(20)
            if (!canvas.isDisplayable) { log("ctrl: canvas never became displayable"); reportEngineMissing(); return }
            log("ctrl: canvas displayable; creating TOP-LEVEL Shell + Browser(SWT.EDGE)")

            // A top-level, SWT-thread-owned Shell (NOT SWT_AWT.new_Shell): WebView2's controller-ready callback then
            // pumps on the SWT thread (WIN05: a bare Shell readies in ~80ms). Under SWT_AWT the embedded Shell was
            // AWT-affine and the first Edge op timed out at 5s. We reparent this Shell into the AWT canvas below.
            val shell = Shell(display, SWT.NO_TRIM)
            this.shell = shell
            shell.layout = FillLayout()

            val tBrowser = System.currentTimeMillis()
            val browser = try {
                Browser(shell, SWT.EDGE)
            } catch (e: Throwable) {
                log("win08: Browser(SWT.EDGE) threw: ${e.message}")
                reportEngineMissing()
                runLoopUntilDisposed(display, shell)
                return
            }
            val type = if (sabotage == "engine") "ie" else runCatching { browser.browserType }.getOrNull()
            browserType = type
            log("win08: browserType=$type (sabotage=$sabotage)")
            if (type != "edge") {
                // Never fall back to IE: a non-edge backend is "no WebView2".
                browser.dispose()
                reportEngineMissing()
                runLoopUntilDisposed(display, shell)
                return
            }
            this.browser = browser

            // Child-HWND embedding FIRST (before any synchronous Edge op): reparent the top-level Shell's HWND into
            // the heavyweight AWT canvas and show it. WebView2 initialises its controller for a shown window; doing
            // this first is what the reliable runs did — reordering it after createFunction left the controller
            // unready. Track the canvas size so the Shell keeps filling it.
            val canvasHwnd = Win08Native.hwndOf(canvas)
            this.canvasHwnd = canvasHwnd
            if (canvasHwnd != null && sabotage != "reparent") {
                Win08Native.embed(Win08Native.hwnd(shell.handle), canvasHwnd)
                if (sabotage == "nofit") shell.setBounds(0, 0, 120, 80) else fitShellToCanvas(shell)
                if (sabotage != "nofit") canvas.addComponentListener(object : java.awt.event.ComponentAdapter() {
                    override fun componentResized(e: java.awt.event.ComponentEvent?) = exec { fitShellToCanvas(shell) }
                })
                shell.isVisible = true
                log("ctrl: reparented shell into canvas hwnd=$canvasHwnd (sabotage=$sabotage)")
            } else if (sabotage == "reparent") {
                // SABOTAGE only: leave the Shell top-level so no WebView2 window is a frame descendant (GA_ROOT reds).
                shell.setBounds(0, 0, 800, 600); shell.isVisible = true
                log("ctrl: shell NOT embedded (sabotage=reparent)")
            } else {
                // Production with no canvas HWND: never show a stray top-level window — report engine missing instead.
                log("ctrl: no canvas HWND; reporting engine missing (no window shown)")
                browser.dispose(); reportEngineMissing(); runLoopUntilDisposed(display, shell); return
            }
            if (disposed) { log("ctrl: disposed during setup — tearing down"); shell.dispose(); runLoopUntilDisposed(display, shell); return }

            // Bridge: a BrowserFunction whose bare-identifier name Studio's bridge.ts picks up (TroubaStageShellReceive).
            // SWT 3.134.0 injects it via AddScriptToExecuteOnDocumentCreated, so it exists before the page's scripts.
            // The handler acts ONLY on calls from the server's origin (S1.4, item 6). SWT's Edge.timeout was raised to
            // 30s in main(), so this SINGLE createFunction waits long enough for a cold controller — no warm-up, no
            // retry (the first timeout would poison the Browser). The "bridge-late" sabotage registers it after load.
            fun registerBridge() {
                object : BrowserFunction(browser, "TroubaStageShellReceive") {
                    override fun function(arguments: Array<out Any?>?): Any? {
                        if (!sameOrigin(browser.url ?: "", origin)) return null
                        val json = arguments?.getOrNull(0) as? String ?: return null
                        when {
                            json.contains("\"cookie\"") -> { docCookie = json; log("win08: document.cookie reported") }
                            json.contains("\"ready\"") -> {
                                if (!bridgeReady) { bridgeReady = true; log("win08: bridge ready"); onEdt(onReady) }
                                else helloAnswered = true // a second ready = our hello was answered
                            }
                        }
                        return null
                    }
                }
            }
            if (sabotage != "bridge" && sabotage != "bridge-late") {
                try {
                    registerBridge()
                    log("ctrl: TIMING createFunction returned ${System.currentTimeMillis() - tBrowser} ms after Browser()")
                } catch (e: Throwable) {
                    // The controller did not become ready within SWT's (raised) timeout — a real startup timeout, NOT
                    // "no WebView2". Its own words, Back offered, no IE fallback.
                    log("win08: createFunction failed — controller not ready in time: ${e.message}")
                    reportStartupTimeout(); runLoopUntilDisposed(display, shell); return
                }
            }

            // Origin lock: veto any navigation off the server's origin.
            if (sabotage != "nav") browser.addLocationListener(object : LocationAdapter() {
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
            var lateBridgeDone = false
            browser.addProgressListener(object : ProgressAdapter() {
                override fun completed(event: ProgressEvent) {
                    // B3 sabotage (b): register the bridge only AFTER the load — the page's head script has already
                    // run without window.TroubaStageShellReceive, so no ready is sent and the bridge check reds.
                    if (sabotage == "bridge-late" && !lateBridgeDone) { lateBridgeDone = true; registerBridge() }
                    runCatching { browser.execute("window.__troubastageShell&&window.__troubastageShell.deliver(JSON.stringify({type:'hello'}))") }
                }
            })

            // Seed the session cookie AS THE SERVER SETS IT (item 5): Path=/ + HttpOnly (+ Secure on https), so
            // Studio's own scripts cannot read the session (document.cookie), while the request header still carries
            // it. SWT's setCookie parses these attributes via HttpCookie.parse.
            sessionCookie?.let { c ->
                val https = origin.startsWith("https")
                val value = "$c; Path=/; HttpOnly" + if (https) "; Secure" else ""
                runCatching { Browser.setCookie(value, origin) }.onSuccess { ok -> log("win08: setCookie=$ok (HttpOnly${if (https) ",Secure" else ""})") }
            }
            browser.setUrl(loadUrl)
            log("ctrl: setUrl($loadUrl); entering readAndDispatch loop")

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
