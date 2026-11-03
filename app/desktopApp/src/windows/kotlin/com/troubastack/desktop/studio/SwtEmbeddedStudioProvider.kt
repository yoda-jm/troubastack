package com.troubastack.desktop.studio

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
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
 * [onController]. The heavyweight SWT browser draws over Compose content in this window; `compose.layers.type=WINDOW`
 * (set in Main, Windows only) makes the ⋮ menu a separate undecorated window so it floats over the browser.
 */
@Composable
internal fun EmbeddedStudioContent(args: EmbeddedStudioArgs, onController: (SwtBrowserController) -> Unit = {}) {
    val forceNoWebview = remember {
        System.getProperty("troubastage.win08.forceNoWebview") == "1" || System.getenv("TROUBA_WIN08_FORCE_NO_WEBVIEW") == "1"
    }
    var engineMissing by remember { mutableStateOf(false) }
    var timedOut by remember { mutableStateOf(false) }
    var menuOpen by remember { mutableStateOf(false) }
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
            onReady = {},
            onEngineMissing = { engineMissing = true },
            onStartupTimeout = { timedOut = true },
            onFullScreenKey = {
                win08log("win08: onFullScreenKey entry; isFullScreen before=${args.isFullScreen()}")
                args.toggleFullScreen()
                win08log("win08: onFullScreenKey after toggleFullScreen; isFullScreen=${args.isFullScreen()}")
            },
            log = ::win08log,
        ).also(onController)
    }
    DisposableEffect(Unit) {
        win08log("content: DisposableEffect -> controller.start()")
        controller.start()
        onDispose { win08log("content: onDispose -> controller.dispose()"); controller.dispose() }
    }

    Column(Modifier.fillMaxSize()) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            TextButton(onClick = args.onBack) { Text("‹  Back") }
            Text(args.title, style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary)
            Spacer(Modifier.weight(1f))
            Box {
                TextButton(onClick = { menuOpen = true }) { Text("⋮") }
                DropdownMenu(expanded = menuOpen, onDismissRequest = { menuOpen = false }) {
                    DropdownMenuItem(text = { Text("Reload") }, onClick = { menuOpen = false; controller.reload() })
                }
            }
        }
        if (engineMissing || timedOut) {
            Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.Center) {
                Text(if (timedOut) TIMEOUT_MESSAGE else NO_WEBVIEW_MESSAGE, style = MaterialTheme.typography.titleMedium)
            }
        } else {
            SwingPanel(
                background = Color.Transparent,
                modifier = Modifier.fillMaxSize(),
                factory = { win08log("content: SwingPanel factory -> JPanel+Canvas"); JPanel(BorderLayout()).apply { add(canvas, BorderLayout.CENTER) } },
            )
        }
    }
}
