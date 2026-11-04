package com.troubastack.desktop.studio

import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.application
import androidx.compose.ui.window.rememberWindowState
import com.troubastack.app.HttpTransport
import com.troubastack.app.Presence
import com.troubastack.shared.seams.SESSION_COOKIE_KEY
import com.troubastack.shared.seams.SESSION_ORIGIN_KEY
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.seams.desktopDataRoot
import java.io.File
import java.net.InetAddress
import java.net.ServerSocket
import java.nio.charset.StandardCharsets

/**
 * WIN08 S1.8 — the embedded-Studio CI smoke (see [Win08SmokeRunner]). Serves a loopback page that runs the bridge
 * handshake as a synchronous script at the top of `<head>` (earlier than Studio's module — binding B3's worst
 * case), opens it in the embedded host inside a real TroubaStage window, and checks: engine is Edge; the seeded
 * cookie arrives; the bridge round-trips (ready, then a hello answered); off-origin navigation is vetoed; the
 * EdgeDataDir is under the data root and `~/.swt` is absent. Each check prints a `WIN08-CHECK …: PASS|FAIL` marker
 * to the durable [win08log] file (a packaged Windows app has no captured stdout); the window then closes and the
 * process exits (the CI step asserts it exits and leaves no msedgewebview2.exe — check 7).
 *
 * The loopback server is a raw [ServerSocket] from `java.base`, not `com.sun.net.httpserver` (which would need the
 * `jdk.httpserver` jlink module and, if missing, blows up a GUI-subsystem app with a blocking error dialog).
 */
class Win08SmokeRunnerImpl : Win08SmokeRunner {
    @Volatile private var cookieHeader: String? = null
    @Volatile private var served = false
    @Volatile private var serveNoReady = false   // S2.1: when set, the loopback serves a page that never handshakes
    @Volatile private var accepting = true        // S2.1 retry: when false, the loopback closes connections at once (server "off")
    @Volatile private var serverSocket: ServerSocket? = null

    private fun line(s: String) = win08log(s)

    override fun run(outDir: File): Int {
        outDir.mkdirs()
        enableWin08FileLog()
        win08log("smoke: run start; outDir=$outDir log=$win08LogFile")
        // Arm the backstop FIRST, so any hang — even before the loopback server is up — is logged and halted.
        Thread {
            try { Thread.sleep(120_000) } catch (_: InterruptedException) { return@Thread }
            win08log("smoke: BACKSTOP firing (>120s) — halting 1")
            Runtime.getRuntime().halt(1)
        }.apply { isDaemon = true; start() }
        win08log("smoke: backstop armed")
        try {
            win08log("smoke: opening loopback ServerSocket")
            val server = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
            serverSocket = server
            val port = server.localPort
            val origin = "http://127.0.0.1:$port"
            Thread { acceptLoop(server) }.apply { isDaemon = true; start() }
            win08log("smoke: loopback server on $origin")

            val edgeDataDir = File(desktopDataRoot(), "webview2")
            val frameHolder = java.util.concurrent.atomic.AtomicReference<java.awt.Window?>()
            val args = EmbeddedStudioArgs(
                serverUrl = origin,
                initialPath = null,
                title = "TroubaStudio",
                sessionCookie = "win08sess=smoke",
                edgeDataDir = edgeDataDir,
                onBack = {},
                toggleFullScreen = {},
                isFullScreen = { false },
                frameWindow = { frameHolder.get() },
                probeReachable = null,   // the loopback is always reachable; the smoke tests the controller, not the cover
            )

            // Check-7 discriminator: WIN08_SABOTAGE=noclose makes onCloseRequest ignore the close (log, don't exit).
            // If the WM_CLOSE still ends the process (jpackage/Compose force-exit), the normal backstop never fires and
            // the exit code is 0 — then check 7 is un-sabotageable, proven positively. If instead the window stays open
            // (AWT delivers WINDOW_CLOSING, we ignore it), the 75 s backstop fires (halt 1) — so the sabotage reds it.
            val sabotageClose = System.getenv("WIN08_SABOTAGE") == "noclose"
            win08log("smoke: entering application{} (sabotageClose=$sabotageClose)")
            application {
                Window(
                    onCloseRequest = if (sabotageClose) {
                        { win08log("smoke: SABOTAGE noclose — onCloseRequest fired (AWT WINDOW_CLOSING from WM_CLOSE); IGNORING, window stays open") }
                    } else ::exitApplication,
                    title = "TroubaStage",
                    state = rememberWindowState(width = 1200.dp, height = 820.dp),
                ) {
                    var controller by remember { mutableStateOf<SwtBrowserController?>(null) }
                    EmbeddedStudioContent(args, onController = { controller = it })
                    LaunchedEffect(Unit) {
                        frameHolder.set(window)
                        win08log("smoke: window composed; starting checker thread")
                        Thread { runChecks(controller, edgeDataDir, origin) { frameHolder.get() } }.apply { isDaemon = true; start() }
                    }
                }
            }
        } catch (e: Throwable) {
            win08log("smoke: run FAILED: $e\n${e.stackTraceToString()}")
            Runtime.getRuntime().halt(1)
        }
        // Reached on the pass path: the checker posted WM_CLOSE, application{} returned. Return 0 and let the caller
        // let the JVM exit on its own (check 7). On a failed check the checker already halted(1).
        return 0
    }

