package com.troubastack.app

import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import com.troubastack.shared.home.HomeScreen
import com.troubastack.shared.ui.SettingsScreen
import com.troubastack.shared.ui.ThemePref
import com.troubastack.shared.home.HomeState
import com.troubastack.shared.home.Identity
import com.troubastack.shared.home.UpdateStatus
import androidx.compose.runtime.setValue
import com.troubastack.shared.bundle.BundleImporter
import com.troubastack.shared.bundle.BundleLoader
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.distribution.ServerDiscovery
import com.troubastack.shared.distribution.UpdatesManager
import com.troubastack.shared.stage.Chrono
import com.troubastack.shared.stage.ClockStyle
import com.troubastack.shared.stage.FitMode
import com.troubastack.shared.stage.ImageDecoder
import com.troubastack.shared.stage.decodeChrono
import com.troubastack.shared.stage.encodeChrono
import com.troubastack.shared.stage.LocalVolumeTurnRegistrar
import com.troubastack.shared.stage.StageColorMode
import com.troubastack.shared.stage.StageScreen
import com.troubastack.shared.stage.StageViewModel
import com.troubastack.shared.stage.resolveIdentity
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.isActive

private const val POLICIES_KEY = "trouba.update.policies"
private const val COLOR_MODE_KEY = "stage.colorMode"
private const val FIT_MODE_KEY = "stage.fitMode" // A14: persisted reading mode (page/width/scroll)
private const val PEDAL_BINDINGS_KEY = "stage.pedalBindings" // A72: device-local learned pedal key codes
private const val STAGE_CLOCK_KEY = "stage.clockVisible" // T147: persisted bottom-right clock preference
private const val STAGE_CLOCK_STYLE_KEY = "stage.clockStyle" // T147: persisted analog/digital clock face
private const val NOTE_TOOL_KEY = "stage.note.tool" // A70: last note tool/width/colour, per device
private const val NOTE_WIDTH_KEY = "stage.note.width"
private const val NOTE_COLOUR_KEY = "stage.note.colour"
private const val LAST_CONCERT_KEY = "home.lastConcertDir" // A27: resume-last from the Home landing
// A38: the persisted server address (written by ConnectScreen on connect; survives signOut). Its
// presence is how Home tells "Guest, server known → Sign in" apart from "nothing set up → Connect".
private const val HOME_CORE_URL_KEY = "coreUrl"

private class OpenedBundle(val vm: StageViewModel, val decoder: ImageDecoder)

internal data class ConcertEntry(
    val dir: String,
    val concertId: String,
    val concertRev: ULong,
    val bakedAt: Long, // T143: epoch seconds — distinguishes two same-named bakes on the row
    val label: String,
    val bandId: String, // T143: grouping key for the by-band accordion
    val bandName: String, // T143: group label ("" for pre-T143 bundles → "Unknown band")
    val damaged: Boolean,
    val songs: List<com.troubastack.shared.distribution.LocalSong> = emptyList(), // A91: "what changed" vs the server
)

// A91: the Perform/Manage ConcertIntent split is retired — there is ONE concert list, in TroubaStage; admin items in a
// row's ⋯ are gated by the band role, not by which tile you came from.

/**
 * The app (WIN01: shared by the JVM platforms; everything platform-specific comes from [platform], and a null
 * capability hides its affordance). The concerts list (Storage bundlesDir) + the shared [StageScreen], plus B03
 * distribution: an optional Connect (session login) surfaces baked concerts as download offers on the list; applying
 * them goes through A05's atomic import. Stage stays offline (I12) — offers live only in the list, never
 * mid-performance.
 */
