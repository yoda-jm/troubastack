package com.troubashare.shared.home

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.troubashare.shared.distribution.Availability
import com.troubashare.shared.distribution.UpdateProgress

/**
 * A27 — the app's HOME landing page (VLL: "a landing page that is not the bake list, so we clearly
 * identify the products inside the app and can log in from here"). Cold start lands HERE.
 *
 * Design per the 2026-07-18 A27 ruling (VLL delegated) + the 2026-07-19 A31 ruling (Concerts nests
 * under Studio):
 *  - Branding SMALL: a compact muted "TroubaShare" wordmark, never a headline.
 *  - Exactly TWO branded products: TroubaStage (perform) + TroubaStudio (author/manage). Concerts is
 *    NOT a Home peer any more — it lives INSIDE Studio (A31: TroubaStage → the concert list in
 *    perform intent; TroubaStudio → the SAME list in manage intent, where import/update/edit live).
 *    (TroubaCore is the server — it lives in the Manage details, never a tile.)
 *  - Hybrid layout: TroubaStage · Perform is the ONE big primary tile (a big on-stage touch target
 *    is function, not decoration); TroubaStudio is a second branded product tile below it.
 *  - Identity is one clean line ("Connected ✓" / "Checking…" / "Offline · …" / "Connect to your
 *    band") — A31: rendered from a LIVE probe on entering Home, never the cached cookie flag; the raw
 *    server IP:port never headlines — it lives behind Manage.
 *
 * Pure commonMain UI so iOS inherits it (§13 nav hoist); all state + actions injected, no platform
 * types leak in. I12: the identity card is OPTIONAL — Perform works fully offline with no login.
 */

/**
 * What Home shows about the viewer's connection (A38: status and action are two different questions —
 * *what am I?* is the icon + colour + line; *what can I do?* is [identityAction]).
 *
 * Three player-facing statuses — **Recognized** ([Connected]), **Guest** ([SignedOut]/[NotSetUp]),
 * **Offline** — plus a transient [Checking]. Guest deliberately covers BOTH "session ended on a known
 * server" and "never set up": as a *status* they're the same (nobody knows who you are), and it's the
 * honest word — you can keep working (I12). They differ only in the action (Sign in vs Connect).
 */
sealed interface Identity {
    /** A38 transient while Home probes the server (short timeout) — a spinner, not a fourth status. */
    data object Checking : Identity

    /** Recognized — the server knows me. [name] fills in "Performing as <name>"; [band] when known. */
    data class Connected(val name: String = "", val band: String = "", val synced: Boolean = true) : Identity

    /** Offline — no server reachable right now. A reassurance, not an error (I12). */
    data class Offline(val band: String = "") : Identity

    /** Guest, server known — the session ended (or a server was configured) but I'm not signed in.
     *  [band] is kept so "Sign in" reads as resuming, not starting over. Action: **Sign in**. */
    data class SignedOut(val band: String = "") : Identity

    /** Guest, nothing set up — no server configured yet. Action: **Connect**. */
    data object NotSetUp : Identity
}

/** Immutable Home view state (built by the host from Storage/transport/installed bundles). Pure. */
data class HomeState(
    val lastConcertName: String = "",  // resume target; "" ⇒ no resume affordance
    val concertCount: Int = 0,
    val identity: Identity = Identity.NotSetUp,
    val update: UpdateStatus = UpdateStatus.Hidden, // A39
)

/** The one-line identity label — pure, unit-testable (no Compose). The raw IP:port is never here
 *  (per the ruling it lives behind Manage); name/band fill in as they become cheaply known. */
fun identityLine(identity: Identity): String = when (identity) {
    is Identity.Checking -> "Checking…"
    is Identity.Connected -> buildString {
        append(if (identity.name.isNotEmpty()) "Performing as ${identity.name}" else "Connected")
        if (identity.band.isNotEmpty()) append(" · ").append(identity.band)
        append(if (identity.synced) " ✓" else " · syncing…")
    }
    is Identity.Offline -> buildString {
        append("Offline")
        if (identity.band.isNotEmpty()) append(" · ").append(identity.band)
        append(" · concerts on device still work")
    }
    // Guest, both variants: the status word is "Guest" (you can keep working — I12); the band, when
    // known, makes "Sign in" read as resuming. The distinction lives in the ACTION, not the label.
    is Identity.SignedOut -> buildString {
        append("Guest")
        if (identity.band.isNotEmpty()) append(" · ").append(identity.band)
    }
    is Identity.NotSetUp -> "Guest · not connected to a band"
}