    override fun runReentry(outDir: File): Int {
        outDir.mkdirs(); enableWin08FileLog()
        win08log("reentry: run start; log=$win08LogFile")
        Thread {
            try { Thread.sleep(90_000) } catch (_: InterruptedException) { return@Thread }
            win08log("reentry: BACKSTOP firing (>90s)"); Runtime.getRuntime().halt(1)
        }.apply { isDaemon = true; start() }
        val server = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
        serverSocket = server
        val origin = "http://127.0.0.1:${server.localPort}"
        Thread { acceptLoop(server) }.apply { isDaemon = true; start() }
        val edgeDataDir = File(desktopDataRoot(), "webview2")
        fun argsNow() = EmbeddedStudioArgs(origin, null, "TroubaStudio", "win08sess=smoke", edgeDataDir, {}, {}, { false }, { null }, null)
        val ctrlRef = java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>()
        val shown = mutableStateOf(true)
        val cyc = mutableStateOf(0)
        application {
            Window(
                onCloseRequest = ::exitApplication,
                title = "TroubaStage",
                state = rememberWindowState(width = 1200.dp, height = 820.dp),
            ) {
                androidx.compose.runtime.key(cyc.value) {
                    if (shown.value) EmbeddedStudioContent(argsNow(), onController = { ctrlRef.set(it) })
                }
                LaunchedEffect(Unit) { Thread { reentryChecks(ctrlRef, shown, cyc) }.apply { isDaemon = true; start() } }
            }
        }
        return 0
    }

    private fun reentryChecks(
        ctrlRef: java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>,
        shown: androidx.compose.runtime.MutableState<Boolean>,
        cyc: androidx.compose.runtime.MutableState<Int>,
    ) {
        val results = LinkedHashMap<String, Boolean>()
        fun mark(name: String, ok: Boolean) { results[name] = ok; line("WIN08-CHECK $name: ${if (ok) "PASS" else "FAIL"}") }
        fun waitFor(timeoutMs: Long, cond: () -> Boolean): Boolean {
            val end = System.currentTimeMillis() + timeoutMs
            while (System.currentTimeMillis() < end) { if (cond()) return true; Thread.sleep(100) }
            return cond()
        }
        try {
            win08log("reentry: checker start")
            // S1.1 — leave and re-enter twice; the bridge round-trips each time (a fresh controller per entry).
            for (c in 1..2) {
                val ok = waitFor(40_000) { ctrlRef.get()?.bridgeReady == true }
                win08log("reentry: cycle $c bridge=$ok")
                mark("reentry-cycle$c-bridge", ok)
                val old = ctrlRef.get()
                shown.value = false                 // leave → onDispose → controller.dispose()
                Thread.sleep(2500)
                ctrlRef.set(null)
                cyc.value = c                        // fresh key → a brand-new EmbeddedStudioContent + controller
                shown.value = true                   // re-enter
                waitFor(20_000) { ctrlRef.get().let { it != null && it !== old } }
            }
            // An immediate Back on entering disposes cleanly — afterwards no embedded-Studio window lingers (the
            // host's Shell is always a child of the AWT canvas, destroyed when the composition unmounts; the
            // controller's run() thread then ends). The frame itself stays. (A brief child show may occur due to
            // Compose recomposition latency; what matters is the clean teardown after Back.)
            cyc.value = 99; shown.value = true
            waitFor(10_000) { ctrlRef.get() != null }
            val c99 = ctrlRef.get()
            shown.value = false                      // Back at once
            val cleaned = c99 != null && waitFor(8_000) { !c99.threadAlive() }
            win08log("reentry: early-back shellEverShown=${c99?.shellEverShown} threadAlive=${c99?.threadAlive()}")
            mark("early-back-clean-dispose", cleaned)
        } catch (e: Throwable) {
            win08log("reentry: checker threw: ${e.message}")
        }
        finish(results) { null }
    }

