package com.troubastack.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.awt.ComposeWindow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.WindowState
import com.sun.jna.platform.win32.Kernel32
import com.sun.jna.platform.win32.WinBase
import com.troubastack.app.AppPlatform
import com.troubastack.app.AppUpdate
import com.troubastack.app.BackDispatcher
import com.troubastack.app.BundlePicker
import com.troubastack.app.FileRehearsalNotes
import com.troubastack.app.HttpTransport
import com.troubastack.app.PedalLink
import com.troubastack.app.RehearsalNotesStore
import com.troubastack.app.SkiaImageDecoder
import com.troubastack.app.SkiaNoteBitmapCodec
import com.troubastack.app.StudioHost
import com.troubastack.shared.distribution.ServerDiscovery
import com.troubastack.shared.buildinfo.UPDATE_RELEASE_URL
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.seams.desktopDataRoot
import com.sun.jna.platform.win32.Advapi32Util
import com.sun.jna.platform.win32.WinReg
import com.troubastack.shared.stage.ImageDecoder
import com.troubastack.shared.stage.PageTurn
import com.troubastack.shared.stage.PointerKind
import kotlinx.coroutines.CoroutineScope
import java.io.File
import java.time.LocalTime
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.util.UUID
import javax.swing.JFileChooser
import javax.swing.filechooser.FileNameExtensionFilter

/**
 * WIN03 — the Windows [AppPlatform] (D1 and the gate's additions). Studio is present but says "Coming soon" (D6);
 * the camera QR scanner, LAN discovery, the BLE-MIDI pedal, the in-app update (until WIN04), `/join/` links and
 * volume keys are null, so their affordances are hidden. Keyboard pedals and learned keys still work (A72).
 */
class DesktopAppPlatform(override val storage: Storage, windowState: WindowState) : AppPlatform {
    /** The app's window, for the file dialog's parent; set by the entrypoint once the window exists. */
    var awtWindow: ComposeWindow? = null

    /** Esc, Alt+Left and the mouse Back button (outside Stage) dispatch here; the entrypoint routes them. */
    val back = BackDispatcher()

    override val window = DesktopWindowControls(windowState, storage)

    /** A concert is open (Stage is composed): closing the window asks first, and Alt+Left / mouse Back do nothing. */
    var inConcert = false
        private set

    override fun imageDecoder(dir: String): ImageDecoder = SkiaImageDecoder(File(dir))

    override fun rehearsalNotes(notesDir: String): RehearsalNotesStore = FileRehearsalNotes(notesDir, SkiaNoteBitmapCodec)

    override val bundlePicker: BundlePicker = object : BundlePicker {
        @Composable
        override fun rememberLauncher(onPicked: (tempPath: String) -> Unit): () -> Unit = {
            pickTstage(awtWindow)?.let { onPicked(copyToTemp(storage, it)) }
        }
    }

    override val discovery: ServerDiscovery? = null
    override val qrScanner: (@Composable (onDecoded: (String) -> Unit, onClose: () -> Unit) -> Unit)? = null

    override val studio: StudioHost = object : StudioHost {
        @Composable
        override fun Browse(
            transport: HttpTransport,
            onOpen: (initialPath: String, bandName: String, bandId: String) -> Unit,
            onShowQr: (bandId: String, bandName: String) -> Unit,
            onBack: () -> Unit,
        ) {
            StudioComingSoon(onBack)
            BackHandler(onBack)
        }

        // Neither is reachable from the Coming-soon screen; if one is ever composed, say why instead of drawing nothing.
        @Composable
        override fun Edit(storage: Storage, onBack: () -> Unit, initialPath: String?, bandTitle: String?) =
            error("Studio's editor is not on Windows yet (WIN05)")

        @Composable
        override fun PresentQr(transport: HttpTransport, bandId: String, bandName: String, onManageInStudio: () -> Unit, onBack: () -> Unit) =
            error("Studio's room QR is not on Windows yet (WIN05)")

        override fun clearCookies() {} // no Studio web view, so no cookies
    }

    /** WIN04 — closes the app for an update's install (the entrypoint sets it: exitApplication). */
    var exitForInstall: () -> Unit = {}

    /** WIN04 — the CI hook's release (`--update-from`, loopback only, digest required), else GitHub's. */
    var updateFrom: String? = null

    /** The app's update, made once per App(); it reports the last handoff's result as it is made. */
    var update: DesktopAppUpdate? = null
        private set

    override val appUpdate: ((Storage, CoroutineScope) -> AppUpdate)? = { s, scope ->
        DesktopAppUpdate(
            s, scope, dataRoot = File(desktopDataRoot()), installedExe = installedMsiExe(),
            releaseUrl = updateFrom ?: UPDATE_RELEASE_URL, requireDigest = updateFrom != null,
            exitForInstall = { exitForInstall() },
        ).also { it.reportLastHandoff(); update = it }
    }
    override val pedal: (() -> PedalLink)? = null

    @Composable
    override fun StageWindow(content: @Composable () -> Unit) {
        // D2: full screen on entering when Parameters says so, the entered mode back on leaving; and the display
        // does not sleep and the screen saver does not start while a concert is open.
        DisposableEffect(Unit) {
            inConcert = true
            window.enterConcert()
            keepDisplayOn(true)
            onDispose {
                keepDisplayOn(false)
                window.leaveConcert()
                inConcert = false
            }
        }
        content()
    }

