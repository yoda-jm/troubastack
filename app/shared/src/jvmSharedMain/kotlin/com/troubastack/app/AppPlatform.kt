package com.troubastack.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.State
import androidx.compose.ui.input.key.Key
import com.troubastack.shared.buildinfo.UpdateVerdict
import com.troubastack.shared.distribution.ServerDiscovery
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.stage.ImageDecoder
import com.troubastack.shared.stage.PageTurn
import com.troubastack.shared.stage.PedalLinkState
import com.troubastack.shared.stage.PointerKind
import com.troubastack.shared.stage.notes.NoteKey
import com.troubastack.shared.stage.notes.RehearsalNotes
import kotlinx.coroutines.CoroutineScope

/**
 * WIN01 — everything platform-specific the shared [App] needs, through ONE interface: app DI like [ImageDecoder],
 * not a fourth seam (I15). Android implements it in `androidApp` (AndroidAppPlatform); the Windows desktop app is
 * the second implementation (WIN03).
 *
 * A capability a platform does not have is **null**, and null HIDES its affordance (the TroubaStudio tile, "Scan a
 * QR", Import, the pedal's Connect, the update banner) — it never shows a broken one.
 */
interface AppPlatform {
    /** Seam 3 (I15): the settings and secrets store, and the bundle/notes/temp directories. */
    val storage: Storage

    /** Stage page decode for the bundle in [dir]. */
    fun imageDecoder(dir: String): ImageDecoder

    /** A70 — the rehearsal-note files under [notesDir]. */
    fun rehearsalNotes(notesDir: String): RehearsalNotesStore

    /** Import a `.tstage` through the platform's file picker. null: no Import. */
    val bundlePicker: BundlePicker?

    /** B06 — LAN server discovery, Connect's prefill. null: Connect shows no discovered servers. */
    val discovery: ServerDiscovery?

    /** A53/A57 — the camera QR scanner; a decode hands the raw string to [onDecoded]. null: no "Scan a QR". */
    val qrScanner: (@Composable (onDecoded: (String) -> Unit, onClose: () -> Unit) -> Unit)?

    /** A65 — everything that opens Studio. null: no TroubaStudio tile. */
    val studio: StudioHost?

    /** A88 — the in-app update, made once per [App] with its coroutine scope. null: no banner, no About lines. */
    val appUpdate: ((storage: Storage, scope: CoroutineScope) -> AppUpdate)?

    /** A76 — the BLE-MIDI pedal link, made once per [App]. null: no MIDI Connect (learned keys still work). */
    val pedal: (() -> PedalLink)?

    /** While performing: keep the display awake and the system chrome out of the way. */
    @Composable
    fun StageWindow(content: @Composable () -> Unit)

    /** System Back while this is composed; the handler composed last wins (A79 relies on it). */
    @Composable
    fun BackHandler(onBack: () -> Unit)

    /** T147 — a monotonic clock in ms that keeps counting through deep sleep. */
    fun monotonicNowMs(): Long

    /** T147 — the local time, written the way the device's 12/24-hour setting asks. */
    fun localTimeText(): String

    /** A31 — bumped each time the app comes back to the foreground (Home re-probes the server). */
    val resumeTicks: State<Int>

    /** A52 — an invite link that arrived from outside the app; the join sheet consumes and clears it. null: none. */
    val pendingInviteLink: MutableState<String?>?

    /** A09/A13 — hardware volume keys: Stage registers its page turn here (null to unregister). null: no such keys. */
    val volumeTurnRegistrar: ((((PageTurn) -> Unit)?) -> Unit)?

    /** A54 — the encrypted settings had to be reset this run; Home shows the one-time notice. */
    val settingsWereReset: Boolean

    /** WIN03 D2/D4 — a desktop window: its full screen, and the keys it keeps for itself. null: no window (Android). */
    val window: WindowControls?

    /** WIN03 D5 — what the performer points with: touch on Android, the mouse on the desktop. Stage names it. */
    val pointer: PointerKind

    /** WIN03 (Fable's ruling) — the server address to start from when none is saved, the platform's one source for it:
     *  Android's emulator alias for its host; null on the desktop, where Connect starts at "http://" and nothing is
     *  contacted until a server is set. */
    val defaultServerUrl: String?
}

/** WIN03 D2/D4 — the desktop window as the shared app sees it. */
interface WindowControls {
    /** The window is full screen now. Backed by Compose state, so a reader recomposes when it changes. */
    val fullScreen: Boolean

    /** F11 and Stage's full-screen button. Inside a concert it does not change [fullScreenWhenPerforming]. */
    fun toggleFullScreen()

    /** Parameters' "Full screen when performing" (default on): entering a concert goes full screen. */
    val fullScreenWhenPerforming: Boolean
    fun chooseFullScreenWhenPerforming(on: Boolean)

    /** Keys the window keeps (F11, Esc): never learnable as pedal keys (A72's isLearnableKey). */
    val reservedKeys: Set<Key>
}

/** A70/T170 — the note files as the Notes tab and the armed uploader use them: Stage's port plus the stored PNG. */
interface RehearsalNotesStore : RehearsalNotes {
    /** The note's stored PNG, or null if it cannot be read. */
    fun pngBytes(concertId: String, key: NoteKey): ByteArray?
}

/** Import a `.tstage`. */
interface BundlePicker {
    /** The launcher. [onPicked] gets a temporary copy of the chosen file, which the caller imports and deletes. */
    @Composable
    fun rememberLauncher(onPicked: (tempPath: String) -> Unit): () -> Unit
}

/** A65/A06 — Studio: the native browse, the Studio web view, and the room QR. */
interface StudioHost {
    @Composable
    fun Browse(
        transport: HttpTransport,
        onOpen: (initialPath: String, bandName: String, bandId: String) -> Unit,
        onShowQr: (bandId: String, bandName: String) -> Unit,
        onBack: () -> Unit,
    )

    @Composable
    fun Edit(storage: Storage, onBack: () -> Unit, initialPath: String?, bandTitle: String?)

    @Composable
    fun PresentQr(transport: HttpTransport, bandId: String, bandName: String, onManageInStudio: () -> Unit, onBack: () -> Unit)

    /** Drop the Studio web view's cookies (sign-out, or the server's origin changed). */
    fun clearCookies()
}

/** A88 — the in-app update: the latest release, its download and install, the once-a-day automatic check. */
interface AppUpdate {
    val verdict: UpdateVerdict?
    val failed: Boolean
    val checking: Boolean
    val progress: Float?
    val bannerText: String?
    val autoChecks: Boolean
    fun chooseAutoChecks(on: Boolean)
    fun maybeAutoCheck()
    fun check(manual: Boolean)
    fun update()
    fun notNow()
    fun cancel()

    /** WIN04 (H4) — when this copy cannot install updates (the Windows app not installed from its MSI), the one plain
     *  line Parameters shows instead of Update. null: it can (Android always can). */
    val installNote: String? get() = null

    /** The flow's questions and outcome messages, composed once at the root of [App]. */
    @Composable
    fun Dialogs()
}

/** A76 — the BLE-MIDI pedal link. */
interface PedalLink {
    /** The link's state changes and each press (MIDI status byte, first data byte). */
    fun setListeners(onStateChange: (PedalLinkState) -> Unit, onPress: (status: Int, data1: Int) -> Unit)

    /** Find and open the pedal; idempotent. Assumes the platform's permissions are granted. */
    fun connect()
    fun close()

    /** Parameters' explicit Connect: asks for the platform's permissions first when needed, then [connect]s. */
    @Composable
    fun rememberConnect(): () -> Unit
}