    /**
     * S2.1 (R1, refinement 1) — the live Retry proof from the pre-load probe's Unreachable cover (the "VLL's server is
     * off" case). The loopback owns its port but starts OFF (accepting=false → sockets closed at once), so the probe
     * and first load can't reach it → the cover. Then it turns ON and fires Retry (controller.reload()); Studio comes
     * back on the SAME controller. Captures `cover.png` + `studio.png`; asserts recovered + same controller + rect.
     */
    override fun runRetry(outDir: File): Int {
        outDir.mkdirs(); enableWin08FileLog()
        win08log("retry: run start; outDir=$outDir")
        Thread {
            try { Thread.sleep(90_000) } catch (_: InterruptedException) { return@Thread }
            win08log("retry: BACKSTOP firing (>90s) — halting 1"); Runtime.getRuntime().halt(1)
        }.apply { isDaemon = true; start() }
        try {
            accepting = false   // server OFF: the probe + first load get the socket closed → Unreachable → cover
            val server = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
            serverSocket = server
            val port = server.localPort
            val origin = "http://127.0.0.1:$port"
            Thread { acceptLoop(server) }.apply { isDaemon = true; start() }
            win08log("retry: loopback bound on $origin (OFF)")

            val storage = Storage()
            storage.putSecret("coreUrl", origin)
            storage.putSecret(SESSION_COOKIE_KEY, "win08retry=x")   // so probePresence GETs /api/me (not short-circuit)
            storage.putSecret(SESSION_ORIGIN_KEY, origin)
            val edgeDataDir = File(desktopDataRoot(), "webview2")
            val frameHolder = java.util.concurrent.atomic.AtomicReference<java.awt.Window?>()
            val args = EmbeddedStudioArgs(
                serverUrl = origin, initialPath = null, title = "TroubaStudio",
                sessionCookie = "win08retry=x", edgeDataDir = edgeDataDir,
                onBack = {}, toggleFullScreen = {}, isFullScreen = { false }, frameWindow = { frameHolder.get() },
                probeReachable = { HttpTransport(storage, defaultServerUrl = null) {}.probePresence() !is Presence.Unreachable },
            )
            val ctrlRef = java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>()
            application {
                Window(onCloseRequest = ::exitApplication, title = "TroubaStage",
                    state = rememberWindowState(width = 1200.dp, height = 820.dp)) {
                    EmbeddedStudioContent(args, onController = { ctrlRef.set(it) })
                    LaunchedEffect(Unit) {
                        frameHolder.set(window)
                        Thread { retryFlow(outDir, ctrlRef, frameHolder) }.apply { isDaemon = true; start() }
                    }
                }
            }
        } catch (e: Throwable) {
            win08log("retry: run FAILED: $e\n${e.stackTraceToString()}"); Runtime.getRuntime().halt(1)
        }
        return 0
    }

