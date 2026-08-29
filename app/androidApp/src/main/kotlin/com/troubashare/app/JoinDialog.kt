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
import com.troubashare.shared.join.RegisterOutcome
import com.troubashare.shared.join.ServerIdentity
import com.troubashare.shared.join.joinDecision
import com.troubashare.shared.join.parseTroubaLink
import com.troubashare.shared.join.reasonSentence
import com.troubashare.shared.seams.SESSION_ORIGIN_KEY
import com.troubashare.shared.seams.Storage
import kotlinx.coroutines.launch

/**
 * A52 — the join sheet: a modal driven by A51's `parseTroubaLink` → `joinDecision`. One string in (a
 * pasted or deep-linked invite), a band membership out, without the person hand-typing a server URL.
 *
 * The steps mirror the decision: **ConfirmServer** (a different/first-ever host — show it, then PROBE it
 * via T123's unauthenticated `/api/version` and refuse the password unless it identifies as TroubaStack)
 * → **SignIn** (the two invite routes are auth-gated) → **preview** (band + role) → **accept**. A `Blocked`
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
                is PreviewResult.Unusable -> JoinStep.Blocked(reasonSentence(r.reason)) // A56: sentence, not the raw word
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
                is AcceptOutcome.Gone -> { token.clear(); JoinStep.Blocked(reasonSentence(r.reason)) } // A56: sentence
                AcceptOutcome.NeedsSignIn -> JoinStep.SignIn(transport.currentOrigin)
                AcceptOutcome.NotFound -> JoinStep.Failed("This invite link wasn't found.")
                is AcceptOutcome.Failed -> JoinStep.Failed(
                    if (r.status == 0) "Couldn't reach the server." else "Couldn't join (${r.status}).",
                )
            }
        }
    }

    // T123: verify the target host BEFORE any password field, then switch + sign in. A host that doesn't
    // identify as TroubaStack is refused here (no password ever shown) — the real protection behind the
    // scanner (A53). Runs on Continue from the ConfirmServer step.
    fun verifyThenSignIn(target: String) {
        step = JoinStep.Busy("Verifying $target…")
        scope.launch {
            step = when (val id = transport.probeServerIdentity(target)) {
                ServerIdentity.TroubaStack -> {
                    transport.switchServer(target)   // point at the target, drop the old session
                    JoinStep.SignIn(target)          // switching means no session here yet
                }
                is ServerIdentity.TooNew -> JoinStep.Blocked(
                    "$target needs a newer TroubaStage (it speaks API v${id.serverApi}, this app knows v${id.clientMax}). Update the app to join.",
                )
                ServerIdentity.Foreign -> JoinStep.Blocked(
                    "$target isn't a TroubaStack server. TroubaStage won't send your password there.",
                )
                ServerIdentity.Unreachable -> JoinStep.Failed(
                    "Couldn't reach $target to verify it. Check the link and your connection.",
                )
            }
        }
    }

    // Route once from the parsed link. currentOrigin is the origin the app holds a SESSION with (null if
    // never connected) — not baseUrl, which defaults to a placeholder and would both hide the genuine
    // first-run case and surface a meaningless emulator address (A52 review, item 2). Arms the token for
    // the three actionable outcomes; a Blocked link never arms it.
    LaunchedEffect(rawLink) {
        val sessionOrigin = storage.getSecret(SESSION_ORIGIN_KEY)?.takeIf { it.isNotBlank() }
        when (val action = joinDecision(parseTroubaLink(rawLink), sessionOrigin, transport.isConnected)) {
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
                        Text(
                            if (s.current == null) "This invite points at a server you haven't used before."
                            else "This invite is for a different server.",
                            style = MaterialTheme.typography.bodyMedium,
                        )
                        HostLine("Invite server", s.target)
                        s.current?.let { HostLine("You're on", it) }
                        // T123: Continue PROBES the target first (verifyThenSignIn) — a password field only
                        // appears if it identifies as a TroubaStack server. So no scary "we can't verify"
                        // warning here; the verification is real, not a disclaimer.
                        Text(
                            "TroubaStage will check this is a TroubaStack server before asking for your password.",
                            style = MaterialTheme.typography.bodySmall,
                        )
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            TextButton(onClick = { token.clear(); onClose() }) { Text("Cancel") }
                            Button(onClick = { verifyThenSignIn(s.target) }) { Text("Continue") }
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

/** A57: the minimum password length the app enforces client-side. C6 records that the SERVER enforces only
 *  `minPasswordLen = 1` and has no rate limiting — both remain open; this is a client-side floor + message,
 *  not a fix for C6. */