/**
 * The PRIMARY action for a state — A38: the action must match the status. Recognized → Disconnect,
 * Offline → Retry, Guest → Sign in (server known) / Connect (nothing set up). Empty while Checking
 * (the button is shown disabled, not hidden).
 */
fun identityAction(identity: Identity): String = when (identity) {
    is Identity.Connected -> "Disconnect"
    is Identity.Offline -> "Retry"
    is Identity.SignedOut -> "Sign in"
    is Identity.NotSetUp -> "Connect"
    is Identity.Checking -> ""
}

/** Whether the row also offers a secondary "Manage" (server/account details, reached via the Connect
 *  modal). Not for a fresh install (nothing to manage) nor mid-probe. */
fun identityHasManage(identity: Identity): Boolean =
    identity !is Identity.NotSetUp && identity !is Identity.Checking

/**
 * The band segment of the Recognized line — A38 multi-band ruling (VLL: "one person can be in multiple
 * groups"). The connection has no single "current" band (P205 resolves band+roster **per concert**),
 * so the line says only what's true: nothing for none, the name for one, a **count** for several. The
 * per-band detail lives behind Manage — a status line is the wrong place to invent a "current band".
 */
fun bandLabel(names: List<String>): String = when (names.size) {
    0 -> ""
    1 -> names[0]
    else -> "${names.size} bands"
}

/**
 * A39 — one tap from Home to pull the current concert(s) onto the device. Fable's reframe: this is
 * **Update** (fetch a newer baked rev), not Bake — baking is admin-only and there's no dirty signal,
 * whereas "is my copy out of date?" is answerable from `currentRev` with no server change.
 *
 * Only meaningful when Recognized; the host sets [Hidden] for Offline/Guest (their row already offers
 * the right next step) and when there's nothing newer it's [UpToDate] (quiet, no dead button).
 */
sealed interface UpdateStatus {
    /** Not shown — Offline / Guest / nothing to update. */
    data object Hidden : UpdateStatus

    /** Recognized and there's nothing to fetch — a quiet, NARROW reassurance ("Nothing to update"), never
     *  a claim of completeness (A43: it must not read as "you have everything you were sent"). */
    data object UpToDate : UpdateStatus

    /** Recognized and something is fetchable — [summary] says what's waiting, [action] is the button verb
     *  ("Update" a stale copy, or "Download" onto an empty device — A43). One tap downloads+installs. */
    data class Available(val summary: String, val action: String = "Update") : UpdateStatus

    /** A download+install is running — cancellable (bundles are large, this may be venue wifi). A42 ①:
     *  [fraction] is the download progress in 0..&lt;1 when a Content-Length is known, or null =
     *  indeterminate (unknown total, or the install tail); [label] is the human line ("Downloading
     *  12.3 / 45.6 MB" / "Installing…"). Never 1f — the terminal state is [UpToDate], not a full bar. */
    data class InFlight(val fraction: Float? = null, val label: String = "Updating…") : UpdateStatus

    /** An update attempt failed — say so (T30: never swallow a gesture silently). [message] is shown
     *  in the row; the button retries. */
    data class Failed(val message: String) : UpdateStatus
}

/**
 * A42 ① — the PURE map from an [UpdateProgress] to the [UpdateStatus.InFlight] the row renders, so the
 * whole "honest bar" contract is unit-testable without a device:
 *  - a known total ⇒ a determinate fraction, CAPPED just below full (never 1f before the swap — a bar
 *    sitting at 100% is the display that reads as "hung"),
 *  - an unknown total (contentLength ≤ 0) ⇒ null ⇒ the row stays indeterminate (NEVER a fabricated
 *    fraction — an invented bar is worse than an honest spinner),
 *  - the install tail ⇒ null + "Installing…" (genuinely not a percentage).
 */
fun inFlightStatus(p: UpdateProgress): UpdateStatus.InFlight = when (p) {
    is UpdateProgress.Downloading ->
        if (p.contentLength > 0L) {
            val f = (p.bytesRead.toDouble() / p.contentLength).coerceIn(0.0, 0.99).toFloat()
            UpdateStatus.InFlight(fraction = f, label = "Downloading ${humanBytes(p.bytesRead)} / ${humanBytes(p.contentLength)}")
        } else {
            UpdateStatus.InFlight(fraction = null, label = "Downloading ${humanBytes(p.bytesRead)}")
        }
    UpdateProgress.Installing -> UpdateStatus.InFlight(fraction = null, label = "Installing…")
}

