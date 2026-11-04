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
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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

/**
 * WIN08 (Windows only) — hosts Studio through WebView2 via SWT's `Browser(SWT.EDGE)`, inside the TroubaStage
 * window: the app bar (‹ Back, title, ⋮ Reload) above a `SwingPanel` holding the heavyweight SWT browser.
 */
class SwtEmbeddedStudioProvider : EmbeddedStudioProvider {
    @Composable
    override fun Host(args: EmbeddedStudioArgs) = EmbeddedStudioContent(args)
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
            log = ::win08log,
        ).also(onController)
    }
    DisposableEffect(Unit) {
        win08log("content: DisposableEffect -> controller.start()")
        controller.requestComposeMenu = { sabotageMenu = true }   // L2 sabotage only
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
            TextButton(onClick = args.onBack) { Text("‹  Back") }
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
                SwingPanel(
                    background = Color.Transparent,
                    modifier = Modifier.fillMaxSize(),
                    factory = { win08log("content: SwingPanel factory -> JPanel+Canvas"); JPanel(BorderLayout()).apply { add(canvas, BorderLayout.CENTER) } },
                )
                if (!ready) {
                    if (covered) {
                        StudioCover(
                            reason = coverReason,
                            onBack = args.onBack,
                            onRetry = { covered = false; controller.reload(); retryNonce++ },
                        )
                    } else {
                        Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.Center) {
                            Text(LOADING_MESSAGE, style = MaterialTheme.typography.titleMedium)
                        }
                    }
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
