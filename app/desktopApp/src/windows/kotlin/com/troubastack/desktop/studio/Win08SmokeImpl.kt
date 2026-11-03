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
    @Volatile private var serverSocket: ServerSocket? = null

    private fun line(s: String) = win08log(s)

    override fun run(outDir: File): Int {
        outDir.mkdirs()
        enableWin08FileLog()
        win08log("smoke: run start; outDir=$outDir log=$win08LogFile")
        // Arm the backstop FIRST, so any hang — even before the loopback server is up — is logged and halted.
        Thread {
            try { Thread.sleep(75_000) } catch (_: InterruptedException) { return@Thread }
            win08log("smoke: BACKSTOP firing (>75s) — halting 1")
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
            )

            win08log("smoke: entering application{}")
            application {
                // Check 7 — WM_CLOSE must end the process (positive proof below). It can't be forced red in the
                // PACKAGED app: a raw WM_CLOSE destroys the window at the Win32 level regardless of onCloseRequest,
                // and the jpackage launcher + Compose's exitProcess(0) always end the process (verified across
                // daemon/non-daemon/exitProcessOnExit/ignored-onCloseRequest runs). So the guarantee is by construction.
                Window(
                    onCloseRequest = ::exitApplication,
                    title = "TroubaStage",
                    state = rememberWindowState(width = 1200.dp, height = 820.dp),
                ) {
                    var controller by remember { mutableStateOf<SwtBrowserController?>(null) }
                    EmbeddedStudioContent(args, onController = { controller = it })
                    LaunchedEffect(Unit) {
                        frameHolder.set(window)
                        win08log("smoke: window composed; starting checker thread")
                        Thread { runChecks(controller, edgeDataDir) { frameHolder.get() } }.apply { isDaemon = true; start() }
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
        fun argsNow() = EmbeddedStudioArgs(origin, null, "TroubaStudio", "win08sess=smoke", edgeDataDir, {}, {}, { false }, { null })
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
            // An immediate Back on entering leaves no window: dispose arrives before the Shell is shown.
            cyc.value = 99; shown.value = true
            waitFor(10_000) { ctrlRef.get() != null }
            val c99 = ctrlRef.get()
            shown.value = false                      // Back at once, before the controller shows the Shell
            Thread.sleep(4000)
            val neverShown = c99 != null && !c99.shellEverShown
            win08log("reentry: early-back c99.shellEverShown=${c99?.shellEverShown}")
            mark("early-back-no-window-shown", neverShown)
        } catch (e: Throwable) {
            win08log("reentry: checker threw: ${e.message}")
        }
        finish(results) { null }
    }

    /** A one-page HTTP/1.0 loopback: read the request headers (record the Cookie), reply with [PAGE]. */
    private fun acceptLoop(server: ServerSocket) {
        while (!server.isClosed) {
            val sock = try { server.accept() } catch (_: Throwable) { break }
            Thread {
                runCatching {
                    sock.use { s ->
                        val reader = s.getInputStream().bufferedReader()
                        var l = reader.readLine()
                        while (l != null && l.isNotEmpty()) {
                            if (l.startsWith("Cookie:", ignoreCase = true)) cookieHeader = l.substringAfter(":").trim()
                            l = reader.readLine()
                        }
                        served = true
                        val body = PAGE.toByteArray(StandardCharsets.UTF_8)
                        val head = "HTTP/1.0 200 OK\r\nContent-Type: text/html; charset=utf-8\r\n" +
                            "Content-Length: ${body.size}\r\nConnection: close\r\n\r\n"
                        s.getOutputStream().apply {
                            write(head.toByteArray(StandardCharsets.US_ASCII))
                            write(body)
                            flush()
                        }
                    }
                }
            }.apply { isDaemon = true; start() }
        }
    }

    private fun runChecks(controller: SwtBrowserController?, edgeDataDir: File, frame: () -> java.awt.Window?) {
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

            c.probeOffOriginNav("https://example.org/win08")
            mark("off-origin-nav-vetoed", waitFor(10_000) { c.navVetoed })

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
    window.__win08 = "shell-present";
  } else {
    window.__win08 = "no-shell-at-head";
  }
})();
</script></head><body><h1>win08 loopback</h1></body></html>"""
    }
}
