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
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp

/**
 * A27 — the app's HOME landing page (VLL: "a landing page that is not the bake list, so we clearly
 * identify the products inside the app and can log in from here"). Cold start lands HERE, never the
 * concerts list. Large stage-friendly product tiles + one identity card. Pure commonMain UI so iOS
 * inherits it (the §13 nav hoist); all state + actions are injected, no platform types leak in.
 *
 * I12: the identity card is OPTIONAL — Perform works fully offline/sideloaded with no login. The
 * landing must NEVER gate Stage.
 */

/** What Home shows about the viewer's connection to a band (the login VLL asked for + P205's future home). */
sealed interface Identity {
    /** Not connected to any server — the card invites "Connect to your band". */
    data object Disconnected : Identity

    /** Connected & reachable: name · server · in-sync. */
    data class Connected(val name: String, val server: String, val synced: Boolean) : Identity

    /** Known identity but currently offline — renders the last-synced state, still fully usable. */
    data class Offline(val name: String, val lastSynced: String) : Identity
}

/** Immutable Home view state (built by the host from Storage/transport/installed bundles). Pure. */
data class HomeState(
    val bandName: String = "",         // shown in the header once connected; "" hides it
    val lastConcertName: String = "",  // resume target; "" ⇒ no resume affordance
    val concertCount: Int = 0,
    val identity: Identity = Identity.Disconnected,
)

/** The label under the identity card for a given [Identity] — pure, unit-testable (no Compose).
 *  Name may be empty until P205 Stage 3a resolves "performing as <name>"; the line degrades to the
 *  server (or a plain "Connected") so the card is never a lone separator. */
fun identityLine(identity: Identity): String = when (identity) {
    is Identity.Disconnected -> "Connect to your band"
    is Identity.Connected -> buildString {
        if (identity.name.isNotEmpty()) append(identity.name).append(" · ")
        append(if (identity.server.isNotEmpty()) identity.server else "Connected")
        append(if (identity.synced) " ✓" else " · syncing…")
    }
    is Identity.Offline -> buildString {
        if (identity.name.isNotEmpty()) append(identity.name).append(" · ")
        append("offline")
        if (identity.lastSynced.isNotEmpty()) append(" · last synced ").append(identity.lastSynced)
    }
}

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
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                Text("TroubaShare", style = MaterialTheme.typography.headlineMedium)
                if (state.bandName.isNotEmpty()) {
                    Surface(shape = MaterialTheme.shapes.small, color = MaterialTheme.colorScheme.secondaryContainer) {
                        Text(state.bandName, style = MaterialTheme.typography.labelLarge, modifier = Modifier.padding(horizontal = 10.dp, vertical = 4.dp))
                    }
                }
            }

            // Primary product: PERFORM — the biggest tile, with a one-tap resume of the last concert.
            ProductTile(
                title = "▶  Perform",
                subtitle = when {
                    state.lastConcertName.isNotEmpty() -> state.lastConcertName
                    state.concertCount > 0 -> "Open a concert"
                    else -> "Import or download a concert to start"
                },
                footnote = if (state.concertCount > 0) "${state.concertCount} on device" else "",
                onClick = onPerform,
                primary = true,
                action = if (state.lastConcertName.isNotEmpty()) TileAction("Resume «${state.lastConcertName}»", onResume) else null,
            )

            // Secondary products: EDIT (embedded Studio, A16/T46) + CONCERTS (download/update offers, B03).
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                ProductTile("✎  Edit", "Studio", "", onEdit, modifier = Modifier.weight(1f))
                ProductTile("⇩  Concerts", "Get & update", "", onConcerts, modifier = Modifier.weight(1f))
            }

            Spacer(Modifier.height(4.dp))

            // Identity card — the login VLL asked for; the future home of P205's resolved identity.
            Card(onClick = onIdentity, modifier = Modifier.fillMaxWidth()) {
                Row(Modifier.fillMaxWidth().padding(16.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text("👤", style = MaterialTheme.typography.headlineSmall)
                    Text(
                        identityLine(state.identity),
                        style = MaterialTheme.typography.titleMedium,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f),
                    )
                }
            }
        }
    }
}

private data class TileAction(val label: String, val onClick: () -> Unit)

@Composable
private fun ProductTile(
    title: String,
    subtitle: String,
    footnote: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    primary: Boolean = false,
    action: TileAction? = null,
) {
    Card(
        onClick = onClick,
        modifier = modifier.fillMaxWidth(),
        colors = if (primary) CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer) else CardDefaults.cardColors(),
    ) {
        Column(Modifier.fillMaxWidth().padding(if (primary) 24.dp else 16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(title, style = if (primary) MaterialTheme.typography.headlineSmall else MaterialTheme.typography.titleLarge)
            if (subtitle.isNotEmpty()) Text(subtitle, style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (footnote.isNotEmpty()) Text(footnote, style = MaterialTheme.typography.bodySmall)
            if (action != null) {
                Spacer(Modifier.height(8.dp))
                Surface(onClick = action.onClick, shape = MaterialTheme.shapes.small, color = MaterialTheme.colorScheme.primary, contentColor = MaterialTheme.colorScheme.onPrimary) {
                    Text(action.label, style = MaterialTheme.typography.labelLarge, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp))
                }
            }
        }
    }
}
