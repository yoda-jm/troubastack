package com.troubashare.app

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import com.troubashare.shared.distribution.ServerDiscovery
import com.troubashare.shared.seams.Storage
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.launch

private const val CORE_URL_KEY = "coreUrl"
private const val DEFAULT_CORE_URL = "http://10.0.2.2:8080"

/**
 * A38 — Connect is a MODAL, not a page. A bounded titled surface overlaying Home (which stays visible
 * behind), dismissible by ✕, tap-outside, Cancel, or system Back — and Back **dismisses rather than
 * leaving the app** (the old full-screen `ConnectScreen` had no `BackHandler`, so Back exited). Compose
 * `Dialog` routes all three of tap-outside / Back / ✕ through [onClose]. Content unchanged from B03/B06.
 */
@Composable
fun ConnectDialog(
    storage: Storage,
    transport: HttpTransport,
    discovery: ServerDiscovery = ServerDiscovery { flowOf(emptyList()) },
    onClose: () -> Unit,
    onInviteLink: (String) -> Unit = {},
    onScan: () -> Unit = {},
) {
    Dialog(onDismissRequest = onClose, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(
            Modifier.fillMaxWidth(0.92f).widthIn(max = 520.dp),
            shape = MaterialTheme.shapes.large,
            color = MaterialTheme.colorScheme.surface,
            tonalElevation = 6.dp,
        ) {
            ConnectContent(storage, transport, discovery, onDone = onClose, onInviteLink = onInviteLink, onScan = onScan)
        }
    }
}

/** The B03 Connect flow — server URL (shared with A06's Edit) + session login (`POST /api/auth/login`).
 *  Offline-first (I12): optional; without it the concerts list just shows local bundles. On success the
 *  session cookie is persisted (encrypted) by [HttpTransport]. Hosted inside [ConnectDialog]. */
@Composable
private fun ConnectContent(
    storage: Storage,
    transport: HttpTransport,
    discovery: ServerDiscovery,
    onDone: () -> Unit,
    onInviteLink: (String) -> Unit = {},
    onScan: () -> Unit = {},
) {
    val scope = rememberCoroutineScope()
    // A52: pasting an invite link is THE path — it names the server, so the person never hand-types a
    // URL. Manual sign-in below is the floor (camera denied/absent, iOS, or no link to hand).
    var inviteLink by remember { mutableStateOf("") }
    var serverUrl by remember { mutableStateOf(storage.getSecret(CORE_URL_KEY) ?: DEFAULT_CORE_URL) }
    // A41: seed the last username so Sign in after a Disconnect needs only a password. The password
    // field is never seeded (below) — the secret stays unpersisted.
    var username by remember { mutableStateOf(storage.getSecret(LAST_USERNAME_KEY) ?: "") }
    var password by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    // B06: servers advertising on this LAN. Tapping a row PREFILLS the URL only — no auto-connect,
    // no credential sent without the explicit Connect tap below (mDNS is unauthenticated).
    val discovered by discovery.servers().collectAsState(initial = emptyList())

    Column(
        Modifier.heightIn(max = 560.dp).verticalScroll(rememberScrollState()).padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        // Title row with an explicit ✕ dismiss — this is what makes it read as a modal, not a page.
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
            Text("Connect to your band", style = MaterialTheme.typography.headlineSmall, color = MaterialTheme.colorScheme.primary)
            TextButton(onClick = onDone, enabled = !busy) { Text("✕") }
        }
        Text(
            "Sign in to see concerts your band has baked and download them. Playing works offline without an account.",
            style = MaterialTheme.typography.bodyMedium,
        )
        // A52: paste an invite link — it names the server, so the person doesn't type a URL. Acting on it
        // is routed through A51's joinDecision by the JoinDialog the host opens; this modal just hands the
        // string up and closes. The Join button is enabled for any non-blank paste; a bad link is refused
        // (with a reason) by the parser in the sheet, not eagerly here.
        Text("Have an invite link?", style = MaterialTheme.typography.labelLarge)
        OutlinedTextField(
            inviteLink, { inviteLink = it }, label = { Text("Paste invite link") },
            singleLine = true, enabled = !busy, modifier = Modifier.fillMaxWidth(),
        )
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.End), verticalAlignment = Alignment.CenterVertically) {
            // A53: scan the QR the studio already renders instead of pasting — same destination (the join
            // sheet), the camera just removes the paste. Denial/absence falls back here.
            TextButton(enabled = !busy, onClick = onScan) { Text("Scan a QR") }
            Button(enabled = !busy && inviteLink.isNotBlank(), onClick = { onInviteLink(inviteLink.trim()) }) { Text("Join") }
        }
        Text("or sign in manually", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
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
            TextButton(onClick = onDone, enabled = !busy) { Text("Cancel") }
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
                        if (err == null) {
                            storage.putSecret(LAST_USERNAME_KEY, username.trim()) // A41: remember on SUCCESS only
                            onDone()
                        } else {
                            error = err
                        }
                    }
                },
            ) { Text(if (busy) "Connecting…" else "Connect") }
        }
    }
}