    private fun retryFlow(
        outDir: File,
        ctrlRef: java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>,
        frameHolder: java.util.concurrent.atomic.AtomicReference<java.awt.Window?>,
    ) {
        fun waitFor(ms: Long, cond: () -> Boolean): Boolean {
            val end = System.currentTimeMillis() + ms
            while (System.currentTimeMillis() < end) { if (cond()) return true; Thread.sleep(100) }
            return cond()
        }
        fun capture(name: String) {
            runCatching {
                val w = frameHolder.get() ?: return
                val b = w.bounds
                val img = java.awt.Robot().createScreenCapture(java.awt.Rectangle(b.x, b.y, b.width, b.height))
                javax.imageio.ImageIO.write(img, "png", File(outDir, "$name.png"))
                win08log("retry: captured $name (${File(outDir, "$name.png").length()} bytes)")
            }.onFailure { win08log("retry: capture $name failed: ${it.message}") }
        }
        waitFor(20_000) { ctrlRef.get() != null }
        val c = ctrlRef.get() ?: run { win08log("retry: no controller"); Runtime.getRuntime().halt(1); return }
        waitFor(9_000) { false }   // the probe (3s, OFF server) brings up the cover; let it paint
        capture("cover")
        val idBefore = System.identityHashCode(c)
        val beforeReady = c.readyCount
        accepting = true           // server ON
        win08log("retry: server ON; firing Retry (reload) on controller id=$idBefore")
        c.reload()                 // what the cover's Retry button does
        val recovered = waitFor(20_000) { c.readyCount > beforeReady }
        val sameController = System.identityHashCode(c) == idBefore
        // Poll measureDpi until the 0.dp→full refit settles: after `ready` the SwingPanel recomposes to full size and
        // fitShellToCanvas refits the Shell on the resize — a one-shot measure right after `ready` can catch the transient.
        var rect = false
        val rectEnd = System.currentTimeMillis() + 10_000
        while (System.currentTimeMillis() < rectEnd && !rect) { c.measureDpi(); Thread.sleep(400); rect = c.rectsMatch }
        capture("studio")
        val ok = recovered && sameController && rect
        win08log("retry: recovered=$recovered sameController=$sameController(id=$idBefore) rectsMatch=$rect OK=$ok")
        runCatching {
            File(outDir, "retry-verdict.txt").writeText(
                "recovered=$recovered sameController=$sameController rectsMatch=$rect OK=$ok")
        }
        Runtime.getRuntime().halt(if (ok) 0 else 1)
    }

    /** A one-page HTTP/1.0 loopback: read the request headers (record the Cookie), reply with [PAGE]. */
    private fun acceptLoop(server: ServerSocket) {
        while (!server.isClosed) {
            val sock = try { server.accept() } catch (_: Throwable) { break }
            Thread {
                runCatching {
                    sock.use { s ->
                        if (!accepting) return@use   // S2.1 retry: server "off" — close the socket at once (connection refused/reset)
                        val reader = s.getInputStream().bufferedReader()
                        val requestLine = reader.readLine() ?: ""   // "GET /path HTTP/1.1"
                        var l = reader.readLine()
                        while (l != null && l.isNotEmpty()) {
                            if (l.startsWith("Cookie:", ignoreCase = true)) cookieHeader = l.substringAfter(":").trim()
                            l = reader.readLine()
                        }
                        served = true
                        val path = requestLine.substringAfter(' ', "").substringBefore(' ')
                        val out = s.getOutputStream()
                        if (path.startsWith("/api/me")) {
                            // S2.1: answer probePresence()'s reachability GET with 401 → Presence.Unauthorized (reachable),
                            // not a 200 HTML body it would fail to parse (→ a false Unreachable).
                            out.write("HTTP/1.0 401 Unauthorized\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".toByteArray(StandardCharsets.US_ASCII))
                            out.flush()
                        } else {
                            // S2.1: a never-ready page (loads but never handshakes) while serveNoReady is set.
                            val pageStr = if (serveNoReady) PAGE_NOREADY else PAGE
                            val body = pageStr.toByteArray(StandardCharsets.UTF_8)
                            val head = "HTTP/1.0 200 OK\r\nContent-Type: text/html; charset=utf-8\r\n" +
                                "Content-Length: ${body.size}\r\nConnection: close\r\n\r\n"
                            out.write(head.toByteArray(StandardCharsets.US_ASCII)); out.write(body); out.flush()
                        }
                    }
                }
            }.apply { isDaemon = true; start() }
        }
    }