    @Composable
    override fun BackHandler(onBack: () -> Unit) = back.Handler(onBack)

    override fun monotonicNowMs(): Long = monotonicMs()

    override fun localTimeText(): String = LocalTime.now().format(DateTimeFormatter.ofLocalizedTime(FormatStyle.SHORT))

    private val resumeTick = mutableStateOf(0)
    override val resumeTicks: State<Int> get() = resumeTick

    /** A31 — the window gained focus: Home re-probes the server, as on Android's resume. */
    fun onWindowFocused() { resumeTick.value++ }

    override val pendingInviteLink: MutableState<String?>? = null
    override val volumeTurnRegistrar: ((((PageTurn) -> Unit)?) -> Unit)? = null

    override val settingsWereReset: Boolean get() = Storage.settingsWereReset

    override val pointer: PointerKind = PointerKind.MOUSE

    /** No starting server: Connect starts at "http://" and nothing is contacted until one is set (never the Android
     *  emulator's alias, which on a 10.x LAN is somebody's machine). */
    override val defaultServerUrl: String? = null
}

/** D6 — the TroubaStudio screen on Windows: its heading, "Coming soon.", and Back. */
@Composable
private fun StudioComingSoon(onBack: () -> Unit) {
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize().padding(20.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                TextButton(onClick = onBack) { Text("‹  Back") }
                Text("TroubaStudio", style = MaterialTheme.typography.headlineSmall, color = MaterialTheme.colorScheme.primary)
            }
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text("Coming soon.", style = MaterialTheme.typography.titleLarge)
            }
        }
    }
}

internal fun isWindows(): Boolean = System.getProperty("os.name").orEmpty().startsWith("Windows", ignoreCase = true)

/**
 * T147's chrono contract: a monotonic clock that keeps counting through sleep. On Windows that is GetTickCount64
 * (documented to include sleep and hibernation; QueryPerformanceCounter, behind System.nanoTime, is not documented
 * either way). Elsewhere System.nanoTime, which on Linux is CLOCK_MONOTONIC and does NOT count a suspend: the Linux
 * desktop is a developer run, not a target.
 */
private fun monotonicMs(): Long =
    if (isWindows()) Kernel32.INSTANCE.GetTickCount64() else System.nanoTime() / 1_000_000

/** SetThreadExecutionState holds for the calling thread; Compose calls this on the AWT event thread, which lives as
 *  long as the app. A no-op off Windows. */
private fun keepDisplayOn(on: Boolean) {
    if (!isWindows()) return
    val flags = if (on) WinBase.ES_CONTINUOUS or WinBase.ES_DISPLAY_REQUIRED or WinBase.ES_SYSTEM_REQUIRED else WinBase.ES_CONTINUOUS
    Kernel32.INSTANCE.SetThreadExecutionState(flags)
}

/** The system file dialog, filtered to `.tstage` with the "All files" fallback (D1). Null when cancelled. */
private fun pickTstage(parent: ComposeWindow?): File? {
    val chooser = JFileChooser().apply {
        dialogTitle = "Import a concert"
        fileFilter = FileNameExtensionFilter("TroubaStage concert (*.tstage)", "tstage")
        isAcceptAllFileFilterUsed = true
    }
    return if (chooser.showOpenDialog(parent) == JFileChooser.APPROVE_OPTION) chooser.selectedFile else null
}

/** The importer takes a temporary copy it deletes afterwards (the BundlePicker contract). */
private fun copyToTemp(storage: Storage, picked: File): String {
    val temp = File(storage.tempDir(), "pick-${UUID.randomUUID()}.tstage")
    picked.copyTo(temp)
    return temp.path
}

/**
 * WIN04 (H4) — the running launcher when it is the MSI-installed TroubaStage (jpackage names it in `jpackage.app-path`;
 * the uninstall entries say where the MSI put it), else null: a `:desktopApp:run` or a copied folder cannot update itself.
 */
internal fun installedMsiExe(): File? {
    if (!isWindows()) return null
    // jpackage's launcher names itself; else the process image (the launcher hosts the JVM in-process).
    val exe = (System.getProperty("jpackage.app-path") ?: ProcessHandle.current().info().command().orElse(null)?.takeIf { it.endsWith("TroubaStage.exe", ignoreCase = true) })
        ?.let(::File) ?: return null
    return exe.takeIf { isMsiInstalled(it, runCatching { uninstallEntries() }.getOrDefault(emptyList())) }
}

/** DisplayName to InstallLocation of every uninstall entry, per-user and machine-wide (as the CI upgrade check reads them). */
private fun uninstallEntries(): List<Pair<String, String>> {
    val keys = listOf(
        WinReg.HKEY_CURRENT_USER to "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall",
        WinReg.HKEY_LOCAL_MACHINE to "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall",
        WinReg.HKEY_LOCAL_MACHINE to "Software\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall",
    )
    return keys.flatMap { (root, base) ->
        runCatching { Advapi32Util.registryGetKeys(root, base).toList() }.getOrDefault(emptyList()).mapNotNull { k ->
            val path = "$base\\$k"
            val name = runCatching { Advapi32Util.registryGetStringValue(root, path, "DisplayName") }.getOrNull() ?: return@mapNotNull null
            name to (runCatching { Advapi32Util.registryGetStringValue(root, path, "InstallLocation") }.getOrNull() ?: "")
        }
    }
}
