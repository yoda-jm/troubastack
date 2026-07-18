package com.troubashare.app

import android.content.Context
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import com.troubashare.shared.home.HomeScreen
import com.troubashare.shared.home.HomeState
import com.troubashare.shared.home.Identity
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.troubashare.shared.bundle.BundleImporter
import com.troubashare.shared.bundle.BundleLoader
import com.troubashare.shared.bundle.ImportResult
import com.troubashare.shared.bundle.LoadResult
import com.troubashare.shared.distribution.Availability
import com.troubashare.shared.distribution.Freeze
import com.troubashare.shared.distribution.UpdatePolicy
import com.troubashare.shared.distribution.UpdatesManager
import com.troubashare.shared.seams.Storage
import com.troubashare.shared.stage.FitMode
import com.troubashare.shared.stage.ImageDecoder
import com.troubashare.shared.stage.LocalVolumeTurnRegistrar
import com.troubashare.shared.stage.PageTurn
import com.troubashare.shared.stage.StageColorMode
import com.troubashare.shared.stage.StageScreen
import com.troubashare.shared.stage.StageViewModel
import com.troubashare.shared.stage.resolveIdentity
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.io.File
import java.util.UUID

private const val POLICIES_KEY = "trouba.update.policies"
private const val COLOR_MODE_KEY = "stage.colorMode"
private const val FIT_MODE_KEY = "stage.fitMode" // A14: persisted reading mode (page/width/scroll)
private const val LAST_CONCERT_KEY = "home.lastConcertDir" // A27: resume-last from the Home landing

/**
 * The thin Android entrypoint (I15). Concerts list (Storage bundlesDir) + the shared [StageScreen],
 * plus B03 distribution: an optional Connect (session login) surfaces baked concerts as download
 * offers on the list; applying them goes through A05's atomic import. Stage stays offline (I12) —
 * offers live only in the list, never mid-performance.
 */
class MainActivity : ComponentActivity() {
    // Set by App() only while a Stage is open (A09) so the hardware VOLUME keys turn pages; null
    // otherwise → volume behaves normally. Volume keys don't reach Compose, so they're intercepted
    // here (keyboard/pedal keys are handled in the shared StageScreen via onPreviewKeyEvent).
    var stageVolumeTurn: ((PageTurn) -> Unit)? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent { MaterialTheme { App() } }
    }

    override fun onKeyDown(keyCode: Int, event: android.view.KeyEvent?): Boolean {
        stageVolumeTurn?.let { turn ->
            when (keyCode) {
                android.view.KeyEvent.KEYCODE_VOLUME_DOWN -> { turn(PageTurn.NEXT); return true }
                android.view.KeyEvent.KEYCODE_VOLUME_UP -> { turn(PageTurn.PREV); return true }
            }
        }
        return super.onKeyDown(keyCode, event)
    }
}

private class OpenedBundle(val vm: StageViewModel, val decoder: ImageDecoder)

private data class ConcertEntry(
    val dir: String,
    val concertId: String,
    val concertRev: ULong,
    val label: String,
    val damaged: Boolean,
)