private const val MIN_PASSWORD = 8

/** The sign-in sub-step: the two invite routes are auth-gated, so joining a server means signing into it
 *  first (the token is held by the parent across this round-trip). A57: an invited NEWCOMER can create an
 *  account here — "New here? Create an account" → register → auto-sign-in → continue the same join. Register
 *  form is join-sheet-only (the invite is what scopes account creation); Connect is untouched. */
@Composable
private fun SignInStep(
    origin: String,
    storage: Storage,
    transport: HttpTransport,
    onCancel: () -> Unit,
    onSignedIn: () -> Unit,
) {
    var registering by remember { mutableStateOf(false) }
    var username by remember { mutableStateOf(storage.getSecret(LAST_USERNAME_KEY) ?: "") }
    var displayName by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    // Sign in against the (already-selected) server, then continue the join. Shared by both paths — after a
    // register we sign in with the same credentials so PendingToken redeems without restarting.
    fun signInThen(u: String, p: String) {
        scope.launch {
            val err = try {
                transport.connect(u, p)
            } catch (e: Exception) {
                "Couldn't reach the server"
            }
            busy = false
            if (err == null) {
                storage.putSecret(LAST_USERNAME_KEY, u) // A41: remember on success only
                onSignedIn()
            } else {
                error = err
            }
        }
    }

    if (registering) {
        Text("Create an account to join.", style = MaterialTheme.typography.bodyMedium)
        OutlinedTextField(username, { username = it }, label = { Text("Username") }, singleLine = true, enabled = !busy, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(displayName, { displayName = it }, label = { Text("Display name") }, singleLine = true, enabled = !busy, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(
            password, { password = it }, label = { Text("Password") },
            singleLine = true, enabled = !busy, visualTransformation = PasswordVisualTransformation(), modifier = Modifier.fillMaxWidth(),
        )
        error?.let { Text(it, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.error) }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            TextButton(onClick = { registering = false; error = null }, enabled = !busy) { Text("Have an account? Sign in") }
            Button(
                enabled = !busy && username.isNotBlank() && displayName.isNotBlank() && password.isNotBlank(),
                onClick = {
                    if (password.length < MIN_PASSWORD) { error = "Use at least $MIN_PASSWORD characters."; return@Button }
                    busy = true; error = null
                    scope.launch {
                        when (transport.register(username, displayName, password)) {
                            RegisterOutcome.Created -> signInThen(username.trim(), password) // keeps busy until sign-in resolves
                            RegisterOutcome.NameTaken -> { busy = false; error = "That name is taken — try another." }
                            is RegisterOutcome.Failed -> { busy = false; error = "Couldn't create the account. Try again." }
                        }
                    }
                },
            ) { Text(if (busy) "Creating…" else "Create account") }
        }
    } else {
        Text("Sign in to $origin to join.", style = MaterialTheme.typography.bodyMedium)
        OutlinedTextField(username, { username = it }, label = { Text("Username") }, singleLine = true, enabled = !busy, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(
            password, { password = it }, label = { Text("Password") },
            singleLine = true, enabled = !busy, visualTransformation = PasswordVisualTransformation(), modifier = Modifier.fillMaxWidth(),
        )
        error?.let { Text(it, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.error) }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            TextButton(onClick = onCancel, enabled = !busy) { Text("Cancel") }
            Button(
                enabled = !busy && username.isNotBlank() && password.isNotBlank(),
                onClick = { busy = true; error = null; signInThen(username.trim(), password) },
            ) { Text(if (busy) "Signing in…" else "Sign in") }
        }
        // A57: the newcomer branch — an invite's whole point is bringing in someone with no account yet.
        TextButton(onClick = { registering = true; error = null; password = "" }, enabled = !busy) {
            Text("New here? Create an account")
        }
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
