package com.troubastack.app

import android.content.Context
import android.content.Intent
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
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.lightColorScheme
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
import com.troubastack.shared.home.HomeScreen
import com.troubastack.shared.home.downloadSummary
import com.troubastack.shared.home.inFlightStatus
import com.troubastack.shared.home.landingUpdate
import com.troubastack.shared.ui.SettingsScreen
import com.troubastack.shared.ui.ThemePref
import com.troubastack.shared.ui.TroubaTheme
import com.troubastack.shared.home.HomeState
import com.troubastack.shared.home.Identity
import com.troubastack.shared.home.UpdateStatus
import com.troubastack.shared.home.updateOutcomeStatus
import com.troubastack.shared.home.updateSummary
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.troubastack.shared.bundle.BundleImporter
import com.troubastack.shared.bundle.BundleLoader
import com.troubastack.shared.bundle.ImportResult
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.distribution.Availability
import com.troubastack.shared.distribution.BakeStatus
import com.troubastack.shared.distribution.bakeLabel
import com.troubastack.shared.distribution.bakePollStep
import com.troubastack.shared.distribution.Freeze
import com.troubastack.shared.distribution.UpdatePolicy
import com.troubastack.shared.distribution.UpdatesManager
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.stage.FitMode
import com.troubastack.shared.stage.ImageDecoder
import com.troubastack.shared.stage.LocalVolumeTurnRegistrar
import com.troubastack.shared.stage.PageTurn
import com.troubastack.shared.stage.StageColorMode
import com.troubastack.shared.stage.StageScreen
import com.troubastack.shared.stage.StageViewModel
import com.troubastack.shared.stage.resolveIdentity
import kotlinx.coroutines.Job
import kotlinx.coroutines.cancelAndJoin
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.io.File
import java.util.UUID