    private fun runChecks(controller: SwtBrowserController?, edgeDataDir: File, origin: String, frame: () -> java.awt.Window?) {
        val results = LinkedHashMap<String, Boolean>()
        fun mark(name: String, ok: Boolean) {
            results[name] = ok
            line("WIN08-CHECK $name: ${if (ok) "PASS" else "FAIL"}")
        }
        fun waitFor(timeoutMs: Long, cond: () -> Boolean): Boolean {
            val end = System.currentTimeMillis() + timeoutMs
            while (System.currentTimeMillis() < end) { if (cond()) return true; Thread.sleep(100) }
            return cond()
        }
        try {
            win08log("checker: start (controller=${controller != null})")
            val c = controller
            if (c == null) { mark("controller-created", false); finish(results, frame); return }

            // S1.6 — forced no-WebView2 (CI sets TROUBA_WIN08_FORCE_NO_WEBVIEW=1). The host must report engine
            // missing and show the exact message, never create a Browser, never fall back to IE.
            if (System.getenv("TROUBA_WIN08_FORCE_NO_WEBVIEW") == "1") {
                val reported = waitFor(20_000) { c.engineMissingReported }
                mark("no-webview-engine-missing", reported && c.browserType == null)
                mark("no-webview-message", NO_WEBVIEW_MESSAGE.contains("doesn't have Microsoft Edge WebView2"))
                finish(results, frame)
                return
            }

            waitFor(30_000) { c.browserType != null }
            mark("engine-is-edge", c.browserType == "edge")

            val bridge = waitFor(30_000) { c.bridgeReady }
            // The loopback page's deliver answers our auto-sent hello with a second ready.
            val hello = bridge && waitFor(10_000) { c.helloAnswered }
            mark("bridge-round-trips", bridge && hello)

            // Check 2 — the embedded browser's top-level window is the TroubaStage frame.
            val f = frame()
            if (f != null) {
                val (owns, detail) = Win08Native.frameOwnsWebView(f)
                win08log("ga_root: $detail")
                mark("frame-owns-webview", owns)
            } else {
                mark("frame-owns-webview", false)
            }

            mark("seeded-cookie-arrives", (cookieHeader ?: "").contains("win08sess=smoke"))

            // Item 5 — the HttpOnly session must NOT be visible to document.cookie (even though the request carried it).
            waitFor(5_000) { c.docCookie != null }
            win08log("doc.cookie=[${c.docCookie ?: "(none)"}]")
            mark("cookie-httponly-hidden-from-document", !(c.docCookie ?: "").contains("win08sess"))

            // Item 7 — a window.open/target=_blank popup must be BLOCKED: the page reported window.open()===null AND no
            // VISIBLE TOP-LEVEL WebView2 window exists (the embedded browser is a WS_CHILD, not enumerated). Sabotage
            // 'noblock' leaves required=false, so a chrome window opens and window.open returns a Window → this reds.
            waitFor(10_000) { c.openWindowBlocked != null }
            val topLevelWv = Win08Native.topLevelWebViewWindows()
            win08log("openwindow: blocked=${c.openWindowBlocked} topLevelWebViewWindows=$topLevelWv")
            mark("new-window-blocked", c.openWindowBlocked == true && topLevelWv == 0)

            c.probeOffOriginNav("https://example.org/win08")
            mark("off-origin-nav-vetoed", waitFor(10_000) { c.navVetoed })

            // S2.1 (R1): a HIDDEN Browser must still load and send `ready` — the premise of the Loading/cover state,
            // which hides the Shell (never removing the canvas) so Compose draws over the Studio area. Hide, reload,
            // and a fresh handshake must arrive. Sabotage `hiddennoready` skips the reload: no fresh ready → the check
            // reds (the positive control that proves it detects a page that loads but never handshakes — S2.1's cover case).
            val beforeReady = c.readyCount
            c.setShellVisible(false)
            if (System.getenv("WIN08_SABOTAGE") != "hiddennoready") c.reload()
            val hiddenReady = waitFor(20_000) { c.readyCount > beforeReady }
            mark("hidden-browser-loads-and-readies", hiddenReady)
            c.setShellVisible(true)   // restore before the DPI rect read; the Shell's bounds survive hide/show
            waitFor(1_500) { false }   // brief pause so the async re-show lands before measureDpi reads the rects

            val dotSwt = File(System.getProperty("user.home").orEmpty(), ".swt")
            if (System.getenv("WIN08_SABOTAGE") == "dotswt") runCatching { dotSwt.mkdirs() }
            val underRoot = edgeDataDir.absolutePath.startsWith(File(desktopDataRoot()).absolutePath)
            val noDotSwt = !dotSwt.exists()
            mark("data-dir-under-root-no-dotswt", edgeDataDir.exists() && underRoot && noDotSwt)
            // Clean up the dotswt sabotage's ~/.swt so it doesn't pollute later runs (user.home is shared in the job).
            if (System.getenv("WIN08_SABOTAGE") == "dotswt") runCatching { dotSwt.deleteRecursively() }

            // B2 — DPI: log awareness on the process, the EDT and the SWT thread; at 100% assert the embedded
            // Shell's client rect equals the canvas's. (Above 100% is VLL's Surface check, not asserted here.)
            c.measureDpi()
            val edtDpi = runCatching {
                var v = -1; javax.swing.SwingUtilities.invokeAndWait { v = Win08Native.threadDpiAwareness() }; v
            }.getOrDefault(-1)
            win08log("dpi: process=${Win08Native.processDpiAwareness()} edt=$edtDpi swt=${c.swtThreadDpiAwareness} " +
                "canvasClient=${c.canvasClient} shellClient=${c.shellClient} rectsMatch=${c.rectsMatch}")
            mark("dpi-shell-rect-equals-canvas-at-100", c.rectsMatch)

            // Item 8 — the ⋮ menu floats over the WebView WITHOUT stealing activation: open it, then the OS foreground
            // window is still the frame AND no window of this process is WS_EX_TOPMOST. Sabotage 'menufocus' shows a
            // FOCUSABLE menu window instead → GetForegroundWindow != frame → this reds.
            val frameWin = frame()
            if (frameWin != null) runCatching { javax.swing.SwingUtilities.invokeAndWait { frameWin.toFront(); frameWin.requestFocus() } }
            Thread.sleep(600)
            val fgBefore = frameWin != null && Win08Native.foregroundIsFrame(frameWin)
            c.openStudioMenu()
            Thread.sleep(1500) // let the popup show on the EDT
            val fgIsFrame = frameWin != null && Win08Native.foregroundIsFrame(frameWin)
            val menuWin = c.lastMenuWindow
            val menuTopmost = menuWin != null && Win08Native.isWindowTopmost(menuWin)
            win08log("menu: fgBefore=$fgBefore foregroundIsFrameWithMenu=$fgIsFrame menuShown=${menuWin != null} menuTopmost=$menuTopmost")
            // Pass needs the frame to have been foreground to begin with (else the runner never activated it, so the
            // check is moot); given that, opening the ⋮ menu must keep the frame foreground AND the menu window must
            // exist and not be WS_EX_TOPMOST. Sabotage 'menufocus' opens the real bad path (a compose.layers DropdownMenu
            // that DOES take activation on this runner) → fgIsFrame goes false → this reds (the foreground half proven).
            mark("menu-no-activation-steal", fgBefore && fgIsFrame && menuWin != null && !menuTopmost)

            // L1 — a click into Studio must dismiss the ⋮ menu. The AWTEventListener can't see a press inside the WebView
            // (another process's window under SWT's Shell), so the menu dismisses on the Browser's FocusIn. Model the
            // click by focusing the Browser; the menu window must be gone. Sabotage 'menusticky' skips that listener → reds.
            if (System.getenv("WIN08_SABOTAGE") != "menufocus") {  // (menufocus uses the Compose path; lastMenuWindow already null)
                c.focusBrowserForSmoke()
                Thread.sleep(1200)
                win08log("menu: lastMenuWindow after Studio focus = ${c.lastMenuWindow}")
                mark("menu-dismissed-on-studio-focus", c.lastMenuWindow == null)
            }

            // ── S2.1 (R1) — the no-ready → cover timer, scoped to host-initiated loads ─────────────────────────────
            // Refinement 2: an IN-PAGE same-origin navigation (a link; S2.4's in-place PDF) must NOT trip the cover —
            // it doesn't go through a host load path, so the timer is never armed. Navigate in-page to a page that
            // never handshakes and confirm navNoReadyFired stays false. Sabotage 'armoncompleted' re-arms on every
            // completed nav → it fires → this reds.
            serveNoReady = true
            c.smokeInPageNav("$origin/inpage")
            val inPageFired = waitFor(12_000) { c.navNoReadyFired }
            mark("in-page-nav-no-cover", !inPageFired)

            // Refinement 3 (the real S2.1 cover check): a HOST-initiated load of a never-ready page brings the cover up
            // within ~12s (navNoReadyFired). Sabotage 'noreadytimer' disables the timer → this reds.
            c.reload()   // host-initiated → arms the timer; serveNoReady still set → the page never readies
            val coverFired = waitFor(13_000) { c.navNoReadyFired }
            mark("host-load-never-ready-covers", coverFired)

            // Refinement 1 (second vector): Retry on the SAME controller recovers. Serve the ready page and reload
            // (what the cover's Retry does) — `ready` must arrive on the same controller instance and the shell rect
            // must match the canvas again (back to full size).
            val idBefore = System.identityHashCode(c)
            val beforeRetry = c.readyCount
            serveNoReady = false
            c.reload()
            val recovered = waitFor(15_000) { c.readyCount > beforeRetry }
            val sameController = System.identityHashCode(c) == idBefore
            c.measureDpi(); waitFor(1_500) { false }
            win08log("retry: recovered=$recovered sameController=$sameController(id=$idBefore) rectsMatch=${c.rectsMatch}")
            mark("retry-recovers-same-controller", recovered && sameController && c.rectsMatch)

            finish(results, frame)
        } catch (e: Throwable) {
            win08log("smoke: checker threw: ${e.message}")
            finish(results, frame)
        }
    }

