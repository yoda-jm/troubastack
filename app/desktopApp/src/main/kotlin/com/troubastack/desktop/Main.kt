// TroubaStage desktop entrypoint (WIN02, WIN03). The app: the shared App() on DesktopAppPlatform, in one window
// that remembers its place, goes full screen, and is the only one (D2). The `--smoke <png>` path is the CI launch
// smoke: it opens the real window, renders one frame of the UI to a PNG, and exits (proves the packaged app
// starts and its skiko render pipeline works on Windows).
package com.troubastack.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.TextButton
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEvent
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.isAltPressed
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.type
import androidx.compose.ui.input.pointer.PointerButton
import androidx.compose.ui.input.pointer.PointerEventType
import androidx.compose.ui.input.pointer.onPointerEvent
import androidx.compose.ui.unit.DpSize
import androidx.compose.ui.window.WindowPlacement
import androidx.compose.ui.window.WindowState
import androidx.compose.runtime.CompositionLocalProvider
import com.troubastack.app.App
import com.troubastack.app.AndroidImportFs
import com.troubastack.app.FileBundleFiles
import com.troubastack.app.SkiaImageDecoder
import com.troubastack.shared.bundle.BundleImporter
import com.troubastack.shared.bundle.BundleLoader
import com.troubastack.shared.bundle.ImportResult
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.stage.StageScreen
import com.troubastack.shared.stage.StageViewModel
import androidx.compose.material3.lightColorScheme
import com.troubastack.shared.stage.LocalGlyphTooltip
import com.troubastack.shared.stage.LocalStagePointerIcons
import com.troubastack.shared.ui.LocalMaxContentWidth
import com.troubastack.shared.ui.ThemePref
import com.troubastack.shared.ui.TroubaTheme
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.collectLatest
import java.awt.Dimension
import java.awt.GraphicsEnvironment
import java.awt.event.WindowAdapter
import java.awt.event.WindowEvent
import javax.swing.SwingUtilities
import javax.swing.UIManager
import kotlin.math.roundToInt
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
        // WIN03 acceptance 1: with a concert, the smoke performs it in Stage (two PNGs, <out>-WxH.png).
        val concert = args.indexOf("--smoke-concert").takeIf { it >= 0 }?.let { args.getOrNull(it + 1) }
        if (concert != null) runStageSmoke(out.removeSuffix(".png"), File(concert)) else runSmoke(File(out))
        return
    }
    // D1: the file dialog in the system's look (Windows' own), not Swing's default.
    if (isWindows()) runCatching { UIManager.setLookAndFeel(UIManager.getSystemLookAndFeelClassName()) }
    // D2: one app per data root; a second launch brings this one forward and exits.
    var bringToFront: () -> Unit = {}
    instanceClaim = SingleInstance.claim(File(desktopDataRoot())) { SwingUtilities.invokeLater { bringToFront() } }
        ?: exitProcess(0)
    // Acceptance 6: the debug wheel log — this run's wheel events, written to the data root on exit (VLL's Surface).
    WheelLog().apply { install(); saveOnExit(File(desktopDataRoot())) }
    runApp(onWindow = { bringToFront = it })
}

/** This launch's single-instance claim, held (lock and port) for the app's lifetime. */
private var instanceClaim: SingleInstance? = null