@Composable
private fun App() {
    val context = LocalContext.current.applicationContext
    val storage = remember { Storage(context) }
    val transport = remember { HttpTransport(storage) }
    val discovery = remember { NsdServerDiscovery(context) } // B06: LAN server discovery for Connect
    val updates = remember {
        UpdatesManager(
            transport = transport,
            tempDir = { storage.tempDir() },
            installedRevs = { listConcerts(storage).filter { !it.damaged }.associate { it.concertId to it.concertRev } },
            importBundle = { BundleImporter(AndroidImportFs(storage)).import(it) },
            readPolicies = { storage.getSecret(POLICIES_KEY) },
            writePolicies = { storage.putSecret(POLICIES_KEY, it) },
        )
    }

    // P205 Stage 3a follow-up: the logged-in member (name/band → Home "Performing as …"; id →
    // roster auto-match on concert open). Best-effort, offline-first — null degrades to "Connected".
    var me by remember { mutableStateOf<CurrentIdentity?>(null) }
    LaunchedEffect(transport.isConnected) { me = if (transport.isConnected) transport.currentIdentity() else null }

    // A27: nav state is rememberSaveable so a landing/product screen survives rotation + process death.
    var atHome by rememberSaveable { mutableStateOf(true) } // cold start lands on Home, not the concert list
    var selectedDir by rememberSaveable { mutableStateOf<String?>(null) }
    var editing by rememberSaveable { mutableStateOf(false) }
    var connecting by rememberSaveable { mutableStateOf(false) }

    if (editing) {
        EditScreen(storage, onBack = { editing = false })
        return
    }
    if (connecting) {
        ConnectScreen(storage, transport, onConnected = { connecting = false }, onBack = { connecting = false }, discovery = discovery)
        return
    }

    // A27: HOME is the task root. Perform → the concerts list; Edit → the embedded Studio; Concerts →
    // the list (download/update offers ride there, B03); the identity card → Connect. I12 intact: Home
    // never gates Stage — Perform works fully offline with no login. Re-entering Home refreshes entries.
    if (atHome && selectedDir == null) {
        val entries = remember { listConcerts(storage).filter { !it.damaged } }
        val lastDir = remember(entries) { storage.getSecret(LAST_CONCERT_KEY)?.takeIf { d -> entries.any { it.dir == d } } }
        HomeScreen(
            state = HomeState(
                lastConcertName = entries.firstOrNull { it.dir == lastDir }?.label ?: "",
                concertCount = entries.size,
                // Stage 3a: "Performing as <name> · <band>" once /api/me resolves; "Connected ✓" until
                // then / if the fetch fails. The raw server host stays behind Manage (A27 ruling).
                identity = if (transport.isConnected) {
                    Identity.Connected(name = me?.displayName ?: "", band = me?.band ?: "")
                } else {
                    Identity.Disconnected
                },
            ),
            onPerform = { atHome = false },
            onResume = { lastDir?.let { selectedDir = it } },
            onEdit = { editing = true },
            onConcerts = { atHome = false },
            onIdentity = { connecting = true },
        )
        return
    }

    val dir = selectedDir
    if (dir == null) {
        ConcertsScreen(
            context, storage, transport, updates,
            onOpen = { selectedDir = it; storage.putSecret(LAST_CONCERT_KEY, it) },
            onEdit = { editing = true },
            onConnect = { connecting = true },
        )
        BackHandler { atHome = true } // A27: back from the concerts list returns to Home (the task root)
        return
    }

    // P205 Stage 3a-ii: resolve the viewer's identity for THIS concert (a local view preference, I12 —
    // no account). A stored pick still in the roster wins; else "" ⇒ the "Who are you?" picker below.
    // An old/-mine/anonymous bundle has no roster ⇒ identity stays "" and Perform is one-tap as today.
    val concertId = remember(dir) { listConcerts(storage).firstOrNull { it.dir == dir }?.concertId ?: "" }
    val loadResult = remember(dir) { BundleLoader().load(dir, FileBundleFiles()) }
    val roster = remember(loadResult) { (loadResult as? LoadResult.Loaded)?.bundle?.roster ?: emptyList() }
    val idKey = "identity.$concertId"
    // Stage 3a: a stored pick wins; else auto-match the logged-in member against the roster; else the
    // Stage shows the "Who are you?" picker (StageScreen owns it). This value SEEDS the VM; the in-Stage
    // picker/switch calls vm.setIdentity + onIdentityChange (below) to persist.
    val identity = remember(dir, me?.userId) { resolveIdentity(roster, storage.getSecret(idKey), autoUserId = me?.userId ?: "") }

    val opened = remember(dir, identity) {
        OpenedBundle(
            // A14: seed the persisted reading mode (page/width/scroll) into the VM (A10 pattern).
            // Stage 3a-ii: seed the resolved identity — picks this member's layers + cues.
            StageViewModel(loadResult, identity = identity, initialFit = FitMode.parse(storage.getSecret(FIT_MODE_KEY))),
            AndroidImageDecoder(File(dir)),
        )
    }
    // A09/A13: route hardware VOLUME keys to page turns while (and only while) Stage is open. Volume
    // keys don't reach Compose, so the Activity intercepts them (onKeyDown) and calls back through the
    // registrar StageScreen provides here. StageScreen owns the turn logic, so two-up turns by a whole
    // spread like every other input (A13) — the entrypoint just forwards the press; register/unregister
    // is handled inside StageScreen keyed on Stage lifetime.
    val activity = LocalContext.current.findActivity() as? MainActivity
    val volumeTurnRegistrar = remember(activity) {
        { handler: ((PageTurn) -> Unit)? -> activity?.stageVolumeTurn = handler }
    }

    // P201 stage 3c — rehearsal auto-update host loop. The concert is server-backed if it
    // carries a concertId (downloaded/known to a server); only then is the toggle offered.
    // While the transient toggle is on (StageState.autoUpdate), poll for a new rev of THIS
    // concert every ~15s; on one, reload the swapped-in bundle from disk and hand it to the
    // VM (applyUpdate → R10 viewport-preserving). The loop is keyed on (dir, autoUpdate), so
    // it starts/stops with the toggle and cancels when Stage is left. Best-effort — a failed
    // tick is a no-op (autoUpdateTick returns null), the current rev keeps performing.
    val stageState by opened.vm.state.collectAsState()
    LaunchedEffect(dir, stageState.autoUpdate) {
        if (!stageState.autoUpdate || concertId.isEmpty()) return@LaunchedEffect
        while (isActive) {
            delay(15_000)
            val newRev = runCatching { updates.autoUpdateTick(concertId) }.getOrNull()
            if (newRev != null) {
                opened.vm.applyUpdate(BundleLoader().load(dir, FileBundleFiles()))
            }
        }
    }

    CompositionLocalProvider(LocalVolumeTurnRegistrar provides volumeTurnRegistrar) {
        StageHost {
            StageScreen(
                opened.vm, opened.decoder, onExit = { selectedDir = null },
                initialColorMode = StageColorMode.parse(storage.getSecret(COLOR_MODE_KEY)),
                onColorModeChange = { storage.putSecret(COLOR_MODE_KEY, it.name) },
                onFitModeChange = { storage.putSecret(FIT_MODE_KEY, it.name) },
                canAutoUpdate = concertId.isNotEmpty(),
                // Stage 3a-ii: StageScreen shows the "Who are you?" picker / "Switch"; persist the pick per concert.
                onIdentityChange = { m -> storage.putSecret(idKey, m) },
            )
        }
    }
    BackHandler { selectedDir = null }
}