    private fun finish(results: Map<String, Boolean>, frame: () -> java.awt.Window?) {
        val ok = results.isNotEmpty() && results.values.all { it }
        line("win08-smoke: ${results.count { it.value }}/${results.size} checks passed; served=$served cookie=[${cookieHeader ?: ""}]")
        if (!ok) {
            win08log("smoke: a check failed — halting 1")
            runCatching { serverSocket?.close() }
            Runtime.getRuntime().halt(1)
            return
        }
        // Check 7 — close the window the way a person does: WM_CLOSE to the frame → onCloseRequest=exitApplication →
        // the composition disposes → onDispose disposes the controller → the PROCESS must end on its OWN. No halt here;
        // the backstop halts only if it hangs (a check-7 failure), and the CI reports which path ended the process.
        val hwnd = frame()?.let { Win08Native.hwndOf(it) }
        if (hwnd == null) {
            win08log("smoke: no frame HWND to close — halting 0")
            runCatching { serverSocket?.close() }
            Runtime.getRuntime().halt(0)
            return
        }
        win08log("smoke: all checks passed — WM_CLOSE to the frame; the process must now exit on its own (check 7)")
        Win08Native.postClose(hwnd)
        // return — let application{} return and the JVM exit naturally (the daemon SWT thread + backstop don't block).
    }

