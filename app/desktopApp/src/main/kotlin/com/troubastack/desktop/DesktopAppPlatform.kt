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
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.State
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.awt.ComposeWindow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.WindowState
import com.sun.jna.platform.win32.Kernel32
import com.troubastack.app.AppPlatform
import com.troubastack.app.AppUpdate
import com.troubastack.app.BackDispatcher
import com.troubastack.app.BundlePicker
import com.troubastack.app.FileRehearsalNotes
import com.troubastack.app.dropSessionIfOriginChanged
import com.troubastack.app.HttpTransport
import com.troubastack.app.PedalLink
import com.troubastack.app.QrPresentScreen
import com.troubastack.app.Presence
import com.troubastack.app.RehearsalNotesStore
import com.troubastack.app.SkiaImageDecoder
import com.troubastack.app.ShippedLicences
import com.troubastack.app.SkiaNoteBitmapCodec
import com.troubastack.app.StudioHost
import com.troubastack.app.sessionCookieFor
import com.troubastack.desktop.studio.EmbeddedStudioArgs
import com.troubastack.desktop.studio.embeddedStudioProvider
import com.troubastack.shared.distribution.ServerDiscovery
import com.troubastack.shared.buildinfo.UPDATE_RELEASE_URL
import com.troubastack.shared.seams.HostOs
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.seams.hostOs
import com.troubastack.shared.seams.desktopDataRoot
import com.sun.jna.platform.win32.Advapi32Util
import com.sun.jna.platform.win32.WinReg
import com.troubastack.shared.stage.ImageDecoder
import com.troubastack.shared.stage.PageTurn
import com.troubastack.shared.stage.PointerKind
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.async
import kotlinx.coroutines.withTimeoutOrNull
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
        // WIN08 — on Windows the embedded-Studio provider hosts Studio (WebView2/SWT) in the window; on Linux and
        // macOS no provider is registered, so both Browse and Edit fall back to "Coming soon." (D6). Browse opens
        // Studio at its root; Edit opens the SAME host at initialPath with the band title. WIN09: Browse's ⋮ "Show band
        // QR…" draws PresentQr over the embedded screen itself (onShowQr stays unused, so Studio keeps its place).
        @Composable
        override fun Browse(
            transport: HttpTransport,
            onOpen: (initialPath: String, bandName: String, bandId: String) -> Unit,
            onShowQr: (bandId: String, bandName: String) -> Unit,
            onBack: () -> Unit,
        ) {
            val provider = embeddedStudioProvider
            if (provider != null) {
                // S2.2: WindowAction.BACK (Alt+← etc. with the chrome focused) must walk Studio's history, not leave —
                // the embedded screen hands back controller.requestBack, and the BackHandler calls it.
                var embeddedBack by remember { mutableStateOf<(() -> Unit)?>(null) }
                var serverNonce by remember { mutableStateOf(0) }   // S2.3b: a re-point bumps it → a fresh controller
                // WIN09 — ⋮ "Show band QR…": the bands this person administers, fetched as Studio opens and again after a
                // re-point (no item until then, nor when there are none). The picker and the QR draw OVER the embedded
                // screen (hostOverlay collapses it to 0.dp), so the Browser lives on underneath and Back returns to Studio
                // where it was; "Manage / revoke in Studio" loads the band's settings in that same Browser.
                // Each ⋮ press refreshes it too (refreshMenu), so it follows the roles while Studio stays open: only the
                // newest fetch writes (a generation), a failed fetch keeps the list (fetchStudioBandsOrNull's null), and a
                // re-point blanks it at once. The menu waits ≤ 800 ms; a later answer updates the list for the next press.
                var adminBands by remember { mutableStateOf<List<HttpTransport.StudioBand>>(emptyList()) }
                var bandQr by remember { mutableStateOf<BandQrView?>(null) }
                val bandsScope = rememberCoroutineScope()
                val bandsFetch = remember { BandsFetch() }
                suspend fun fetchAdminBands(generation: Int) {
                    val bands = transport.fetchStudioBandsOrNull()
                    if (generation != bandsFetch.generation) return   // a newer fetch (or a re-point) owns the list
                    if (bands != null) adminBands = bandQrChoices(bands)
                    // Sabotage blankonfail (the --win09-qr smoke only): a failed refresh empties the list → the stays check reds.
                    else if (System.getProperty(WIN09_SABOTAGE_PROPERTY) == "blankonfail") adminBands = emptyList()
                }
                LaunchedEffect(serverNonce) {
                    val generation = ++bandsFetch.generation
                    bandsFetch.inFlight = null
                    adminBands = emptyList()
                    fetchAdminBands(generation)
                }
                fun bandQrItem(): (() -> Unit)? = bandQrEntry(adminBands)?.let { entry -> { bandQr = entry } }
                val refreshMenu: suspend () -> (() -> Unit)? = {
                    // Sabotage norefresh (the --win09-qr smoke only): the press never refetches → the follows-roles check reds.
                    if (System.getProperty(WIN09_SABOTAGE_PROPERTY) != "norefresh") {
                        val job = bandsFetch.inFlight?.takeIf { it.isActive }
                            ?: bandsScope.async { fetchAdminBands(++bandsFetch.generation) }.also { bandsFetch.inFlight = it }
                        withTimeoutOrNull(MENU_REFRESH_WAIT_MS) { job.await() }
                    }
                    bandQrItem()
                }
                var openPath by remember { mutableStateOf<((String) -> Unit)?>(null) }
                // Sabotage qrreplace (the --win09-qr smoke only): drop Studio while the QR shows, as an App-level screen
                // would → Back finds a new controller, and the where-it-was check reds.
                val replaceStudio = bandQr != null && System.getProperty(WIN09_SABOTAGE_PROPERTY) == "qrreplace"
                Box(Modifier.fillMaxSize()) {
                    if (!replaceStudio) androidx.compose.runtime.key(serverNonce) {
                        provider.Host(embeddedArgs(
                            initialPath = null, title = "TroubaStudio", onBack = onBack, provideRequestBack = { embeddedBack = it },
                            onServerUrlChange = { repointServer(it); serverNonce++ },
                            onShowBandQr = bandQrItem(),
                            hostOverlay = bandQr != null,
                            provideOpenPath = { openPath = it },
                            refreshMenu = refreshMenu,
                        ))
                    }
                    BackHandler { (embeddedBack ?: onBack).invoke() }
                    bandQr?.let { view ->
                        BandQrOverlay(
                            view = view,
                            onView = { bandQr = it },
                            qr = { band, back ->
                                PresentQr(transport, band.id, band.name, onManageInStudio = { bandQr = null; openPath?.invoke("/bands/${band.id}/settings") }, onBack = back)
                            },
                            backHandler = { BackHandler(it) },
                            takeWindowFocus = ::takeWindowFocus,
                        )
                    }
                }
            } else {
                StudioComingSoon(onBack)
                BackHandler(onBack)
            }
        }

        @Composable
        override fun Edit(storage: Storage, onBack: () -> Unit, initialPath: String?, bandTitle: String?) {
            val provider = embeddedStudioProvider
            if (provider != null) {
                var embeddedBack by remember { mutableStateOf<(() -> Unit)?>(null) }
                var serverNonce by remember { mutableStateOf(0) }   // S2.3b: a re-point bumps it → a fresh controller
                androidx.compose.runtime.key(serverNonce) {
                    provider.Host(embeddedArgs(initialPath = initialPath, title = bandTitle?.ifBlank { null } ?: "TroubaStudio", onBack = onBack, provideRequestBack = { embeddedBack = it }, onServerUrlChange = { repointServer(it); serverNonce++ }))
                }
                BackHandler { (embeddedBack ?: onBack).invoke() }   // S2.2: chrome Back walks history
            } else {
                error("Studio's editor is not on Windows yet (WIN05)")
            }
        }

        // WIN09 — the shared room QR. Keep-awake is Stage's display hold (WIN03/DESK01), held while the QR shows.
        @Composable
        override fun PresentQr(transport: HttpTransport, bandId: String, bandName: String, onManageInStudio: () -> Unit, onBack: () -> Unit) {
            QrPresentScreen(
                transport = transport, bandId = bandId, bandName = bandName, onManageInStudio = onManageInStudio, onBack = onBack,
                // Sabotage nokeepawake (the --win09-qr smoke only): never hold the display → check 4 reds.
                keepScreenOn = { on -> if (System.getProperty(WIN09_SABOTAGE_PROPERTY) != "nokeepawake") keepDisplayOn(on) },
            )
            BackHandler(onBack)
        }

        // S2.3: clear the WebView2 cookies for the old session. The provider (Windows) applies it on a live Browser
        // at once, or persists a pending clear under the data root that the next Studio open applies before seeding.
        override fun clearCookies(oldOrigin: String?, cookieName: String?) =
            embeddedStudioProvider?.clearCookies(File(desktopDataRoot(), "webview2"), oldOrigin, cookieName) ?: Unit
    }

    /**
     * WIN09 — bring the keyboard focus back to this window while the Studio WebView (a child window that SWT's thread
     * parents, hosted by another process) holds it. Measured on the runner: AWT still counts the frame focused, with its
     * own focus owner, so neither Compose's request nor AWT's moves anything, and a real Alt+← walked Studio's history
     * behind the QR. The native focus on the frame's own window is what moves it (Windows only; elsewhere no embedded
     * Studio exists).
     */
    private fun takeWindowFocus() {
        val w = awtWindow ?: return
        if (hostOs == HostOs.WINDOWS) nativeFocus(w)
    }

    // S2.3b: ⋮ "Server URL…" re-point. Clear the OLD origin's cookie through the NEXT-OPEN path (never the live
    // controller — both controllers share one EdgeDataDir, and clearing on the old thread could wipe the new session),
    // then save the new URL. The caller bumps the serverNonce, so a fresh controller loads the new server and applies
    // the pending clear before seeding.
    private fun repointServer(newUrl: String) {
        val edge = File(desktopDataRoot(), "webview2")
        dropSessionIfOriginChanged(storage, newUrl) { o, n -> embeddedStudioProvider?.clearCookiesOnNextOpen(edge, o, n) }
        storage.putSecret(CORE_URL_KEY, newUrl.trim())
    }

    /** WIN08 — gather what the embedded-Studio host needs (the server, the session cookie bound to that origin,
     *  the EdgeDataDir under the data root, and the window hooks). The provider builds the URL + origin from these. */
    private fun embeddedArgs(
        initialPath: String?, title: String, onBack: () -> Unit, provideRequestBack: (() -> Unit) -> Unit, onServerUrlChange: (String) -> Unit,
        onShowBandQr: (() -> Unit)? = null, hostOverlay: Boolean = false, provideOpenPath: (((String) -> Unit) -> Unit)? = null,
        refreshMenu: (suspend () -> (() -> Unit)?)? = null,
    ): EmbeddedStudioArgs {
        val serverUrl = storage.getSecret(CORE_URL_KEY)
        // S2.1 nit (Fable): build the probe's transport ONCE per screen, not once per probe call (the cover re-probes
        // on every Retry). It carries no real server default and a no-op cookie clear — it only does probePresence().
        val probeTransport = serverUrl?.let { HttpTransport(storage, defaultServerUrl = null) { _, _ -> } }
        return EmbeddedStudioArgs(
            serverUrl = serverUrl,
            initialPath = initialPath,
            title = title,
            sessionCookie = serverUrl?.let { sessionCookieFor(storage::getSecret, it) },
            edgeDataDir = File(desktopDataRoot(), "webview2"),
            onBack = onBack,
            toggleFullScreen = { window.toggleFullScreen() },
            isFullScreen = { window.fullScreen },
            frameWindow = { awtWindow },
            // S2.1 (R1): reachable iff probePresence() is not Unreachable (Online or Unauthorized both let Studio load).
            probeReachable = probeTransport?.let { t -> { t.probePresence() !is Presence.Unreachable } },
            provideRequestBack = provideRequestBack,   // S2.2: the provider hands back controller.requestBack
            onServerUrlChange = serverUrl?.let { onServerUrlChange },   // S2.3b: only offer "Server URL…" when a server is set
            // S2.4: off-origin http(s) links open in the person's default browser (a separate browser → no WebView2
            // session). true iff the OS accepted it; the controller already applied the http(s) scheme filter.
            openInDefaultBrowser = { url ->
                runCatching {
                    if (java.awt.Desktop.isDesktopSupported()) {
                        val d = java.awt.Desktop.getDesktop()
                        if (d.isSupported(java.awt.Desktop.Action.BROWSE)) { d.browse(java.net.URI(url)); true } else false
                    } else false
                }.getOrDefault(false)
            },
            onShowBandQr = onShowBandQr,         // WIN09: the ⋮ "Show band QR…" item (null hides it)
            hostOverlay = hostOverlay,           // WIN09: the picker or the QR is drawn over the embedded screen
            provideOpenPath = provideOpenPath,   // WIN09: "Manage / revoke in Studio" loads in the same Browser
            refreshMenu = refreshMenu,           // WIN09: each ⋮ press refreshes the item first (≤ 800 ms)
        )
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
            releaseUrl = updateFrom ?: UPDATE_RELEASE_URL, hookMode = updateFrom != null,
            // DESK01: Windows installs its MSI (WIN04); Linux and macOS open their download in the browser.
            assetName = updateAssetFor(hostOs),
            openInBrowser = if (hostOs == HostOs.WINDOWS) null else { url -> openUrl(url) },
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

    /** WIN08 S2.8 + A101 — every open-source text this image ships (SWT on Windows, the generated notices, the Java
     *  runtime's `legal/`); null when the image ships none, so About shows no line. */
    override val licences: ShippedLicences? by lazy { shippedLicences(resourcesDir(), System.getProperty("java.home")?.let(::File)) }
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

/** The saved server URL key (file-private per-screen, as in HttpTransport/ConnectScreen/EditScreen). */
private const val CORE_URL_KEY = "coreUrl"

internal fun isWindows(): Boolean = System.getProperty("os.name").orEmpty().startsWith("Windows", ignoreCase = true)

/**
 * T147's chrono contract: a monotonic clock that keeps counting through sleep. On Windows that is GetTickCount64
 * (documented to include sleep and hibernation; QueryPerformanceCounter, behind System.nanoTime, is not documented
 * either way). On Linux (DESK01) CLOCK_BOOTTIME: System.nanoTime there is CLOCK_MONOTONIC, which stops during a
 * suspend; on macOS CLOCK_MONOTONIC through clock_gettime_nsec_np, which counts sleep (nanoTime there does not).
 * Chosen once, so the clock never switches source mid-run; System.nanoTime only if none is available.
 */
private val monotonicMs: () -> Long by lazy {
    when {
        isWindows() -> { -> Kernel32.INSTANCE.GetTickCount64() }
        isLinux() && linuxBootTimeMs() != null -> { -> linuxBootTimeMs() ?: (System.nanoTime() / 1_000_000) }
        macMonotonicMs() != null -> { -> macMonotonicMs() ?: (System.nanoTime() / 1_000_000) }
        else -> { -> System.nanoTime() / 1_000_000 }
    }
}

private val displaySleep: DisplaySleep by lazy { displaySleepForThisOs() }

/** Stage's hold on the display (see [DisplaySleep]); never throws. */
private fun keepDisplayOn(on: Boolean) = runCatching { displaySleep.hold(on) }.let { }

/**
 * WIN09 — Windows: SetFocus on [frame]'s own HWND from the calling thread (the EDT). SetFocus only reaches a window on
 * the caller's input queue, so attach to the frame's thread for the call and detach after. Never throws.
 */
private fun nativeFocus(frame: java.awt.Window) {
    runCatching {
        val hwnd = com.sun.jna.Native.getComponentPointer(frame)?.let { com.sun.jna.platform.win32.WinDef.HWND(it) } ?: return
        val user32 = com.sun.jna.platform.win32.User32.INSTANCE
        val target = user32.GetWindowThreadProcessId(hwnd, null)
        val me = Kernel32.INSTANCE.GetCurrentThreadId()
        val dMe = com.sun.jna.platform.win32.WinDef.DWORD(me.toLong())
        val dTarget = com.sun.jna.platform.win32.WinDef.DWORD(target.toLong())
        val attached = target != me && user32.AttachThreadInput(dMe, dTarget, true)
        try { user32.SetFocus(hwnd) } finally { if (attached) user32.AttachThreadInput(dMe, dTarget, false) }
    }
}

/**
 * DESK01 — an https address in the default browser: AWT's Desktop where it can, else the system's opener. A101 —
 * Linux: always `xdg-open`, with FONTCONFIG_FILE as it was at launch (LinuxFontConfig.childEnvironment); AWT's browse
 * goes through GTK, which passes on this process's environment, our fontconfig file included. Only without xdg-open
 * does Linux fall back to AWT's. Returns the opener's process when it started one (the `--open-link` smoke waits on it).
 */
internal fun openUrl(url: String): Process? {
    if (hostOs == HostOs.LINUX && System.getenv("A101_SABOTAGE") != "fontenvleak") {
        runCatching { LinuxFontConfig.childEnvironment(ProcessBuilder("xdg-open", url)).start() }.onSuccess { return it }
    }
    val desktop = runCatching { java.awt.Desktop.getDesktop() }.getOrNull()
    return if (desktop?.isSupported(java.awt.Desktop.Action.BROWSE) == true) {
        desktop.browse(java.net.URI(url)); null
    } else {
        ProcessBuilder(if (hostOs == HostOs.MACOS) "/usr/bin/open" else "xdg-open", url).start()
    }
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