@OptIn(ExperimentalComposeUiApi::class)
private fun runApp(onWindow: (bringToFront: () -> Unit) -> Unit) = application {
    val storage = remember { Storage() }
    val opening = remember { openingPlacement(parsePlacement(storage.getSecret(WINDOW_PLACEMENT_KEY)), screens(), primaryScreen()) }
    // D2: windowed at launch (maximized if it was), at the remembered place.
    val windowState = rememberWindowState(
        placement = if (opening.maximized) WindowPlacement.Maximized else WindowPlacement.Floating,
        position = WindowPosition(opening.bounds.x.dp, opening.bounds.y.dp),
        size = DpSize(opening.bounds.width.dp, opening.bounds.height.dp),
    )
    val platform = remember { DesktopAppPlatform(storage, windowState) }
    var themePref by remember { mutableStateOf(ThemePref.parse(storage.getSecret(ThemePref.KEY))) }
    var askClose by remember { mutableStateOf(false) }
    Window(
        // D2: closing mid-concert asks first (an accidental close is A79's failure again).
        onCloseRequest = { if (platform.inConcert) askClose = true else exitApplication() },
        title = "TroubaStage",
        state = windowState,
        onKeyEvent = { e -> windowKey(e, platform) },
    ) {
        LaunchedEffect(Unit) {
            window.minimumSize = Dimension(MIN_WINDOW_WIDTH, MIN_WINDOW_HEIGHT)
            platform.awtWindow = window
            window.addWindowFocusListener(object : WindowAdapter() {
                override fun windowGainedFocus(e: WindowEvent?) = platform.onWindowFocused()
            })
            onWindow { windowState.isMinimized = false; window.toFront(); window.requestFocus() }
        }
        RememberPlacement(windowState, storage, opening)
        TroubaTheme(dark = themePref.resolveDark()) {
            // D3: Stage's pointer icons and glyph tooltips exist here only (Android passes neither).
            val stagePointer = remember { desktopStagePointerIcons() }
            // D7: list screens keep a readable width on a big window (Android leaves it full width).
            CompositionLocalProvider(
                LocalStagePointerIcons provides stagePointer,
                LocalGlyphTooltip provides desktopGlyphTooltip,
                LocalMaxContentWidth provides 960.dp,
            ) {
                // D4: the mouse Back button is Back, outside a concert only (side buttons never leave one, A79).
                Box(
                    Modifier.fillMaxSize().onPointerEvent(PointerEventType.Press) { e ->
                        if (e.button == PointerButton.Back && !platform.inConcert) platform.back.dispatch()
                    },
                ) {
                    App(platform, themePref = themePref, onThemePref = { themePref = it; storage.putSecret(ThemePref.KEY, it.name) })
                }
            }
            if (askClose) {
                AlertDialog(
                    onDismissRequest = { askClose = false },
                    title = { Text("Close TroubaStage?") },
                    text = { Text("A concert is open. Your place in it and your notes are kept.") },
                    confirmButton = { TextButton(onClick = ::exitApplication) { Text("Close") } },
                    dismissButton = { TextButton(onClick = { askClose = false }) { Text("Cancel") } },
                )
            }
        }
    }
}

/**
 * D2/D4 — the window's own keys, after the focused screen has had them (a dialog closes on its own Esc first):
 * F11 toggles full screen everywhere; Esc is Back (in a concert, stageBackAction: the drawer, then full screen,
 * never the concert); Alt+Left is Back outside a concert. Where nothing handles Back, Esc leaves full screen.
 */
private fun windowKey(e: KeyEvent, platform: DesktopAppPlatform): Boolean {
    if (e.type != KeyEventType.KeyDown) return false
    return when {
        e.key == Key.F11 -> { platform.window.toggleFullScreen(); true }
        e.key == Key.Escape -> platform.back.dispatch() || (platform.window.fullScreen && run { platform.window.toggleFullScreen(); true })
        e.key == Key.DirectionLeft && e.isAltPressed && !platform.inConcert -> platform.back.dispatch()
        else -> false
    }
}

/** D2 — save the windowed bounds and the maximized state as they change (not full screen, which is never restored
 *  at launch), half a second after the last change, so a crash or a kill keeps them too. */
@Composable
private fun RememberPlacement(state: WindowState, storage: Storage, opening: SavedPlacement) {
    LaunchedEffect(state) {
        var last = opening
        snapshotFlow { Triple(state.placement, state.position, state.size) }.collectLatest { (placement, position, size) ->
            delay(500)
            last = when (placement) {
                WindowPlacement.Floating -> {
                    if (position !is WindowPosition.Absolute) return@collectLatest
                    SavedPlacement(
                        Bounds(position.x.value.roundToInt(), position.y.value.roundToInt(), size.width.value.roundToInt(), size.height.value.roundToInt()),
                        maximized = false,
                    )
                }
                WindowPlacement.Maximized -> last.copy(maximized = true)
                WindowPlacement.Fullscreen -> return@collectLatest
            }
            storage.putSecret(WINDOW_PLACEMENT_KEY, last.encode())
        }
    }
}

private fun java.awt.Rectangle.toBounds() = Bounds(x, y, width, height)

private fun screens(): List<Bounds> =
    GraphicsEnvironment.getLocalGraphicsEnvironment().screenDevices.map { it.defaultConfiguration.bounds.toBounds() }

private fun primaryScreen(): Bounds =
    GraphicsEnvironment.getLocalGraphicsEnvironment().defaultScreenDevice.defaultConfiguration.bounds.toBounds()

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

