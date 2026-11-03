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
import com.sun.net.httpserver.HttpServer
import com.troubastack.shared.seams.desktopDataRoot
import java.io.File
import java.net.InetSocketAddress
import java.nio.charset.StandardCharsets

/**
 * WIN08 S1.8 — the embedded-Studio CI smoke (see [Win08SmokeRunner]). Serves a loopback page that runs the bridge
 * handshake as a synchronous script at the top of `<head>` (earlier than Studio's module — binding B3's worst
 * case), opens it in the embedded host inside a real TroubaStage window, and checks: engine is Edge; the seeded
 * cookie arrived; the bridge round-trips (ready, then a hello answered); off-origin navigation is vetoed; the
 * EdgeDataDir is under the data root and `~/.swt` is absent. Each check prints `WIN08-CHECK …: PASS|FAIL`; the
 * window then closes and the process exits (the CI step asserts it exits and leaves no msedgewebview2.exe — check 7).
 */
class Win08SmokeRunnerImpl : Win08SmokeRunner {
    @Volatile private var cookieHeader: String? = null
    @Volatile private var served = false

    /** Markers and stages all go to the durable [win08log] file (a packaged Windows app has no captured stdout). */
    private fun line(s: String) = win08log(s)

    override fun run(outDir: File): Int {
        outDir.mkdirs()
        win08log("smoke: run start; outDir=$outDir log=$win08LogFile")
        val server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0)
        val port = server.address.port
        val origin = "http://127.0.0.1:$port"
        server.createContext("/") { ex ->
            served = true
            cookieHeader = ex.requestHeaders.getFirst("Cookie")
            val body = PAGE.toByteArray(StandardCharsets.UTF_8)
            ex.responseHeaders.add("Content-Type", "text/html; charset=utf-8")
            ex.sendResponseHeaders(200, body.size.toLong())
            ex.responseBody.use { it.write(body) }
        }
        server.start()
        win08log("smoke: loopback server on $origin")

        // Backstop: never hang the CI job — halt non-zero after 75 s if the checker has not already exited.
        Thread {
            try { Thread.sleep(75_000) } catch (_: InterruptedException) { return@Thread }
            win08log("smoke: BACKSTOP firing (>75s) — halting 1")
            Runtime.getRuntime().halt(1)
        }.apply { isDaemon = true; start() }
        win08log("smoke: backstop armed")

        val edgeDataDir = File(desktopDataRoot(), "webview2")
        val args = EmbeddedStudioArgs(
            serverUrl = origin,
            initialPath = null,
            title = "TroubaStudio",
            sessionCookie = "win08sess=smoke",
            edgeDataDir = edgeDataDir,
            onBack = {},
            toggleFullScreen = {},
            isFullScreen = { false },
            frameWindow = { null },
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
                    win08log("smoke: window composed; starting checker thread")
                    Thread { runChecks(controller, edgeDataDir, server) }.apply { isDaemon = true; start() }
                }
            }
        }
        return 0 // unreached: runChecks halts
    }

    private fun runChecks(controller: SwtBrowserController?, edgeDataDir: File, server: HttpServer) {
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
            if (c == null) { mark("controller-created", false); finish(results, server); return }

            waitFor(30_000) { c.browserType != null }
            mark("engine-is-edge", c.browserType == "edge")

            val bridge = waitFor(30_000) { c.bridgeReady }
            // The loopback page's deliver answers our auto-sent hello with a second ready.
            val hello = bridge && waitFor(10_000) { c.helloAnswered }
            mark("bridge-round-trips", bridge && hello)

            mark("seeded-cookie-arrives", (cookieHeader ?: "").contains("win08sess=smoke"))

            c.probeOffOriginNav("https://example.org/win08")
            mark("off-origin-nav-vetoed", waitFor(10_000) { c.navVetoed })

            val underRoot = edgeDataDir.absolutePath.startsWith(File(desktopDataRoot()).absolutePath)
            val noDotSwt = !File(System.getProperty("user.home").orEmpty(), ".swt").exists()
            mark("data-dir-under-root-no-dotswt", edgeDataDir.exists() && underRoot && noDotSwt)
        } catch (e: Throwable) {
            System.err.println("win08-smoke: checker threw: ${e.message}")
        }
        finish(results, server)
    }

    private fun finish(results: Map<String, Boolean>, server: HttpServer) {
        val ok = results.isNotEmpty() && results.values.all { it }
        line("win08-smoke: ${results.count { it.value }}/${results.size} checks passed; served=$served cookie=[${cookieHeader ?: ""}]")
        runCatching { server.stop(0) }
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
