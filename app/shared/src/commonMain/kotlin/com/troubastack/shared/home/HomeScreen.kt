package com.troubastack.shared.home

import com.troubastack.shared.ui.LocalBrandAccents

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
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.troubastack.shared.distribution.Availability
import com.troubastack.shared.distribution.BakeStatus
import com.troubastack.shared.distribution.UpdateProgress

/**
 * A27 — the app's HOME landing page (VLL: "a landing page that is not the bake list, so we clearly
 * identify the products inside the app and can log in from here"). Cold start lands HERE.
 *
 * Design per the 2026-07-18 A27 ruling (VLL delegated) + the 2026-07-19 A31 ruling (Concerts nests
 * under Studio):
 *  - Branding SMALL: a compact muted "TroubaStage" wordmark, never a headline.
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
    // A42②: one-tap re-bake. [canReBake] is true only for a connected ADMIN of the resume concert's band
    // (the row is hidden otherwise); [bake] is the live re-bake status driven by the progress poll.
    val canReBake: Boolean = false,
    val bake: BakeStatus = BakeStatus.Hidden,
    // A54: after the encrypted store had to be reset to recover from an unreadable-KeyStore crash, Home
    // says so once — a non-modal line, not a blocking prompt (VLL-approved self-heal). The user was
    // signed out and settings reset; their concerts (files) are intact.
    val settingsReset: Boolean = false,
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

/** A55 — the TroubaStudio tile's state, derived from the SAME [Identity] as the status line (one source of
 *  truth). The reason travels WITH the disabled state so the caption can't drift from a parallel `when`.
 *  Studio needs the server (list / download / manage), so it is live only when [Identity.Connected]. */
sealed interface StudioTile {
    data object Enabled : StudioTile
    /** [reason] is the caption; empty for the neutral [Identity.Checking] case (no reason to show yet). */
    data class Disabled(val reason: String) : StudioTile
}

/**
 * A55 — VLL: *"TroubaStudio should be greyed with a reason … it should be clear it is disabled and cannot
 * click on it."* Enabled only when [Identity.Connected]; disabled with a reason otherwise. [Identity.Checking]
 * is disabled with a **neutral** (empty) caption on purpose: the presence probe runs on every resume, so a
 * tile that flashed "Sign in to manage concerts" for a beat on each return Home — then enabled — would be
 * worse than one that is briefly and quietly unavailable.
 */
fun studioEnablement(identity: Identity): StudioTile = when (identity) {
    is Identity.Connected -> StudioTile.Enabled
    is Identity.SignedOut, is Identity.NotSetUp -> StudioTile.Disabled("Sign in to manage concerts")
    is Identity.Offline -> StudioTile.Disabled("No connection")
    is Identity.Checking -> StudioTile.Disabled("") // neutral — don't flash a wrong reason on every resume
}

/**
 * The PRIMARY action for a state — A38: the action must match the status. Recognized → Disconnect,
 * Offline → Retry, Empty while Checking (the button is shown disabled, not hidden).
 *
 * A57: both Guest states say **"Join or sign in"** (was "Sign in"/"Connect"). Both open the same Connect
 * modal, which now LEADS with the invite (paste / Scan / Join) before manual sign-in — so a person holding
 * an invite must not read a button that says only "Sign in" and conclude it isn't for them.
 */
fun identityAction(identity: Identity): String = when (identity) {
    is Identity.Connected -> "Disconnect"
    is Identity.Offline -> "Retry"
    is Identity.SignedOut -> "Join or sign in"
    is Identity.NotSetUp -> "Join or sign in"
    is Identity.Checking -> ""
}

/** Whether the row also offers a secondary "Manage" (server/account details, reached via the Connect
 *  modal). Not for a fresh install (nothing to manage) nor mid-probe. */
fun identityHasManage(identity: Identity): Boolean =
    identity !is Identity.NotSetUp && identity !is Identity.Checking

