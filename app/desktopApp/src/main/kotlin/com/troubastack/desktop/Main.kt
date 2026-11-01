// TroubaStage desktop entrypoint (WIN02). A skeleton window — the title and the build line, nothing a user
// could mistake for a feature. WIN03 replaces AppRoot with the real Stage. The `--smoke <png>` path is the
// CI launch smoke: it opens the real window, renders one frame of the UI to a PNG, and exits (proves the
// packaged app starts and its skiko render pipeline works on Windows).
package com.troubastack.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.ImageComposeScene
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.WindowPosition
import androidx.compose.ui.window.application
import androidx.compose.ui.window.rememberWindowState
import com.troubastack.shared.buildinfo.appBuildLine
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.seams.desktopDataRoot
import org.jetbrains.skia.EncodedImageFormat
import java.io.File
import kotlin.system.exitProcess

/**
 * The whole desktop UI for now: "TroubaStage" and the build line (commit + UTC build time), from :shared.
 * [onDraw] fires after this content has actually been PAINTED (B5): the smoke uses it to prove the real
 * window drew a frame before declaring success — a throw inside the real window's draw must fail the smoke.
 */
@Composable
private fun AppRoot(onDraw: () -> Unit = {}) {
    MaterialTheme {
        Surface(
            modifier = Modifier.fillMaxSize().drawWithContent {
                drawContent()
                onDraw()
            },
        ) {
            Column(
                modifier = Modifier.fillMaxSize().padding(24.dp),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text("TroubaStage", style = MaterialTheme.typography.headlineMedium)
                Spacer(Modifier.height(12.dp))
                Text(appBuildLine, style = MaterialTheme.typography.bodyMedium)
            }
        }
    }
}

/** Set by the REAL window's draw (not the off-screen scene) once it has painted a frame (B5). */
@Volatile
private var windowDrew = false

fun main(args: Array<String>) {
    // F2.3 nit — CI reads the data root the app actually resolves, instead of hard-coding the folder name.
    // Writes to the given file (a GUI app's stdout is not captured on CI); falls back to stdout locally.
    val pdrIdx = args.indexOf("--print-data-root")
    if (pdrIdx >= 0) {
        val path = desktopDataRoot()
        val outArg = args.getOrNull(pdrIdx + 1)?.takeUnless { it.startsWith("--") }
        if (outArg != null) File(outArg).writeText(path) else println(path)
        return
    }
    // F2.2 — measure the secret store's get/put cost (DPAPI on Windows) so WIN03 knows whether writes must
    // move off the UI thread. Writes the report to the given file (a GUI app's stdout is not captured on CI),
    // and uses Storage() (the real root — the CI step points TROUBASTAGE_HOME at a temp dir).
    val benchIdx = args.indexOf("--bench")
    if (benchIdx >= 0) {
        runBench(File(args.getOrNull(benchIdx + 1) ?: "bench.txt"))
        return
    }
    val smokeIdx = args.indexOf("--smoke")
    if (smokeIdx >= 0) {
        val out = args.getOrNull(smokeIdx + 1)
        if (out == null) {
            System.err.println("--smoke needs a PNG path")
            exitProcess(2)
        }
        runSmoke(File(out))
        return
    }
    application {
        Window(
            onCloseRequest = ::exitApplication,
            title = "TroubaStage",
            state = rememberWindowState(width = 640.dp, height = 400.dp),
        ) {
            AppRoot()
        }
    }
}

/**
 * Open the REAL window (so a packaging failure to open one still fails the job), then render exactly the same
 * UI one frame to [out] as a PNG via an off-screen skiko scene and exit. Rendering off-screen rather than
 * screen-grabbing the window is deliberate: a live screenshot races the window manager's compositor (a capture
 * a frame early shows the desktop through an unpainted window) and is flaky on a CI runner; the off-screen
 * scene renders through the same skiko pipeline the packaged app uses, so a broken render still fails here.
 * Any failure exits non-zero → the CI job goes red; a hang is caught by the job's timeout.
 */
