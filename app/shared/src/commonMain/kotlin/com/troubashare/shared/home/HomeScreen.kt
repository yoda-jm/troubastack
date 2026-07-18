package com.troubashare.shared.home

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp

/**
 * A27 — the app's HOME landing page (VLL: "a landing page that is not the bake list, so we clearly
 * identify the products inside the app and can log in from here"). Cold start lands HERE.
 *
 * Design per the 2026-07-18 ruling (VLL delegated):
 *  - Branding SMALL: a compact muted "TroubaShare" wordmark, never a headline.
 *  - Exactly TWO branded products: TroubaStage (Perform) + TroubaStudio (Edit). "Concerts" is an
 *    unbranded UTILITY. (TroubaCore is the server — it lives in the Manage details, never a tile.)
 *  - Hybrid layout: TroubaStage · Perform is the ONE big primary tile (a big on-stage touch target
 *    is function, not decoration); TroubaStudio + Concerts collapse to a compact button pair below.
 *  - Identity is one clean line ("Connected ✓" / "Offline · …" / "Connect to your band"); the raw
 *    server IP:port never headlines — it lives behind Manage.
 *
 * Pure commonMain UI so iOS inherits it (§13 nav hoist); all state + actions injected, no platform
 * types leak in. I12: the identity card is OPTIONAL — Perform works fully offline with no login.
 */

/** What Home shows about the viewer's connection to a band (the login VLL asked for + P205's future home). */
sealed interface Identity {
    /** Not connected to any server — the line invites "Connect to your band". */
    data object Disconnected : Identity

    /** Connected. [name] is empty until P205 Stage 3a resolves "Performing as <name>"; [band] shows
     *  when cheaply available. The raw server host is NOT here — it lives behind Manage. */
    data class Connected(val name: String = "", val band: String = "", val synced: Boolean = true) : Identity

    /** Known identity but offline — a reassurance, not an error (I12: concerts on device still work). */
    data class Offline(val band: String = "") : Identity
}

/** Immutable Home view state (built by the host from Storage/transport/installed bundles). Pure. */
data class HomeState(
    val lastConcertName: String = "",  // resume target; "" ⇒ no resume affordance
    val concertCount: Int = 0,
    val identity: Identity = Identity.Disconnected,
)

/** The one-line identity label — pure, unit-testable (no Compose). The raw IP:port is never here
 *  (per the ruling it lives behind Manage); name/band fill in as they become cheaply known. */
fun identityLine(identity: Identity): String = when (identity) {
    is Identity.Disconnected -> "Connect to your band"
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
}

/** The trailing affordance on the identity line: connect when disconnected, else manage the account/server. */
fun identityAction(identity: Identity): String =
    if (identity is Identity.Disconnected) "Connect" else "Manage"

@Composable
fun HomeScreen(
    state: HomeState,
    onPerform: () -> Unit,
    onResume: () -> Unit,
    onEdit: () -> Unit,
    onConcerts: () -> Unit,
    onIdentity: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(modifier.fillMaxSize()) {
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            // Small, muted wordmark — a brand mark, not a headline.
            Text(
                "TroubaShare",
                style = MaterialTheme.typography.labelLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            // TroubaStage · Perform — the ONE big primary tile (the on-stage button).
            Card(
                onClick = onPerform,
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer),
            ) {
                Column(Modifier.fillMaxWidth().padding(24.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text("▶  TroubaStage", style = MaterialTheme.typography.headlineSmall)
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

            // Secondary actions as a compact button pair: TroubaStudio (Edit) · Concerts (utility).
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                FilledTonalButton(onClick = onEdit, modifier = Modifier.weight(1f)) { Text("✎  TroubaStudio") }
                FilledTonalButton(onClick = onConcerts, modifier = Modifier.weight(1f)) { Text("⇩  Concerts") }
            }

            Spacer(Modifier.height(4.dp))

            // Identity — one clean line at the bottom; Manage/Connect opens server + account details.
            Card(onClick = onIdentity, modifier = Modifier.fillMaxWidth()) {
                Row(Modifier.fillMaxWidth().padding(16.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text("👤", style = MaterialTheme.typography.titleLarge)
                    Text(
                        identityLine(state.identity),
                        style = MaterialTheme.typography.titleMedium,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f),
                    )
                    TextButton(onClick = onIdentity) { Text(identityAction(state.identity)) }
                }
            }
        }
    }
}