private const val POLICIES_KEY = "trouba.update.policies"
private const val COLOR_MODE_KEY = "stage.colorMode"
private const val FIT_MODE_KEY = "stage.fitMode" // A14: persisted reading mode (page/width/scroll)
private const val LAST_CONCERT_KEY = "home.lastConcertDir" // A27: resume-last from the Home landing
// A38: the persisted server address (written by ConnectScreen on connect; survives signOut). Its
// presence is how Home tells "Guest, server known → Sign in" apart from "nothing set up → Connect".
private const val HOME_CORE_URL_KEY = "coreUrl"

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

    // A31: bumped on every ON_RESUME so Home can re-run its live connection probe when the app comes
    // back to the foreground (not just on nav re-entry). A Compose State, so reads are tracked.
    val resumeTick = mutableStateOf(0)

    // A52: the invite link that arrived via a VIEW intent (the /join/ deep link), exposed to Compose and
    // consumed (then cleared) by the join sheet. A bearer token rides in this string, so it lives ONLY in
    // this in-memory State — never persisted. Set on cold start (onCreate) and while running (onNewIntent).
    val pendingLink = mutableStateOf<String?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        pendingLink.value = inviteLinkFrom(intent)
        setContent {
            // A36: the theme choice lives ABOVE TroubaTheme (it drives it), so hold it here and thread
            // the setter down to the Parameters screen. Persisted; defaults to SYSTEM.
            val themeStore = remember { Storage(applicationContext) }
            var themePref by remember { mutableStateOf(ThemePref.parse(themeStore.getSecret(ThemePref.KEY))) }
            TroubaTheme(dark = themePref.resolveDark()) {
                App(themePref = themePref, onThemePref = { themePref = it; themeStore.putSecret(ThemePref.KEY, it.name) })
            }
        }
    }

    // A52: with launchMode=singleTask, a /join/ deep link into the RUNNING app is delivered here (not a
    // fresh Activity). setIntent so getIntent() reflects it; publish it for the join sheet.
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        inviteLinkFrom(intent)?.let { pendingLink.value = it }
    }

    override fun onResume() {
        super.onResume()
        resumeTick.value++
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

// A52: the raw invite URL of a VIEW (deep-link) intent, or null for a plain launch. Kept as a string so
// the shared parser (A51's parseTroubaLink) — not Android's Uri — is the single grammar the app trusts.
private fun inviteLinkFrom(intent: Intent?): String? =
    intent?.takeIf { it.action == Intent.ACTION_VIEW }?.data?.toString()

private class OpenedBundle(val vm: StageViewModel, val decoder: ImageDecoder)

private data class ConcertEntry(
    val dir: String,
    val concertId: String,
    val concertRev: ULong,
    val label: String,
    val damaged: Boolean,
)

/** A31: the concert list is ONE screen, two intents. Perform is reached via TroubaStage — lean rows,
 *  tap to perform, fully offline (no server affordances). Manage is reached via TroubaStudio — adds
 *  Import / download offers / Edit / freeze·pin·delete. */
private enum class ConcertIntent { Perform, Manage }

@Composable
private fun App(themePref: ThemePref, onThemePref: (ThemePref) -> Unit) {
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

    // P205 Stage 3a: the logged-in member (id → roster auto-match on concert open; name/band → Home
    // line). A31: SET FROM THE LIVE PROBE below — a single source of truth. The old
    // LaunchedEffect(transport.isConnected) went stale when a session expired server-side while the
    // cookie lingered (isConnected never flipped false→true on reconnect, so `me` never refreshed and
    // auto-match silently used a stale/empty id). Now `me` refreshes on every Home entry + resume.
    var me by remember { mutableStateOf<CurrentIdentity?>(null) }

    // A27/A31: nav state is rememberSaveable so a landing/product screen survives rotation + process
    // death — EXCEPT `connecting`, which must NOT resurrect the login screen after a process kill
    // (A31: the diagnosed "cold-start onto Connect" glitch), so it is plain remember.
    var atHome by rememberSaveable { mutableStateOf(true) } // cold start lands on Home, not the concert list
    var selectedDir by rememberSaveable { mutableStateOf<String?>(null) }
    var editing by rememberSaveable { mutableStateOf(false) }
    // A31: the concert list is ONE screen with two intents — entered via TroubaStage (perform: lean,
    // offline, tap-to-perform) or via TroubaStudio (manage: import/update/edit affordances).
    var manageIntent by rememberSaveable { mutableStateOf(false) }
    var connecting by remember { mutableStateOf(false) }
    var settings by rememberSaveable { mutableStateOf(false) }

    // A52: the join sheet overlays ANY screen. It opens from a /join/ deep link (the activity's in-memory
    // pendingLink, set by onCreate/onNewIntent) or a link pasted into ConnectDialog (onInviteLink below).
    // Placed before the screen ladder so it isn't gated by the settings/editing early-returns; it is a
    // Dialog (its own window) so it floats over whatever screen shows. joinedTick forces Home to re-probe
    // after a join so the new band's label/concerts appear without a manual refresh.
    val mainActivity = LocalContext.current.findActivity() as? MainActivity
    var joinLink by remember { mutableStateOf<String?>(null) }
    var joinedTick by remember { mutableStateOf(0) }
    LaunchedEffect(mainActivity?.pendingLink?.value) {
        mainActivity?.pendingLink?.value?.let { joinLink = it; mainActivity.pendingLink.value = null }
    }
    joinLink?.let { link ->
        JoinDialog(
            rawLink = link, storage = storage, transport = transport,
            onClose = { joinLink = null },
            onJoined = { joinLink = null; joinedTick++ },
        )
    }

    // A53: the camera scanner is a full-screen mode. A decode hands the raw string to the join sheet (via
    // joinLink → parseTroubaLink), exactly like a paste or deep link; the scanner itself inspects nothing.
    // Cancel/deny/Back returns to the Connect modal so the paste field is one tap away (never a dead end).
    var scanning by remember { mutableStateOf(false) }
    if (scanning) {
        QrScanScreen(
            onDecoded = { joinLink = it; scanning = false },
            onClose = { scanning = false; connecting = true },
        )
        BackHandler { scanning = false; connecting = true }
        return
    }

    if (settings) {
        // A36 Parameters hub. Theme comes from the entrypoint (it drives TroubaTheme); the Stage
        // reading/colour modes are the SAME persisted keys Stage's ⚙ writes, so editing here sets the
        // default the next Stage open reads (VLL: keep them in concert mode too — both edit one value).
        var fitSel by remember { mutableStateOf(FitMode.parse(storage.getSecret(FIT_MODE_KEY))) }
        var colorSel by remember { mutableStateOf(StageColorMode.parse(storage.getSecret(COLOR_MODE_KEY))) }
        SettingsScreen(
            themePref = themePref,
            onThemePref = onThemePref,
            fitMode = fitSel,
            onFitMode = { fitSel = it; storage.putSecret(FIT_MODE_KEY, it.name) },
            colorMode = colorSel,
            onColorMode = { colorSel = it; storage.putSecret(COLOR_MODE_KEY, it.name) },
            onBack = { settings = false },
        )
        BackHandler { settings = false }
        return
    }

    if (editing) {
        // A36: the studio WebView host keeps its own look — the brand theme is for the NATIVE
        // screens only, and the embedded studio already carries the website's CSS (VLL: don't
        // touch the webview). Reset to the M3 baseline the screen was built under (a bare
        // MaterialTheme would INHERIT the brand scheme, so pass it explicitly).
        MaterialTheme(colorScheme = lightColorScheme()) { EditScreen(storage, onBack = { editing = false }) }
        return
    }
    // A38: Connect is no longer a full-screen page that replaces the view — it's a modal (ConnectDialog)
    // rendered OVER whichever screen is showing (Home / concert list), so this early return is gone.

    // A27/A31: HOME is the task root, TWO branded products. TroubaStage → the concert list in PERFORM
    // intent; TroubaStudio → the SAME list in MANAGE intent (import/update/edit). The identity card →
    // Connect. I12 intact: Home never gates Stage — perform works fully offline with no login.
    if (atHome && selectedDir == null) {
        // A39: refreshTick re-lists concerts after an Update installs, so the count/resume reflect the
        // new state without a restart.
        var refreshTick by remember { mutableStateOf(0) }
        val entries = remember(refreshTick) { listConcerts(storage).filter { !it.damaged } }
        val lastDir = remember(entries) { storage.getSecret(LAST_CONCERT_KEY)?.takeIf { d -> entries.any { it.dir == d } } }
        val scope = rememberCoroutineScope()
        // A39: the Home Update state, the offers to apply, and their names for the summary.
        var homeUpdate by remember { mutableStateOf<UpdateStatus>(UpdateStatus.Hidden) }
        var updateOffers by remember { mutableStateOf<List<Availability>>(emptyList()) }
        var updateNames by remember { mutableStateOf<List<String>>(emptyList()) }
        var updateJob by remember { mutableStateOf<Job?>(null) }
        // A42②: the Home re-bake row. canReBake gates on the signed-in user being an ADMIN of the resume
        // concert's band; homeBake is the live row; bakeJob is the running kick+poll (T103: the POST kicks
        // and the progress poll is the source of truth for the outcome).
        var canReBake by remember { mutableStateOf(false) }
        var homeBake by remember { mutableStateOf<BakeStatus>(BakeStatus.Hidden) }
        var bakeJob by remember { mutableStateOf<Job?>(null) }

        // A31: LIVE connection status — never the cached cookie flag. Probe the server on entry AND on
        // every foreground resume (resumeTick); show "Checking…" while it's in flight, then resolve to
        // Connected / Offline / Disconnected from the RESULT.
        val activity = LocalContext.current.findActivity() as? MainActivity
        var homeIdentity by remember { mutableStateOf<Identity>(Identity.Checking) }
        // A38: Retry re-runs the probe without a foreground round-trip; bump this to re-key the effect.
        var retryTick by remember { mutableStateOf(0) }
        LaunchedEffect(activity?.resumeTick?.value, retryTick, refreshTick, joinedTick) {
            // No session cookie → Guest. Split by whether a server was ever configured: known → offer
            // Sign in (address saved, password only); unknown → offer the full Connect set-up.
            if (!transport.isConnected) {
                homeIdentity = if (storage.getSecret(HOME_CORE_URL_KEY) != null) {
                    Identity.SignedOut(band = me?.band ?: "")
                } else {
                    Identity.NotSetUp
                }
                homeUpdate = UpdateStatus.Hidden // A39: no Update for Guest — Sign in is the next step
                canReBake = false // A42②: no re-bake for Guest
                return@LaunchedEffect
            }
            homeIdentity = Identity.Checking
            when (val p = transport.probePresence()) {
                is Presence.Online -> {
                    me = CurrentIdentity(userId = p.userId, displayName = p.displayName, band = p.band)
                    homeIdentity = Identity.Connected(name = p.displayName, band = p.band)
                    // A39: Recognized → is any INSTALLED concert out of date? (UpdateOffered only; a
                    // manifest that won't load is not an error — just don't nag.) Skip while an update
                    // is running OR after a failure, so a background resume doesn't stomp the in-flight
                    // row or silently erase the "couldn't update" message before the user retries.
                    if (homeUpdate !is UpdateStatus.InFlight && homeUpdate !is UpdateStatus.Failed) {
                        // A43: the landing tells the truth — the honest state is a PURE decision
                        // (landingUpdate, tested in shared). manifest == null ⇒ couldn't check ⇒ no
                        // currency claim; zero of the band's set installed ⇒ offer the download; a stale
                        // installed copy ⇒ Update (A39); otherwise the narrow "Nothing to update".
                        val manifest = runCatching { updates.fetchManifest() }.getOrNull()
                        val diffed = manifest?.let { updates.diff(it) }.orEmpty()
                        val nameOf = manifest?.concerts?.associate { it.concertId to it.name }.orEmpty()
                        val landing = landingUpdate(
                            manifestSize = manifest?.concerts?.size,
                            offered = diffed.filterIsInstance<Availability.UpdateOffered>(),
                            newlyAvailable = diffed.filterIsInstance<Availability.NewlyAvailable>(),
                        ) { id -> nameOf[id]?.takeIf { it.isNotEmpty() } ?: id }
                        updateOffers = landing.offers
                        updateNames = landing.names
                        homeUpdate = landing.status
                    }
                    // A42②: offer Re-bake only to an ADMIN of the resume concert's band (the server also
                    // 403s a non-admin — this just hides a control that would only fail). Skip while a bake
                    // is running so a background probe doesn't flip the row out from under it.
                    if (bakeJob == null) {
                        val resumeCid = entries.firstOrNull { it.dir == lastDir }?.concertId ?: ""
                        canReBake = resumeCid.isNotEmpty() &&
                            runCatching { transport.isBandAdmin(resumeCid) }.getOrDefault(false)
                    }
                }
                // Keep last-known `me` so offline auto-match still resolves the performer's own view (I12).
                Presence.Unreachable -> { homeIdentity = Identity.Offline(band = me?.band ?: ""); homeUpdate = UpdateStatus.Hidden; canReBake = false }
                // A38: expired session on a known server → Guest. KEEP the band (don't null `me`) so
                // "Sign in" reads as resuming, not starting over — matching the Unreachable branch.
                Presence.Unauthorized -> { homeIdentity = Identity.SignedOut(band = me?.band ?: ""); homeUpdate = UpdateStatus.Hidden; canReBake = false }
            }
        }

        var showDisconnect by remember { mutableStateOf(false) }
        HomeScreen(
            state = HomeState(
                lastConcertName = entries.firstOrNull { it.dir == lastDir }?.label ?: "",
                concertCount = entries.size,
                identity = homeIdentity,
                update = homeUpdate,
                canReBake = canReBake, // A42②
                bake = homeBake, // A42②
                settingsReset = Storage.settingsWereReset, // A54: one-time recovery notice after a self-heal
            ),
            onPerform = { manageIntent = false; atHome = false },
            onResume = { lastDir?.let { selectedDir = it } },
            onStudio = { manageIntent = true; atHome = false },
            // A38: the primary action routes by the current status.
            onPrimaryAction = {
                when (homeIdentity) {
                    is Identity.Connected -> showDisconnect = true      // confirm before ending a session
                    is Identity.Offline -> retryTick++                  // re-probe
                    is Identity.SignedOut, is Identity.NotSetUp -> connecting = true // Sign in / Connect
                    is Identity.Checking -> {}
                }
            },
            onManage = { connecting = true },
            onScanToJoin = { scanning = true }, // A57: direct scan entry for a Guest holding an invite
            onSettings = { settings = true },
            // A39: one tap → download+install the newer bake(s). apply() downloads to a temp then does
            // the A05 ATOMIC import, so a failure/cancel leaves the installed bundle intact (I12). On
            // completion, bump refreshTick to re-list + re-diff (→ UpToDate, or Available if any failed).
            onUpdate = {
                val offers = updateOffers
                if (offers.isNotEmpty() && updateJob == null) {
                    homeUpdate = UpdateStatus.InFlight()
                    updateJob = scope.launch {
                        // Apply each offer; COLLECT failures so we can say something (T30: never swallow
                        // a gesture silently). apply() rethrows CancellationException, so Cancel stops the
                        // loop here rather than being turned into a Failed and marching to the next offer.
                        // A42 ①: forward apply's progress into the InFlight row (download bar → install tail).
                        val failed = mutableListOf<String>()
                        offers.forEachIndexed { i, offer ->
                            if (updates.apply(offer) { p -> homeUpdate = inFlightStatus(p) } is ImportResult.Failed) {
                                failed += updateNames.getOrNull(i) ?: "a concert"
                            }
                        }
                        updateJob = null
                        // A44: the terminal status is a PURE decision (updateOutcomeStatus, tested in
                        // shared) — a success MUST be terminal, never InFlight, or the re-diff below (guarded
                        // by `homeUpdate !is InFlight`) can't clear it and the row deadlocks on "Installing…"
                        // (the A42① bug). Set it first, THEN bump refresh so the re-diff can refine UpToDate
                        // → Available if a newer rev landed. On failure refreshTick is skipped so the
                        // result-driven message survives until the user retries.
                        homeUpdate = updateOutcomeStatus(failed)
                        if (failed.isEmpty()) refreshTick++
                    }
                }
            },
            onCancelUpdate = {
                val job = updateJob
                scope.launch {
                    job?.cancelAndJoin() // AWAIT completion before cleanup, so we don't race a mid-write download
                    updateJob = null
                    // Delete ONLY this run's temps (not every *.tstage — Stage's P201 autoUpdate uses the
                    // same temp dir). The atomic importer never ran, so the installed bundle is intact.
                    runCatching {
                        val dir = storage.tempDir().trimEnd('/')
                        updateOffers.forEach { o ->
                            // A43: cover NewlyAvailable too (an empty-device download is cancellable now),
                            // not just UpdateOffered — apply() writes both to $dir/$concertId.tstage.
                            val cid = when (o) {
                                is Availability.UpdateOffered -> o.concertId
                                is Availability.NewlyAvailable -> o.concertId
                                is Availability.SongChanged -> o.concertId
                            }
                            java.io.File("$dir/$cid.tstage").delete()
                        }
                    }
                    // Restore the offer row we cancelled, matching its verb (A43: Download vs Update).
                    homeUpdate = when {
                        updateOffers.isEmpty() -> UpdateStatus.UpToDate
                        updateOffers.any { it is Availability.NewlyAvailable } ->
                            UpdateStatus.Available(downloadSummary(updateNames), action = "Download")
                        else -> UpdateStatus.Available(updateSummary(updateNames))
                    }
                }
            },
            // A42②: one-tap re-bake of the resume concert (admin only — canReBake). T103: the POST is a
            // KICK that returns 202; the PROGRESS POLL is the source of truth for the outcome. So we kick,
            // then poll bakePollStep to a terminal state and drive the row from THAT — never from the POST
            // (the old blocking shape cancelled the poller and, on T103's async server, would hide the row
            // while the bake ran). A dropped socket no longer cancels the bake (server context), so a lost
            // poll just degrades to "Baking…"; a hard backstop keeps a stuck 404 from looping forever.
            onReBake = {
                val cid = entries.firstOrNull { it.dir == lastDir }?.concertId ?: ""
                if (cid.isNotEmpty() && bakeJob == null) {
                    homeBake = BakeStatus.Baking(bakeLabel(null)) // "Baking…" until the first poll answers
                    bakeJob = scope.launch {
                        val bakeId = java.util.UUID.randomUUID().toString()
                        val kickErr = runCatching { transport.reBake(cid, bakeId) }
                            .getOrElse { "Couldn't reach the server" }
                        if (kickErr != null) {
                            homeBake = BakeStatus.Failed(kickErr)
                            bakeJob = null
                            return@launch
                        }
                        // Poll to a terminal state (bakePollStep decides on `state`, tested in shared).
                        var step = bakePollStep(null)
                        var polls = 0
                        while (!step.done && isActive && polls < 600) { // ~10 min backstop; real bakes end far sooner
                            delay(1000)
                            polls++
                            step = bakePollStep(runCatching { transport.bakeProgress(cid, bakeId) }.getOrNull())
                            homeBake = step.status
                        }
                        bakeJob = null
                        // Success (or the backstop after a kicked bake that ran server-side) ⇒ clear + re-list
                        // so the new rev shows; a failure keeps its message until the user retries.
                        if (step.status !is BakeStatus.Failed) {
                            homeBake = BakeStatus.Hidden
                            refreshTick++
                        }
                    }
                }
            },
        )
        if (showDisconnect) {
            // A38: Disconnect signs out but keeps the server address (CORE_URL_KEY survives signOut) and
            // never touches concerts (I12). Confirm first — a silent sign-out mid-gig is worse.
            AlertDialog(
                onDismissRequest = { showDisconnect = false },
                title = { Text("Disconnect?") },
                text = {
                    val ofBand = me?.band?.takeIf { it.isNotEmpty() }?.let { " of $it" } ?: ""
                    Text("You'll sign out$ofBand. Your concerts stay on this device and keep working offline.")
                },
                confirmButton = {
                    TextButton(onClick = {
                        transport.signOut()
                        homeIdentity = Identity.SignedOut(band = me?.band ?: "")
                        // Reset the connection-derived rows immediately (matching the Guest early-return),
                        // so a signed-out Guest doesn't briefly keep an admin Re-bake / Update affordance
                        // until the next probe. Found via A45's device pass; A42②/A38 didn't clear these.
                        homeUpdate = UpdateStatus.Hidden
                        canReBake = false
                        homeBake = BakeStatus.Hidden
                        showDisconnect = false
                    }) { Text("Disconnect") }
                },
                dismissButton = { TextButton(onClick = { showDisconnect = false }) { Text("Cancel") } },
            )
        }
        // A38: Connect is a MODAL overlaying Home (Home stays visible behind), not a full-screen page.
        if (connecting) {
            ConnectDialog(
                storage, transport, discovery,
                onClose = { connecting = false; retryTick++ },
                onInviteLink = { joinLink = it; connecting = false }, // A52: hand the pasted link to the join sheet
                onScan = { scanning = true; connecting = false },     // A53: open the camera scanner
            )
        }
        return
    }

    val dir = selectedDir
    if (dir == null) {
        ConcertsScreen(
            context, storage, transport, updates,
            intent = if (manageIntent) ConcertIntent.Manage else ConcertIntent.Perform,
            onOpen = { selectedDir = it; storage.putSecret(LAST_CONCERT_KEY, it) },
            onEdit = { editing = true },
            onConnect = { connecting = true },
            onHome = { atHome = true }, // A31: visible Home affordance in the top bar
        )
        // A38: Connect modal also overlays the concert list (Manage → Connect).
        if (connecting) {
            ConnectDialog(
                storage, transport, discovery,
                onClose = { connecting = false },
                onInviteLink = { joinLink = it; connecting = false }, // A52: hand the pasted link to the join sheet
                onScan = { scanning = true; connecting = false },     // A53: open the camera scanner
            )
        }
        BackHandler { atHome = true } // A27: system-back from the concert list also returns to Home
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

    // A46 (A33 drill 2): the persisted reading POSITION for this concert (logical "songId#pageInSong"), so a
    // process death / exit / A27-Resume reopens where the performer left off, not page 0. Read once at
    // open; guarded on a known concertId so a local/unknown bundle just starts at the top.
    val posKey = "stage.pos.$concertId"
    // A48: decode through the named pure seam (round-trip + malformed rejection unit-tested).
    val initialPos = remember(dir) {
        if (concertId.isEmpty()) null else decodeStagePosition(storage.getSecret(posKey))
    }

    val opened = remember(dir, identity) {
        OpenedBundle(
            // A14: seed the persisted reading mode (page/width/scroll) into the VM (A10 pattern).
            // Stage 3a-ii: seed the resolved identity — picks this member's layers + cues.
            // A46: seed the persisted reading position (resolves the logical page in the current bundle).
            StageViewModel(
                loadResult,
                identity = identity,
                initialFit = FitMode.parse(storage.getSecret(FIT_MODE_KEY)),
                initialSongId = initialPos?.first ?: "",
                initialPageInSong = initialPos?.second ?: 0,
            ),
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

    // A36: concert mode keeps its own performance look + A34's tuned amber/aqua beat — the brand
    // theme stops at Stage's door (VLL: "don't interact with the concert mode, ok as-is"). Restore
    // the M3 baseline Stage was designed and approved against (a bare MaterialTheme would INHERIT the
    // brand scheme, so pass it explicitly), so the brand palette can never re-tint the page well,
    // the chrome pills or the beat.
    MaterialTheme(colorScheme = lightColorScheme()) {
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
                    // A46 (A33 drill 2): persist the reading position per concert on every page move.
                    onPositionChange = { s, p -> if (concertId.isNotEmpty()) storage.putSecret(posKey, encodeStagePosition(s, p)) },
                )
            }
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
    intent: ConcertIntent,
    onOpen: (String) -> Unit,
    onEdit: () -> Unit,
    onConnect: () -> Unit,
    onHome: () -> Unit,
) {
    val manage = intent == ConcertIntent.Manage
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

    // Perform intent is lean + fully offline (I12): damaged concerts aren't performable, so they only
    // appear in Manage (where you can delete them).
    val entries = remember(refresh, manage) { listConcerts(storage).filter { manage || !it.damaged } }

    // Pull the manifest + recompute offers whenever connection or install state changes (I13). Manage
    // intent only — the perform path makes NO network call (offline-first entering via TroubaStage).
    LaunchedEffect(connected, refresh, manage) {
        if (!manage || !connected) { offers = emptyList(); return@LaunchedEffect }
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
            // A31: a visible Home affordance on every sub-screen (placement consistent with the Edit
            // bar's "‹ Back"), so returning to the landing page never depends on the system-back gesture.
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                TextButton(onClick = onHome) { Text("‹  Home") }
                Text(
                    if (manage) "Concerts" else "Perform",
                    style = MaterialTheme.typography.headlineMedium,
                    modifier = Modifier.weight(1f),
                )
                // Manage-only affordances (import / download offers / edit / sign-in). Perform stays lean.
                if (manage) {
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
            }
            if (manage && syncing) Text("Syncing…", style = MaterialTheme.typography.bodySmall)
            message?.let { Text(it, style = MaterialTheme.typography.bodyMedium) }

            // Offer chips (I13): download-available + update-offered, applied only on explicit tap. Manage only.
            if (manage) offers.forEach { offer -> OfferChip(offer, names, onApply = { applyOffer(offer) }) }

            if (entries.isEmpty()) {
                Text(
                    if (manage) "No concerts yet. Import a .tstage or Connect to download one."
                    else "No concerts on device yet. Open TroubaStudio to import or download one.",
                    style = MaterialTheme.typography.bodyMedium,
                )
            }
            LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                items(entries) { entry ->
                    ConcertRow(
                        entry,
                        lean = !manage, // A31: perform intent = tap-to-perform only, no manage menu
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
    lean: Boolean,
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
                lean -> {} // A31 perform intent: the whole row is tap-to-perform, no trailing controls
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
