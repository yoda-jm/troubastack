package com.troubastack.desktop.studio

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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import org.eclipse.swt.SWT

/**
 * WIN08 (Windows only) — hosts Studio through WebView2 via SWT's `Browser(SWT.EDGE)`.
 *
 * INCREMENT 1 is a structural scaffold: it imports SWT and reports `SWT.getVersion()`, which proves the dependency
 * resolves, this `src/windows` source set compiles, the jar is bundled into the Windows app image, and the native
 * loads at runtime — before the SWT_AWT/SwingPanel browser embedding (daemon display thread, EdgeDataDir session
 * seed, nav-veto, the TroubaStageShellReceive bridge, the no-WebView2 guard, F11 forwarding) lands on top.
 *
 * It is registered via `src/windows/resources/META-INF/services/…EmbeddedStudioProvider`, which is added to the jar
 * only on a Windows host build, so on Linux and macOS `embeddedStudioProvider` is null and "Coming soon." stands.
 */
class SwtEmbeddedStudioProvider : EmbeddedStudioProvider {
    @Composable
    override fun Host(args: EmbeddedStudioArgs) {
        Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            Column(Modifier.fillMaxSize().padding(20.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    TextButton(onClick = args.onBack) { Text("‹  Back") }
                    Text(args.title, style = MaterialTheme.typography.headlineSmall, color = MaterialTheme.colorScheme.primary)
                }
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    Text(
                        "Embedded Studio scaffold — SWT ${SWT.getVersion()} (${SWT.getPlatform()}); server=${args.serverUrl ?: "none"}",
                        style = MaterialTheme.typography.titleMedium,
                    )
                }
            }
        }
    }
}
