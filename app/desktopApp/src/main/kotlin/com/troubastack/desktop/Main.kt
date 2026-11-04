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
import androidx.compose.ui.input.key.isCtrlPressed
import androidx.compose.ui.input.key.isMetaPressed
import androidx.compose.ui.graphics.painter.BitmapPainter
import androidx.compose.ui.graphics.painter.Painter
import androidx.compose.ui.graphics.toComposeImageBitmap
import org.jetbrains.skia.Image as SkiaImage
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
import com.troubastack.app.HttpTransport
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
import com.troubastack.shared.stage.CommandModifier
import com.troubastack.shared.stage.LocalCommandModifier
import com.troubastack.shared.stage.LocalStagePointerIcons
import com.troubastack.shared.ui.LocalMaxContentWidth
import com.troubastack.shared.ui.ThemePref
import com.troubastack.shared.ui.TroubaTheme
import com.troubastack.shared.buildinfo.UpdateVerdict
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
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
import com.troubastack.desktop.studio.win08SmokeRunner
import com.troubastack.shared.buildinfo.appBuildLine
import com.troubastack.shared.seams.HostOs
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.seams.hostOs
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
    if (isWindows()) {
        // NB: WIN08 does NOT set compose.layers.type=WINDOW. Making ALL Compose popups separate windows stole window
        // activation from the frame on every tooltip/dialog (the frame's title bar went inactive) and drew over the
        // taskbar — unacceptable for Stage (a concert pedal). Only the Studio ⋮ menu floats over the heavyweight
        // browser, as a heavyweight, never-activating, never-topmost owned Swing window (SwtBrowserController.openStudioMenu).
        // L2 sabotage ONLY: re-enable the bad path so the smoke's foreground check has a proven red (a WINDOW-layer
        // Compose popup DOES take activation on the CI runner, unlike a plain focusable JWindow). Dormant otherwise.
        if (System.getenv("WIN08_SABOTAGE") == "menufocus") System.setProperty("compose.layers.type", "WINDOW")
        // WIN08 (S1.2 / WIN07) — SWT unpacks its native DLLs under the data root, never the shared ~/.swt.
        System.setProperty("swt.library.path", File(desktopDataRoot(), "swt").also { runCatching { it.mkdirs() } }.path)
        // WIN08 — raise SWT's Edge operation timeout (R4_40 Edge.java: MAXIMUM_OPERATION_TIME, default 5000, read once
        // into a static final before any Browser class loads). A cold WebView2 controller can exceed 5s; the FIRST
        // timeout poisons the Browser (getWebViewWrapper completeExceptionally → the late controller is released), so
        // raise the single wait instead of warming up or retrying. Internal property, acceptable under the exact 3.134.0 pin.
        System.setProperty("org.eclipse.swt.internal.win32.Edge.timeout", "30000")
    }
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
    // WIN08 S1.8 — the embedded-Studio CI smoke: launch into the Studio screen against a loopback page and run
    // the checks (engine, frame-owned, cookie, bridge, nav-veto, data dir), each with a marker line, then exit.
    val win08Idx = args.indexOf("--win08-smoke")
    if (win08Idx >= 0) {
        val outDir = args.getOrNull(win08Idx + 1)?.takeUnless { it.startsWith("--") }
            ?: run { System.err.println("--win08-smoke needs an output dir"); exitProcess(2) }
        val runner = win08SmokeRunner
            ?: run { System.err.println("win08-smoke: no runner on this OS (Windows only)"); exitProcess(2) }
        val code = runner.run(File(outDir))
        // Do NOT force-exit on success: the smoke posted WM_CLOSE and the process must end on its OWN (check 7).
        // The runner halts(1) itself on a failed check, so this guard is belt-and-suspenders.
        if (code != 0) exitProcess(code)
        return
    }
    // WIN08 S1.1 (item 3) — re-entry: leave and re-enter the embedded Studio screen twice, and an immediate Back.
    val reentryIdx = args.indexOf("--win08-reentry")
    if (reentryIdx >= 0) {
        val outDir = args.getOrNull(reentryIdx + 1)?.takeUnless { it.startsWith("--") }
            ?: run { System.err.println("--win08-reentry needs an output dir"); exitProcess(2) }
        val runner = win08SmokeRunner
            ?: run { System.err.println("win08-reentry: no runner on this OS (Windows only)"); exitProcess(2) }
        val code = runner.runReentry(File(outDir))
        if (code != 0) exitProcess(code)
        return
    }
    // WIN08 item 2 — the real-Studio CI hook (`--win08-studio <loopback URL>`): sign in THROUGH the app's own
    // sign-in code (HttpTransport.connect, the call Connect's "Sign in" makes), storing the session exactly as
    // Connect does, then open the embedded editor at the demo song. Loopback only; credentials from the environment
    // (TROUBA_WIN08_USER / _PASS), never from argv; the song path from TROUBA_WIN08_SONG_PATH.
    val studioIdx = args.indexOf("--win08-studio")
    if (studioIdx >= 0) {
        val url = args.getOrNull(studioIdx + 1)?.takeUnless { it.startsWith("--") }
            ?: run { System.err.println("--win08-studio needs a loopback URL"); exitProcess(2) }
        val host = runCatching { java.net.URI(url).host }.getOrNull()
        if (host != "127.0.0.1" && host != "localhost") { System.err.println("--win08-studio: loopback URL only"); exitProcess(2) }
        val user = System.getenv("TROUBA_WIN08_USER") ?: run { System.err.println("--win08-studio: TROUBA_WIN08_USER unset"); exitProcess(5) }
        val pass = System.getenv("TROUBA_WIN08_PASS") ?: run { System.err.println("--win08-studio: TROUBA_WIN08_PASS unset"); exitProcess(5) }
        runWin08Studio(url, user, pass, System.getenv("TROUBA_WIN08_SONG_PATH"))
        return
    }
    // WIN08 item 8 — the Stage CI hook (`--win08-stage <concert.tstage>`): import the demo concert and show it
    // INTERACTIVELY in Stage (no sign-in), with the desktop glyph-tooltip provider, so a screenshot can capture a
    // Stage tooltip (a compose.layers popup) and check a page-turn key still turns the page while it shows.
    val stageIdx = args.indexOf("--win08-stage")
    if (stageIdx >= 0) {
        val concert = args.getOrNull(stageIdx + 1)?.takeUnless { it.startsWith("--") }
            ?: run { System.err.println("--win08-stage needs a .tstage path"); exitProcess(2) }
        runWin08Stage(concert)
        return
    }
    // WIN08 S2.1 (R1) SPIKE — the cover render hook (`--win08-cover <out.png>`): point the embedded studio at an
    // UNREACHABLE server so the cover shows, then capture the window to prove the cover draws OVER the SwingPanel (the
    // open rendering question), not occluded by the heavyweight canvas. Windows only (needs the SWT provider).
    val coverIdx = args.indexOf("--win08-cover")
    if (coverIdx >= 0) {
        val out = args.getOrNull(coverIdx + 1)?.takeUnless { it.startsWith("--") }
            ?: run { System.err.println("--win08-cover needs an output PNG path"); exitProcess(2) }
        runWin08Cover(out)
        return
    }
    // WIN04 (C1) — the CI hook: a release on the loopback interface (refused otherwise), digest required.
    val updateFrom = args.indexOf("--update-from").takeIf { it >= 0 }?.let { i ->
        val raw = args.getOrNull(i + 1) ?: run { System.err.println("--update-from needs a URL"); exitProcess(2) }
        runCatching { loopbackReleaseUrl(raw) }.getOrElse { System.err.println(it.message); exitProcess(2) }
    }
    if ("--update-now" in args) {
        runUpdateNow(updateFrom ?: run { System.err.println("--update-now needs --update-from"); exitProcess(2) })
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
    runApp(updateFrom, onWindow = { bringToFront = it })
}