/**
 * A45 — the SHORT label for the always-visible top-right account chip (a state-tinted dot + this text,
 * glanceable with NO tap; the full detail + actions live in the bottom sheet the chip opens). Band name
 * when Recognized (the one thing a player wants at a glance before a gig), else the status word; "…"
 * while probing. Distinct from [identityLine], which is the fuller sentence shown inside the sheet.
 */
fun accountChipLabel(identity: Identity): String = when (identity) {
    is Identity.Connected -> identity.band.ifEmpty { identity.name.ifEmpty { "Connected" } }
    is Identity.Offline -> "Offline"
    is Identity.SignedOut -> "Guest"
    is Identity.NotSetUp -> "Guest"
    is Identity.Checking -> "…"
}

/**
 * A45 — what the account bottom sheet offers for a given [identity]. Parameters is always present
 * (chrome, not account); [manage] follows [identityHasManage]; [primaryAction] is [identityAction]
 * (""/blank while Checking ⇒ the sheet shows it disabled, never a live action mid-probe — the row's
 * "disabled, not hidden" rule, A38). Disconnect still routes through A38's confirm dialog at the host.
 */
data class AccountMenu(
    val manage: Boolean,
    val primaryAction: String,
    val settings: Boolean = true,
)

fun accountMenu(identity: Identity): AccountMenu =
    AccountMenu(manage = identityHasManage(identity), primaryAction = identityAction(identity))

/**
 * A45 regression guard: the Home update affordance is only ever eligible when **Recognized** — the host
 * forces [UpdateStatus.Hidden] for Offline / Guest / Checking, and `UpdateRow` gates on that. Moving the
 * connection controls into the account sheet must not change this coupling (a player who is Offline or a
 * Guest must not be offered an update). Pure, so the relationship is pinned by a test.
 */
fun updateRowEligible(identity: Identity): Boolean = identity is Identity.Connected

/** A45: the account chip shows its text label only when the header has room; below this it collapses to
 *  the dot alone (T58's phone-width behaviour), so the header never overflows at 320 dp. Pure/testable. */
