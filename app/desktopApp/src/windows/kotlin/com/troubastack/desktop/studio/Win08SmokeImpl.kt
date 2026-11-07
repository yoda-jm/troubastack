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
    @Volatile private var smokeForcedDark = false   // S2.7: the "app theme" the ⋮ menu reads (menu-follows-app-theme toggles it)
    @Volatile private var serveNoReady = false   // S2.1: when set, the loopback serves a page that never handshakes
    @Volatile private var accepting = true        // S2.1 retry: when false, the loopback closes connections at once (server "off")
    @Volatile private var serverSocket: ServerSocket? = null
    @Volatile private var serveSetCookie = false  // S2.3: when set, the page response sets win08sess as a PERSISTENT cookie (Expires +30d)
    @Volatile private var reqCount = 0            // S2.3: count served page requests, so a post-clear reload can be confirmed to have arrived
    private val cookieByPort = java.util.concurrent.ConcurrentHashMap<Int, String>()  // S2.3b: the last Cookie header per server port
    // S2.10 — all dormant unless a probe run sets them, so the existing flows' acceptLoop behaviour is unchanged.
    @Volatile private var probeS210Expiry = false            // gate /api/me validation + /api/bands JSON (expiry probe)
    @Volatile private var validSession = ""                  // the cookie value /api/me treats as valid (else 401)
    @Volatile private var reauthSetCookie: String? = null    // when set, a page response Set-Cookies this (an in-Studio re-auth)
    @Volatile private var pageCookieHeader: String? = null   // S2.10 Probe A: the Cookie of the last WebView PAGE request only (never an /api call)
    private val s210PageByPort = java.util.concurrent.ConcurrentHashMap<Int, String>()  // port -> "top"/"child" for the iframe probe

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
                resolveDark = { smokeForcedDark },   // S2.7: the check toggles the "app theme" to sample the ⋮ menu in both
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
                        Thread { runChecks(controller, edgeDataDir, origin, outDir) { frameHolder.get() } }.apply { isDaemon = true; start() }
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
            // Budget: this vector fires TWO clicks, and each drop arrives through the same slow SWT-thread chain
            // (click → page capture → TroubaStageShellReceive → routeOpenUrl) as the other link vectors, which wait
            // 6 s for their one click. The old 4 s was the shortest budget on the heaviest vector, and under peak
            // combined load it expired ~0.7 s before the drops landed (measured: droppedUrls=[] read, both drops
            // logged 718 ms later, on a run where createFunction alone took 28.7 s) — a false red. Give it clear
            // headroom. The wait returns as soon as both drops land (sub-second on a healthy run), so the larger
            // ceiling costs nothing except on the nolinkfilter sabotage, which never drops and so still reds.
            val bothDropped = waitFor(10_000) { c.droppedUrls.contains("search-ms") && c.droppedUrls.contains("file:") }
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

    override fun runContextMenu(outDir: File): Int {
        outDir.mkdirs(); enableWin08FileLog()
        win08log("contextmenu: run start; log=$win08LogFile")
        Thread { try { Thread.sleep(120_000) } catch (_: InterruptedException) { return@Thread }
            win08log("contextmenu: BACKSTOP firing (>120s)"); Runtime.getRuntime().halt(1) }.apply { isDaemon = true; start() }
        val server = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
        serverSocket = server
        val origin = "http://127.0.0.1:${server.localPort}"
        Thread { acceptLoop(server) }.apply { isDaemon = true; start() }
        val edgeDataDir = File(desktopDataRoot(), "webview2")
        val ctrlRef = java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>()
        val frameHolder = java.util.concurrent.atomic.AtomicReference<java.awt.Window?>()
        val args = EmbeddedStudioArgs(origin, null, "TroubaStudio", null, edgeDataDir, {}, {}, { false }, { frameHolder.get() }, null)
        application {
            Window(onCloseRequest = ::exitApplication, title = "TroubaStage", state = rememberWindowState(width = 1200.dp, height = 820.dp)) {
                EmbeddedStudioContent(args, onController = { ctrlRef.set(it) })
                LaunchedEffect(Unit) { frameHolder.set(window); Thread { contextMenuFlow(outDir, ctrlRef, frameHolder) }.apply { isDaemon = true; start() } }
            }
        }
        return 0
    }

    private fun contextMenuFlow(
        outDir: File,
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
            val c = if (waitFor(40_000) { ctrlRef.get()?.ctxInjected == true }) ctrlRef.get() else null
            if (c == null) { mark("contextmenu-listener", false); finish(results) { null }; return }

            mark("contextmenu-listener", true)
            // DOM logic: a synthetic bubbling contextmenu on each target reports how the host listener treated it.
            c.smokeInjectCtxElements(); Thread.sleep(400)
            fun ctxFor(id: String): String? {
                val seq = c.ctxSeq
                c.smokeDispatchCtx(id)
                return if (waitFor(5_000) { c.ctxSeq > seq }) c.lastCtx else null
            }
            fun kept(s: String?) = s != null && s.contains("\"editable\":true") && s.contains("\"suppressed\":false")
            fun suppressed(s: String?) = s != null && s.contains("\"editable\":false") && s.contains("\"suppressed\":true")

            val chart = ctxFor("chart"); win08log("contextmenu: chart -> $chart")
            mark("ctx-chart-suppressed", suppressed(chart))
            // Editable = text entry: a textarea AND an <input type=text> both keep WebView2's edit menu (Fable revise).
            val field = ctxFor("field"); win08log("contextmenu: field -> $field")
            val textinput = ctxFor("textinput"); win08log("contextmenu: textinput -> $textinput")
            mark("ctx-editable-kept", kept(field) && kept(textinput))
            // Non-text inputs (a checkbox and a range) are NOT editable → suppressed, so they never get the page menu.
            val checkbox = ctxFor("checkbox"); win08log("contextmenu: checkbox -> $checkbox")
            val range = ctxFor("range"); win08log("contextmenu: range -> $range")
            mark("ctx-noninput-suppressed", suppressed(checkbox) && suppressed(range))
            val studio = ctxFor("studioel"); win08log("contextmenu: studioel -> $studio")
            mark("ctx-studio-untouched", studio != null && studio.contains("\"studio\":true"))

            // Screenshots: a REAL right-click. No menu on the chart or the checkbox (a non-text input); WebView2's edit
            // menu in a text field (with the More tools submenu). Left on disk for Fable. In the ctxanyinput run the
            // checkbox shot shows whether Chromium gives a non-text input the full page menu.
            val f = frameHolder.get()
            if (f != null) {
                val b = f.bounds; val cx = b.x + b.width / 2; val cy = b.y + b.height / 2
                val r = java.awt.Robot()
                fun capture(name: String) {
                    val img = r.createScreenCapture(java.awt.Rectangle(b.x, b.y, b.width, b.height))
                    val png = File(outDir, name); javax.imageio.ImageIO.write(img, "png", png); win08log("contextmenu: shot $name")
                }
                fun rightClick() { r.mouseMove(cx, cy); r.mousePress(java.awt.event.InputEvent.BUTTON3_DOWN_MASK); r.mouseRelease(java.awt.event.InputEvent.BUTTON3_DOWN_MASK); Thread.sleep(900) }
                fun esc() { r.keyPress(java.awt.event.KeyEvent.VK_ESCAPE); r.keyRelease(java.awt.event.KeyEvent.VK_ESCAPE); Thread.sleep(300) }
                c.smokeFullChart(); Thread.sleep(600); rightClick(); capture("ctx-chart.png"); esc()
                c.smokeFullField(); Thread.sleep(600); rightClick(); capture("ctx-field.png")
                r.mouseMove(b.x + 600, b.y + 335); Thread.sleep(1300); capture("ctx-moretools.png"); esc(); esc()
                c.smokeFullCheckbox(); Thread.sleep(600); rightClick(); capture("ctx-checkbox.png"); esc()
            }
            mark("ctx-flow-complete", true)   // reached the end without throwing
        } catch (e: Throwable) {
            win08log("contextmenu: flow threw: ${e.message}")
            mark("ctx-flow-complete", false)   // Fable revise: a thrown flow must FAIL, not silently pass
        }
        // Fable revise: require the fixed set — a check the flow never reached becomes FAIL, not an absence finish() ignores.
        listOf("contextmenu-listener", "ctx-chart-suppressed", "ctx-noninput-suppressed", "ctx-editable-kept", "ctx-studio-untouched", "ctx-flow-complete")
            .forEach { results.putIfAbsent(it, false) }
        finish(results) { null }
    }

    private fun writeReport(outDir: File, name: String, report: StringBuilder) {
        runCatching { File(outDir, name).writeText(report.toString()) }
        win08log("s210: report written to ${File(outDir, name).absolutePath}")
    }

    // ── S2.10 Probe A — session expiry seam (report only) ───────────────────────────────────────────────────────────
    override fun runS210Expiry(outDir: File): Int {
        outDir.mkdirs(); enableWin08FileLog()
        win08log("s210expiry: run start; WIN08_PROBE_S210_EXPIRY=[${System.getenv("WIN08_PROBE_S210_EXPIRY")}] log=$win08LogFile")
        Thread { try { Thread.sleep(90_000) } catch (_: InterruptedException) { return@Thread }
            win08log("s210expiry: BACKSTOP firing (>90s)"); Runtime.getRuntime().halt(1) }.apply { isDaemon = true; start() }
        try {
            probeS210Expiry = true
            val server = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
            serverSocket = server
            val origin = "http://127.0.0.1:${server.localPort}"
            Thread { acceptLoop(server) }.apply { isDaemon = true; start() }
            val edgeDataDir = File(desktopDataRoot(), "webview2")
            // The app's STORED session (what HttpTransport replays) and the WebView's seeded session start IDENTICAL.
            val storage = Storage()
            storage.putSecret("coreUrl", origin)
            storage.putSecret(SESSION_COOKIE_KEY, "win08sess=appold")
            storage.putSecret(SESSION_ORIGIN_KEY, origin)
            validSession = "win08sess=appold"   // at the start, appold IS the server's valid session
            val ctrlRef = java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>()
            val args = EmbeddedStudioArgs(origin, null, "TroubaStudio", "win08sess=appold", edgeDataDir, {}, {}, { false }, { null }, null)
            application {
                Window(onCloseRequest = ::exitApplication, title = "TroubaStage", state = rememberWindowState(width = 1200.dp, height = 820.dp)) {
                    EmbeddedStudioContent(args, onController = { ctrlRef.set(it) })
                    LaunchedEffect(Unit) { Thread { s210ExpiryFlow(outDir, storage, ctrlRef) }.apply { isDaemon = true; start() } }
                }
            }
        } catch (e: Throwable) { win08log("s210expiry: run FAILED: $e\n${e.stackTraceToString()}"); Runtime.getRuntime().halt(1) }
        return 0
    }

    private fun s210ExpiryFlow(
        outDir: File, storage: Storage,
        ctrlRef: java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>,
    ) {
        fun waitFor(ms: Long, cond: () -> Boolean): Boolean {
            val end = System.currentTimeMillis() + ms
            while (System.currentTimeMillis() < end) { if (cond()) return true; Thread.sleep(100) }
            return cond()
        }
        val report = StringBuilder()
        fun rec(s: String) { report.appendLine(s); win08log("s210expiry: $s") }
        try {
            rec("WIN08 S2.10 Probe A — session expiry seam (report only)")
            val c = if (waitFor(40_000) { ctrlRef.get()?.bridgeReady == true }) ctrlRef.get() else null
            if (c == null) { rec("FAIL: no controller"); writeReport(outDir, "s210-expiry-report.txt", report); Runtime.getRuntime().halt(1); return }
            rec("runtime: SWT=${org.eclipse.swt.SWT.getVersion()}  userAgent=[${c.s210UserAgent()}]")

            // 1. Baseline — the WebView carries the app's session; document.cookie cannot see it (HttpOnly). Read the
            //    page-cookie observation (pageCookieHeader), written ONLY by WebView page requests — never an /api call.
            waitFor(5_000) { c.docCookie != null }
            val webSendsAppold = (pageCookieHeader ?: "").contains("win08sess=appold")
            rec("baseline: WebView page request sends appold=$webSendsAppold  document.cookie hides the session=${!(c.docCookie ?: "").contains("win08sess")}  docCookie=[${c.docCookie}]  pageCookie=[$pageCookieHeader]")
            val storeBefore = storage.getSecret(SESSION_COOKIE_KEY)
            rec("store before: SESSION_COOKIE_KEY=[$storeBefore]  SESSION_ORIGIN_KEY=[${storage.getSecret(SESSION_ORIGIN_KEY)}]")

            // 2. The WebView's session is revoked server-side and RE-AUTHED in place to a fresh value. The reload that
            //    RECEIVES the Set-Cookie still SENDS the old cookie, so the re-auth is proven only by a SECOND same-origin
            //    page request whose Cookie header carries the fresh value. The app's store is never told.
            validSession = "win08sess=webv2"       // appold is now revoked; only webv2 is valid
            reauthSetCookie = "win08sess=webv2"     // reload #1 re-auths the WebView (fresh HttpOnly Set-Cookie)
            var n = reqCount; c.reload()
            waitFor(6_000) { reqCount > n }         // reload #1 arrives carrying appold, RECEIVES Set-Cookie webv2
            reauthSetCookie = null                  // stop re-issuing; the WebView now holds webv2
            pageCookieHeader = null; n = reqCount; c.reload()
            val reloaded2 = waitFor(6_000) { reqCount > n }   // reload #2 must CARRY webv2 iff the WebView really re-authed
            val webReauthed = reloaded2 && (pageCookieHeader ?: "").contains("win08sess=webv2")
            rec("re-auth: second reload arrived=$reloaded2  WebView now sends webv2=$webReauthed  pageCookie=[$pageCookieHeader]")
            val storeAfter = storage.getSecret(SESSION_COOKIE_KEY)
            val storeUnchanged = storeAfter == storeBefore
            rec("store after re-auth: SESSION_COOKIE_KEY=[$storeAfter]  unchanged=$storeUnchanged  (the WebView's Set-Cookie never reaches the app's Storage)")

            // 3. THE FINDING — the app's OWN request (HttpTransport.probePresence, exactly what Home uses) still replays
            //    the STALE stored cookie, so the server 401s while the local store is non-empty (signed-in-but-stale).
            val stale = kotlinx.coroutines.runBlocking { HttpTransport(storage, defaultServerUrl = null) { _, _ -> }.probePresence() }
            val storeNonEmpty = !storage.getSecret(SESSION_COOKIE_KEY).isNullOrEmpty()
            rec("FINDING app request with STALE stored session: presence=${stale.javaClass.simpleName}  storeNonEmpty=$storeNonEmpty  " +
                "stale-store(=401 while signed-in-locally)=${stale is Presence.Unauthorized && storeNonEmpty}")

            // 4. POSITIVE CONTROL — update the store to the fresh session; the SAME request now returns Online, proving
            //    the 401 above is cookie-specific (a stale store), not a blanket failure. Without this the probe is blind.
            storage.putSecret(SESSION_COOKIE_KEY, "win08sess=webv2")
            val fresh = kotlinx.coroutines.runBlocking { HttpTransport(storage, defaultServerUrl = null) { _, _ -> }.probePresence() }
            rec("CONTROL app request with FRESH stored session: presence=${fresh.javaClass.simpleName}  Online=${fresh is Presence.Online}")

            // Exit 0 REQUIRES the WebView to have really re-authed (else store-unchanged is vacuous), the store to be
            // unchanged by it, and the stand-in to discriminate stale vs fresh (so the probe isn't blind).
            val valid = webReauthed && storeUnchanged && stale is Presence.Unauthorized && fresh is Presence.Online
            rec("control valid (webReauthed=$webReauthed AND storeUnchanged=$storeUnchanged AND stale→Unauthorized AND fresh→Online) = $valid")
            writeReport(outDir, "s210-expiry-report.txt", report)
            Runtime.getRuntime().halt(if (valid) 0 else 1)
        } catch (e: Throwable) {
            rec("threw: ${e.message}"); writeReport(outDir, "s210-expiry-report.txt", report); Runtime.getRuntime().halt(1)
        }
    }

    // ── S2.10 Probe B — cross-origin iframe injection (report only) ─────────────────────────────────────────────────
    override fun runS210Iframe(outDir: File): Int {
        outDir.mkdirs(); enableWin08FileLog()
        val relax = System.getenv("WIN08_PROBE_S210_IFRAME_RELAX") == "1"
        win08log("s210iframe: run start (pass ${if (relax) 2 else 1}); WIN08_PROBE_S210_IFRAME=[${System.getenv("WIN08_PROBE_S210_IFRAME")}] " +
            "WIN08_PROBE_S210_IFRAME_RELAX=[${System.getenv("WIN08_PROBE_S210_IFRAME_RELAX")}] log=$win08LogFile")
        Thread { try { Thread.sleep(120_000) } catch (_: InterruptedException) { return@Thread }
            win08log("s210iframe: BACKSTOP firing (>120s)"); Runtime.getRuntime().halt(1) }.apply { isDaemon = true; start() }
        try {
            val serverA = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
            val serverB = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
            serverSocket = serverA
            val originA = "http://127.0.0.1:${serverA.localPort}"
            val originB = "http://127.0.0.1:${serverB.localPort}"
            s210PageByPort[serverA.localPort] = "top"
            s210PageByPort[serverB.localPort] = "child"
            Thread { acceptLoop(serverA) }.apply { isDaemon = true; start() }
            Thread { acceptLoop(serverB) }.apply { isDaemon = true; start() }
            win08log("s210iframe: A=$originA (top, the locked origin) B=$originB (child, cross-origin)")
            val edgeDataDir = File(desktopDataRoot(), "webview2")
            val ctrlRef = java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>()
            // openInDefaultBrowser is a no-op here; routeOpenUrl ALSO suppresses the launch under the probe knob, so even
            // a production launcher could not start a browser on the runner (Start-Process -Wait would wait for its tree).
            val args = EmbeddedStudioArgs(originA, null, "TroubaStudio", "win08sess=smoke", edgeDataDir,
                {}, {}, { false }, { null }, null, openInDefaultBrowser = { false })
            application {
                Window(onCloseRequest = ::exitApplication, title = "TroubaStage", state = rememberWindowState(width = 1200.dp, height = 820.dp)) {
                    EmbeddedStudioContent(args, onController = { ctrlRef.set(it) })
                    LaunchedEffect(Unit) { Thread { s210IframeFlow(outDir, originB, relax, ctrlRef) }.apply { isDaemon = true; start() } }
                }
            }
        } catch (e: Throwable) { win08log("s210iframe: run FAILED: $e\n${e.stackTraceToString()}"); Runtime.getRuntime().halt(1) }
        return 0
    }

    private fun s210IframeFlow(
        outDir: File, originB: String, relax: Boolean,
        ctrlRef: java.util.concurrent.atomic.AtomicReference<SwtBrowserController?>,
    ) {
        fun waitFor(ms: Long, cond: () -> Boolean): Boolean {
            val end = System.currentTimeMillis() + ms
            while (System.currentTimeMillis() < end) { if (cond()) return true; Thread.sleep(100) }
            return cond()
        }
        fun lines(s: String?) = (s ?: "").split("\n").filter { it.isNotBlank() }
        val report = StringBuilder()
        fun rec(s: String) { report.appendLine(s); win08log("s210iframe: ${s.replace("\n", " | ")}") }
        try {
            rec("WIN08 S2.10 Probe B — cross-origin iframe injection (report only); PASS ${if (relax) 2 else 1} relax=$relax")
            val c = if (waitFor(40_000) { ctrlRef.get()?.bridgeReady == true }) ctrlRef.get() else null
            if (c == null) { rec("FAIL: no controller"); writeReport(outDir, "s210-iframe-report.txt", report); Runtime.getRuntime().halt(1); return }
            // The page-level injections (window.open override, link capture, ctx listener) land in the TOP frame from
            // the controller's completed() handler; wait for them so the TOP control genuinely exercises each surface.
            val injected = waitFor(15_000) { c.linksInjected && c.ctxInjected }
            rec("top injections present (links+ctx) = $injected")
            // Scope the conclusion: the bridge/frame behaviour is a property of this runner's WebView2 runtime + SWT.
            rec("runtime: SWT=${org.eclipse.swt.SWT.getVersion()}  userAgent=[${c.s210UserAgent()}]  (findings hold for this runner's WebView2 runtime + SWT version)")

            // TOP PHASE (the positive control): run the identical trigger sequence in the TOP frame, so a later absence
            // of a child marker is meaningful. It ends with a top-phase-end bridge call; append the iframe only once
            // Java has recorded it, so ctx/back (fixed payloads) are attributable to a frame by time window.
            c.smokeS210RunTop()
            val topEnded = waitFor(8_000) { c.s210BridgeMarkers.contains("top-phase-end") }
            Thread.sleep(600)
            val topBridge = c.s210BridgeMarkers; val topSink = c.s210SinkMarkers; val topNewWin = c.s210NewWindowMarkers
            val topBoundary = lines(topBridge).size
            rec("TOP phase ended=$topEnded")
            rec("TOP bridge markers:\n$topBridge")
            rec("TOP sink markers:\n$topSink")
            rec("TOP newwindow markers:\n$topNewWin")

            // CHILD PHASE: append the cross-origin iframe (origin B) from the TOP frame; the child self-runs on load. It
            // posts a typeof table (LOADED signal), runs its triggers, then posts a child-phase-end carrying its direct
            // call's outcome. Wait for that END marker (so a late async child-direct call isn't missed) + a settle.
            c.smokeS210AddIframe("$originB/child")
            val forwarded = waitFor(15_000) { c.s210ForwardedChildReport != null }     // child LOADED (first forward)
            val childEnded = waitFor(10_000) { c.s210ForwardedChildEnd != null }        // child FINISHED its triggers (phase-end forward)
            Thread.sleep(3_000)                                                         // settle for any late async child-direct Java marker
            rec("CHILD iframe appended; forwarded typeof-report arrived=$forwarded  child-phase-end arrived=$childEnded")
            rec("subframe origin lock: changingFired=${c.s210IframeChangingFired} vetoed=${c.s210IframeVetoed} " +
                "relaxed=${c.s210IframeVetoRelaxed} location=[${c.s210IframeLocation}]")
            rec("CHILD typeof table (reliable, via parent forward): ${c.s210ForwardedChildReport}")
            rec("CHILD direct-call OUTCOME (from child-phase-end; distinguishes threw vs called-but-dropped): ${c.s210ForwardedChildEnd}")
            val childBridge = lines(c.s210BridgeMarkers).drop(topBoundary)
            rec("CHILD bridge markers (after the top boundary):\n${childBridge.joinToString("\n")}")
            rec("CHILD sink markers:\n${c.s210SinkMarkers}")
            rec("CHILD newwindow markers:\n${c.s210NewWindowMarkers}")

            // FINDINGS: only a child's OWN direct call can bear child-direct (the forward can't), so that is "the child
            // reached Java"; and whether a child action drove the app's external-launch sink (the security question).
            // ── SURFACE × FRAME TABLE (computed from the markers, not hand-derived) ──────────────────────────────────
            // Attribution: receive/window.open/link by the route/URL tag in the payload; ctx/back by phase window (fixed
            // payloads carry no route); the native new-window door by phase window too (no URL on the SWT event). A child
            // cell is INCONCLUSIVE when the TOP cell for that surface shows no arrival — then the child's absence proves
            // nothing (the top control never exercised it).
            val allBridge = lines(c.s210BridgeMarkers)
            val topBridgeLines = allBridge.take(topBoundary)
            val childBridgeLines = allBridge.drop(topBoundary)
            val allNewWin = lines(c.s210NewWindowMarkers)
            val topNewWinN = lines(topNewWin).size
            val topNewWinLines = allNewWin.take(topNewWinN)
            val childNewWinLines = allNewWin.drop(topNewWinN)
            val allSink = lines(c.s210SinkMarkers)
            fun guardOf(line: String) = when { line.contains("guardPass=true") -> "pass"; line.contains("guardPass=false") -> "fail"; else -> "-" }
            fun sinkFor(key: String): String {
                val m = allSink.firstOrNull { it.contains(key) } ?: return "none"
                return when { m.contains("external") -> "external-reached"; m.contains("inplace") -> "inplace"; m.contains("dropped") -> "dropped"; else -> "?" }
            }
            data class Cell(val door: String, val guard: String, val sink: String)
            fun cellFor(surface: String, frame: String): Cell {
                val bridge = if (frame == "top") topBridgeLines else childBridgeLines
                val newWin = if (frame == "top") topNewWinLines else childNewWinLines
                return when (surface) {
                    "receive" -> bridge.firstOrNull { it.contains("\"surface\":\"receive\"") && it.contains("\"route\":\"$frame-direct\"") }
                        ?.let { Cell("bridge", guardOf(it), "none") } ?: Cell("none", "-", "none")
                    "window.open", "link" -> {
                        val key = (if (surface == "window.open") "open-" else "link-") + "$frame-direct"
                        val m = bridge.firstOrNull { it.contains(key) }
                        when {
                            m != null -> Cell("bridge", guardOf(m), sinkFor(key))        // routed through the app's bridge
                            // Native new-window door: URL-less on the SWT event, and the child fires TWO such triggers
                            // (window.open + link) so one recorded event can't be pinned to a surface — see the NOTE below.
                            newWin.isNotEmpty() -> Cell("native-newwindow(undetermined)", "-", "none")
                            else -> Cell("none", "-", "none")
                        }
                    }
                    "ctx" -> bridge.firstOrNull { it.contains("\"ctxmenu\"") }?.let { Cell("bridge", guardOf(it), "none") } ?: Cell("none", "-", "none")
                    "back" -> bridge.firstOrNull { it.contains("\"mousebackseen\"") }?.let { Cell("bridge", guardOf(it), "none") } ?: Cell("none", "-", "none")
                    else -> Cell("none", "-", "none")
                }
            }
            val surfaces = listOf("receive", "window.open", "link", "ctx", "back")
            val table = StringBuilder("surface | frame | door | guard | sink\n")
            for (surface in surfaces) {
                val top = cellFor(surface, "top")
                val child = cellFor(surface, "child")
                table.append("$surface | top | ${top.door} | ${top.guard} | ${top.sink}\n")
                val childDoor = if (top.door == "none") "INCONCLUSIVE(top-absent)" else child.door
                table.append("$surface | child | $childDoor | ${child.guard} | ${child.sink}\n")
            }
            rec("SURFACE x FRAME TABLE:\n$table")
            // Native new-window attribution: the child fires TWO popup triggers (window.open with no override + a
            // target=_blank link click), but SWT's new-window event carries no URL, and Chromium's no-gesture popup
            // blocker may swallow one before it raises an event. So the ${childNewWinLines.size} recorded child
            // native-new-window event(s) are shared between the window.open and link rows — which surface raised which
            // is NOT determined; both are blocked regardless (required=true). Count this round: ${childNewWinLines.size}.
            rec("NOTE native-new-window: child recorded ${childNewWinLines.size} event(s) across {window.open, link}; " +
                "attribution undetermined (no URL on the SWT event; popup blocker may swallow one); all blocked.")

            // Headline findings — only a child's OWN direct call can bear route child-direct (the parent-forward can't):
            val receiveChild = cellFor("receive", "child")
            val childBridgeReached = childBridgeLines.any { it.contains("\"route\":\"child-direct\"") }
            val childSinkReached = allSink.any { it.contains("-child-direct") }
            rec("FINDING child-direct bridge call reached Java = $childBridgeReached  (guard on that call = ${receiveChild.guard})")
            rec("FINDING a child action reached the app's external-launch sink = $childSinkReached")

            // ── CONTROLS ─────────────────────────────────────────────────────────────────────────────────────────────
            val noLateTop = childBridgeLines.none { it.contains("\"route\":\"top-direct\"") }
            rec("CONTROL no top-tagged marker arrived after the phase boundary = $noLateTop")
            val topReceivePresent = cellFor("receive", "top").door != "none"
            rec("CONTROL top control exercised the bridge (receive-top arrival) = $topReceivePresent")
            val ok: Boolean
            if (relax) {
                // pass 2: the iframe actually LOADED (the parent forward arrived), so a "surface absent" reading is a
                // true negative, not a failed load; and the relax must have been honoured (recorded).
                ok = forwarded && c.s210IframeVetoRelaxed && topReceivePresent && noLateTop
                rec("CONTROL pass2: iframe loaded (forward arrived)=$forwarded relaxRecorded=${c.s210IframeVetoRelaxed} → ok=$ok")
            } else {
                // pass 1: the origin lock vetoed the dedicated TOP control nav — checked on s210TopVetoed, NOT the shared
                // navVetoed (a subframe veto also sets navVetoed, so it could not prove the top nav was vetoed).
                c.probeOffOriginNav("https://example.org/s210-lock-control")
                val topVetoed = waitFor(8_000) { c.s210TopVetoed }
                ok = topVetoed && topReceivePresent && noLateTop
                rec("CONTROL pass1: origin lock vetoed the TOP control nav=$topVetoed loc=[${c.s210TopVetoLocation}] " +
                    "(subframeChangingFired=${c.s210IframeChangingFired} subframeVetoed=${c.s210IframeVetoed}) → ok=$ok")
            }
            writeReport(outDir, "s210-iframe-report.txt", report)
            Runtime.getRuntime().halt(if (ok) 0 else 1)
        } catch (e: Throwable) {
            rec("threw: ${e.message}"); writeReport(outDir, "s210-iframe-report.txt", report); Runtime.getRuntime().halt(1)
        }
    }

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
                        val path = requestLine.substringAfter(' ', "").substringBefore(' ')
                        var reqCookie = ""
                        var l = reader.readLine()
                        while (l != null && l.isNotEmpty()) {
                            if (l.startsWith("Cookie:", ignoreCase = true)) {
                                cookieHeader = l.substringAfter(":").trim(); cookieByPort[s.localPort] = cookieHeader!!; reqCookie = cookieHeader!!
                                // S2.10 Probe A: a SEPARATE page-cookie observation written ONLY by WebView page requests, so
                                // HttpTransport's own /api calls (probePresence) can't clobber it. The shared cookieHeader/
                                // cookieByPort keep their meaning (the default-path checks read them) — unchanged on purpose.
                                if (!path.startsWith("/api")) pageCookieHeader = reqCookie
                            }
                            l = reader.readLine()
                        }
                        served = true
                        val out = s.getOutputStream()
                        if (path.startsWith("/api/me")) {
                            // S2.1: answer probePresence()'s reachability GET with 401 → Presence.Unauthorized (reachable),
                            // not a 200 HTML body it would fail to parse (→ a false Unreachable).
                            // S2.10 expiry probe: validate the Cookie — 200 only if it carries the CURRENT valid session
                            // (validSession); else 401. So the app's stored OLD cookie reads 401 (the stale-store finding),
                            // and the SAME request with the fresh cookie reads 200 (Online) — the positive control. Gated,
                            // so every other flow keeps the fixed-401 reachability answer above.
                            val valid = probeS210Expiry && validSession.isNotEmpty() && reqCookie.contains(validSession)
                            if (valid) {
                                val b = "{\"user\":{\"id\":\"ci\",\"displayName\":\"CI\"}}".toByteArray(StandardCharsets.US_ASCII)
                                out.write(("HTTP/1.0 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${b.size}\r\nConnection: close\r\n\r\n").toByteArray(StandardCharsets.US_ASCII)); out.write(b); out.flush()
                            } else {
                                out.write("HTTP/1.0 401 Unauthorized\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".toByteArray(StandardCharsets.US_ASCII)); out.flush()
                            }
                        } else if (path.startsWith("/api/bands") && probeS210Expiry) {
                            // S2.10 expiry probe: probePresence() follows a 200 /api/me with GET /api/bands for the band
                            // label; answer with an empty list so it resolves to Presence.Online (bandLabel([]) = "").
                            val b = "{\"bands\":[]}".toByteArray(StandardCharsets.US_ASCII)
                            out.write(("HTTP/1.0 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${b.size}\r\nConnection: close\r\n\r\n").toByteArray(StandardCharsets.US_ASCII)); out.write(b); out.flush()
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
                            // S2.10 iframe probe serves the top page (carrying an <iframe> to origin B) or the child page,
                            // keyed by this socket's port; otherwise the S2.1 never-ready page or the normal PAGE.
                            val pageStr = when (s210PageByPort[s.localPort]) {
                                "top" -> PAGE_S210_TOP
                                "child" -> PAGE_S210_CHILD
                                else -> if (serveNoReady) PAGE_NOREADY else PAGE
                            }
                            val body = pageStr.toByteArray(StandardCharsets.UTF_8)
                            // S2.3: the server sets win08sess as a PERSISTENT cookie — same name, Path=/, no Domain (host-only),
                            // Expires +30d — so the clear test proves Max-Age=0 on (name,domain,path) deletes a persistent cookie,
                            // which clearSessions() (option A) alone would NOT. HttpOnly matches the seeded session cookie.
                            // S2.10 expiry: reauthSetCookie issues a FRESH HttpOnly win08sess (an in-Studio re-auth) when set.
                            val setCookie = when {
                                serveSetCookie -> "Set-Cookie: win08sess=persist; Path=/; Expires=Tue, 01 Dec 2026 00:00:00 GMT; HttpOnly\r\n"
                                reauthSetCookie != null -> "Set-Cookie: $reauthSetCookie; Path=/; HttpOnly\r\n"
                                else -> ""
                            }
                            val head = "HTTP/1.0 200 OK\r\nContent-Type: text/html; charset=utf-8\r\n" + setCookie +
                                "Content-Length: ${body.size}\r\nConnection: close\r\n\r\n"
                            out.write(head.toByteArray(StandardCharsets.US_ASCII)); out.write(body); out.flush()
                        }
                    }
                }
            }.apply { isDaemon = true; start() }
        }
    }

    private fun runChecks(controller: SwtBrowserController?, edgeDataDir: File, origin: String, outDir: File, frame: () -> java.awt.Window?) {
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
            // Wait on the CONDITION, not a fixed sleep (a slow runner under load raced the 1500 ms): let the owned menu
            // window appear (under menufocus it never does — the DropdownMenu path takes activation instead).
            waitFor(5_000) { c.lastMenuWindow != null }
            // menu-no-activation-steal checks that something does NOT happen, so POLL the foreground over ~1.5 s — it must
            // be the frame EVERY time (a single read after a settle could pass on a steal that arrives a few hundred ms late).
            var fgIsFrame = frameWin != null
            run { val end = System.currentTimeMillis() + 1_500
                while (System.currentTimeMillis() < end) { if (frameWin == null || !Win08Native.foregroundIsFrame(frameWin)) fgIsFrame = false; Thread.sleep(150) } }
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
                val gone = waitFor(6_000) { c.lastMenuWindow == null }   // bounded wait on the dismiss, not a fixed sleep
                win08log("menu: lastMenuWindow after Studio focus = ${c.lastMenuWindow} gone=$gone")
                mark("menu-dismissed-on-studio-focus", gone)
            }

            // S2.7 — the THREE-row ⋮ menu (stub the dialogs so "Server URL…" + "Show band QR…" are offered too), read and
            // pressed through its accessibility tree (both smokes' one path). menu-items-accessible = the three names in
            // order, each press dispatches, and Reload really reloads (readied, since reloadCount++ runs before exec).
            // menu-rows-fit = every row's screen rect inside the window (menushort clips the last). menu-follows-app-theme
            // = the menu matches the APP theme not the OS (sampled in forced-Dark; menusystheme falls back to the OS →
            // light on the runner → reds). Two captures (Light + Dark) for review. Skipped under menufocus (no owned menu).
            if (System.getenv("WIN08_SABOTAGE") != "menufocus") {
                val serverOrig = c.requestServerDialog; val qrOrig = c.requestBandQr
                var serverFired = false; var qrFired = false
                c.requestServerDialog = { serverFired = true }; c.requestBandQr = { qrFired = true }
                try {
                    val want = listOf("Reload", "Server URL…", "Show band QR…")
                    val darkSurface = java.awt.Color(0x19, 0x17, 0x22); val lightSurface = java.awt.Color(0xFF, 0xFD, 0xFA)
                    fun frameShot(name: String): java.awt.image.BufferedImage? { val b = frameWin?.bounds ?: return null
                        val img = java.awt.Robot().createScreenCapture(java.awt.Rectangle(b.x, b.y, b.width, b.height)); javax.imageio.ImageIO.write(img, "png", File(outDir, name)); return img }
                    // A menu-relative pixel from the on-screen FRAME capture (the menu's nominal 240 px runs past the screen
                    // edge, so a menu-region capture reads off-screen black on the right).
                    fun px(img: java.awt.image.BufferedImage, mb: java.awt.Rectangle, dx: Int, dy: Int): java.awt.Color {
                        val fb = frameWin!!.bounds; val sx = (mb.x - fb.x) + dx; val sy = (mb.y - fb.y) + dy
                        return if (sx in 0 until img.width && sy in 0 until img.height) java.awt.Color(img.getRGB(sx, sy)) else java.awt.Color(0, 0, 0) }
                    fun near(a: java.awt.Color, b: java.awt.Color, tol: Int) = kotlin.math.abs(a.red - b.red) <= tol && kotlin.math.abs(a.green - b.green) <= tol && kotlin.math.abs(a.blue - b.blue) <= tol

                    // Light: enumerate names + each row's rect + the window; sample the row-1 surface (x=140, right of
                    // "Reload", inside the rendered menu); capture.
                    smokeForcedDark = false; c.openStudioMenu(); Thread.sleep(1200)
                    var names = emptyList<String>(); var rowRects = emptyList<java.awt.Rectangle>(); var winRect: java.awt.Rectangle? = null
                    (c.lastMenuWindow as? javax.accessibility.Accessible)?.let { acc -> javax.swing.SwingUtilities.invokeAndWait {
                        names = accessibleActionNames(acc); rowRects = accessibleActionRects(acc); winRect = (acc as java.awt.Window).bounds } }
                    var lightPx = java.awt.Color(0, 0, 0)
                    frameShot("menu-light.png")?.let { img -> c.lastMenuWindow?.bounds?.let { lightPx = px(img, it, 140, 24) } }
                    runCatching { javax.swing.SwingUtilities.invokeAndWait { c.lastMenuWindow?.dispose() } }

                    // Dark, on a dark stand-in page: sample the row-1 surface AND the window edges (corners + bottom-row
                    // middle) — these must be the theme colour, not the opaque JWindow's white; capture.
                    c.smokePageDark(); Thread.sleep(300); smokeForcedDark = true; c.openStudioMenu(); Thread.sleep(1200)
                    var darkPx = java.awt.Color(255, 255, 255); var edges = emptyList<java.awt.Color>()
                    frameShot("menu-dark.png")?.let { img -> c.lastMenuWindow?.bounds?.let { mb ->
                        darkPx = px(img, mb, 140, 24)
                        edges = listOf(px(img, mb, 2, 2), px(img, mb, 2, mb.height - 2), px(img, mb, 90, mb.height - 2), px(img, mb, 90, 2)) } }
                    runCatching { javax.swing.SwingUtilities.invokeAndWait { c.lastMenuWindow?.dispose() } }
                    smokeForcedDark = false; c.smokePageLight(); Thread.sleep(300)

                    // Presses (each disposes the menu): Server URL…, Show band QR…, then Reload (which reloads; wait re-ready).
                    var pS = false; var pQ = false; var pR = false
                    c.openStudioMenu(); Thread.sleep(1000); (c.lastMenuWindow as? javax.accessibility.Accessible)?.let { a -> javax.swing.SwingUtilities.invokeAndWait { pS = pressAccessibleNamed(a, "Server URL…") } }; Thread.sleep(300)
                    c.openStudioMenu(); Thread.sleep(1000); (c.lastMenuWindow as? javax.accessibility.Accessible)?.let { a -> javax.swing.SwingUtilities.invokeAndWait { pQ = pressAccessibleNamed(a, "Show band QR…") } }; Thread.sleep(300)
                    val beforeR = c.reloadCount; val readyBefore = c.readyCount
                    c.openStudioMenu(); Thread.sleep(1000); (c.lastMenuWindow as? javax.accessibility.Accessible)?.let { a -> javax.swing.SwingUtilities.invokeAndWait { pR = pressAccessibleNamed(a, "Reload") } }
                    val reloaded = pR && waitFor(5_000) { c.reloadCount > beforeR }
                    val readied = reloaded && waitFor(12_000) { c.readyCount > readyBefore }
                    runCatching { javax.swing.SwingUtilities.invokeAndWait { c.lastMenuWindow?.dispose() } }

                    // menu-rows-fit: read the RENDERED rows — exactly 3 rects, each ROW_HEIGHT±1 tall and inside the window
                    // (a window-vs-literal compare can't fail once Compose clips the bounds). Vacuous when a11y is cleared
                    // (menunoa11y), which menu-items-accessible already reds.
                    val rowsFit = names.isEmpty() || (rowRects.size == 3 && winRect != null && rowRects.all { it.height in 47..49 && winRect!!.contains(it) })
                    // menu-follows-app-theme: forced-Dark is BrandDark.surface AND Light is BrandLight.surface (tol ≤ 6).
                    val themeOk = near(darkPx, darkSurface, 6) && near(lightPx, lightSurface, 6)
                    // menu-edges-match-theme: the menu's corners + bottom-row middle match its OWN surface (darkPx) — not
                    // the opaque JWindow bleeding through. Comparing to the surface (not an absolute dark) means a menu
                    // that is wholly the wrong theme (menusystheme) reds menu-follows-app-theme, not this. menulightedge's
                    // old code (rounded clip + default bg + the +2 strip) shows the JWindow's white at the edges → reds here.
                    val edgesOk = edges.isNotEmpty() && edges.all { near(it, darkPx, 24) }
                    win08log("menu-items: names=$names serverFired=$serverFired qrFired=$qrFired pS=$pS pQ=$pQ pR=$pR reloaded=$reloaded readied=$readied")
                    win08log("menu-size: rows=${rowRects.size} heights=${rowRects.map { it.height }} window=$winRect rowsFit=$rowsFit")
                    win08log("menu-theme: darkPx=(${darkPx.red},${darkPx.green},${darkPx.blue}) lightPx=(${lightPx.red},${lightPx.green},${lightPx.blue}) themeOk=$themeOk")
                    win08log("menu-edges: ${edges.map { "(${it.red},${it.green},${it.blue})" }} edgesOk=$edgesOk")
                    mark("menu-items-accessible", names == want && pS && serverFired && pQ && qrFired && readied)
                    mark("menu-rows-fit", rowsFit)
                    mark("menu-follows-app-theme", themeOk)
                    mark("menu-edges-match-theme", edgesOk)
                } finally { c.requestServerDialog = serverOrig; c.requestBandQr = qrOrig; runCatching { c.smokePageLight() } }
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

        // S2.10 — the frame trigger sequence, shared by the top page (origin A) and the child page (origin B). For the
        // given [route] it (1) DIRECT-calls the host bridge carrying route + this frame's typeof table of every injected
        // surface, then exercises (2) window.open, (3) a target=_blank link, (4) a contextmenu and (5) a mouse-back — each
        // carrying the route in its URL/payload where it can, so a Java-side door marker is attributable to this frame.
        const val S210_RUN_JS = """
function s210run(route){
  var fns={receive:typeof window.TroubaStageShellReceive,shell:typeof window.__troubastageShell,win08links:typeof window.__win08links,win08ctx:typeof window.__win08ctx,win08back:typeof window.__win08back,openOverridden:((''+window.open).indexOf('native')<0)};
  // Capture the direct bridge call's OUTCOME so "no Java marker" can be told apart from a throw (binding missing in the
  // frame) vs a call that returned but was dropped. Read later via window.__s210directOutcome and reported by the child.
  var outcome;
  try{var r=window.TroubaStageShellReceive;
      if(typeof r==='function'){var rv=r(JSON.stringify({type:'s210call',surface:'receive',route:route,origin:location.origin,fns:fns}));outcome='called;returned='+String(rv);}
      else{outcome='receive-not-a-function(typeof '+(typeof r)+')';}}
  catch(e){outcome='threw '+((e&&e.name)||'Error')+': '+((e&&e.message)||String(e));}
  window.__s210directOutcome=outcome;
  try{window.open('http://s210.invalid/open-'+route,'_blank');}catch(e){}
  try{var a=document.createElement('a');a.href='http://s210.invalid/link-'+route;a.target='_blank';a.textContent='x';document.body.appendChild(a);a.click();}catch(e){}
  try{document.body.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));}catch(e){}
  try{window.dispatchEvent(new MouseEvent('mouseup',{button:3,bubbles:true}));}catch(e){}
}
"""

        // S2.10 top page (origin A): the bridge handshake (so bridgeReady latches and the flow proceeds) + a message
        // listener that forwards any child postMessage to the host bridge tagged route "child-forwarded" (the reliable
        // channel — a TOP-frame call, so it can NEVER bear child-direct). It does NOT embed the iframe; the flow appends
        // it only after the top phase ends, so ctx/back are attributable to a frame by time window.
        const val PAGE_S210_TOP = """<!doctype html><html><head><meta charset="utf-8"><script>""" + S210_RUN_JS + """
(function(){
  if(typeof window.TroubaStageShellReceive==='function'){
    window.__troubastageShell={deliver:function(j){try{if(JSON.parse(j).type==='hello')window.TroubaStageShellReceive(JSON.stringify({type:'ready'}));}catch(e){}}};
    window.TroubaStageShellReceive(JSON.stringify({type:'ready'}));
    window.addEventListener('message',function(e){try{var r=window.TroubaStageShellReceive;if(r)r(JSON.stringify({type:'s210fwd',route:'child-forwarded',child:String(e.data)}));}catch(x){}});
    window.__win08='shell-present';
  }
})();
</script></head><body><h1>win08 s210 top (origin A)</h1></body></html>"""

        // S2.10 child page (origin B, loaded into a cross-origin iframe): first postMessage its typeof table to the
        // parent (the reliable "the child loaded" signal — carries NO route, so the forward can't be mistaken for a
        // child-direct), then run the DIRECT trigger sequence tagged route "child-direct".
        const val PAGE_S210_CHILD = """<!doctype html><html><head><meta charset="utf-8"><script>""" + S210_RUN_JS + """
(function(){
  try{parent.postMessage(JSON.stringify({type:'s210childreport',origin:location.origin,fns:{receive:typeof window.TroubaStageShellReceive,shell:typeof window.__troubastageShell,win08links:typeof window.__win08links,win08ctx:typeof window.__win08ctx,win08back:typeof window.__win08back,openOverridden:((''+window.open).indexOf('native')<0)}}),'*');}catch(e){}
  s210run('child-direct');
  // AFTER the triggers: a child-phase-end report carrying the direct call's outcome, so the flow can WAIT for the
  // child to finish (not snapshot early) and tell a throw from a call-but-dropped. Routed via the parent forward
  // because the child can't reach Java directly — the whole question being measured.
  try{parent.postMessage(JSON.stringify({type:'s210childend',origin:location.origin,directOutcome:window.__s210directOutcome}),'*');}catch(e){}
})();
</script></head><body><h1>win08 s210 child (origin B)</h1></body></html>"""
    }
}