/**
 * WIN04 — CI only (`--update-from <loopback URL> --update-now`): the Update button's path without the window — the
 * check (manual), the download and its verification, then the handoff to the helper, which installs and reopens the
 * app. Exit codes: 3 no update offered, 4 refused (nothing installed, no helper started).
 */
private fun runUpdateNow(releaseUrl: String) {
    val storage = Storage()
    val u = DesktopAppUpdate(
        storage, CoroutineScope(Dispatchers.Default), dataRoot = File(desktopDataRoot()), installedExe = installedMsiExe(),
        releaseUrl = releaseUrl, hookMode = true, exitForInstall = { Runtime.getRuntime().halt(0) },
    )
    runBlocking {
        val release = when (val v = u.checkNow(manual = true)) {
            is UpdateVerdict.Available -> v.release
            is UpdateVerdict.Dismissed -> v.release
            else -> { System.err.println("update-now: no update offered ($v, failed=${u.failed})"); exitProcess(3) }
        }
        if (installedMsiExe() == null) { System.err.println("update-now: $CANNOT_INSTALL_LINE"); exitProcess(4) }
        val msi = try {
            u.downloadAndVerify(release)
        } catch (e: UpdateRefused) {
            System.err.println("update-now: refused: ${e.message}"); exitProcess(4)
        }
        println("update-now: verified ${msi.name}; handing off")
        System.out.flush()
        u.install(msi)
    }
}