private fun runSmoke(out: File) {
    // Hard backstop: if the window never composes/renders at all, halt non-zero rather than hang the job.
    // (The CI step also has a timeout; this gives a clean red with a message instead of a silent wait.)
    Thread {
        try { Thread.sleep(60_000) } catch (_: InterruptedException) { return@Thread }
        System.err.println("smoke: backstop timeout — no frame in 60s")
        Runtime.getRuntime().halt(1)
    }.apply { isDaemon = true; start() }

    application {
        Window(
            onCloseRequest = ::exitApplication,
            title = "TroubaStage",
            state = rememberWindowState(width = 640.dp, height = 400.dp, position = WindowPosition(0.dp, 0.dp)),
        ) {
            // onDraw fires from the REAL window's paint (B5) — not the off-screen scene.
            AppRoot(onDraw = { windowDrew = true })
            LaunchedEffect(Unit) {
                // Log which backend skiko gave the real window (DIRECT3D / OPENGL / SOFTWARE).
                runCatching { println("smoke: window render API = ${window.renderApi}") }
                // Render one frame off-screen for the PNG (deterministic, no compositor race).
                val rendered = runCatching { renderFrame(out) }
                rendered.exceptionOrNull()?.printStackTrace()
                // B5: do NOT declare success until the REAL window has actually painted a frame — so a throw
                // inside the window's own draw fails the smoke even though the off-screen raster succeeded.
                var frames = 0
                while (!windowDrew && frames < 240) {
                    withFrameNanos { }
                    frames++
                }
                val ok = rendered.isSuccess && out.isFile && out.length() > 0 && windowDrew
                // Terminate with halt(), NOT System.exit/exitProcess: System.exit from the Compose/AWT thread
                // deadlocks on the AWT shutdown (the PNG lands but the JVM hangs). halt() skips shutdown hooks
                // and exits immediately — the PNG is already flushed to disk.
                if (ok) println("smoke: OK — window drew after $frames frame ticks, PNG ${out.length()} bytes")
                else System.err.println("smoke: FAILED — rendered=${rendered.isSuccess} windowDrew=$windowDrew frames=$frames")
                System.out.flush()
                System.err.flush()
                Runtime.getRuntime().halt(if (ok) 0 else 1)
            }
        }
    }
}

/** Render one frame of [AppRoot] off-screen (the packaged app's skiko pipeline) and write it as a PNG. */
private fun renderFrame(out: File) {
    val scene = ImageComposeScene(width = 640, height = 400, density = Density(1f)) { AppRoot() }
    try {
        val png = scene.render().encodeToData(EncodedImageFormat.PNG)
            ?: error("skiko produced no PNG data")
        out.absoluteFile.parentFile?.mkdirs()
        out.writeBytes(png.bytes)
    } finally {
        scene.close()
    }
}

/**
 * F2.2 — the secret store's get/put cost on the calling thread (DPAPI on Windows). A realistic store (a handful
 * of keys), then 200 puts overwriting ONE key — the Stage page-turn scenario ("position written on every page
 * turn") — and 200 gets of it. Reports p50/p95 in ms, and flags a put p95 over 16 ms (one display frame at
 * 60 Hz), which would mean WIN03 must write off the UI thread or the store needs a shared per-root map.
 */
private fun runBench(out: File) {
    val s = Storage()
    repeat(10) { s.putSecret("key-$it", "val-$it") } // a realistic resident store size
    repeat(20) { s.putSecret("position", "$it"); s.getSecret("position") } // warm up (JIT, DPAPI handles)

    val n = 200
    val putNs = LongArray(n)
    val getNs = LongArray(n)
    for (i in 0 until n) {
        val t0 = System.nanoTime(); s.putSecret("position", "$i"); putNs[i] = System.nanoTime() - t0
    }
    for (i in 0 until n) {
        val t0 = System.nanoTime(); s.getSecret("position"); getNs[i] = System.nanoTime() - t0
    }

    fun ms(a: LongArray, p: Double): Double {
        val sorted = a.sorted()
        return sorted[((sorted.size - 1) * p).toInt()] / 1_000_000.0
    }
    val putP95 = ms(putNs, 0.95)
    val codec = if (System.getProperty("os.name").orEmpty().startsWith("Windows", true)) "DPAPI" else "plaintext"
    val report = buildString {
        appendLine("bench: os=${System.getProperty("os.name")} codec=$codec n=$n")
        appendLine("bench: get  p50=%.3f ms  p95=%.3f ms".format(ms(getNs, 0.50), ms(getNs, 0.95)))
        appendLine("bench: put  p50=%.3f ms  p95=%.3f ms".format(ms(putNs, 0.50), putP95))
        appendLine(
            if (putP95 > 16.0) "bench: WARNING put p95 %.3f ms > 16 ms — WIN03 must write off the UI thread or add a shared per-root map".format(putP95)
            else "bench: OK put p95 within one 60 Hz frame (16 ms)",
        )
    }
    out.absoluteFile.parentFile?.mkdirs()
    out.writeText(report)
    print(report)
    System.out.flush()
}
