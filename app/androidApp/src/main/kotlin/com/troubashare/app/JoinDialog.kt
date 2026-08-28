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
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
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
import com.troubashare.shared.join.AcceptOutcome
import com.troubashare.shared.join.JoinAction
import com.troubashare.shared.join.PendingToken
import com.troubashare.shared.join.PreviewResult
import com.troubashare.shared.join.joinDecision
import com.troubashare.shared.join.parseTroubaLink
import com.troubashare.shared.seams.Storage
import kotlinx.coroutines.launch

/**
 * A52 — the join sheet: a modal driven by A51's `parseTroubaLink` → `joinDecision`. One string in (a
 * pasted or deep-linked invite), a band membership out, without the person hand-typing a server URL.
 *
 * The steps mirror the decision: **ConfirmServer** (a different/first-ever host — show it, warn it's
 * unverified until T123 lands, never silently switch) → **SignIn** (the two invite routes are auth-gated,
 * so a session for the target server is required) → **preview** (band + role) → **accept**. A `Blocked`
 * link (reset link / unsupported / hostile) explains and offers nothing.
 *
 * Token hygiene: the token is a bearer credential held ONLY in [PendingToken] (in memory) and cleared on
 * every exit via [DisposableEffect] `onDispose` — success, failure, cancel, or navigating away. It is
 * never persisted and never logged.
 */
@Composable
fun JoinDialog(
    rawLink: String,
    storage: Storage,
    transport: HttpTransport,
    onClose: () -> Unit,
    onJoined: () -> Unit,
) {
    val scope = rememberCoroutineScope()
    val token = remember { PendingToken() }
    // Cleared on ANY departure from composition — the single guarantee behind "never outlives the flow".
    DisposableEffect(Unit) { onDispose { token.clear() } }

    var step by remember { mutableStateOf<JoinStep>(JoinStep.Starting) }

    // Preview against whatever server baseUrl now points at (a Redeem is already there; a SignIn/Confirm
    // has arranged it). NeedsSignIn routes back to the sign-in step (a session lapsed mid-flow).
    fun kickPreview() {
        val t = token.value ?: return
        step = JoinStep.Busy("Loading the invite…")
        scope.launch {
            step = when (val r = transport.previewInvite(t)) {
                is PreviewResult.Ready -> JoinStep.Review(r.band, r.role)
                is PreviewResult.Unusable -> JoinStep.Blocked(r.reason)
                PreviewResult.NeedsSignIn -> JoinStep.SignIn(transport.currentOrigin)
                PreviewResult.NotFound -> JoinStep.Failed("This invite link wasn't found.")
                is PreviewResult.Failed -> JoinStep.Failed(
                    if (r.status == 0) "Couldn't reach the server." else "Couldn't load the invite (${r.status}).",
                )
            }
        }
    }

    fun accept() {
        val t = token.value ?: return
        step = JoinStep.Busy("Joining…")
        scope.launch {
            step = when (val r = transport.acceptInvite(t)) {
                is AcceptOutcome.Joined -> { token.clear(); JoinStep.Done(r.band) }
                is AcceptOutcome.Gone -> { token.clear(); JoinStep.Blocked(r.reason) }
                AcceptOutcome.NeedsSignIn -> JoinStep.SignIn(transport.currentOrigin)
                AcceptOutcome.NotFound -> JoinStep.Failed("This invite link wasn't found.")
                is AcceptOutcome.Failed -> JoinStep.Failed(
                    if (r.status == 0) "Couldn't reach the server." else "Couldn't join (${r.status}).",
                )
            }
        }
    }

    // Route once from the parsed link. Arms the token for the three actionable outcomes; a Blocked link
    // never arms it.
    LaunchedEffect(rawLink) {
        when (val action = joinDecision(parseTroubaLink(rawLink), transport.currentOrigin, transport.isConnected)) {
            is JoinAction.Blocked -> step = JoinStep.Blocked(action.reason)
            is JoinAction.ConfirmServer -> { token.arm(action.token); step = JoinStep.Confirm(action.target, action.current) }
            is JoinAction.SignIn -> { token.arm(action.token); step = JoinStep.SignIn(action.origin) }
            is JoinAction.Redeem -> { token.arm(action.token); kickPreview() }
        }
    }

    Dialog(
        onDismissRequest = { token.clear(); onClose() },
        properties = DialogProperties(usePlatformDefaultWidth = false),
    ) {
        Surface(
            Modifier.fillMaxWidth(0.92f).widthIn(max = 520.dp),
            shape = MaterialTheme.shapes.large,
            color = MaterialTheme.colorScheme.surface,
            tonalElevation = 6.dp,
        ) {
            Column(
                Modifier.heightIn(max = 560.dp).verticalScroll(rememberScrollState()).padding(20.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                    Text("Join a band", style = MaterialTheme.typography.headlineSmall, color = MaterialTheme.colorScheme.primary)
                    TextButton(onClick = { token.clear(); onClose() }) { Text("✕") }
                }

                when (val s = step) {
                    JoinStep.Starting, is JoinStep.Busy -> {
                        val msg = (s as? JoinStep.Busy)?.message ?: "…"
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            CircularProgressIndicator()
                            Text(msg, style = MaterialTheme.typography.bodyMedium)
                        }
                    }

                    is JoinStep.Confirm -> {
                        Text("This invite is for a different server.", style = MaterialTheme.typography.bodyMedium)
                        HostLine("Invite server", s.target)
                        s.current?.let { HostLine("You're on", it) }
                        // T123 (server-identity probe) is on HOLD, so we cannot verify the target IS a
                        // TroubaStack server. Say so plainly rather than implying trust — the person is
                        // about to type a password into this host.
                        Text(
                            "TroubaStage can't verify this server yet. Only continue if you trust who gave you this link.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.error,
                        )
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            TextButton(onClick = { token.clear(); onClose() }) { Text("Cancel") }
                            Button(onClick = {
                                transport.switchServer(s.target)     // point at the target, drop the old session
                                step = JoinStep.SignIn(s.target)      // switching means no session here yet
                            }) { Text("Continue") }
                        }
                    }

                    is JoinStep.SignIn -> SignInStep(
                        origin = s.origin,
                        storage = storage,
                        transport = transport,
                        onCancel = { token.clear(); onClose() },
                        onSignedIn = { kickPreview() },
                    )

                    is JoinStep.Review -> {
                        Text("You've been invited to", style = MaterialTheme.typography.bodyMedium)
                        Text(s.band, style = MaterialTheme.typography.titleLarge, color = MaterialTheme.colorScheme.primary)
                        if (s.role.isNotBlank()) Text("as ${s.role}", style = MaterialTheme.typography.bodyMedium)
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            TextButton(onClick = { token.clear(); onClose() }) { Text("Cancel") }
                            Button(onClick = { accept() }) { Text("Join") }
                        }
                    }

                    is JoinStep.Done -> {
                        Text("You've joined ${s.band}.", style = MaterialTheme.typography.bodyMedium)
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                            Button(onClick = onJoined) { Text("Done") }
                        }
                    }

                    is JoinStep.Blocked -> {
                        Text(s.reason, style = MaterialTheme.typography.bodyMedium)
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                            Button(onClick = { token.clear(); onClose() }) { Text("Close") }
                        }
                    }

                    is JoinStep.Failed -> {
                        Text(s.message, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.error)
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                            Button(onClick = { token.clear(); onClose() }) { Text("Close") }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun HostLine(label: String, host: String) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Text("$label:", style = MaterialTheme.typography.labelLarge)
        Text(host, style = MaterialTheme.typography.bodyMedium)
    }
}

