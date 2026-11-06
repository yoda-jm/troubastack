package com.troubastack.desktop.studio

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.awt.SwingPanel
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import com.troubastack.shared.distribution.embeddedUrl
import java.awt.BorderLayout
import java.awt.Canvas
import javax.swing.JPanel

/** S1.6 — the exact words shown when this copy of Windows has no Microsoft Edge WebView2. Never an IE fallback. */
internal const val NO_WEBVIEW_MESSAGE =
    "Studio can't open here: this copy of Windows doesn't have Microsoft Edge WebView2."

/** Item 1 — a real startup timeout (the WebView2 controller never readied) gets its OWN words, not the no-WebView2
 *  sentence (which would be false). */
internal const val TIMEOUT_MESSAGE = "Studio didn't start in time. Go ‹ Back and open it again."

// S2.1 (R1) — the Loading placeholder and the cover shown when a load fails (server unreachable, or a navigation
// completed with no `ready` within 10s). Android's EditScreen words; S2.9 greps every quoted label from here.
internal const val LOADING_MESSAGE = "Loading…"
internal const val COVER_TITLE = "Can't reach the server"
internal const val COVER_REASON_UNREACHABLE = "The server didn't answer."
internal const val COVER_REASON_NO_READY = "Studio didn't respond."
internal const val COVER_BACK_LABEL = "Back"
internal const val COVER_RETRY_LABEL = "Retry"
internal const val SERVER_URL_TITLE = "Core server URL"
internal const val SERVER_URL_LABEL = "Server address"
internal const val SERVER_URL_SAVE = "Save"
internal const val SERVER_URL_CANCEL = "Cancel"

/**
 * WIN08 (Windows only) — hosts Studio through WebView2 via SWT's `Browser(SWT.EDGE)`, inside the TroubaStage
 * window: the app bar (‹ Back, title, ⋮ Reload) above a `SwingPanel` holding the heavyweight SWT browser.
 */
class SwtEmbeddedStudioProvider : EmbeddedStudioProvider {
    @Composable
    override fun Host(args: EmbeddedStudioArgs) = EmbeddedStudioContent(args, onController = { liveController = it })

    // S2.3: clear the old session's WebView2 cookies. A live Browser clears at once (server change from inside Studio);
    // with none live (sign-out from Home), persist a pending clear the next Studio open applies before seeding. A stale
    // (disposed) controller has a dead thread, so threadAlive() routes it to the pending-clear path.
    override fun clearCookies(edgeDataDir: java.io.File, oldOrigin: String?, cookieName: String?) {
        val live = liveController
        if (live != null && live.threadAlive()) live.clearCookiesNow(oldOrigin, cookieName)
        else appendPending(edgeDataDir, oldOrigin, cookieName)
    }

    // S2.3b: a re-point from inside Studio must NOT clear on the old controller — both controllers share one
    // EdgeDataDir, so the old thread's clearSessions() could run AFTER the new controller has seeded its fresh session
    // cookie and sign it out. So ALWAYS persist the clear; the new controller applies it before seeding, on its thread.
    override fun clearCookiesOnNextOpen(edgeDataDir: java.io.File, oldOrigin: String?, cookieName: String?) {
        // Sabotage repointnowrite: skip the write, as if repointServer forgot to request the clear. The next controller
        // then finds no pending record and applies nothing → the serverurl re-point-applied check reds.
        if (System.getenv("WIN08_SABOTAGE") == "repointnowrite") return
        appendPending(edgeDataDir, oldOrigin, cookieName)
    }

    // APPEND one record per clear (origin<TAB>name), so signing out of A then B before any reopen keeps BOTH — a single
    // overwritten record would let the first origin's cookie survive. The next open applies every record, then deletes.
    private fun appendPending(edgeDataDir: java.io.File, oldOrigin: String?, cookieName: String?) {
        runCatching { edgeDataDir.mkdirs(); java.io.File(edgeDataDir, "pending-cookie-clear").appendText("${oldOrigin ?: ""}\t${cookieName ?: ""}\n") }
    }

    companion object {
        @Volatile internal var liveController: SwtBrowserController? = null
        /** S2.3b smoke only: the Server URL dialog's current text, so the focus check can confirm the TextField took
         *  the Robot's keystrokes with the SwingPanel collapsed to 0.dp. */
        @Volatile internal var smokeDialogText: String? = null
    }
}

/**
 * The embedded screen, factored so the CI smoke can drive the same path and read the controller's flags via
 * [onController]. The heavyweight SWT browser draws over Compose content in this window; the ⋮ menu floats over it as a
 * heavyweight, never-activating Swing JPopupMenu (controller.openStudioMenu) — NOT via compose.layers.type=WINDOW.
 */
