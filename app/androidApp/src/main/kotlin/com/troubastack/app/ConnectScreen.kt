package com.troubastack.app

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
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
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
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import com.troubastack.shared.distribution.ServerDiscovery
import com.troubastack.shared.seams.Storage
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
    val uriHandler = LocalUriHandler.current
    // A68: two mutually-exclusive journeys as tabs — 0 = Sign in (DEFAULT), 1 = Invite.
    var tab by remember { mutableStateOf(0) }

    Column(
        Modifier.heightIn(max = 560.dp).verticalScroll(rememberScrollState()).padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        // Title row with an explicit ✕ dismiss — this is what makes it read as a modal, not a page.
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
            Text("Connect to your band", style = MaterialTheme.typography.headlineSmall, color = MaterialTheme.colorScheme.primary)
            TextButton(onClick = onDone, enabled = !busy) { Text("✕") }
        }
        // A68: the offline reassurance sits ABOVE the tabs — true of both journeys, and the one line that
        // tells a hesitant person they can ignore all of this. It earns prominence, not a footnote.
        Text(
            "Connect to see the concerts your band has baked and download them. Playing works offline without an account.",
            style = MaterialTheme.typography.bodyMedium,
        )
        // A68: choosing a tab is choosing a journey; you then read only that half (the old modal stacked
        // both inside one scroll, so on a phone the lead could be the only thing visible or scroll away).
        // Sign in is the default — it recurs (new device, expired session); an invite is redeemed once per
        // band. This RETIRES A57's invite-first ORDER, not its reason: with both labels always visible an
        // invite holder always sees an "Invite" tab, so the fear that drove A57 (a lone "Sign in" button)
        // is gone. Keep the default fixed — not conditional on stored state (a modal that opens differently
        // by invisible history is harder to trust; the other tab is one tap regardless).
        val tabs = listOf("Sign in", "Invite")
        SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
            tabs.forEachIndexed { i, label ->
                SegmentedButton(
                    selected = tab == i,
                    onClick = { tab = i },
                    shape = SegmentedButtonDefaults.itemShape(i, tabs.size),
                    enabled = !busy,
                ) { Text(label) }
            }
        }

        if (tab == 1) {
            // Invite journey (A52): paste an invite link — it names the server, so the person never types a
            // URL. Acting on it is routed through A51's joinDecision by the JoinDialog the host opens; this
            // modal just hands the string up and closes. Join is enabled for any non-blank paste; a bad
            // link is refused (with a reason) by the parser in the sheet, not eagerly here.
            OutlinedTextField(
                inviteLink, { inviteLink = it }, label = { Text("Paste invite link") },
                singleLine = true, enabled = !busy, modifier = Modifier.fillMaxWidth(),
            )
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.End), verticalAlignment = Alignment.CenterVertically) {
                // A53: scan the QR the studio already renders instead of pasting — same destination (the
                // join sheet), the camera just removes the paste. Denial/absence falls back to the field above.
                TextButton(enabled = !busy, onClick = onScan) { Text("Scan a QR") }
                Button(enabled = !busy && inviteLink.isNotBlank(), onClick = { onInviteLink(inviteLink.trim()) }) { Text("Join") }
            }
        } else {
            // Sign-in journey: discovery → URL → credentials → Connect. Discovery prefills the URL, so it
            // belongs here (B06: renders only when non-empty, above the URL field). Tap a row to prefill.
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
            OutlinedTextField(serverUrl, { serverUrl = it }, label = { Text("Server URL") }, singleLine = true, enabled = !busy, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(username, { username = it }, label = { Text("Username") }, singleLine = true, enabled = !busy, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(
                password, { password = it }, label = { Text("Password") },
                singleLine = true, enabled = !busy, visualTransformation = PasswordVisualTransformation(), modifier = Modifier.fillMaxWidth(),
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
            // A68: the app can create neither an account nor a band (an in-app bare account would land on a
            // Home with no band, no way forward). Registration lives in Studio, and every invite-made account
            // arrives already attached to a band. So send a newcomer to <serverUrl>/register in the browser
            // (LocalUriHandler — the mechanism BRAND11 §2 shares). It depends on the URL: /register on
            // nothing is nothing, so disable it (with a reason) until a server URL is present.
            val registerUrl = registerUrlFor(serverUrl)
            TextButton(
                enabled = !busy && registerUrl != null,
                onClick = { registerUrl?.let(uriHandler::openUri) },
            ) { Text("Create an account in your browser ↗") }
            if (registerUrl == null) {
                Text(
                    "Enter your band's server URL above to create an account.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}

/** A68: the browser target for "Create an account" — Studio's own `/register` on the entered server, or
 *  null when there is no server to register against (which disables the link). Trims a trailing slash so
 *  a pasted `http://host:8080/` still yields `…:8080/register`, not `…//register`. */
internal fun registerUrlFor(serverUrl: String): String? {
    val base = serverUrl.trim().trimEnd('/')
    return if (base.isEmpty()) null else "$base/register"
}