@Composable
fun App(platform: AppPlatform, themePref: ThemePref, onThemePref: (ThemePref) -> Unit) {
    val storage = platform.storage
    // The Studio web view's cookie jar is dropped with the session (sign-out, a new server origin).
    val transport = remember { HttpTransport(storage, defaultServerUrl = platform.defaultServerUrl, clearWebCookies = { platform.studio?.clearCookies() }) }
    // B06: LAN server discovery for Connect; without it Connect simply lists no servers.
    val discovery = remember { platform.discovery ?: ServerDiscovery { flowOf(emptyList()) } }
    val studio = platform.studio
    val qrScanner = platform.qrScanner
    // A88 — the in-app update from the GitHub `latest` release (one controller for Home's banner and About).
    val appScope = rememberCoroutineScope()
    val appUpdate = platform.appUpdate?.let { make -> remember { make(storage, appScope) } }
    appUpdate?.Dialogs()
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

    // A76 — the BLE-MIDI pedal, app-scoped so Parameters (learn a button) and Stage (turn pages) share ONE
    // link. Each press becomes a MidiSignal both screens observe: Parameters shows it (⟨D1⟩ diagnostic) and
    // learns it if armed; Stage matches it to a page turn. The link State drives the panel's diagnostic.
    val midiPedal = platform.pedal?.let { open -> remember { open() } }
    var midiState by remember { mutableStateOf(com.troubastack.shared.stage.PedalLinkState.NOT_CONNECTED) }
    var midiSignal by remember { mutableStateOf<com.troubastack.shared.stage.MidiSignal?>(null) }
    var midiSeq by remember { mutableStateOf(0L) }
    var pedalEverConnected by remember { mutableStateOf(false) } // A76 ⟨D1⟩: gates auto-reconnect on Stage entry
    DisposableEffect(midiPedal) {
        midiPedal?.setListeners(
            onStateChange = { s ->
                if (s == com.troubastack.shared.stage.PedalLinkState.CONNECTED) pedalEverConnected = true
                midiState = s
            },
            onPress = { status, data1 ->
                midiSeq += 1
                midiSignal = com.troubastack.shared.stage.MidiSignal(
                    midiSeq,
                    com.troubastack.shared.stage.midiToken(status, data1),
                    com.troubastack.shared.stage.midiLabel(status, data1),
                )
            },
        )
        onDispose { midiPedal?.close() }
    }
    // A76 ⟨D1⟩ — explicit Connect (the platform asks for its permissions first). Reconnect on Stage entry is
    // automatic once connected once (below).
    val connectPedal: (() -> Unit)? = midiPedal?.rememberConnect()

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
    // A76 ⟨D1⟩ — the pedal drops the link aggressively, so reconnect on entering a concert (Stage) — but only
    // after a first successful connect, so a stand with no pedal never scans on open. connect() is idempotent.
    LaunchedEffect(selectedDir) { if (selectedDir != null && pedalEverConnected) midiPedal?.connect() }
    var editing by rememberSaveable { mutableStateOf(false) }
    // A31: the concert list is ONE screen with two intents — entered via TroubaStage (perform: lean,
    // offline, tap-to-perform) or via TroubaStudio (manage: import/update/edit affordances).
    var connecting by remember { mutableStateOf(false) }
    var settings by rememberSaveable { mutableStateOf(false) }
    var showEdited by rememberSaveable { mutableStateOf(false) } // A92: the "edited since their last bake" list
    // A65: the native Studio browse (Concerts | Bands launchers) is the TroubaStudio entry point; a row
    // taps into the Studio WebView at a deep-link path, with the band name for the frame title.
    var studioBrowse by rememberSaveable { mutableStateOf(false) }
    var editInitialPath by rememberSaveable { mutableStateOf<String?>(null) }
    var editTitle by rememberSaveable { mutableStateOf<String?>(null) }
    var editBandId by rememberSaveable { mutableStateOf<String?>(null) }
    var showingQr by rememberSaveable { mutableStateOf(false) }

    // A52: the join sheet overlays ANY screen. It opens from a /join/ deep link (the platform's in-memory
    // pendingInviteLink; on Android set by onCreate/onNewIntent) or a link pasted into ConnectDialog (onInviteLink
    // below). Placed before the screen ladder so it isn't gated by the settings/editing early-returns; it is a
    // Dialog (its own window) so it floats over whatever screen shows. joinedTick forces Home to re-probe
    // after a join so the new band's label/concerts appear without a manual refresh.
    val pendingLink = platform.pendingInviteLink
    var joinLink by remember { mutableStateOf<String?>(null) }
    var joinedTick by remember { mutableStateOf(0) }
    LaunchedEffect(pendingLink?.value) {
        pendingLink?.value?.let { joinLink = it; pendingLink.value = null }
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
    if (scanning && qrScanner != null) {
        qrScanner(
            { joinLink = it; scanning = false }, // onDecoded
            { scanning = false; connecting = true }, // onClose
        )
        platform.BackHandler { scanning = false; connecting = true }
        return
    }

    if (settings) {
        // A36 Parameters hub. Theme comes from the entrypoint (it drives TroubaTheme); the Stage
        // reading/colour modes are the SAME persisted keys Stage's ⚙ writes, so editing here sets the
        // default the next Stage open reads (VLL: keep them in concert mode too — both edit one value).
        var fitSel by remember { mutableStateOf(FitMode.parse(storage.getSecret(FIT_MODE_KEY))) }
        var colorSel by remember { mutableStateOf(StageColorMode.parse(storage.getSecret(COLOR_MODE_KEY))) }
        var pedalSel by remember { mutableStateOf(com.troubastack.shared.stage.parsePedalBindings(storage.getSecret(PEDAL_BINDINGS_KEY))) }
        // A84 ⟨D2⟩ — the joined server (the app joins one server at a time: coreUrl) and its /api/version, fetched
        // each time the screen opens; unreachable is a line, not a dialog.
        val joinedServer = storage.getSecret(HOME_CORE_URL_KEY)?.trim()?.takeIf { it.isNotEmpty() }
        var joinedVersion by remember { mutableStateOf<com.troubastack.shared.buildinfo.ServerVersion>(com.troubastack.shared.buildinfo.ServerVersion.Checking) }
        LaunchedEffect(joinedServer) {
            if (joinedServer != null) {
                joinedVersion = transport.serverVersion(joinedServer)?.let { com.troubastack.shared.buildinfo.ServerVersion.Known(it) }
                    ?: com.troubastack.shared.buildinfo.ServerVersion.Unreachable
            }
        }
        SettingsScreen(
            themePref = themePref,
            onThemePref = onThemePref,
            fitMode = fitSel,
            onFitMode = { fitSel = it; storage.putSecret(FIT_MODE_KEY, it.name) },
            colorMode = colorSel,
            onColorMode = { colorSel = it; storage.putSecret(COLOR_MODE_KEY, it.name) },
            pedalBindings = pedalSel,
            onLearnPedal = { action, token ->
                val r = com.troubastack.shared.stage.learnPedalBinding(pedalSel, action, token)
                pedalSel = r.bindings
                storage.putSecret(PEDAL_BINDINGS_KEY, com.troubastack.shared.stage.encodePedalBindings(r.bindings))
                r.takenFrom
            },
            onForgetPedals = { pedalSel = emptyMap(); storage.putSecret(PEDAL_BINDINGS_KEY, "") },
            midiState = midiState,
            onConnectPedal = connectPedal,
            midiSignal = midiSignal,
            servers = joinedServer?.let { listOf(it to joinedVersion) } ?: emptyList(),
            // A88 — the latest release, a by-hand check (works on a dev build too), Update, and the Automatic switch.
            // No update capability ⇒ onCheckForUpdates null ⇒ Parameters shows none of it.
            latestLine = appUpdate?.let { com.troubastack.shared.buildinfo.latestLine(it.verdict, it.failed) } ?: "",
            checkingForUpdates = appUpdate?.checking ?: false,
            onCheckForUpdates = appUpdate?.let { u -> { u.check(manual = true) } },
            onUpdateApp = when (appUpdate?.verdict) {
                is com.troubastack.shared.buildinfo.UpdateVerdict.Available, is com.troubastack.shared.buildinfo.UpdateVerdict.Dismissed -> ({ appUpdate?.update() })
                else -> null
            },
            autoChecks = appUpdate?.autoChecks ?: false,
            onAutoChecks = { appUpdate?.chooseAutoChecks(it) },
            // WIN03 — a desktop window: its reserved keys, and "Full screen when performing" (null hides it).
            reservedKeys = platform.window?.reservedKeys ?: emptySet(),
            fullScreenWhenPerforming = platform.window?.fullScreenWhenPerforming,
            onFullScreenWhenPerforming = { platform.window?.chooseFullScreenWhenPerforming(it) },
            onBack = { settings = false },
        )
        platform.BackHandler { settings = false }
        return
    }

    // A65: the room QR present-view — checked BEFORE EditScreen so it overlays it; Back returns to Edit.
    if (showingQr && editBandId != null && studio != null) {
        MaterialTheme(colorScheme = lightColorScheme()) {
            studio.PresentQr(
                transport = transport,
                bandId = editBandId!!,
                bandName = editTitle ?: "",
                // Revoke is destructive → open Studio's InviteLinks (band settings), never native.
                onManageInStudio = { showingQr = false; editInitialPath = "/bands/$editBandId/settings"; editing = true },
                onBack = { showingQr = false },
            )
        }
        return
    }

    if (editing && studio != null) {
        // A36: the studio WebView host keeps its own look — the brand theme is for the NATIVE
        // screens only, and the embedded studio already carries the website's CSS (VLL: don't
        // touch the webview). Reset to the M3 baseline the screen was built under (a bare
        // MaterialTheme would INHERIT the brand scheme, so pass it explicitly).
        MaterialTheme(colorScheme = lightColorScheme()) {
            studio.Edit(storage, onBack = { editing = false }, initialPath = editInitialPath, bandTitle = editTitle)
        }
        return
    }

    // A65: the TroubaStudio browse — two native launcher lists; a row opens the Studio WebView at its
    // deep-link path (checked AFTER `editing` so a tapped row shows EditScreen, and Back returns here).
    if (studioBrowse && studio != null) {
        MaterialTheme(colorScheme = lightColorScheme()) {
            studio.Browse(
                transport = transport,
                onOpen = { path, band, bandId -> editInitialPath = path; editTitle = band; editBandId = bandId; editing = true },
                onShowQr = { bandId, band -> editBandId = bandId; editTitle = band; showingQr = true },
                onBack = { studioBrowse = false; atHome = true },
            )
        }
        return
    }
    // A38: Connect is no longer a full-screen page that replaces the view — it's a modal (ConnectDialog)
    // rendered OVER whichever screen is showing (Home / concert list), so this early return is gone.

    // A27/A31: HOME is the task root, TWO branded products. TroubaStage → the concert list in PERFORM
    // intent; TroubaStudio → the SAME list in MANAGE intent (import/update/edit). The identity card →
    // Connect. I12 intact: Home never gates Stage — perform works fully offline with no login.
    if (showEdited) {
        EditedConcertsScreen(transport, onBack = { showEdited = false })
        platform.BackHandler { showEdited = false }
        return
    }
    if (atHome && selectedDir == null) {
        // A88: Home showing ⇒ not in a concert — the once-a-day automatic check may run (a "dev" build never does).
        LaunchedEffect(Unit) { appUpdate?.maybeAutoCheck() }
        // A39: refreshTick re-lists concerts after an Update installs, so the count/resume reflect the
        // new state without a restart.
        var refreshTick by remember { mutableStateOf(0) }
        val entries = remember(refreshTick) { listConcerts(storage).filter { !it.damaged } }
        val lastDir = remember(entries) { storage.getSecret(LAST_CONCERT_KEY)?.takeIf { d -> entries.any { it.dir == d } } }
        val scope = rememberCoroutineScope()
        // A92 — how many concerts the server reports as edited since their last bake (admins only; 0 ⇒ no Home line).
        var editedCount by remember { mutableStateOf(0) }
        // A91 (Fable, Q1): Home's A39 "Update / Nothing to update" row is retired — concert updates live in the ONE
        // concert list (TroubaStage), written on each row; Home no longer announces them.
        // A91 (Fable GO, binding): Home's A42② always-on "Re-bake «last opened»" row is removed — it showed "even when
        // nothing seems to have changed" (VLL). Re-bake lives in each concert's ⋯ in the TroubaStage list; the honest
        // replacement ("N concerts edited since their last bake →") is A92, after T201's server signal.

        // A31: LIVE connection status — never the cached cookie flag. Probe the server on entry AND on
        // every foreground resume (resumeTicks); show "Checking…" while it's in flight, then resolve to
        // Connected / Offline / Disconnected from the RESULT.
        var homeIdentity by remember { mutableStateOf<Identity>(Identity.Checking) }
        // A38: Retry re-runs the probe without a foreground round-trip; bump this to re-key the effect.
        var retryTick by remember { mutableStateOf(0) }
        LaunchedEffect(platform.resumeTicks.value, retryTick, refreshTick, joinedTick) {
            // No session cookie → Guest. Split by whether a server was ever configured: known → offer
            // Sign in (address saved, password only); unknown → offer the full Connect set-up.
            if (!transport.isConnected) {
                homeIdentity = if (storage.getSecret(HOME_CORE_URL_KEY) != null) {
                    Identity.SignedOut(band = me?.band ?: "")
                } else {
                    Identity.NotSetUp
                }
                return@LaunchedEffect
            }
            homeIdentity = Identity.Checking
            when (val p = transport.probePresence()) {
                is Presence.Online -> {
                    me = CurrentIdentity(userId = p.userId, displayName = p.displayName, band = p.band)
                    homeIdentity = Identity.Connected(name = p.displayName, band = p.band)
                    editedCount = fetchServerConcertsWithEdits(transport)?.let { com.troubastack.shared.distribution.editedConcerts(it).size } ?: 0
                }
                // Keep last-known `me` so offline auto-match still resolves the performer's own view (I12).
                Presence.Unreachable -> { homeIdentity = Identity.Offline(band = me?.band ?: ""); editedCount = 0 }
                // A38: expired session on a known server → Guest. KEEP the band (don't null `me`) so
                // "Sign in" reads as resuming, not starting over — matching the Unreachable branch.
                Presence.Unauthorized -> { homeIdentity = Identity.SignedOut(band = me?.band ?: ""); editedCount = 0 }
            }
        }

        var showDisconnect by remember { mutableStateOf(false) }
        HomeScreen(
            state = HomeState(
                lastConcertName = entries.firstOrNull { it.dir == lastDir }?.label ?: "",
                concertCount = entries.size,
                identity = homeIdentity,
                update = UpdateStatus.Hidden, // A91: retired (see above)
                settingsReset = platform.settingsWereReset, // A54: one-time recovery notice after a self-heal
            ),
            editedLine = com.troubastack.shared.distribution.editedHomeLine(editedCount), // A92
            onEdited = { showEdited = true },
            appUpdateText = appUpdate?.bannerText, // A88
            appUpdateProgress = appUpdate?.progress,
            onAppUpdate = { appUpdate?.update() },
            onAppUpdateNotNow = { appUpdate?.notNow() },
            onAppUpdateCancel = { appUpdate?.cancel() },
            onPerform = { atHome = false },
            onResume = { lastDir?.let { selectedDir = it } },
            onStudio = if (studio != null) ({ studioBrowse = true; atHome = false }) else null,
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
            onScanToJoin = if (qrScanner != null) ({ scanning = true }) else null, // A57: direct scan entry for a Guest holding an invite
            onSettings = { settings = true },
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
                onScan = if (qrScanner != null) ({ scanning = true; connecting = false }) else null, // A53: open the camera scanner
            )
        }
        return
    }

    val dir = selectedDir
    if (dir == null) {
        ConcertsScreen(
            platform, storage, transport, updates,
            onOpen = { selectedDir = it; storage.putSecret(LAST_CONCERT_KEY, it) },
            onHome = { atHome = true }, // A31: visible Home affordance in the top bar
        )
        // A38: Connect modal also overlays the concert list (Manage → Connect).
        if (connecting) {
            ConnectDialog(
                storage, transport, discovery,
                onClose = { connecting = false },
                onInviteLink = { joinLink = it; connecting = false }, // A52: hand the pasted link to the join sheet
                onScan = if (qrScanner != null) ({ scanning = true; connecting = false }) else null, // A53: open the camera scanner
            )
        }
        platform.BackHandler { atHome = true } // A27: system-back from the concert list also returns to Home
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

    // T147: restore the chronometer across process death (per concert) and the clock preference (global).
    val chronoKey = "stage.chrono.$concertId"
    // T147 restore: decode with BOTH clocks so a running chrono re-anchors to this boot's monotonic base
    // (wall-clock start survives reboot; never the ~16h garbage a stale monotonic instant produced).
    val initialChrono = remember(dir) {
        if (concertId.isEmpty()) Chrono()
        else decodeChrono(storage.getSecret(chronoKey), platform.monotonicNowMs(), System.currentTimeMillis())
    }

    // A70 — the rehearsal-notes host port (bitmaps beside bundles/), and the concert's note index seeded
    // into the VM so counts/badges are right from the first frame.
    val notesPort = remember { platform.rehearsalNotes(storage.notesDir()) }
    val opened = remember(dir, identity) {
        OpenedBundle(
            // A14: seed the persisted reading mode (page/width/scroll) into the VM (A10 pattern).
            // Stage 3a-ii: seed the resolved identity — picks this member's layers + cues.
            // A46: seed the persisted reading position (resolves the logical page in the current bundle).
            StageViewModel(
                loadResult,
                identity = identity,
                initialFit = FitMode.parse(storage.getSecret(FIT_MODE_KEY)),
                initialSongId = initialPos?.first, // null: nothing saved, the top (A94: "" is an intermission card)
                initialPageInSong = initialPos?.second ?: 0,
                // T147: a MONOTONIC source that advances through deep sleep — a paused chrono then can't
                // drift and a running one counts the sleep, both correct (Android: elapsedRealtime, not wall time).
                monotonicNow = { platform.monotonicNowMs() },
                initialChrono = initialChrono,
                initialClockVisible = storage.getSecret(STAGE_CLOCK_KEY) == "true",
                initialClockStyle = ClockStyle.parse(storage.getSecret(STAGE_CLOCK_STYLE_KEY)), // T157: restore by name (BOTH survives)
                initialNotes = if (concertId.isEmpty()) emptyList() else notesPort.index(concertId),
                initialNoteTool = if (storage.getSecret(NOTE_TOOL_KEY) == "ERASER") com.troubastack.shared.stage.notes.NoteTool.ERASER else com.troubastack.shared.stage.notes.NoteTool.PENCIL,
                initialNoteWidth = storage.getSecret(NOTE_WIDTH_KEY)?.toIntOrNull() ?: com.troubastack.shared.stage.notes.NoteTools.FINE,
                initialNoteColour = storage.getSecret(NOTE_COLOUR_KEY)?.toLongOrNull() ?: com.troubastack.shared.stage.notes.NoteTools.BLACK,
            ),
            platform.imageDecoder(dir),
        )
    }
    // A77 — the armed auto-upload loop. Host owns the WHEN (debounce + flush); the "whether" (overwrite, the
    // 409 verdict) is the shared, tested policy. Rebuilt with the VM (per dir/identity). onDisarm fires the
    // flush at every disarm/expiry/bake transition — the transition itself, not a diffed state edge (Fable).
    val armedScope = rememberCoroutineScope()
    val armedUploader = remember(opened, concertId) {
        ArmedUploader(
            scope = armedScope, transport = transport, notes = notesPort, concertId = concertId,
            isArmed = { opened.vm.isArmed() },
            onConflict = { opened.vm.disarm(it) },
            onStatus = { opened.vm.notify(it) },
            onHealth = { opened.vm.setAutoUploadFailing(it) },
        )
    }
    DisposableEffect(armedUploader) {
        opened.vm.onDisarm = { armedUploader.flush() }
        onDispose { opened.vm.onDisarm = null }
    }
    // A09/A13: route hardware VOLUME keys to page turns while (and only while) Stage is open. Volume
    // keys don't reach Compose, so the Activity intercepts them (onKeyDown) and calls back through the
    // registrar StageScreen provides here. StageScreen owns the turn logic, so two-up turns by a whole
    // spread like every other input (A13) — the entrypoint just forwards the press; register/unregister
    // is handled inside StageScreen keyed on Stage lifetime. No volume keys on the platform ⇒ a no-op.
    val volumeTurnRegistrar = remember { platform.volumeTurnRegistrar ?: {} }

    // P201 stage 3c — rehearsal auto-update host loop. The concert is server-backed if it
    // carries a concertId (downloaded/known to a server); only then is the toggle offered.
    // While the transient toggle is on (StageState.autoUpdate), poll for a new rev of THIS
    // concert every ~15s; on one, reload the swapped-in bundle from disk and hand it to the
    // VM (applyUpdate → R10 viewport-preserving). The loop is keyed on (dir, autoUpdate), so
    // it starts/stops with the toggle and cancels when Stage is left. Best-effort — a failed
    // tick is a no-op (autoUpdateTick returns null), the current rev keeps performing.
    val stageState by opened.vm.state.collectAsState()
    // A79 follow-up — the Stage's ☰ drawer state (reported up) and the Back handler's close request (sent down).
    var stageDrawerOpen by remember(opened) { mutableStateOf(false) }
    var closeDrawerRequest by remember(opened) { mutableStateOf(0) }
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

    // T147: persist the chrono (start instant + accumulated, per concert) and the clock preference
    // (global) as they change, so the timer survives process death and the clock stays where it was set.
    // The chrono object is stable while running (fixed start instant), so this writes only on
    // start/pause/reset — not every second.
    LaunchedEffect(stageState.chrono) {
        if (concertId.isNotEmpty()) storage.putSecret(chronoKey, encodeChrono(stageState.chrono, platform.monotonicNowMs(), System.currentTimeMillis()))
    }
    LaunchedEffect(stageState.clockVisible) {
        storage.putSecret(STAGE_CLOCK_KEY, stageState.clockVisible.toString())
    }
    LaunchedEffect(stageState.clockStyle) {
        storage.putSecret(STAGE_CLOCK_STYLE_KEY, stageState.clockStyle.name)
    }
    // A70 — persist the note tool/width/colour as they change (per device, §3.6).
    LaunchedEffect(stageState.noteTool) { storage.putSecret(NOTE_TOOL_KEY, stageState.noteTool.name) }
    LaunchedEffect(stageState.noteWidth) { storage.putSecret(NOTE_WIDTH_KEY, stageState.noteWidth.toString()) }
    LaunchedEffect(stageState.noteColour) { storage.putSecret(NOTE_COLOUR_KEY, stageState.noteColour.toString()) }

    // A36: concert mode keeps its own performance look + A34's tuned amber/aqua beat — the brand
    // theme stops at Stage's door (VLL: "don't interact with the concert mode, ok as-is"). Restore
    // the M3 baseline Stage was designed and approved against (a bare MaterialTheme would INHERIT the
    // brand scheme, so pass it explicitly), so the brand palette can never re-tint the page well,
    // the chrome pills or the beat.
    MaterialTheme(colorScheme = lightColorScheme()) {
        CompositionLocalProvider(LocalVolumeTurnRegistrar provides volumeTurnRegistrar) {
            platform.StageWindow {
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
                    // T147: the bottom-right clock reads the device's local time, honouring the user's 12/24h setting.
                    nowClockText = { platform.localTimeText() },
                    // T147: local h/m/s for the analog face (the default clock style).
                    nowLocalHms = {
                        val cal = java.util.Calendar.getInstance()
                        Triple(cal.get(java.util.Calendar.HOUR_OF_DAY), cal.get(java.util.Calendar.MINUTE), cal.get(java.util.Calendar.SECOND))
                    },
                    // A70 — rehearsal notes: the host bitmap port + the concert + a monotonic save clock.
                    notes = notesPort,
                    concertId = concertId,
                    // A74 ⟨D6⟩: a note's updatedAt is PERSISTED metadata, so it must be WALL-CLOCK epoch millis
                    // — elapsedRealtime restarts at every reboot, so a note saved yesterday could outrank one
                    // saved today. (The chrono's monotonicNow above is a separate, correct monotonic source.)
                    noteNow = { System.currentTimeMillis() },
                    // A72 — device-local learned pedal bindings, read at Stage entry (Parameters writes them).
                    pedalBindings = com.troubastack.shared.stage.parsePedalBindings(storage.getSecret(PEDAL_BINDINGS_KEY)),
                    midiSignal = midiSignal, // A76 — BLE-MIDI presses turn pages in Stage
                    // A77 — each note commit/clear while armed → the uploader reconciles with Studio; the flush
                    // hint sends a debounced note promptly on page turn / leaving note mode.
                    onNoteCommitted = { armedUploader.onCommitted(it) },
                    onFlushNotes = { armedUploader.flush() },
                    onDrawerOpenChange = { stageDrawerOpen = it },
                    closeDrawerRequest = closeDrawerRequest,
                    fullScreen = platform.window?.fullScreen, // WIN03 D2: null on Android ⇒ no button
                    onToggleFullScreen = { platform.window?.toggleFullScreen() },
                    pointerKind = platform.pointer, // WIN03 D5
                )
            }
        }
    }
    // A79: while performing, system Back is CONSUMED — an edge swipe meant as a page turn used to leave the
    // concert (VLL: "I already exited several times a concert"). ✕ is the only exit; the failure/empty screens
    // (no ✕) still leave on Back. The pure decision is stageBackAction, tested in shared.
    platform.BackHandler {
        when (com.troubastack.shared.stage.stageBackAction(stageState, drawerOpen = stageDrawerOpen, fullScreen = platform.window?.fullScreen == true)) {
            com.troubastack.shared.stage.StageBack.LEAVE -> selectedDir = null
            com.troubastack.shared.stage.StageBack.CLOSE_DRAWER -> closeDrawerRequest++ // A79 follow-up: drawer only
            com.troubastack.shared.stage.StageBack.STAY -> {}
            com.troubastack.shared.stage.StageBack.EXIT_FULL_SCREEN -> platform.window?.toggleFullScreen() // WIN03 D4: Esc
        }
    }
}
