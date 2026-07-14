package com.troubashare.app

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import com.troubashare.shared.distribution.ServerDiscovery
import com.troubashare.shared.seams.Storage
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.launch

private const val CORE_URL_KEY = "coreUrl"
private const val DEFAULT_CORE_URL = "http://10.0.2.2:8080"

/**
 * B03 Connect flow — server URL (shared with A06's Edit) + the existing session login
 * (`POST /api/auth/login`). Offline-first: this is optional; without it the concerts list just shows
 * local bundles (I12). On success the session cookie is persisted (encrypted) by [HttpTransport].
 */
@Composable
fun ConnectScreen(
    storage: Storage,
    transport: HttpTransport,
    onConnected: () -> Unit,
    onBack: () -> Unit,
    // B06: LAN discovery source. Collected only while this screen is composed (start on open, stop on
    // close). Defaults to none so the screen still works (and tests/previews compose) without discovery.
    discovery: ServerDiscovery = ServerDiscovery { flowOf(emptyList()) },
) {
    val scope = rememberCoroutineScope()
    var serverUrl by remember { mutableStateOf(storage.getSecret(CORE_URL_KEY) ?: DEFAULT_CORE_URL) }
    var username by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    // B06: servers advertising on this LAN. Tapping a row PREFILLS the URL only — no auto-connect,
    // no credential sent without the explicit Connect tap below (mDNS is unauthenticated).
    val discovered by discovery.servers().collectAsState(initial = emptyList())

    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            Modifier.fillMaxSize().statusBarsPadding().padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text("Connect to your band", style = MaterialTheme.typography.headlineMedium)
            Text(
                "Sign in to see concerts your band has baked and download them. Playing works offline without an account.",
                style = MaterialTheme.typography.bodyMedium,
            )
            // B06: discovered servers on this network, above the URL field. Tap to prefill the URL.
            if (discovered.isNotEmpty()) {
                Text("Servers on this network", style = MaterialTheme.typography.labelLarge)
                discovered.forEach { server ->
                    Surface(
                        onClick = { serverUrl = server.url },
                        enabled = !busy,
                        modifier = Modifier.fillMaxWidth(),
                        tonalElevation = 2.dp,
                        color = MaterialTheme.colorScheme.surfaceVariant,
                    ) {
                        Text(
                            "🎵  ${server.label}",
                            Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 10.dp),
                            style = MaterialTheme.typography.bodyMedium,
                        )
                    }
                }
            }
            OutlinedTextField(serverUrl, { serverUrl = it }, label = { Text("Server URL") }, singleLine = true, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(username, { username = it }, label = { Text("Username") }, singleLine = true, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(
                password, { password = it }, label = { Text("Password") },
                singleLine = true, visualTransformation = PasswordVisualTransformation(), modifier = Modifier.fillMaxWidth(),
            )
            error?.let { Text(it, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.error) }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                TextButton(onClick = onBack, enabled = !busy) { Text("Cancel") }
                Button(
                    enabled = !busy && username.isNotBlank() && password.isNotBlank(),
                    onClick = {
                        dropSessionIfOriginChanged(storage, serverUrl.trim()) // new server ⇒ drop old session
                        storage.putSecret(CORE_URL_KEY, serverUrl.trim())
                        busy = true; error = null
                        scope.launch {
                            val err = try {
                                transport.connect(username.trim(), password)
                            } catch (e: Exception) {
                                "Couldn't reach the server"
                            }
                            busy = false
                            if (err == null) onConnected() else error = err
                        }
                    },
                ) { Text(if (busy) "Connecting…" else "Connect") }
            }
        }
    }
}