@Composable
private fun ConcertsScreen(
    context: Context,
    storage: Storage,
    transport: HttpTransport,
    updates: UpdatesManager,
    onOpen: (String) -> Unit,
    onEdit: () -> Unit,
    onConnect: () -> Unit,
) {
    val scope = rememberCoroutineScope()
    var refresh by remember { mutableStateOf(0) }
    var message by remember { mutableStateOf<String?>(null) }
    var connected by remember { mutableStateOf(transport.isConnected) }
    var syncing by remember { mutableStateOf(false) }
    var offers by remember { mutableStateOf<List<Availability>>(emptyList()) }
    var names by remember { mutableStateOf<Map<String, String>>(emptyMap()) }

    val picker = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri: Uri? ->
        if (uri != null) {
            val temp = copyToTemp(context, storage, uri)
            message = when (val r = BundleImporter(AndroidImportFs(storage)).import(temp)) {
                is ImportResult.Imported -> "Imported ${r.concertId}"
                is ImportResult.Failed -> r.reason
            }
            File(temp).delete()
            refresh++
        }
    }

    val entries = remember(refresh) { listConcerts(storage) }

    // Pull the manifest + recompute offers whenever connection or install state changes (I13).
    LaunchedEffect(connected, refresh) {
        if (!connected) { offers = emptyList(); return@LaunchedEffect }
        syncing = true
        try {
            val manifest = updates.fetchManifest()
            names = manifest.concerts.associate { it.concertId to it.name.ifEmpty { it.concertId } }
            offers = updates.diff(manifest)
        } catch (e: Exception) {
            message = "Couldn't reach the server"
        }
        syncing = false
    }

    fun applyOffer(offer: Availability) {
        scope.launch {
            syncing = true
            message = when (val r = updates.apply(offer)) {
                is ImportResult.Imported -> "Downloaded"
                is ImportResult.Failed -> r.reason   // old bundle left untouched (importer guarantees)
            }
            syncing = false
            refresh++
        }
    }

    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize().statusBarsPadding().padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Text("Concerts", style = MaterialTheme.typography.headlineMedium, modifier = Modifier.weight(1f))
                if (connected) {
                    TextButton(onClick = { transport.signOut(); connected = false; offers = emptyList() }) { Text("Sign out") }
                } else {
                    TextButton(onClick = onConnect) { Text("Connect") }
                }
                TextButton(onClick = onEdit) { Text("Edit") }
                // ".tstage" has no registered MIME type, so accept zip + anything and validate on import.
                Button(onClick = { picker.launch(arrayOf("application/zip", "application/octet-stream", "*/*")) }) {
                    Text("Import")
                }
            }
            if (syncing) Text("Syncing…", style = MaterialTheme.typography.bodySmall)
            message?.let { Text(it, style = MaterialTheme.typography.bodyMedium) }

            // Offer chips (I13): download-available + update-offered, applied only on explicit tap.
            offers.forEach { offer -> OfferChip(offer, names, onApply = { applyOffer(offer) }) }

            if (entries.isEmpty()) {
                Text("No concerts yet. Import a .tstage or Connect to download one.", style = MaterialTheme.typography.bodyMedium)
            }
            LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                items(entries) { entry ->
                    ConcertRow(
                        entry,
                        onOpen = { if (!entry.damaged) onOpen(entry.dir) },
                        onDelete = { File(entry.dir).deleteRecursively(); refresh++ },
                        onFreeze = { updates.setPolicy(entry.concertId, UpdatePolicy.FROZEN); refresh++ },
                        onUnfreeze = { updates.setPolicy(entry.concertId, UpdatePolicy.PROMPT); refresh++ },
                        onPin = { updates.setFreeze(entry.concertId, Freeze.LocalPin(entry.concertRev)); refresh++ },
                        onUnpin = { updates.setFreeze(entry.concertId, null); refresh++ },
                    )
                }
            }
        }
    }
}