    private companion object {
        // The handshake runs SYNCHRONOUSLY at the top of <head> — earlier than a module script, so a pass covers
        // Studio's ordering (B3). With SWT 3.134.0 the BrowserFunction is injected before this runs.
        const val PAGE = """<!doctype html><html><head><meta charset="utf-8"><script>
(function(){
  if (typeof window.TroubaStageShellReceive === "function") {
    window.__troubastageShell = { deliver: function(j){ try { if (JSON.parse(j).type === "hello") window.TroubaStageShellReceive(JSON.stringify({type:"ready"})); } catch(e){} } };
    window.TroubaStageShellReceive(JSON.stringify({type:"ready"}));
    // Item 5 — report what document.cookie can see. With an HttpOnly session cookie it must NOT contain the session,
    // while the request's Cookie header still carried it.
    window.TroubaStageShellReceive(JSON.stringify({type:"cookie", value: document.cookie}));
    // Item 7 — a popup must be BLOCKED: window.open() must return null (the host sets required=true, no NewWindow).
    // Under the 'noblock' sabotage a chrome window opens and window.open returns a Window object.
    var __ow = null; try { __ow = window.open('https://example.com/win08-openwindow','_blank'); } catch(e) {}
    window.TroubaStageShellReceive(JSON.stringify({type:"openwindow", blocked: (__ow === null)}));
    window.__win08 = "shell-present";
  } else {
    window.__win08 = "no-shell-at-head";
  }
})();
</script></head><body><h1>win08 loopback</h1></body></html>"""

        // S2.1: loads and completes, but NEVER calls TroubaStageShellReceive → no `ready`. Used for the never-ready
        // cover check and the in-page-nav-no-cover vector.
        const val PAGE_NOREADY = """<!doctype html><html><head><meta charset="utf-8"></head><body><h1>no ready</h1></body></html>"""
    }
}