/** The sign-in sub-step: the two invite routes are auth-gated, so joining a server means signing into it
 *  first (the token is held by the parent across this round-trip). Mirrors ConnectContent's login. */
@Composable
private fun SignInStep(
    origin: String,
    storage: Storage,
    transport: HttpTransport,
    onCancel: () -> Unit,
    onSignedIn: () -> Unit,
) {
    var username by remember { mutableStateOf(storage.getSecret(LAST_USERNAME_KEY) ?: "") }
    var password by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    Text("Sign in to $origin to join.", style = MaterialTheme.typography.bodyMedium)
    OutlinedTextField(username, { username = it }, label = { Text("Username") }, singleLine = true, modifier = Modifier.fillMaxWidth())
    OutlinedTextField(
        password, { password = it }, label = { Text("Password") },
        singleLine = true, visualTransformation = PasswordVisualTransformation(), modifier = Modifier.fillMaxWidth(),
    )
    error?.let { Text(it, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.error) }
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        TextButton(onClick = onCancel, enabled = !busy) { Text("Cancel") }
        Button(
            enabled = !busy && username.isNotBlank() && password.isNotBlank(),
            onClick = {
                busy = true; error = null
                scope.launch {
                    val err = try {
                        transport.connect(username.trim(), password)
                    } catch (e: Exception) {
                        "Couldn't reach the server"
                    }
                    busy = false
                    if (err == null) {
                        storage.putSecret(LAST_USERNAME_KEY, username.trim()) // A41: remember on success only
                        onSignedIn()
                    } else {
                        error = err
                    }
                }
            },
        ) { Text(if (busy) "Signing in…" else "Sign in") }
    }
}

/** The steps of the join sheet — a linear-ish walk from the parsed link to a membership. Local to the
 *  sheet; the cross-platform routing lives in A51's `joinDecision`. */
private sealed interface JoinStep {
    data object Starting : JoinStep
    data class Busy(val message: String) : JoinStep
    data class Confirm(val target: String, val current: String?) : JoinStep
    data class SignIn(val origin: String) : JoinStep
    data class Review(val band: String, val role: String) : JoinStep
    data class Done(val band: String) : JoinStep
    data class Blocked(val reason: String) : JoinStep
    data class Failed(val message: String) : JoinStep
}