@Composable
internal fun EmbeddedStudioContent(args: EmbeddedStudioArgs, onController: (SwtBrowserController) -> Unit = {}) {
    val forceNoWebview = remember {
        System.getProperty("troubastage.win08.forceNoWebview") == "1" || System.getenv("TROUBA_WIN08_FORCE_NO_WEBVIEW") == "1"
    }
    var engineMissing by remember { mutableStateOf(false) }
    var timedOut by remember { mutableStateOf(false) }
    var sabotageMenu by remember { mutableStateOf(false) }  // L2 only: the Compose DropdownMenu, opened via requestComposeMenu
    // S2.1 (R1) phase: `ready` once Studio handshakes (the controller reveals the Shell then); `covered` when a load
    // failed (server unreachable, or a completed navigation with no `ready` in 10s) — the cover draws over the Studio
    // area while the Shell stays hidden. Until either, "Loading…" shows.
    var ready by remember { mutableStateOf(false) }
    var covered by remember { mutableStateOf(false) }
    var coverReason by remember { mutableStateOf(COVER_REASON_UNREACHABLE) }
    var retryNonce by remember { mutableStateOf(0) }
    var showServerDialog by remember { mutableStateOf(false) }   // S2.3b: the ⋮ "Server URL…" in-canvas dialog
    val base = args.serverUrl.orEmpty()
    val loadUrl = remember(base, args.initialPath) { embeddedUrl(base, args.initialPath) }
    val origin = remember(base) { originOfUrl(base) }
    val canvas = remember { Canvas() }
    val controller = remember {
        SwtBrowserController(
            canvas = canvas,
            loadUrl = loadUrl,
            origin = origin,
            edgeDataDir = args.edgeDataDir,
            sessionCookie = args.sessionCookie,
            forceNoWebview = forceNoWebview,
            onReady = { ready = true; covered = false },
            onEngineMissing = { engineMissing = true },
            onStartupTimeout = { timedOut = true },
            onNavNoReady = { if (!ready) { coverReason = COVER_REASON_NO_READY; covered = true } },
            onFullScreenKey = { args.toggleFullScreen() },
            onExitBack = args.onBack,   // S2.2: leave the Studio screen only when Studio's history is exhausted
            log = ::win08log,
        ).also(onController)
    }
    DisposableEffect(Unit) {
        win08log("content: DisposableEffect -> controller.start()")
        controller.requestComposeMenu = { sabotageMenu = true }   // L2 sabotage only
        // S2.3b: the ⋮ "Server URL…" item opens the in-canvas dialog — only offered when the host set onServerUrlChange.
        controller.requestServerDialog = args.onServerUrlChange?.let { { showServerDialog = true } }
        // S2.1 (R1): a host reload (⋮ Reload or the cover's Retry) resets to Loading and re-probes reachability; a
        // reload that fails then brings the cover back. In-page navigations don't call reload(), so they're unaffected.
        controller.onReloadStarted = { ready = false; covered = false; retryNonce++ }
        // S2.2: hand the host the embedded Back action so WindowAction.BACK (chrome-focused Alt+← etc.) walks history.
        // Sabotage backprovidesexit hands back onBack instead → the chrome Back leaves without walking (the check reds).
        val backProvidesExit = System.getenv("WIN08_SABOTAGE") == "backprovidesexit"
        args.provideRequestBack?.invoke { if (backProvidesExit) args.onBack() else controller.requestBack() }
        controller.start()
        onDispose { win08log("content: onDispose -> controller.dispose()"); controller.dispose() }
    }

    // S2.1 (R1) pre-load reachability: on first show and on each Retry, probe the server through the app's own
    // HttpTransport. Unreachable → the cover at once (not after the 10s no-ready timer). Online/Unauthorized → let the
    // load proceed (Studio handles its own sign-in). A `ready` that has already arrived wins.
    LaunchedEffect(retryNonce) {
        if (!ready) {
            val reachable = args.probeReachable?.invoke() ?: true
            if (!reachable && !ready) { coverReason = COVER_REASON_UNREACHABLE; covered = true }
        }
    }

    Column(Modifier.fillMaxSize()) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            TextButton(onClick = { controller.requestBack() }) { Text("‹  Back") }   // S2.2: walk Studio history first, leave when nothing left
            Text(args.title, style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary)
            Spacer(Modifier.weight(1f))
            // The ⋮ menu is a HEAVYWEIGHT, non-activating owned Swing window (openStudioMenu), not a Compose DropdownMenu:
            // it floats over the heavyweight WebView without the process-wide compose.layers.type=WINDOW that stole frame
            // activation. The Compose DropdownMenu below is DORMANT — only the L2 sabotage opens it (the bad path).
            Box {
                TextButton(onClick = { controller.openStudioMenu() }) { Text("⋮") }
                DropdownMenu(expanded = sabotageMenu, onDismissRequest = { sabotageMenu = false }) {
                    DropdownMenuItem(text = { Text("Reload") }, onClick = { sabotageMenu = false; controller.reload() })
                }
            }
        }
        if (engineMissing || timedOut) {
            Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.Center) {
                Text(if (timedOut) TIMEOUT_MESSAGE else NO_WEBVIEW_MESSAGE, style = MaterialTheme.typography.titleMedium)
            }
        } else {
            // S2.1 (R1): the SwingPanel (the canvas hosting the WebView Shell) stays in the composition ALWAYS —
            // removing it would destroy the Browser, which Retry reloads. While the Shell is hidden (Loading/cover),
            // Compose draws over the Studio area: "Loading…" until `ready`, or the cover when a load fails.
            Box(Modifier.fillMaxSize()) {
                // S2.1 (R1): while Loading/the cover shows, collapse the SwingPanel to 0.dp so its heavyweight hole
                // closes and Compose draws over the Studio area. On `ready` it returns to full size; fitShellToCanvas
                // refits the embedded Shell on that resize. The SwingPanel stays in the composition, so the Browser
                // survives (removing it would dispose the canvas peer and destroy the Shell).
                // DO NOT "simplify" this to hiding the Shell (shell.isVisible=false) or the canvas (canvas.isVisible=
                // false): both were measured and leave the area blank. Compose Desktop renders a heavyweight SwingPanel
                // through an interop layer that reserves (punches) that rectangle in the Compose surface for the whole
                // time the component is laid out, regardless of whether its content is visible. Only taking the
                // component out of the layout (0.dp) closes the hole; disposing it would kill the Browser. So: 0.dp.
                // S2.3b: the Server URL dialog ALSO collapses the panel to 0.dp — the same proven mechanism — so the
                // Compose dialog (and its focusable TextField) draws over the Studio area, not behind the heavyweight hole.
                SwingPanel(
                    background = Color.Transparent,
                    modifier = if (ready && !showServerDialog) Modifier.fillMaxSize() else Modifier.size(0.dp),
                    factory = { win08log("content: SwingPanel factory -> JPanel+Canvas"); JPanel(BorderLayout()).apply { add(canvas, BorderLayout.CENTER) } },
                )
                if (!ready) {
                    if (covered) {
                        StudioCover(
                            reason = coverReason,
                            onBack = args.onBack,
                            onRetry = { controller.reload() },   // reload() fires onReloadStarted → Loading + re-probe
                        )
                    } else {
                        Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.Center) {
                            Text(LOADING_MESSAGE, style = MaterialTheme.typography.titleMedium)
                        }
                    }
                }
                if (showServerDialog) {
                    ServerUrlDialog(
                        current = base,
                        onSave = { newUrl -> showServerDialog = false; args.onServerUrlChange?.invoke(newUrl) },
                        onCancel = { showServerDialog = false },
                    )
                }
            }
        }
    }
}

