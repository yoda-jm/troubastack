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
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import com.troubashare.shared.seams.Storage
import kotlinx.coroutines.launch

private const val CORE_URL_KEY = "coreUrl"
private const val DEFAULT_CORE_URL = "http://10.0.2.2:8080"

/**
 * B03 Connect flow — server URL (shared with A06's Edit) + the existing session login
 * (`POST /api/auth/login`). Offline-first: this is optional; without it the concerts list just shows
 * local bundles (I12). On success the session cookie is persisted (encrypted) by [HttpTransport].
 */
@Composable
fun ConnectScreen(storage: Storage, transport: HttpTransport, onConnected: () -> Unit, onBack: () -> Unit) {
    val scope = rememberCoroutineScope()
    var serverUrl by remember { mutableStateOf(storage.getSecret(CORE_URL_KEY) ?: DEFAULT_CORE_URL) }
    var username by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

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
