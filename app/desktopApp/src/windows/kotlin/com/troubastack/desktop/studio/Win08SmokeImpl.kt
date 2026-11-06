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
    @Volatile private var capturedBack: (() -> Unit)? = null   // S2.2: the back action the provider hands the host (chrome-back check)
    @Volatile private var serveNoReady = false   // S2.1: when set, the loopback serves a page that never handshakes
    @Volatile private var accepting = true        // S2.1 retry: when false, the loopback closes connections at once (server "off")
    @Volatile private var serverSocket: ServerSocket? = null
    @Volatile private var serveSetCookie = false  // S2.3: when set, the page response sets win08sess as a PERSISTENT cookie (Expires +30d)
    @Volatile private var reqCount = 0            // S2.3: count served page requests, so a post-clear reload can be confirmed to have arrived
    private val cookieByPort = java.util.concurrent.ConcurrentHashMap<Int, String>()  // S2.3b: the last Cookie header per server port

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
                provideRequestBack = { capturedBack = it },   // S2.2: capture the host's Back action (what WindowAction.BACK calls)
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
                probeReachable = { HttpTransport(storage, defaultServerUrl = null) { _, _ -> }.probePresence() !is Presence.Unreachable },
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

    /**
     * S2.2 (recon) — the real mouse back button (XBUTTON1). Builds same-origin history, moves the cursor over the
     * Studio area and synthesises XBUTTON1 with Robot, then records: did SWT's MouseListener see it (swtSawMouseBack)?
     * did the injected page listener fire (mouseBackListenerFired)? how many navigations did one press cause (navDelta
     * — 1 = preventDefault held, 2 = WebView2 double-navigated)? Also tries Alt+←. Writes back-verdict.txt + back.png.
     */
    override fun runBack(outDir: File): Int {
        outDir.mkdirs(); enableWin08FileLog()
        win08log("back: run start; outDir=$outDir")
        Thread { try { Thread.sleep(90_000) } catch (_: InterruptedException) { return@Thread }
            win08log("back: BACKSTOP firing (>90s)"); Runtime.getRuntime().halt(2) }.apply { isDaemon = true; start() }
        try {
            val server = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
            serverSocket = server
            val origin = "http://127.0.0.1:${server.localPort}"
            Thread { acceptLoop(server) }.apply { isDaemon = true; start() }
            val edgeDataDir = File(desktopDataRoot(), "webview2")
            val frameHolder = java.util.concurrent.atomic.AtomicReference<java.awt.Window?>()
            val args = EmbeddedStudioArgs(origin, null, "TroubaStudio", "win08sess=smoke", edgeDataDir, {}, {}, { false }, { frameHolder.get() }, null)
            val ctrlRef = java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>()
            application {
                Window(onCloseRequest = ::exitApplication, title = "TroubaStage", state = rememberWindowState(width = 1200.dp, height = 820.dp)) {
                    EmbeddedStudioContent(args, onController = { ctrlRef.set(it) })
                    LaunchedEffect(Unit) { frameHolder.set(window); Thread { backFlow(outDir, origin, ctrlRef, frameHolder) }.apply { isDaemon = true; start() } }
                }
            }
        } catch (e: Throwable) { win08log("back: run FAILED: $e\n${e.stackTraceToString()}"); Runtime.getRuntime().halt(2) }
        return 0
    }

    private fun backFlow(
        outDir: File, origin: String,
        ctrlRef: java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>,
        frameHolder: java.util.concurrent.atomic.AtomicReference<java.awt.Window?>,
    ) {
        fun waitFor(ms: Long, cond: () -> Boolean): Boolean {
            val end = System.currentTimeMillis() + ms
            while (System.currentTimeMillis() < end) { if (cond()) return true; Thread.sleep(100) }
            return cond()
        }
        waitFor(25_000) { ctrlRef.get()?.bridgeReady == true }
        val c = ctrlRef.get() ?: run { win08log("back: no controller"); Runtime.getRuntime().halt(2); return }
        for (n in 1..3) { c.smokeInPageNav("$origin/b$n"); waitFor(3_500) { false } }   // same-origin history
        val f = frameHolder.get() ?: run { win08log("back: no frame"); Runtime.getRuntime().halt(2); return }
        val robot = java.awt.Robot()
        val b = f.bounds
        robot.mouseMove(b.x + b.width / 2, b.y + 60 + (b.height - 60) / 2)   // cursor over the Studio area
        waitFor(400) { false }
        val navBefore = c.navCount; val urlBefore = c.lastUrl
        val extraOk = java.awt.Toolkit.getDefaultToolkit().areExtraMouseButtonsEnabled()
        val mask = runCatching { java.awt.event.InputEvent.getMaskForButton(4) }.getOrDefault(0)
        if (extraOk && mask != 0) runCatching { robot.mousePress(mask); robot.mouseRelease(mask) }
        waitFor(5_000) { c.navCount > navBefore || c.mouseBackListenerFired }
        val mouseNavDelta = c.navCount - navBefore
        win08log("back: XBUTTON1 extraOk=$extraOk mask=$mask swtSaw=${c.swtSawMouseBack} listenerFired=${c.mouseBackListenerFired} navDelta=$mouseNavDelta urlBefore=$urlBefore urlAfter=${c.lastUrl}")
        // Alt+← via Robot (focus the browser first).
        c.focusBrowserForSmoke(); waitFor(600) { false }
        val navBeforeAlt = c.navCount
        runCatching { robot.keyPress(java.awt.event.KeyEvent.VK_ALT); robot.keyPress(java.awt.event.KeyEvent.VK_LEFT); robot.keyRelease(java.awt.event.KeyEvent.VK_LEFT); robot.keyRelease(java.awt.event.KeyEvent.VK_ALT) }
        waitFor(4_000) { c.navCount > navBeforeAlt }
        val altNavDelta = c.navCount - navBeforeAlt
        win08log("back: Alt+Left navDelta=$altNavDelta wentBack=${c.wentBack}")
        // Walk to the root and confirm the next Back LEAVES the screen (requestBack returns wentBack=false → onExitBack).
        var leftAtRoot = false
        for (i in 1..8) { c.requestBack(); waitFor(1_800) { false }; if (c.wentBack == false) { leftAtRoot = true; break } }   // fixed wait: requestBack is async
        win08log("back: leftAtRoot=$leftAtRoot")
        runCatching { val img = robot.createScreenCapture(java.awt.Rectangle(b.x, b.y, b.width, b.height)); javax.imageio.ImageIO.write(img, "png", File(outDir, "back.png")) }
        runCatching { File(outDir, "back-verdict.txt").writeText(
            "swtSawMouseBack=${c.swtSawMouseBack} mouseListenerFired=${c.mouseBackListenerFired} mouseNavDelta=$mouseNavDelta " +
            "altNavDelta=$altNavDelta leftAtRoot=$leftAtRoot extraMouseButtons=$extraOk") }
        Runtime.getRuntime().halt(0)
    }

    override fun runCookieClear(outDir: File): Int {
        outDir.mkdirs(); enableWin08FileLog()
        win08log("cookieclear: run start; log=$win08LogFile")
        Thread { try { Thread.sleep(90_000) } catch (_: InterruptedException) { return@Thread }
            win08log("cookieclear: BACKSTOP firing (>90s)"); Runtime.getRuntime().halt(1) }.apply { isDaemon = true; start() }
        val server = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
        serverSocket = server
        val origin = "http://127.0.0.1:${server.localPort}"
        Thread { acceptLoop(server) }.apply { isDaemon = true; start() }
        val edgeDataDir = File(desktopDataRoot(), "webview2")
        // Controller 1 carries a session cookie; controller 2 reopens SIGNED OUT (seed = null), so nothing re-seeds
        // win08sess after the pending clear — the reopen's Cookie header is clean only if the clear actually ran.
        fun argsFor(cyc: Int) = EmbeddedStudioArgs(
            origin, null, "TroubaStudio", if (cyc == 0) "win08sess=smoke" else null, edgeDataDir, {}, {}, { false }, { null }, null)
        val ctrlRef = java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>()
        val cyc = mutableStateOf(0)
        val shown = mutableStateOf(true)
        application {
            Window(onCloseRequest = ::exitApplication, title = "TroubaStage", state = rememberWindowState(width = 1200.dp, height = 820.dp)) {
                androidx.compose.runtime.key(cyc.value) { if (shown.value) EmbeddedStudioContent(argsFor(cyc.value), onController = { ctrlRef.set(it) }) }
                LaunchedEffect(Unit) { Thread { cookieClearFlow(origin, edgeDataDir, ctrlRef, cyc, shown) }.apply { isDaemon = true; start() } }
            }
        }
        return 0
    }

    private fun cookieClearFlow(
        origin: String, edgeDataDir: File,
        ctrlRef: java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>,
        cyc: androidx.compose.runtime.MutableState<Int>,
        shown: androidx.compose.runtime.MutableState<Boolean>,
    ) {
        val results = LinkedHashMap<String, Boolean>()
        fun mark(name: String, ok: Boolean) { results[name] = ok; line("WIN08-CHECK $name: ${if (ok) "PASS" else "FAIL"}") }
        fun waitFor(ms: Long, cond: () -> Boolean): Boolean {
            val end = System.currentTimeMillis() + ms
            while (System.currentTimeMillis() < end) { if (cond()) return true; Thread.sleep(100) }
            return cond()
        }
        try {
            // Controller 1: let the server set win08sess as a PERSISTENT cookie, and confirm it round-trips.
            val c1 = if (waitFor(40_000) { ctrlRef.get()?.bridgeReady == true }) ctrlRef.get() else null
            if (c1 == null) { mark("cookieclear-controller1", false); finish(results) { null }; return }
            // Register controller 1 as the provider's live controller (Host does this in production). After it is
            // disposed, the provider sees threadAlive()==false and routes the clear to the pending-file path — so the
            // smoke exercises the real live-vs-pending routing, not just a null liveController.
            SwtEmbeddedStudioProvider.liveController = c1
            serveSetCookie = true
            c1.reload(); waitFor(4_000) { false }
            var n = reqCount; c1.reload()
            val hadPersist = waitFor(5_000) { reqCount > n } && (cookieHeader ?: "").contains("win08sess=persist")
            serveSetCookie = false
            win08log("cookieclear: hadPersist=$hadPersist cookie=[${cookieHeader ?: ""}]")

            // Sign out while Studio is OPEN→CLOSED: dispose controller 1 FIRST (shown=false, no rebuild yet), then
            // request the clear with no live Browser — the provider has no live controller (threadAlive false) → it
            // persists a pending-clear file. Disposing before the clear is what makes this the pending path, not the
            // live path, and it must finish before controller 2 starts, or 2 could read the file before it is written.
            val old = c1
            shown.value = false
            waitFor(4_000) { !old.threadAlive() }
            ctrlRef.set(null)
            SwtEmbeddedStudioProvider().clearCookies(edgeDataDir, origin, "win08sess")
            // A SECOND sign-out (switch to another server, sign out again) BEFORE reopening: a single-record file would
            // overwrite origin's record with this decoy, so origin's persistent cookie would survive the reopen. The
            // append fix keeps both records and applies them all — so applied-on-reopen below discriminates that bug.
            SwtEmbeddedStudioProvider().clearCookies(edgeDataDir, "http://127.0.0.1:1", "win08sess")
            val pendingWritten = File(edgeDataDir, "pending-cookie-clear").exists()
            win08log("cookieclear: controller1 disposed; pendingWritten=$pendingWritten records=${runCatching { File(edgeDataDir, "pending-cookie-clear").readLines().count { it.isNotBlank() } }.getOrDefault(-1)}")
            mark("pending-clear-file-written-when-closed", pendingWritten)

            // Reopen SIGNED OUT: a fresh key + controller (seed = null). Its run() applies the pending clear BEFORE
            // seeding, and nothing re-seeds. Confirm its load carries no win08sess.
            cookieHeader = null; n = reqCount
            cyc.value = 1; shown.value = true
            val c2 = if (waitFor(40_000) { ctrlRef.get().let { it != null && it !== old && it.bridgeReady } }) ctrlRef.get() else null
            if (c2 == null) { mark("cookieclear-controller2", false); finish(results) { null }; return }
            val reloaded = waitFor(8_000) { reqCount > n }
            val clean = reloaded && !(cookieHeader ?: "").contains("win08sess")
            val pendingGone = !File(edgeDataDir, "pending-cookie-clear").exists()
            win08log("cookieclear: reopen reloaded=$reloaded header=[${cookieHeader ?: ""}] pendingGone=$pendingGone")
            mark("pending-clear-applied-on-reopen", hadPersist && clean)
            mark("pending-clear-record-consumed", pendingGone)
        } catch (e: Throwable) {
            win08log("cookieclear: flow threw: ${e.message}")
        }
        finish(results) { null }
    }

    override fun runServerUrl(outDir: File): Int {
        outDir.mkdirs(); enableWin08FileLog()
        win08log("serverurl: run start; log=$win08LogFile")
        Thread { try { Thread.sleep(150_000) } catch (_: InterruptedException) { return@Thread }
            win08log("serverurl: BACKSTOP firing (>150s)"); Runtime.getRuntime().halt(1) }.apply { isDaemon = true; start() }
        val server1 = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
        val server2 = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
        serverSocket = server1
        val origin1 = "http://127.0.0.1:${server1.localPort}"
        val origin2 = "http://127.0.0.1:${server2.localPort}"
        Thread { acceptLoop(server1) }.apply { isDaemon = true; start() }
        Thread { acceptLoop(server2) }.apply { isDaemon = true; start() }
        val edgeDataDir = File(desktopDataRoot(), "webview2")
        // The re-point ALWAYS clears through the next-open (pending) path, never the old live controller. The "clear on
        // the old controller" sabotage (repointclearlive) was DROPPED: it is not reproducible on CI — run 42151247550
        // showed the old-controller clear ran (origin1 cleared) and the new session survived, so neither guard could
        // red against it. The pending path removes the old thread from the re-point entirely, so it is correct by
        // construction; the two guards below have deterministic reds (clearafterseed on survival, repointnowrite on the
        // re-point's pending write).
        val ctrlRef = java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>()
        val cyc = mutableStateOf(0)
        val server = mutableStateOf(origin1)
        val repoint: (String) -> Unit = { _ ->
            val oldOrigin = server.value
            SwtEmbeddedStudioProvider().clearCookiesOnNextOpen(edgeDataDir, oldOrigin, "win08sess")
            server.value = if (oldOrigin == origin1) origin2 else origin1
            cyc.value = cyc.value + 1
        }
        fun argsFor(c: Int) = EmbeddedStudioArgs(
            server.value, null, "TroubaStudio", "win08sess=sess$c", edgeDataDir,
            {}, {}, { false }, { null }, null, onServerUrlChange = repoint)
        application {
            Window(onCloseRequest = ::exitApplication, title = "TroubaStage", state = rememberWindowState(width = 1200.dp, height = 820.dp)) {
                androidx.compose.runtime.key(cyc.value) { EmbeddedStudioContent(argsFor(cyc.value), onController = { ctrlRef.set(it); SwtEmbeddedStudioProvider.liveController = it }) }
                LaunchedEffect(Unit) { Thread { serverUrlFlow(origin1, origin2, ctrlRef, cyc, repoint) }.apply { isDaemon = true; start() } }
            }
        }
        return 0
    }

    private fun serverUrlFlow(
        origin1: String, origin2: String,
        ctrlRef: java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>,
        cyc: androidx.compose.runtime.MutableState<Int>,
        repoint: (String) -> Unit,
    ) {
        fun portOf(o: String) = o.substringAfterLast(':').toInt()
        val results = LinkedHashMap<String, Boolean>()
        fun mark(name: String, ok: Boolean) { results[name] = ok; line("WIN08-CHECK $name: ${if (ok) "PASS" else "FAIL"}") }
        fun waitFor(ms: Long, cond: () -> Boolean): Boolean {
            val end = System.currentTimeMillis() + ms
            while (System.currentTimeMillis() < end) { if (cond()) return true; Thread.sleep(80) }
            return cond()
        }
        try {
            var c = if (waitFor(40_000) { ctrlRef.get()?.bridgeReady == true }) ctrlRef.get() else null
            if (c == null) { mark("serverurl-controller1", false); finish(results) { null }; return }

            // FOCUS proof: open the dialog (panel collapses to 0.dp), then type into the Compose TextField via Robot.
            // smokeDialogText is seeded to origin1 on open; if the keystrokes land, it grows past origin1's length.
            c.requestServerDialog?.invoke()
            waitFor(4_000) { SwtEmbeddedStudioProvider.smokeDialogText == origin1 }
            val robot = java.awt.Robot()
            for (k in intArrayOf(java.awt.event.KeyEvent.VK_Z, java.awt.event.KeyEvent.VK_Z)) { robot.keyPress(k); robot.keyRelease(k); Thread.sleep(120) }
            val typed = waitFor(3_000) { (SwtEmbeddedStudioProvider.smokeDialogText ?: "").length > origin1.length }
            win08log("serverurl: focus dialogText=[${SwtEmbeddedStudioProvider.smokeDialogText}] typed=$typed")
            mark("serverurl-textfield-takes-keystrokes-at-0dp", typed)

            // RE-POINT 1→2 (mimics the dialog's Save). Two guards:
            //  (a) the new origin carries its FRESH session — the re-point clear runs on the new controller BEFORE it
            //      seeds, so the new session is never wiped. Sabotage clearafterseed defers the clear past the seed → red.
            //  (b) the re-point actually WROTE a pending record that the new controller APPLIED — repointServer must
            //      request the clear. Sabotage repointnowrite skips the write → appliedPendingRecords is 0 → red (and
            //      nopending reds it too, by skipping the apply). This guards the re-point's own wiring, which cookieclear
            //      (a different flow) cannot.
            // (The earlier signed-out "old origin cleared" check was dropped: the old origin's cookie did not survive the
            //  cross-origin controller cycling to the return — origins were fixed strings and the persistent cookie was
            //  set (hadPersist=true), yet it read empty regardless of any clear, so the check could not discriminate. The
            //  clear-applied invariant it meant to guard is covered by cookieclear's pending-clear-applied-on-reopen.)
            val expect1 = cyc.value + 1
            repoint("")
            c = if (waitFor(40_000) { ctrlRef.get().let { it != null && it !== c && it.bridgeReady } }) ctrlRef.get() else null
            if (c == null) { mark("serverurl-controller2", false); finish(results) { null }; return }
            val survived = waitFor(8_000) { (cookieByPort[portOf(origin2)] ?: "").contains("win08sess=sess$expect1") }
            win08log("serverurl: repoint 1->2 cookie@2=[${cookieByPort[portOf(origin2)] ?: ""}] survived=$survived")
            mark("serverurl-new-session-survives-repoint", survived)
            val applied = (c?.appliedPendingRecords ?: 0) >= 1
            win08log("serverurl: repoint applied pending records=${c?.appliedPendingRecords}")
            mark("serverurl-repoint-applies-pending-clear", applied)
        } catch (e: Throwable) {
            win08log("serverurl: flow threw: ${e.message}")
        }
        finish(results) { null }
    }

    override fun runLinks(outDir: File): Int {
        outDir.mkdirs(); enableWin08FileLog()
        win08log("links: run start; log=$win08LogFile")
        Thread { try { Thread.sleep(120_000) } catch (_: InterruptedException) { return@Thread }
            win08log("links: BACKSTOP firing (>120s)"); Runtime.getRuntime().halt(1) }.apply { isDaemon = true; start() }
        val server = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
        serverSocket = server
        val origin = "http://127.0.0.1:${server.localPort}"
        Thread { acceptLoop(server) }.apply { isDaemon = true; start() }
        val edgeDataDir = File(desktopDataRoot(), "webview2")
        val recorded = java.util.concurrent.atomic.AtomicReference<String?>(null)   // the CI stub records the off-origin URL
        val ctrlRef = java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>()
        val args = EmbeddedStudioArgs(origin, null, "TroubaStudio", "win08sess=smoke", edgeDataDir,
            {}, {}, { false }, { null }, null, openInDefaultBrowser = { url -> recorded.set(url); true })
        application {
            Window(onCloseRequest = ::exitApplication, title = "TroubaStage", state = rememberWindowState(width = 1200.dp, height = 820.dp)) {
                EmbeddedStudioContent(args, onController = { ctrlRef.set(it) })
                LaunchedEffect(Unit) { Thread { linksFlow(origin, recorded, ctrlRef) }.apply { isDaemon = true; start() } }
            }
        }
        return 0
    }

    private fun linksFlow(
        origin: String,
        recorded: java.util.concurrent.atomic.AtomicReference<String?>,
        ctrlRef: java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>,
    ) {
        val results = LinkedHashMap<String, Boolean>()
        fun mark(name: String, ok: Boolean) { results[name] = ok; line("WIN08-CHECK $name: ${if (ok) "PASS" else "FAIL"}") }
        fun waitFor(ms: Long, cond: () -> Boolean): Boolean {
            val end = System.currentTimeMillis() + ms
            while (System.currentTimeMillis() < end) { if (cond()) return true; Thread.sleep(80) }
            return cond()
        }
        try {
            val c = if (waitFor(40_000) { ctrlRef.get()?.bridgeReady == true }) ctrlRef.get() else null
            if (c == null) { mark("links-controller", false); finish(results) { null }; return }
            // The capture is attached in completed(), which can fire AFTER the head-script bridge handshake — wait for
            // its linksready signal so the first click isn't missed (seen as a race: only the native listener fired).
            if (!waitFor(15_000) { c.linksInjected }) { mark("links-capture-attached", false); finish(results) { null }; return }

            // Vector 2 first (no main-browser nav): another http(s) origin → the default browser (the stub records it),
            // WITHOUT the session, and the main browser stays put. Sabotage linksalwaysinplace reds this (setUrl vetoed,
            // stub never called).
            val external = "https://external.example.invalid/x"
            c.smokeClickLink(external, "_blank")
            val v2 = waitFor(6_000) { recorded.get() == external } && c.lastOpenExternal == external
            win08log("links: other-origin recorded=${recorded.get()} lastExternal=${c.lastOpenExternal}")
            mark("links-other-origin-to-default-browser", v2)

            // Vector 3: non-http(s) schemes are DROPPED by the scheme filter — not opened, not navigated. BOTH a
            // search-ms: and a file: link must be dropped (each caught, the stub untouched). Sabotage nolinkfilter reds
            // this (the scheme reaches the default browser / setUrl, so it isn't dropped).
            val recBeforeBad = recorded.get()
            c.smokeClickLink("search-ms:query=win08", "_blank")
            c.smokeClickLink("file:///C:/win08", "_blank")
            // Both must be dropped. Use the accumulator (droppedUrls), not lastOpenDropped — the two drops can land a
            // couple of ms apart, so the last-one field would lose the first to a race.
            val bothDropped = waitFor(4_000) { c.droppedUrls.contains("search-ms") && c.droppedUrls.contains("file:") }
            val v3 = bothDropped && recorded.get() == recBeforeBad
            win08log("links: bad-scheme droppedUrls=[${c.droppedUrls.replace("\n", " ")}] recorded=${recorded.get()}")
            mark("links-bad-scheme-dropped", v3)

            // Vector 1 last (navigates in place): the server's own origin opens in the embedded view AND the main
            // browser ENDS on that page (the spec's "end on the right page"), with the stub NOT called. Sabotage
            // linksalwaysexternal reds it (same-origin goes external); linksnoplaceload reds it (decided, never loaded).
            val recBeforeSame = recorded.get()
            c.smokeClickLink("$origin/inplace", "_blank")
            val routedInPlace = waitFor(6_000) { c.lastOpenInPlace == "$origin/inplace" }
            val landedInPlace = waitFor(8_000) { c.lastUrl == "$origin/inplace" }
            val v1 = routedInPlace && landedInPlace && recorded.get() == recBeforeSame
            win08log("links: same-origin lastInPlace=${c.lastOpenInPlace} navUrl=${c.lastUrl} landed=$landedInPlace")
            mark("links-same-origin-in-place", v1)

            // No in-app window ever: count top-level WebView2 windows (item 7). The embedded browser is a WS_CHILD, not
            // enumerated, so >0 means a real popup window opened. Sabotage noblock (the OpenWindowListener backstop off)
            // lets one open → this reds.
            val topWv = Win08Native.topLevelWebViewWindows()
            win08log("links: topLevelWebViewWindows=$topWv (expect 0)")
            mark("links-no-new-window", topWv == 0)
        } catch (e: Throwable) {
            win08log("links: flow threw: ${e.message}")
        }
        finish(results) { null }
    }

    override fun runDownload(outDir: File): Int {
        outDir.mkdirs(); enableWin08FileLog()
        win08log("download: run start; log=$win08LogFile")
        Thread { try { Thread.sleep(120_000) } catch (_: InterruptedException) { return@Thread }
            win08log("download: BACKSTOP firing (>120s)"); Runtime.getRuntime().halt(1) }.apply { isDaemon = true; start() }
        val server = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
        serverSocket = server
        val origin = "http://127.0.0.1:${server.localPort}"
        Thread { acceptLoop(server) }.apply { isDaemon = true; start() }
        val edgeDataDir = File(desktopDataRoot(), "webview2")
        val ctrlRef = java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>()
        val frameHolder = java.util.concurrent.atomic.AtomicReference<java.awt.Window?>()
        val args = EmbeddedStudioArgs(origin, null, "TroubaStudio", "win08sess=smoke", edgeDataDir, {}, {}, { false }, { frameHolder.get() }, null)
        application {
            Window(onCloseRequest = ::exitApplication, title = "TroubaStage", state = rememberWindowState(width = 1200.dp, height = 820.dp)) {
                EmbeddedStudioContent(args, onController = { ctrlRef.set(it) })
                LaunchedEffect(Unit) { frameHolder.set(window); Thread { downloadFlow(outDir, origin, ctrlRef, frameHolder) }.apply { isDaemon = true; start() } }
            }
        }
        return 0
    }

    private fun downloadFlow(
        outDir: File, origin: String,
        ctrlRef: java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>,
        frameHolder: java.util.concurrent.atomic.AtomicReference<java.awt.Window?>,
    ) {
        val results = LinkedHashMap<String, Boolean>()
        fun mark(name: String, ok: Boolean) { results[name] = ok; line("WIN08-CHECK $name: ${if (ok) "PASS" else "FAIL"}") }
        fun waitFor(ms: Long, cond: () -> Boolean): Boolean {
            val end = System.currentTimeMillis() + ms
            while (System.currentTimeMillis() < end) { if (cond()) return true; Thread.sleep(100) }
            return cond()
        }
        try {
            val c = if (waitFor(40_000) { ctrlRef.get()?.linksInjected == true }) ctrlRef.get() else null
            if (c == null) { mark("download-controller", false); finish(results) { null }; return }

            // Fable #2: the PDF endpoint needs the session cookie — a raw GET with no Cookie gets 401.
            val noCookie = rawGetStatus(origin, "/pdf")
            win08log("download: /pdf no-cookie status=$noCookie (expect 401)")
            mark("download-requires-cookie", noCookie == 401)

            // The real saveBlob path. Assert the exact non-ASCII file is ABSENT before, then PRESENT with a SHA-256
            // equal to the served bytes — proving the credentialed fetch, the Content-Disposition name, and that the
            // immediate revokeObjectURL did not race the save. Sabotage catchdownload cancels the click → no file → red.
            val expected = File(Win08Native.downloadsFolder(), "Étude n°1.pdf")   // Étude n°1.pdf
            runCatching { expected.delete() }
            val absentBefore = !expected.exists()
            c.smokeSaveBlob("$origin/pdf")
            val done = waitFor(10_000) { c.downloadDone || c.downloadErr != null }
            val landed = waitFor(25_000) { expected.exists() && expected.length() == PDF_BYTES.size.toLong() }
            val shaOk = landed && sha256(expected.readBytes()) == sha256(PDF_BYTES)
            win08log("download: name=[${c.downloadName}] err=[${c.downloadErr}] absentBefore=$absentBefore done=$done landed=$landed shaOk=$shaOk dir=${expected.parent}")
            // Fable #7: if CI prompts for a save location, the file won't land — report it, don't fight.
            if (!landed) win08log("download: file did NOT land — if WebView2 showed a save PROMPT in CI, that is the cause (reported, not fought)")
            mark("download-saves-to-downloads", absentBefore && c.downloadName == "Étude n°1.pdf" && landed && shaOk)

            // Fable #5: capture the window (WebView2's download panel open) and leave the PNG for inspection.
            runCatching {
                val f = frameHolder.get(); if (f != null) {
                    val b = f.bounds
                    val img = java.awt.Robot().createScreenCapture(java.awt.Rectangle(b.x, b.y, b.width, b.height))
                    val png = File(outDir, "download-panel.png"); javax.imageio.ImageIO.write(img, "png", png)
                    win08log("download: panel PNG at ${png.absolutePath}")
                }
            }
        } catch (e: Throwable) {
            win08log("download: flow threw: ${e.message}")
        }
        finish(results) { null }
    }

    /** SHA-256 hex of [b]. */
    private fun sha256(b: ByteArray): String =
        java.security.MessageDigest.getInstance("SHA-256").digest(b).joinToString("") { "%02x".format(it) }

    /** A raw HTTP/1.0 GET with NO Cookie; returns the status code (or -1). Proves the /pdf endpoint needs the session. */
    private fun rawGetStatus(origin: String, path: String): Int = runCatching {
        val port = origin.substringAfterLast(':').toInt()
        java.net.Socket(InetAddress.getLoopbackAddress(), port).use { s ->
            s.getOutputStream().write("GET $path HTTP/1.0\r\nConnection: close\r\n\r\n".toByteArray(StandardCharsets.US_ASCII)); s.getOutputStream().flush()
            val line = s.getInputStream().bufferedReader().readLine() ?: ""   // "HTTP/1.0 401 Unauthorized"
            line.split(' ').getOrNull(1)?.toIntOrNull() ?: -1
        }
    }.getOrDefault(-1)

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
                        var reqCookie = ""
                        var l = reader.readLine()
                        while (l != null && l.isNotEmpty()) {
                            if (l.startsWith("Cookie:", ignoreCase = true)) { cookieHeader = l.substringAfter(":").trim(); cookieByPort[s.localPort] = cookieHeader!!; reqCookie = cookieHeader!! }
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
                        } else if (path.startsWith("/pdf")) {
                            // S2.5: the PDF endpoint needs the seeded HttpOnly session cookie (credentials:"include"); 401
                            // without it. The name comes from Content-Disposition — both filename and filename*=UTF-8''
                            // carry a synthetic non-ASCII name "Étude n°1.pdf" (T162 sends exact UTF-8).
                            if (!reqCookie.contains("win08sess")) {
                                out.write("HTTP/1.0 401 Unauthorized\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".toByteArray(StandardCharsets.US_ASCII)); out.flush()
                            } else {
                                val cd = "attachment; filename=\"Etude n1.pdf\"; filename*=UTF-8''%C3%89tude%20n%C2%B01.pdf"
                                val head = "HTTP/1.0 200 OK\r\nContent-Type: application/pdf\r\nContent-Disposition: $cd\r\n" +
                                    "Content-Length: ${PDF_BYTES.size}\r\nConnection: close\r\n\r\n"
                                out.write(head.toByteArray(StandardCharsets.US_ASCII)); out.write(PDF_BYTES); out.flush()
                            }
                        } else {
                            reqCount++
                            // S2.1: a never-ready page (loads but never handshakes) while serveNoReady is set.
                            val pageStr = if (serveNoReady) PAGE_NOREADY else PAGE
                            val body = pageStr.toByteArray(StandardCharsets.UTF_8)
                            // S2.3: the server sets win08sess as a PERSISTENT cookie — same name, Path=/, no Domain (host-only),
                            // Expires +30d — so the clear test proves Max-Age=0 on (name,domain,path) deletes a persistent cookie,
                            // which clearSessions() (option A) alone would NOT. HttpOnly matches the seeded session cookie.
                            val setCookie = if (serveSetCookie)
                                "Set-Cookie: win08sess=persist; Path=/; Expires=Tue, 01 Dec 2026 00:00:00 GMT; HttpOnly\r\n" else ""
                            val head = "HTTP/1.0 200 OK\r\nContent-Type: text/html; charset=utf-8\r\n" + setCookie +
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
        // S2.3: time each check (since the previous mark) and the cumulative, so the backstop + engine wait get sized
        // from real green-run numbers (slowest of 3 + margin) instead of by feel. Totals logged at finish().
        val t0 = System.currentTimeMillis(); var lastMark = t0
        fun mark(name: String, ok: Boolean) {
            results[name] = ok
            val now = System.currentTimeMillis()
            line("WIN08-CHECK $name: ${if (ok) "PASS" else "FAIL"} (+${now - lastMark}ms, @${now - t0}ms)")
            lastMark = now
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

            // ── S2.2 Back/history ─────────────────────────────────────────────────────────────────────────────────
            // ‹ Back / Alt+← / the mouse back button walk Studio's own history first (browser.back() while isBackEnabled),
            // leaving the screen only at the root. Build history (same-origin navs), then requestBack must WALK.
            serveNoReady = false
            c.smokeInPageNav("$origin/back1"); waitFor(4_000) { false }
            c.smokeInPageNav("$origin/back2"); waitFor(4_000) { false }
            // The injected mouse-back listener (button===3) forwards through the bridge → requestBack; dispatch a synthetic
            // mouseup to exercise that chain. Sabotage nomouseback skips the injection → the listener never fires.
            c.smokeDispatchMouseBack()
            val mouseFwd = waitFor(5_000) { c.mouseBackListenerFired }
            mark("mouse-back-injection-forwards", mouseFwd)
            waitFor(2_000) { false }   // let that back settle
            // requestBack with history left → walks (wentBack=true). Sabotage backalwaysleaves ignores history → reds.
            c.requestBack()
            val walked = waitFor(4_000) { c.wentBack == true }
            win08log("back: mouseListenerFired=${c.mouseBackListenerFired} swtSawMouseBack=${c.swtSawMouseBack} wentBack=${c.wentBack}")
            mark("host-back-walks-history", walked)

            // S2.2: WindowAction.BACK (chrome-focused Alt+←/Cmd+[/Cmd+←) must walk history too — the host's BackHandler
            // calls the provider-supplied Back action (captured here). Build history, invoke it, assert a navigation
            // happened (it WALKED, not left). Sabotage backprovidesexit hands back onBack → no nav → reds.
            c.smokeInPageNav("$origin/chrome1"); waitFor(4_000) { false }
            val navBeforeChrome = c.navCount
            capturedBack?.invoke()
            val chromeWalked = waitFor(5_000) { c.navCount > navBeforeChrome }
            win08log("back: chrome capturedBack=${capturedBack != null} navDelta=${c.navCount - navBeforeChrome}")
            mark("chrome-back-walks-history", chromeWalked)

            // ── S2.3 cookie clearing (server change from inside Studio, live Browser) ────────────────────────────────
            // Upgrade win08sess to a PERSISTENT cookie (server Set-Cookie +Expires), confirm it round-trips, then
            // clearCookiesNow (C: Max-Age=0 on win08sess + A: clearSessions) and reload: the request must carry NO
            // win08sess. Sabotage clearonlya skips C (A only) → the persistent cookie SURVIVES → this check reds,
            // proving C — not clearSessions — is what deletes the persistent half (Fable #3 "measure it").
            serveNoReady = false
            serveSetCookie = true
            c.reload(); waitFor(4_000) { false }                                   // server sets win08sess=persist
            var n = reqCount; c.reload()
            val hadPersist = waitFor(5_000) { reqCount > n } && (cookieHeader ?: "").contains("win08sess=persist")
            serveSetCookie = false
            c.clearCookiesNow(origin, "win08sess"); waitFor(2_000) { false }
            cookieHeader = null; n = reqCount; c.reload()
            val reloaded = waitFor(5_000) { reqCount > n }
            val cleared = reloaded && !(cookieHeader ?: "").contains("win08sess")
            win08log("cookieclear(live): hadPersist=$hadPersist reloaded=$reloaded afterClear=[${cookieHeader ?: ""}]")
            mark("cookie-clear-removes-persistent-session", hadPersist && cleared)

            line("WIN08 TIMING total=${System.currentTimeMillis() - t0}ms for ${results.size} checks (size the backstop + engine wait from this)")
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
        // S2.5: the bytes the /pdf endpoint serves; the smoke SHA-256s the saved file against these. Any bytes work
        // (saveBlob saves the Blob verbatim); it begins with %PDF so it reads as a PDF to anything that peeks.
        val PDF_BYTES: ByteArray = "%PDF-1.4\n% win08 S2.5 download smoke\n1 0 obj<</Type/Catalog>>endobj\ntrailer<<>>\n%%EOF\n".toByteArray(StandardCharsets.US_ASCII)
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
