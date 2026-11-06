package com.troubastack.app

import android.content.Context
import android.net.Uri
import android.os.Build
import android.os.SystemClock
import android.view.WindowManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.State
import com.troubastack.shared.buildinfo.UpdatePhase
import com.troubastack.shared.buildinfo.UpdateVerdict
import com.troubastack.shared.distribution.ServerDiscovery
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.stage.ImageDecoder
import com.troubastack.shared.stage.PageTurn
import com.troubastack.shared.stage.PedalLinkState
import com.troubastack.shared.stage.PointerKind
import kotlinx.coroutines.CoroutineScope
import java.io.File
import java.util.UUID
import androidx.activity.compose.BackHandler as ActivityBackHandler

// ".tstage" has no registered MIME type, so accept zip + anything and validate on import.
private val TSTAGE_PICKER_TYPES = arrayOf("application/zip", "application/octet-stream", "*/*")

/**
 * WIN01 — Android's [AppPlatform]: every capability is present, and each one is the code the shared App() called
 * directly before the hoist. [MainActivity] remembers one per composition, the lifetime App()'s own `remember`s have.
 */
class AndroidAppPlatform(private val activity: MainActivity) : AppPlatform {
    private val context: Context = activity.applicationContext

    // Lazy: opened at App()'s first composition, after the theme's own Storage, as before the hoist (A54's self-heal).
    override val storage: Storage by lazy { Storage(context) }

    override fun imageDecoder(dir: String): ImageDecoder = AndroidImageDecoder(File(dir))

    override fun rehearsalNotes(notesDir: String): RehearsalNotesStore = FileRehearsalNotes(notesDir, AndroidNoteBitmapCodec)

    override val bundlePicker: BundlePicker = object : BundlePicker {
        @Composable
        override fun rememberLauncher(onPicked: (tempPath: String) -> Unit): () -> Unit {
            val picker = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri: Uri? ->
                if (uri != null) onPicked(copyToTemp(context, storage, uri))
            }
            return { picker.launch(TSTAGE_PICKER_TYPES) }
        }
    }

    override val discovery: ServerDiscovery = NsdServerDiscovery(context) // B06: LAN server discovery for Connect

    override val qrScanner: @Composable (onDecoded: (String) -> Unit, onClose: () -> Unit) -> Unit =
        { onDecoded, onClose -> QrScanScreen(onDecoded = onDecoded, onClose = onClose) }

    override val studio: StudioHost = object : StudioHost {
        @Composable
        override fun Browse(
            transport: HttpTransport,
            onOpen: (initialPath: String, bandName: String, bandId: String) -> Unit,
            onShowQr: (bandId: String, bandName: String) -> Unit,
            onBack: () -> Unit,
        ) {
            StudioBrowseScreen(transport = transport, onOpen = onOpen, onShowQr = onShowQr, onBack = onBack)
        }

        @Composable
        override fun Edit(storage: Storage, onBack: () -> Unit, initialPath: String?, bandTitle: String?) {
            EditScreen(storage, onBack = onBack, initialPath = initialPath, bandTitle = bandTitle)
        }

        @Composable
        override fun PresentQr(transport: HttpTransport, bandId: String, bandName: String, onManageInStudio: () -> Unit, onBack: () -> Unit) {
            // WIN09: the screen is shared; Android keeps the screen on with the window flag, as A65 did.
            val window = activity.window
            QrPresentScreen(
                transport = transport, bandId = bandId, bandName = bandName, onManageInStudio = onManageInStudio, onBack = onBack,
                keepScreenOn = { on ->
                    if (on) window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
                    else window.clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
                },
            )
            ActivityBackHandler { onBack() }
        }

        override fun clearCookies(oldOrigin: String?, cookieName: String?) = clearWebViewCookies()  // S2.3: Android drops all cookies; args ignored
    }

    override val appUpdate: (Storage, CoroutineScope) -> AppUpdate =
        { storage, scope ->
            // A96: Android's "should we explain first" answer needs the Activity (false after a refusal for good).
            val rationale = { Build.VERSION.SDK_INT >= 33 && activity.shouldShowRequestPermissionRationale(android.Manifest.permission.POST_NOTIFICATIONS) }
            AndroidAppUpdate(AppUpdateController(context, storage, scope, rationale))
        }

    override val pedal: () -> PedalLink = { AndroidPedalLink(MidiPedal(context)) }

    @Composable
    override fun StageWindow(content: @Composable () -> Unit) = StageHost(content)

    @Composable
    override fun BackHandler(onBack: () -> Unit) = ActivityBackHandler(onBack = onBack)

    override fun monotonicNowMs(): Long = SystemClock.elapsedRealtime()

    override fun localTimeText(): String = android.text.format.DateFormat.getTimeFormat(activity).format(java.util.Date())

    override val resumeTicks: State<Int> get() = activity.resumeTick

    override val pendingInviteLink: MutableState<String?> get() = activity.pendingLink

    override val volumeTurnRegistrar: (((PageTurn) -> Unit)?) -> Unit = { handler -> activity.stageVolumeTurn = handler }

    override val settingsWereReset: Boolean get() = Storage.settingsWereReset

    override val window: WindowControls? = null // one full-screen activity: no window modes

    override val pointer: PointerKind = PointerKind.TOUCH

    override val defaultServerUrl: String = DEFAULT_SERVER_URL

    companion object {
        /** The host machine, seen from the Android emulator: Android's one source for the starting server address
         *  (Connect and the Studio editor). On a real tablet LAN discovery fills the field instead. */
        const val DEFAULT_SERVER_URL = "http://10.0.2.2:8080"
    }
}