/** This launch's single-instance claim, held (lock and port) for the app's lifetime. */
private var instanceClaim: SingleInstance? = null

@OptIn(ExperimentalComposeUiApi::class)
private fun runApp(updateFrom: String?, onWindow: (bringToFront: () -> Unit) -> Unit) = application {
    val storage = remember { Storage() }
    val opening = remember { openingPlacement(parsePlacement(storage.getSecret(WINDOW_PLACEMENT_KEY)), screens(), primaryScreen()) }
    // D2: windowed at launch (maximized if it was), at the remembered place.
    val windowState = rememberWindowState(
        placement = if (opening.maximized) WindowPlacement.Maximized else WindowPlacement.Floating,
        position = WindowPosition(opening.bounds.x.dp, opening.bounds.y.dp),
        size = DpSize(opening.bounds.width.dp, opening.bounds.height.dp),
    )
    val platform = remember { DesktopAppPlatform(storage, windowState).apply { this.updateFrom = updateFrom; exitForInstall = ::exitApplication } }
    var themePref by remember { mutableStateOf(ThemePref.parse(storage.getSecret(ThemePref.KEY))) }
    var askClose by remember { mutableStateOf(false) }
    // D2: closing mid-concert asks first (an accidental close is A79's failure again).
    val requestClose = { if (platform.inConcert) askClose = true else exitApplication() }
    // DESK01 D3: Cmd+Q (and the app menu's and the Dock's Quit) take the window's close path, question included,
    // instead of the JDK's default System.exit.
    LaunchedEffect(Unit) {
        if (hostOs == HostOs.MACOS) runCatching {
            java.awt.Desktop.getDesktop().setQuitHandler { _, response -> response.cancelQuit(); requestClose() }
        }
    }
    Window(
        onCloseRequest = requestClose,
        title = "TroubaStage",
        icon = appIcon,
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
                LocalCommandModifier provides if (hostOs == HostOs.MACOS) CommandModifier.META else CommandModifier.CTRL,
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
 * WIN08 item 2 — sign in through the app's OWN sign-in code (HttpTransport.connect, what Connect's "Sign in" calls)
 * against a loopback core, storing the session exactly as Connect does, then open the embedded editor at [songPath].
 * The CI screenshots the window and injects a pen stroke. Windows only in practice (Edit shows the embedded host).
 */
@OptIn(ExperimentalComposeUiApi::class)
private fun runWin08Studio(url: String, user: String, pass: String, songPath: String?) {
    System.setProperty("win08.log", "1") // diagnostic path: write the durable win08.log (F11 key trace etc.)
    val storage = Storage()
    storage.putSecret("coreUrl", url)
    val transport = HttpTransport(storage, defaultServerUrl = null) { }
    val err = runBlocking { transport.connect(user, pass) }
    if (err != null) { System.err.println("win08-studio: sign-in failed: $err"); exitProcess(5) }
    println("win08-studio: signed in as $user; opening ${songPath ?: "/"}")
    application {
        val windowState = rememberWindowState(width = 1400.dp, height = 900.dp, position = WindowPosition(0.dp, 0.dp))
        val platform = remember { DesktopAppPlatform(storage, windowState) }
        Window(
            onCloseRequest = ::exitApplication,
            title = "TroubaStage",
            state = windowState,
            onKeyEvent = { e -> windowKey(e, platform) },
        ) {
            LaunchedEffect(Unit) { platform.awtWindow = window }
            TroubaTheme(dark = false) {
                platform.studio.Edit(storage, onBack = { exitApplication() }, initialPath = songPath, bandTitle = "CI Band")
            }
        }
    }
}

/** WIN08 S2.1 (R1) SPIKE — render the embedded studio against an unreachable server (the cover shows), capture the
 *  window after the probe + a paint beat, and write the PNG so the cover's rendering over the SwingPanel can be
 *  eyeballed. Also prints a crude centre-strip light/dark count as a quick hint. Windows only (SWT provider). */
private fun runWin08Cover(outPng: String) {
    System.setProperty("win08.log", "1")
    val storage = Storage()
    storage.putSecret("coreUrl", "http://127.0.0.1:59999") // nothing listening → probePresence = Unreachable → cover
    println("win08-cover: coreUrl = a dead loopback port; expecting the cover over the Studio area")
    application {
        val windowState = rememberWindowState(width = 1400.dp, height = 900.dp, position = WindowPosition(0.dp, 0.dp))
        val platform = remember { DesktopAppPlatform(storage, windowState) }
        Window(onCloseRequest = ::exitApplication, title = "TroubaStage", state = windowState) {
            LaunchedEffect(Unit) {
                platform.awtWindow = window
                kotlinx.coroutines.delay(9000) // probe (3s) + a beat for the cover to paint
                runCatching {
                    val b = window.bounds
                    val img = java.awt.Robot().createScreenCapture(java.awt.Rectangle(b.x, b.y, b.width, b.height))
                    javax.imageio.ImageIO.write(img, "png", File(outPng))
                    var light = 0; var dark = 0
                    val y = img.height / 2
                    for (x in 0 until img.width step 10) {
                        val c = java.awt.Color(img.getRGB(x, y)); if (c.red + c.green + c.blue > 360) light++ else dark++
                    }
                    // A packaged GUI app has no captured stdout, so write the verdict to a sidecar file the CI echoes.
                    File("$outPng.txt").writeText("bytes=${File(outPng).length()} centreStrip light=$light dark=$dark")
                }.onFailure { runCatching { File("$outPng.txt").writeText("capture failed: ${it.message}") } }
                exitApplication()
            }
            TroubaTheme(dark = false) {
                platform.studio.Edit(storage, onBack = { exitApplication() }, initialPath = null, bandTitle = "CI Band")
            }
        }
    }
}

/** Item 8 — show the demo concert in Stage INTERACTIVELY (no sign-in), with the desktop glyph-tooltip provider, so a
 *  screenshot can capture a Stage tooltip (a compose.layers popup) and verify a page-turn key still turns the page. */
private fun runWin08Stage(tstagePath: String) {
    System.setProperty("win08.log", "1")
    val storage = Storage()
    val fs = AndroidImportFs(storage)
    val temp = File(storage.tempDir(), "win08stage-${System.nanoTime()}.tstage").also { File(tstagePath).copyTo(it) }
    val imported = BundleImporter(fs).import(temp.path); temp.delete()
    val concertId = (imported as? ImportResult.Imported)?.concertId
        ?: run { System.err.println("win08-stage: import refused: $imported"); exitProcess(5) }
    val dir = fs.bundleTargetDir(concertId)
    val loaded = BundleLoader().load(dir, FileBundleFiles())
    val roster = (loaded as? LoadResult.Loaded)?.bundle?.roster.orEmpty()
    val vm = StageViewModel(loaded, identity = roster.firstOrNull()?.memberId ?: "")
    val decoder = SkiaImageDecoder(File(dir))
    println("win08-stage: concert $concertId in Stage")
    application {
        Window(
            onCloseRequest = ::exitApplication,
            title = "TroubaStage",
            state = rememberWindowState(width = 1400.dp, height = 900.dp, position = WindowPosition(0.dp, 0.dp)),
        ) {
            val stagePointer = remember { desktopStagePointerIcons() }
            TroubaTheme(dark = false) {
                CompositionLocalProvider(
                    LocalStagePointerIcons provides stagePointer,
                    LocalCommandModifier provides CommandModifier.CTRL,
                    LocalGlyphTooltip provides desktopGlyphTooltip,
                ) {
                    StageScreen(vm, decoder, onExit = { exitApplication() }, fullScreen = false)
                }
            }
        }
    }
}

/** D2/D4 — what a key does to the window, whatever screen has the focus. */
internal enum class WindowAction { TOGGLE_FULL_SCREEN, ESCAPE, BACK }

/**
 * D2/D4 — the window's own keys, per OS (DESK01 D3), after the focused screen has had them (a dialog closes on its own
 * Esc first). Full screen: F11 on Windows and Linux, Ctrl+Cmd+F on macOS (F11 is the system's there). Esc everywhere.
 * Back outside a concert: Alt+Left; on macOS Cmd+[ and Cmd+Left, as in its browsers. Pure, for the per-OS tests.
 */
internal fun windowAction(os: HostOs, key: Key, ctrl: Boolean, alt: Boolean, meta: Boolean, inConcert: Boolean): WindowAction? = when {
    key == Key.Escape -> WindowAction.ESCAPE
    os == HostOs.MACOS -> when {
        key == Key.F && ctrl && meta -> WindowAction.TOGGLE_FULL_SCREEN
        meta && !ctrl && !alt && (key == Key.LeftBracket || key == Key.DirectionLeft) && !inConcert -> WindowAction.BACK
        else -> null
    }
    key == Key.F11 -> WindowAction.TOGGLE_FULL_SCREEN
    key == Key.DirectionLeft && alt && !inConcert -> WindowAction.BACK
    else -> null
}

/** Esc is Back (in a concert, stageBackAction: the drawer, then full screen, never the concert); where nothing
 *  handles Back, it leaves full screen. */
private fun windowKey(e: KeyEvent, platform: DesktopAppPlatform): Boolean {
    if (e.type != KeyEventType.KeyDown) return false
    return when (windowAction(hostOs, e.key, e.isCtrlPressed, e.isAltPressed, e.isMetaPressed, platform.inConcert)) {
        WindowAction.TOGGLE_FULL_SCREEN -> { platform.window.toggleFullScreen(); true }
        WindowAction.ESCAPE -> platform.back.dispatch() || (platform.window.fullScreen && run { platform.window.toggleFullScreen(); true })
        WindowAction.BACK -> platform.back.dispatch()
        null -> false
    }
}

/** DESK01 — the brand mark as the window's icon (the title bar and task bar on Windows and Linux; Java's cup before). */
private val appIcon: Painter? by lazy {
    runCatching {
        WindowAction::class.java.getResourceAsStream("/troubastage.png")?.use { BitmapPainter(SkiaImage.makeFromEncoded(it.readBytes()).toComposeImageBitmap()) }
    }.getOrNull()
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
                // DESK01 — the PNGs are rendered off-screen BEFORE the window is checked, so they are no proof; this file is,
                // written only here, once the real window drew (for a launch whose exit code is not seen: LaunchServices).
                if (ok) File("$prefix-ok.txt").writeText("drew after $frames frame ticks\n")
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