@Composable
private fun OfferChip(offer: Availability, names: Map<String, String>, onApply: () -> Unit) {
    val (label, action) = when (offer) {
        is Availability.NewlyAvailable -> "New: ${names[offer.concertId] ?: offer.concertId}" to "Download"
        is Availability.UpdateOffered -> "${names[offer.concertId] ?: offer.concertId} — update to rev ${offer.serverRev}" to "Update"
        is Availability.SongChanged -> return
    }
    ElevatedCard(Modifier.fillMaxWidth()) {
        Row(Modifier.fillMaxWidth().padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(label, style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
            Button(onClick = onApply) { Text(action) }
        }
    }
}

@Composable
private fun ConcertRow(
    entry: ConcertEntry,
    onOpen: () -> Unit,
    onDelete: () -> Unit,
    onFreeze: () -> Unit,
    onUnfreeze: () -> Unit,
    onPin: () -> Unit,
    onUnpin: () -> Unit,
) {
    var menuOpen by remember { mutableStateOf(false) }
    Card(onClick = onOpen, modifier = Modifier.fillMaxWidth()) {
        Row(Modifier.fillMaxWidth().padding(16.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(entry.label, style = MaterialTheme.typography.titleMedium, modifier = Modifier.weight(1f))
            when {
                entry.damaged -> TextButton(onClick = onDelete) { Text("Delete") }
                else -> {
                    TextButton(onClick = { menuOpen = true }) { Text("⋮") }
                    DropdownMenu(expanded = menuOpen, onDismissRequest = { menuOpen = false }) {
                        DropdownMenuItem(text = { Text("Freeze (no updates)") }, onClick = { menuOpen = false; onFreeze() })
                        DropdownMenuItem(text = { Text("Unfreeze") }, onClick = { menuOpen = false; onUnfreeze() })
                        DropdownMenuItem(text = { Text("Pin this version") }, onClick = { menuOpen = false; onPin() })
                        DropdownMenuItem(text = { Text("Unpin") }, onClick = { menuOpen = false; onUnpin() })
                    }
                }
            }
        }
    }
}

/** List installed bundles; a directory that fails to load is shown as damaged (with a delete action). */
private fun listConcerts(storage: Storage): List<ConcertEntry> {
    val dirs = File(storage.bundlesDir()).listFiles()?.filter { it.isDirectory }?.sortedBy { it.name } ?: emptyList()
    return dirs.map { d ->
        when (val r = BundleLoader().load(d.path, FileBundleFiles())) {
            is LoadResult.Loaded -> ConcertEntry(
                d.path, r.bundle.concertId, r.bundle.concertRev,
                r.bundle.name.ifEmpty { r.bundle.concertId.ifEmpty { d.name } }, damaged = false,
            )
            is LoadResult.Failed -> ConcertEntry(d.path, "", 0uL, "Damaged bundle (${d.name})", damaged = true)
        }
    }
}

/** Copy a picked document stream into a temp `.tstage` the importer can read. */
private fun copyToTemp(context: Context, storage: Storage, uri: Uri): String {
    val temp = File(storage.tempDir(), "pick-${UUID.randomUUID()}.tstage")
    context.contentResolver.openInputStream(uri)?.use { input -> temp.outputStream().use { input.copyTo(it) } }
    return temp.path
}