/** S2.1 (R1) — the cover over the Studio area when a load fails: title, the one-line reason, Back and Retry. */
@Composable
private fun StudioCover(reason: String, onBack: () -> Unit, onRetry: () -> Unit) {
    Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text(COVER_TITLE, style = MaterialTheme.typography.titleMedium)
            Spacer(Modifier.height(8.dp))
            Text(reason, style = MaterialTheme.typography.bodyMedium)
            Spacer(Modifier.height(16.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                TextButton(onClick = onBack) { Text(COVER_BACK_LABEL) }
                TextButton(onClick = onRetry) { Text(COVER_RETRY_LABEL) }
            }
        }
    }
}

/**
 * S2.3b — the in-canvas "Server URL…" dialog. Drawn while the SwingPanel is collapsed to 0.dp (the heavyweight hole
 * closed), so this Compose TextField can hold keyboard focus. Save hands the new URL to the host, which clears the old
 * origin's cookie through the next-open path and re-points to a fresh controller; Cancel just restores the panel.
 */
@Composable
private fun ServerUrlDialog(current: String, onSave: (String) -> Unit, onCancel: () -> Unit) {
    // TextFieldValue with the caret at the end, so the prefilled URL is edited (not prepended to) when the user types.
    var url by remember { mutableStateOf(TextFieldValue(current, selection = TextRange(current.length))) }
    val focus = remember { FocusRequester() }
    LaunchedEffect(Unit) { SwtEmbeddedStudioProvider.smokeDialogText = current; runCatching { focus.requestFocus() } }
    Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text(SERVER_URL_TITLE, style = MaterialTheme.typography.titleMedium)
            Spacer(Modifier.height(12.dp))
            OutlinedTextField(
                value = url,
                onValueChange = { url = it; SwtEmbeddedStudioProvider.smokeDialogText = it.text },   // S2.3b smoke reads this
                label = { Text(SERVER_URL_LABEL) },
                singleLine = true,
                modifier = Modifier.focusRequester(focus),
            )
            Spacer(Modifier.height(16.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                TextButton(onClick = onCancel) { Text(SERVER_URL_CANCEL) }
                TextButton(onClick = { onSave(url.text.trim()) }, enabled = url.text.isNotBlank()) { Text(SERVER_URL_SAVE) }
            }
        }
    }
}