/** Copy a picked document stream into a temp `.tstage` the importer can read. */
private fun copyToTemp(context: Context, storage: Storage, uri: Uri): String {
    val temp = File(storage.tempDir(), "pick-${UUID.randomUUID()}.tstage")
    context.contentResolver.openInputStream(uri)?.use { input -> temp.outputStream().use { input.copyTo(it) } }
    return temp.path
}

/** A88 — [AppUpdateController] as the shared [AppUpdate]. */
private class AndroidAppUpdate(private val c: AppUpdateController) : AppUpdate {
    override val verdict: UpdateVerdict? get() = c.verdict
    override val failed: Boolean get() = c.failed
    override val checking: Boolean get() = c.checking
    override val phase: UpdatePhase? get() = c.phase
    override val bannerText: String? get() = c.bannerText
    override val autoChecks: Boolean get() = c.autoChecks
    override fun chooseAutoChecks(on: Boolean) = c.chooseAutoChecks(on)
    override fun maybeAutoCheck() = c.maybeAutoCheck()
    override fun check(manual: Boolean) = c.check(manual)
    override fun update() = c.update()
    override fun notNow() = c.notNow()
    override fun cancel() = c.cancel()

    @Composable
    override fun Dialogs() = AppUpdateDialogs(c)
}

/** A76 — [MidiPedal] as the shared [PedalLink]. */
private class AndroidPedalLink(private val p: MidiPedal) : PedalLink {
    override fun setListeners(onStateChange: (PedalLinkState) -> Unit, onPress: (status: Int, data1: Int) -> Unit) =
        p.setListeners(
            onStateChange = { s ->
                onStateChange(
                    when (s) {
                        MidiPedal.State.CONNECTED -> PedalLinkState.CONNECTED
                        MidiPedal.State.SCANNING -> PedalLinkState.SCANNING
                        MidiPedal.State.NOT_CONNECTED -> PedalLinkState.NOT_CONNECTED
                    },
                )
            },
            onPressReceived = { press -> onPress(press.status, press.data1) },
        )

    override fun connect() = p.connect()
    override fun close() = p.close()

    // A76 ⟨D1⟩ — explicit Connect: request the BLE permissions (Android 12+) then scan+connect; already
    // granted → connect straight away. Reconnect on Stage entry is automatic once connected once (App).
    @Composable
    override fun rememberConnect(): () -> Unit {
        val perms = rememberLauncherForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { result ->
            if (result.values.all { it }) p.connect()
        }
        return {
            if (p.hasPermissions()) p.connect()
            else perms.launch(arrayOf(android.Manifest.permission.BLUETOOTH_SCAN, android.Manifest.permission.BLUETOOTH_CONNECT))
        }
    }
}