fun accountChipShowsLabel(availableWidthDp: Int): Boolean = availableWidthDp >= 360

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
    // A57: a DIRECT scan entry for a Guest holding an invite QR — on Home, not behind the account panel.
    // Straight into A53's scanner (which falls back to paste when the camera is denied, so it can't dead-end).
    onScanToJoin: () -> Unit = {},
    // A39: pull the newer bake(s) / cancel an in-flight update. No-ops when the update row is Hidden.
    onUpdate: () -> Unit = {},
    onCancelUpdate: () -> Unit = {},
    // A42②: one-tap re-bake of the resume concert (admin only — the row is hidden unless canReBake).
    onReBake: () -> Unit = {},
    modifier: Modifier = Modifier,
) {
    // A45: the account sheet — the ONE top-right trigger's detail + actions (Parameters / Manage /
    // Connect·Sign in·Disconnect). Opened by the chip; state is local to Home.
    var showAccount by remember { mutableStateOf(false) }
    Surface(modifier.fillMaxSize()) {
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            // A45: small muted wordmark (left) + the ONE top-right account chip (right) — a state-tinted
            // dot + a short label, glanceable with no tap; it replaces the standalone ⚙ Parameters button
            // and the old ConnectionRow, moving the ACTIONS into a bottom sheet while the STATUS stays
            // visible (T58 concept, phone-shaped). Collapses to the dot alone at narrow width.
            BoxWithConstraints(Modifier.fillMaxWidth()) {
                val showLabel = accountChipShowsLabel(maxWidth.value.toInt())
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        "TroubaStage",
                        style = MaterialTheme.typography.labelLarge,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    AccountChip(state.identity, showLabel = showLabel, onClick = { showAccount = true })
                }
            }

            // A57: a Guest holding an invite QR is offered a DIRECT scan, on Home, not buried in the account
            // sheet. Only in the Guest states (a Recognized/Offline user has no reason to join from here);
            // the scanner falls back to paste if the camera is denied, so it can't dead-end.
            if (state.identity is Identity.SignedOut || state.identity is Identity.NotSetUp) {
                TextButton(onClick = onScanToJoin, modifier = Modifier.align(Alignment.Start)) {
                    Text("⧉  Scan a QR to join a band")
                }
            }

            // A54: the after-the-fact recovery notice — shown once, the launch on which the encrypted
            // store had to be reset. Non-modal (nothing to tap through), honest about what was lost.
            if (state.settingsReset) {
                Surface(
                    color = MaterialTheme.colorScheme.surfaceVariant,
                    shape = MaterialTheme.shapes.medium,
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Text(
                        "Your saved settings were reset and you'll need to sign in again. Your concerts are safe.",
                        Modifier.padding(horizontal = 14.dp, vertical = 10.dp),
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }

            // BRAND09: the two tiles wear their PRODUCT accent (this-is-that-product content), while
            // act-here chrome (Resume, the perform action) keeps the indigo primary — A36's one-hue
            // rule scoped to chrome, per the gate ruling. Accents are theme-aware via LocalBrandAccents.
            val accents = LocalBrandAccents.current
            // TroubaStage · Perform — the ONE big primary tile (the on-stage button). A36: warm paper
            // surface; BRAND09: a Stage-accent outline + heading so the tile identifies the product.
            Card(
                onClick = onPerform,
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
                border = BorderStroke(1.5.dp, accents.stage),
            ) {
                Column(Modifier.fillMaxWidth().padding(24.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text("▶  TroubaStage", style = MaterialTheme.typography.headlineSmall, color = accents.stage)
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
            // A55: it needs the server, so it's disabled (NOT clickable — `enabled=false`, not just an
            // alpha change) when not Connected, with the reason as its subtitle. TroubaStage above stays
            // enabled always (offline perform is I12).
            val studio = studioEnablement(state.identity)
            val studioEnabled = studio is StudioTile.Enabled
            Card(
                onClick = onStudio,
                enabled = studioEnabled,
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(
                    // BRAND09: a CONNECTED tile looks connected — a branded studioActive fill; the
                    // disabled fill is studioIdle, DERIVED from the same accent, so "grey ⇒ disabled"
                    // reads reliably instead of the old neutral surfaceVariant used for both states.
                    containerColor = accents.studioActive,
                    disabledContainerColor = accents.studioIdle,
                ),
                border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
            ) {
                Column(Modifier.fillMaxWidth().padding(20.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text(
                        "✎  TroubaStudio",
                        style = MaterialTheme.typography.titleLarge,
                        color = if (studioEnabled) accents.studio
                        else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
                    )
                    Text(
                        when (studio) {
                            is StudioTile.Disabled -> if (studio.reason.isNotEmpty()) studio.reason else "Author, import & manage concerts"
                            StudioTile.Enabled -> "Author, import & manage concerts"
                        },
                        style = MaterialTheme.typography.bodyMedium,
                        color = if (studioEnabled) MaterialTheme.colorScheme.onSurface
                        else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }

            Spacer(Modifier.height(4.dp))

            // A39: one-tap Update, shown only when Recognized. A45: gate on updateRowEligible so the
            // status↔update coupling (Hidden for Offline/Guest) survives the connection-controls move —
            // pinned by a test. UpdateRow itself still returns early on a Hidden status (e.g. manifest
            // unreadable while Connected).
            if (updateRowEligible(state.identity)) {
                UpdateRow(state.update, onUpdate = onUpdate, onCancel = onCancelUpdate)
            }

            // A42②: one-tap re-bake — shown only to an admin of the resume concert's band (canReBake).
            BakeRow(state.bake, state.canReBake, state.lastConcertName, onReBake = onReBake)
        }
    }
    if (showAccount) {
        AccountSheet(
            identity = state.identity,
            onSettings = { showAccount = false; onSettings() },
            onManage = { showAccount = false; onManage() },
            // A38: Disconnect (and every primary action) routes through the host, which keeps the
            // disconnect confirmation dialog. Close the sheet first so the confirm isn't behind it.
            onPrimaryAction = { showAccount = false; onPrimaryAction() },
            onDismiss = { showAccount = false },
        )
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

/**
 * A42 ② — the admin re-bake affordance + live progress line. Nothing for a non-admin (canReBake=false)
 * and no bake running — a control that can only 403 is not an affordance. When idle it's an explicit,
 * unconditional "Re-bake" (there is no honest staleness signal — A42②(b) — so it never implies one);
 * while running it shows T99's live line via [bakePollStep]; on failure it shows the server's user-safe
 * message (T102) with a retry.
 */
@Composable
private fun BakeRow(status: BakeStatus, canReBake: Boolean, concertName: String, onReBake: () -> Unit) {
    if (status is BakeStatus.Hidden && !canReBake) return
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
                is BakeStatus.Hidden -> {
                    Text(
                        if (concertName.isNotEmpty()) "Re-bake «$concertName»" else "Re-bake concert",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.weight(1f),
                    )
                    Button(onClick = onReBake) { Text("Re-bake") }
                }
                is BakeStatus.Baking -> {
                    androidx.compose.material3.CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                    Text(status.label, style = MaterialTheme.typography.titleSmall, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                }
                is BakeStatus.Failed -> {
                    Text(
                        status.message,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.error,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f),
                    )
                    Button(onClick = onReBake) { Text("Retry") }
                }
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

/**
 * A45 — the ONE top-right account trigger (T58's concept). A state-tinted [StatusIcon] dot + a short
 * label ([accountChipLabel]), glanceable with no tap; collapses to the dot alone when [showLabel] is
 * false (narrow width). Tapping opens [AccountSheet]. Looks like a chip, not a button — the status colour
 * is deliberately not the brand indigo.
 */
@Composable
private fun AccountChip(identity: Identity, showLabel: Boolean, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        shape = MaterialTheme.shapes.small,
        color = MaterialTheme.colorScheme.surfaceVariant,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
    ) {
        Row(
            Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            StatusIcon(identity, statusColor(identity))
            if (showLabel) {
                Text(
                    accountChipLabel(identity),
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.widthIn(max = 160.dp),
                )
            }
        }
    }
}

/**
 * A45 — the account bottom sheet the chip opens (a `ModalBottomSheet`, reusing the Stage's pattern:
 * thumb-reachable actions, per §4b — not a top-anchored dropdown). Detail line + the actions that used to
 * sit on the surface: the primary identity action (Connect / Sign in / Disconnect), Manage, Parameters.
 * The action is disabled — not hidden — while Checking (A38), and Disconnect still confirms at the host.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun AccountSheet(
    identity: Identity,
    onSettings: () -> Unit,
    onManage: () -> Unit,
    onPrimaryAction: () -> Unit,
    onDismiss: () -> Unit,
) {
    val menu = accountMenu(identity)
    val checking = identity is Identity.Checking
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState()) {
        Column(
            Modifier.fillMaxWidth().padding(horizontal = 24.dp).padding(bottom = 32.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                StatusIcon(identity, statusColor(identity))
                Text(identityLine(identity), style = MaterialTheme.typography.titleMedium, modifier = Modifier.weight(1f))
            }
            if (menu.primaryAction.isNotEmpty() || checking) {
                Button(onClick = onPrimaryAction, enabled = !checking, modifier = Modifier.fillMaxWidth()) {
                    Text(if (checking) "…" else menu.primaryAction)
                }
            }
            if (menu.manage) {
                // A56: "Manage" collided with the Studio tile's "…manage concerts" (the CONTENT); this opens
                // the Connect modal — server URL, credentials, invite paste/scan, discovered servers — i.e.
                // the CONNECTION. Name what's behind it. (Behaviour via identityHasManage is unchanged.)
                TextButton(onClick = onManage, modifier = Modifier.fillMaxWidth()) { Text("Server & account") }
            }
            if (menu.settings) {
                TextButton(onClick = onSettings, modifier = Modifier.fillMaxWidth()) { Text("⚙  Parameters") }
            }
        }
    }
}
