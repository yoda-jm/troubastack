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
import androidx.compose.ui.Alignment
import androidx.compose.ui.ImageComposeScene
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.WindowPosition
import androidx.compose.ui.window.application
import androidx.compose.ui.window.rememberWindowState
import com.troubastack.shared.buildinfo.appBuildLine
import org.jetbrains.skia.EncodedImageFormat
import java.io.File
import kotlin.system.exitProcess

/** The whole desktop UI for now: "TroubaStage" and the build line (commit + UTC build time), from :shared. */
@Composable
private fun AppRoot() {
    MaterialTheme {
        Surface(modifier = Modifier.fillMaxSize()) {
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

fun main(args: Array<String>) {
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
            AppRoot()
            LaunchedEffect(Unit) {
                // The window is up (we are in its composition). Render one frame, then terminate with halt(),
                // NOT System.exit/exitProcess: System.exit from the Compose/AWT thread deadlocks on the AWT
                // shutdown sequence (the PNG lands but the JVM hangs). halt() skips shutdown hooks and exits
                // immediately — the PNG is already flushed to disk — so both the happy and the error path end
                // cleanly with the right code even when the render threw on the very first frame.
                val attempt = runCatching { renderFrame(out) }
                attempt.exceptionOrNull()?.printStackTrace()
                val ok = attempt.isSuccess && out.isFile && out.length() > 0
                if (ok) println("smoke: wrote ${out.absolutePath} (${out.length()} bytes)")
                else System.err.println("smoke: FAILED — no frame rendered")
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