/** Compact human byte size for the download readout (decimal MB/KB, one dp). Pure. */
internal fun humanBytes(n: Long): String = when {
    n >= 1_000_000L -> "${(n / 100_000L) / 10.0} MB"
    n >= 1_000L -> "${n / 1_000L} KB"
    else -> "$n B"
}

/** What's waiting to update, for [UpdateStatus.Available] — pure/testable. One concert names it; several
 *  are counted (the per-concert detail lives in Manage, like [bandLabel]). */
fun updateSummary(names: List<String>): String = when (names.size) {
    0 -> ""
    1 -> "${names[0]} — new version"
    else -> "${names.size} concerts to update"
}

/**
 * A44 — the PURE terminal status for a FINISHED update run, lifted out of MainActivity's `onUpdate`
 * lambda so it's reachable from commonTest. The A42① deadlock lived exactly here, inline in a Composable
 * where no test could touch it: a successful run left the row on `InFlight("Installing…")` and the
 * re-diff that would clear it is itself guarded by `homeUpdate !is InFlight` — so the row hung forever.
 *
 * [failed] is the display names of the concerts whose apply FAILED (empty ⇒ every offer installed). The
 * load-bearing invariant: **all-succeeded is TERMINAL ([UpToDate]), NEVER [InFlight]** — an InFlight tail
 * here re-creates the deadlock. A partial failure yields the result-driven [Failed] message (never an
 * optimistic UpToDate); the row's guard keeps that message until the user retries. The host still bumps
 * its refresh after a clean run so the re-diff can refine [UpToDate] → [Available] if a newer rev landed.
 */
fun updateOutcomeStatus(failed: List<String>): UpdateStatus =
    if (failed.isEmpty()) {
        UpdateStatus.UpToDate
    } else {
        val more = if (failed.size > 1) " +${failed.size - 1} more" else ""
        UpdateStatus.Failed("Couldn't update ${failed.first()}$more — try again")
    }

/** What's NOT on this device yet, for [UpdateStatus.Available] in the empty-device case (A43) — pure. */
fun downloadSummary(names: List<String>): String = when (names.size) {
    0 -> ""
    1 -> "${names[0]} — not on this device"
    else -> "${names.size} concerts to download"
}

/** The Home landing's update row + the offers its button will act on. Bundled so the whole decision is
 *  one pure return value (A43). */
data class LandingUpdate(
    val status: UpdateStatus,
    val offers: List<Availability> = emptyList(),
    val names: List<String> = emptyList(),
)

/**
 * A43 — the Home landing is a **pre-gig glance** answering "am I ready?", so its row must never overstate
 * what the app knows. Three states that used to all render "Up to date" are separated here, as a PURE
 * decision testable off a fake manifest + fake diff (VLL: *"if I delete the latest bake, is 'Up to date'
 * still there?"* — it was, and worse: an EMPTY device and an UNREADABLE manifest read as current too).
 *
 * [manifestSize] is the number of concerts the band's manifest lists, or **null** if it couldn't be
 * fetched. [offered] are installed-but-stale concerts (A39); [newlyAvailable] are manifest concerts not
 * on the device. Rules, in order:
 *  1. **Couldn't check ([manifestSize] == null) ⇒ [Hidden]** — make NO currency claim; silence, never a
 *     green light (keeps the "don't nag on a transient failure" intent).
 *  2. **Stale installed ⇒ [Available] "Update"** — A39, unchanged.
 *  3. **Every listed concert is missing (empty device) while the band HAS concerts ⇒ [Available]
 *     "Download"** — an empty device is not "up to date". This is the ONLY place the landing surfaces
 *     [Availability.NewlyAvailable] (bounded B; blanket re-offering is nagware).
 *  4. **Otherwise ⇒ [UpToDate]** — the quiet, narrow reassurance. A partial set with one deleted concert
 *     lands here (still-quiet: the deleted one is a Manage-screen download, not a landing nag).
 */
fun landingUpdate(
    manifestSize: Int?,
    offered: List<Availability.UpdateOffered>,
    newlyAvailable: List<Availability.NewlyAvailable>,
    nameOf: (String) -> String,
): LandingUpdate = when {
    manifestSize == null -> LandingUpdate(UpdateStatus.Hidden)
    offered.isNotEmpty() -> {
        val names = offered.map { nameOf(it.concertId) }
        LandingUpdate(UpdateStatus.Available(updateSummary(names)), offered, names)
    }
    // Zero of the band's concerts installed (every listed one is NewlyAvailable) — offer the download.
    manifestSize > 0 && newlyAvailable.size == manifestSize -> {
        val names = newlyAvailable.map { nameOf(it.concertId) }
        LandingUpdate(UpdateStatus.Available(downloadSummary(names), action = "Download"), newlyAvailable, names)
    }
    else -> LandingUpdate(UpdateStatus.UpToDate)
}