/**
 * WIN03 acceptance 1 — the smoke performs a real concert: [tstage] is imported through the app's own import path
 * (BundleImporter over AndroidImportFs, as Import does) into the data root, loaded, and shown in Stage. The real window
 * shows it (B5: success only once that window has drawn); the PNGs are off-screen renders of the same Stage at
 * 1920×1080 and 800×600, after its pages have had time to decode.
 */
private fun runStageSmoke(prefix: String, tstage: File) {
    Thread {
        try { Thread.sleep(90_000) } catch (_: InterruptedException) { return@Thread }
        System.err.println("smoke: backstop timeout — no frame in 90s")
        Runtime.getRuntime().halt(1)
    }.apply { isDaemon = true; start() }

    val storage = Storage()
    val fs = AndroidImportFs(storage)
    val temp = File(storage.tempDir(), "smoke-${System.nanoTime()}.tstage").also { tstage.copyTo(it) }
    val imported = BundleImporter(fs).import(temp.path)
    temp.delete()
    val concertId = (imported as? ImportResult.Imported)?.concertId
    if (concertId == null) {
        System.err.println("smoke: FAILED — the import refused the concert: $imported")
        Runtime.getRuntime().halt(1)
        return
    }
    val dir = fs.bundleTargetDir(concertId)
    println("smoke: imported concert $concertId into $dir")
    fun stage(): @Composable () -> Unit {
        val loaded = BundleLoader().load(dir, FileBundleFiles())
        val roster = (loaded as? LoadResult.Loaded)?.bundle?.roster.orEmpty()
        // A part picked, so the smoke shows the page, not the "Who are you?" picker.
        val vm = StageViewModel(loaded, identity = roster.firstOrNull()?.memberId ?: "")
        val decoder = SkiaImageDecoder(File(dir))
        return { MaterialTheme(colorScheme = lightColorScheme()) { StageScreen(vm, decoder, onExit = {}, fullScreen = false) } }
    }

    application {
        Window(
            onCloseRequest = ::exitApplication,
            title = "TroubaStage",
            state = rememberWindowState(width = 1920.dp, height = 1080.dp, position = WindowPosition(0.dp, 0.dp)),
        ) {
            val content = remember { stage() }
            Box(Modifier.fillMaxSize().drawWithContent { drawContent(); windowDrew = true }) { content() }
            LaunchedEffect(Unit) {
                runCatching { println("smoke: window render API = ${window.renderApi}") }
                val rendered = runCatching {
                    renderStage(stage(), 1920, 1080, File("$prefix-1920x1080.png"))
                    renderStage(stage(), 800, 600, File("$prefix-800x600.png"))
                }
                rendered.exceptionOrNull()?.printStackTrace()
                var frames = 0
                while (!windowDrew && frames < 240) {
                    withFrameNanos { }
                    frames++
                }
                val pngs = listOf(File("$prefix-1920x1080.png"), File("$prefix-800x600.png"))
                val ok = rendered.isSuccess && pngs.all { it.isFile && it.length() > 0 } && windowDrew
                if (ok) println("smoke: OK — Stage drew in the window after $frames frame ticks; PNGs ${pngs.map { "${it.name} ${it.length()} B" }}")
                else System.err.println("smoke: FAILED — rendered=${rendered.isSuccess} windowDrew=$windowDrew pngs=${pngs.map { it.length() }}")
                System.out.flush()
                System.err.flush()
                Runtime.getRuntime().halt(if (ok) 0 else 1)
            }
        }
    }
}

/** Off-screen Stage at [w]×[h]: frames over 4 s of real time so the pages decode (the decode is asynchronous), then the
 *  PNG. By then the chrome has hidden itself (its auto-hide waits in real time), so the PNG shows the page alone. */
private fun renderStage(content: @Composable () -> Unit, w: Int, h: Int, out: File) {
    val scene = ImageComposeScene(width = w, height = h, density = Density(1f), content = content)
    try {
        var t = 0L
        repeat(40) { scene.render(t); t += 100_000_000L; Thread.sleep(100) }
        val png = scene.render(t).encodeToData(EncodedImageFormat.PNG) ?: error("skiko produced no PNG data")
        out.absoluteFile.parentFile?.mkdirs()
        out.writeBytes(png.bytes)
    } finally {
        scene.close()
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
