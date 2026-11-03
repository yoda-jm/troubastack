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
        return 0 // unreached: runChecks/backstop halts
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
            if (c == null) { mark("controller-created", false); finish(results, null); return }

            // S1.6 — forced no-WebView2 (CI sets TROUBA_WIN08_FORCE_NO_WEBVIEW=1). The host must report engine
            // missing and show the exact message, never create a Browser, never fall back to IE.
            if (System.getenv("TROUBA_WIN08_FORCE_NO_WEBVIEW") == "1") {
                val reported = waitFor(20_000) { c.engineMissingReported }
                mark("no-webview-engine-missing", reported && c.browserType == null)
                mark("no-webview-message", NO_WEBVIEW_MESSAGE.contains("doesn't have Microsoft Edge WebView2"))
                finish(results, c)
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

            c.probeOffOriginNav("https://example.org/win08")
            mark("off-origin-nav-vetoed", waitFor(10_000) { c.navVetoed })

            val underRoot = edgeDataDir.absolutePath.startsWith(File(desktopDataRoot()).absolutePath)
            val noDotSwt = !File(System.getProperty("user.home").orEmpty(), ".swt").exists()
            mark("data-dir-under-root-no-dotswt", edgeDataDir.exists() && underRoot && noDotSwt)

            // B2 — DPI: log awareness on the process, the EDT and the SWT thread; at 100% assert the embedded
            // Shell's client rect equals the canvas's. (Above 100% is VLL's Surface check, not asserted here.)
            c.measureDpi()
            val edtDpi = runCatching {
                var v = -1; javax.swing.SwingUtilities.invokeAndWait { v = Win08Native.threadDpiAwareness() }; v
            }.getOrDefault(-1)
            win08log("dpi: process=${Win08Native.processDpiAwareness()} edt=$edtDpi swt=${c.swtThreadDpiAwareness} " +
                "canvasClient=${c.canvasClient} shellClient=${c.shellClient} rectsMatch=${c.rectsMatch}")
            mark("dpi-shell-rect-equals-canvas-at-100", c.rectsMatch)

            finish(results, c)
        } catch (e: Throwable) {
            win08log("smoke: checker threw: ${e.message}")
            finish(results, controller)
        }
    }

    private fun finish(results: Map<String, Boolean>, controller: SwtBrowserController?) {
        val ok = results.isNotEmpty() && results.values.all { it }
        line("win08-smoke: ${results.count { it.value }}/${results.size} checks passed; served=$served cookie=[${cookieHeader ?: ""}]")
        // Check 7 — tear down like a window close: dispose the controller so SWT closes the WebView2 environment
        // and its msedgewebview2.exe children exit (a survivor would hold the EdgeDataDir — B1). Then halt.
        win08log("smoke: disposing controller before exit")
        runCatching { controller?.dispose() }
        Thread.sleep(2500) // let the WebView2 child processes finish exiting after Close
        runCatching { serverSocket?.close() }
        win08log("smoke: exit code ${if (ok) 0 else 1}")
        System.out.flush(); System.err.flush()
        Runtime.getRuntime().halt(if (ok) 0 else 1)
    }

    private companion object {
        // The handshake runs SYNCHRONOUSLY at the top of <head> — earlier than a module script, so a pass covers
        // Studio's ordering (B3). With SWT 3.134.0 the BrowserFunction is injected before this runs.
        const val PAGE = """<!doctype html><html><head><meta charset="utf-8"><script>
(function(){
  if (typeof window.TroubaStageShellReceive === "function") {
    window.__troubastageShell = { deliver: function(j){ try { if (JSON.parse(j).type === "hello") window.TroubaStageShellReceive(JSON.stringify({type:"ready"})); } catch(e){} } };
    window.TroubaStageShellReceive(JSON.stringify({type:"ready"}));
    window.__win08 = "shell-present";
  } else {
    window.__win08 = "no-shell-at-head";
  }
})();
</script></head><body><h1>win08 loopback</h1></body></html>"""
    }
}