@Composable
fun HomeScreen(
    state: HomeState,
    onPerform: () -> Unit,
    onResume: () -> Unit,
    onStudio: () -> Unit,
    // A38: the PRIMARY connection action for the current status (Disconnect / Retry / Sign in /
    // Connect) — the host routes it by state. onManage opens the server/account details (Connect modal).
    onPrimaryAction: () -> Unit,
    onManage: () -> Unit,
    onSettings: () -> Unit,
    // A39: pull the newer bake(s) / cancel an in-flight update. No-ops when the update row is Hidden.
    onUpdate: () -> Unit = {},
    onCancelUpdate: () -> Unit = {},
    modifier: Modifier = Modifier,
) {
    Surface(modifier.fillMaxSize()) {
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            // Small, muted wordmark — a brand mark, not a headline — with the Parameters gear on the right.
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "TroubaShare",
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                TextButton(onClick = onSettings) { Text("⚙  Parameters", style = MaterialTheme.typography.labelLarge) }
            }

            // TroubaStage · Perform — the ONE big primary tile (the on-stage button). A36: warm paper
            // surface + an indigo outline (the brand accent), not a lavender fill — matches the
            // website's ochre-paper-with-indigo look instead of reading as stock Material purple.
            Card(
                onClick = onPerform,
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
                border = BorderStroke(1.5.dp, MaterialTheme.colorScheme.primary),
            ) {
                Column(Modifier.fillMaxWidth().padding(24.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text("▶  TroubaStage", style = MaterialTheme.typography.headlineSmall, color = MaterialTheme.colorScheme.primary)
                    Text(
                        when {
                            state.lastConcertName.isNotEmpty() -> state.lastConcertName
                            state.concertCount > 0 -> "Perform · open a concert"
                            else -> "Perform · import or download a concert"
                        },
                        style = MaterialTheme.typography.bodyMedium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    if (state.concertCount > 0) {
                        Text("${state.concertCount} on device", style = MaterialTheme.typography.bodySmall)
                    }
                    if (state.lastConcertName.isNotEmpty()) {
                        Spacer(Modifier.height(8.dp))
                        Surface(
                            onClick = onResume,
                            shape = MaterialTheme.shapes.small,
                            color = MaterialTheme.colorScheme.primary,
                            contentColor = MaterialTheme.colorScheme.onPrimary,
                        ) {
                            Text(
                                "Resume «${state.lastConcertName}»",
                                style = MaterialTheme.typography.labelLarge,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                                modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
                            )
                        }
                    }
                }
            }

            // TroubaStudio — the SECOND branded product (A31). Author/manage lives here, and Concerts
            // (import / download / update / edit) nests INSIDE it — it is no longer a Home peer.
            Card(
                onClick = onStudio,
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
                border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
            ) {
                Column(Modifier.fillMaxWidth().padding(20.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text("✎  TroubaStudio", style = MaterialTheme.typography.titleLarge, color = MaterialTheme.colorScheme.primary)
                    Text(
                        "Author, import & manage concerts",
                        style = MaterialTheme.typography.bodyMedium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }

            Spacer(Modifier.height(4.dp))

            // A38: the connection control — status (icon + semantic colour + line) plus the action that
            // matches it. Status colour is deliberately NOT the brand indigo, so "connected" never
            // looks like "this is a button"; each state also has a distinct icon shape (legible in bad
            // stage light, and for colour-blind users).
            ConnectionRow(state.identity, onPrimaryAction = onPrimaryAction, onManage = onManage)

            // A39: one-tap Update, shown only when Recognized (the host sets Hidden otherwise).
            UpdateRow(state.update, onUpdate = onUpdate, onCancel = onCancelUpdate)
        }
    }
}

@Composable
private fun UpdateRow(status: UpdateStatus, onUpdate: () -> Unit, onCancel: () -> Unit) {
    if (status is UpdateStatus.Hidden) return
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
    ) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            when (status) {
                is UpdateStatus.UpToDate -> Text(
                    // A43: narrowed — speaks only about what you have (nothing stale), never "you have
                    // everything you were sent". The empty-device / couldn't-check cases never land here.
                    "Nothing to update",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.weight(1f),
                )
                is UpdateStatus.Available -> {
                    Text(status.summary, style = MaterialTheme.typography.titleSmall, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                    Button(onClick = onUpdate) { Text(status.action) } // A43: "Update" or "Download"
                }
                is UpdateStatus.InFlight -> {
                    val f = status.fraction
                    if (f != null) {
                        // A42 ①: determinate download bar (only when Content-Length was known).
                        LinearProgressIndicator(progress = { f }, modifier = Modifier.size(width = 96.dp, height = 4.dp))
                    } else {
                        androidx.compose.material3.CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                    }
                    Text(status.label, style = MaterialTheme.typography.titleSmall, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                    TextButton(onClick = onCancel) { Text("Cancel") }
                }
                is UpdateStatus.Failed -> {
                    Text(
                        status.message,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.error,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f),
                    )
                    Button(onClick = onUpdate) { Text("Retry") }
                }
                is UpdateStatus.Hidden -> {}
            }
        }
    }
}

// A38 — semantic STATUS colours, their own tokens (NOT derived from A36's indigo brand). Light/dark
// pairs chosen to read on both the warm-paper and near-black grounds; guest/checking use the neutral.
private val StatusOnlineLight = androidx.compose.ui.graphics.Color(0xFF2E7D32)
private val StatusOnlineDark = androidx.compose.ui.graphics.Color(0xFF66BB6A)
private val StatusOfflineLight = androidx.compose.ui.graphics.Color(0xFFB26A00)
private val StatusOfflineDark = androidx.compose.ui.graphics.Color(0xFFFFB74D)

@Composable
private fun statusColor(identity: Identity): androidx.compose.ui.graphics.Color {
    val dark = androidx.compose.foundation.isSystemInDarkTheme()
    return when (identity) {
        is Identity.Connected -> if (dark) StatusOnlineDark else StatusOnlineLight
        is Identity.Offline -> if (dark) StatusOfflineDark else StatusOfflineLight
        else -> MaterialTheme.colorScheme.onSurfaceVariant // Guest / Checking → neutral
    }
}

/** A distinct icon SHAPE per status (never colour alone): filled dot = recognized, hollow = guest,
 *  slashed = offline, spinner = checking. */
@Composable
private fun StatusIcon(identity: Identity, tint: androidx.compose.ui.graphics.Color) {
    if (identity is Identity.Checking) {
        androidx.compose.material3.CircularProgressIndicator(Modifier.size(16.dp), color = tint, strokeWidth = 2.dp)
        return
    }
    androidx.compose.foundation.Canvas(Modifier.size(16.dp)) {
        val sw = 2.dp.toPx()
        val r = size.minDimension / 2 - sw
        when (identity) {
            is Identity.Connected -> drawCircle(tint, r, center) // filled
            is Identity.Offline -> { // hollow + slash
                drawCircle(tint, r, center, style = androidx.compose.ui.graphics.drawscope.Stroke(sw))
                drawLine(
                    tint,
                    androidx.compose.ui.geometry.Offset(center.x - r * 0.72f, center.y + r * 0.72f),
                    androidx.compose.ui.geometry.Offset(center.x + r * 0.72f, center.y - r * 0.72f),
                    strokeWidth = sw,
                )
            }
            else -> drawCircle(tint, r, center, style = androidx.compose.ui.graphics.drawscope.Stroke(sw)) // hollow = guest
        }
    }
}

@Composable
private fun ConnectionRow(identity: Identity, onPrimaryAction: () -> Unit, onManage: () -> Unit) {
    val tint = statusColor(identity)
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
    ) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            StatusIcon(identity, tint)
            Text(
                identityLine(identity),
                style = MaterialTheme.typography.titleSmall,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f),
            )
            // A38: keep BOTH buttons present across the probe, DISABLED while Checking, so the row
            // doesn't reflow/pop each time Home resumes. Checking only ever resolves to a state that
            // has Manage + an action (a no-cookie start goes straight to Guest, never through Checking),
            // so mirroring that layout while probing is safe. "Disabled, not hidden."
            val checking = identity is Identity.Checking
            if (identityHasManage(identity) || checking) {
                TextButton(onClick = onManage, enabled = !checking) { Text("Manage") }
            }
            val action = identityAction(identity)
            if (action.isNotEmpty() || checking) {
                Button(onClick = onPrimaryAction, enabled = !checking) { Text(if (checking) "…" else action) }
            }
        }
    }
}
