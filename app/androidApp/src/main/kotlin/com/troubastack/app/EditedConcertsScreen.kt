// A92 — "Edited since their last bake": the concerts the server (T201) reports as edited since their last bake, each
// with what changed in plain words and a Re-bake button with inline progress. A re-baked concert leaves the list on its
// own: the next listing says it is no longer edited. Admins only — the server sends the flag to admins only.
package com.troubastack.app

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.troubastack.shared.distribution.ServerConcert
import com.troubastack.shared.distribution.editedConcerts
import com.troubastack.shared.distribution.editsLine
import kotlinx.coroutines.launch

/** The server's concerts with the T201 extras merged in (name from the manifest, edited flag + changes + date). */
internal suspend fun fetchServerConcertsWithEdits(transport: HttpTransport): List<ServerConcert>? {
    val manifest = runCatching { transport.fetchManifest() }.getOrNull() ?: return null
    val extras = transport.lastExtras
    return manifest.concerts.filter { it.concertId.isNotEmpty() }.map { c ->
        val x = extras[c.concertId]
        ServerConcert(
            id = c.concertId, rev = c.currentRev, name = c.name.ifBlank { c.concertId }, eventDate = x?.eventDate ?: "",
            finalLocked = c.finalLocked, archived = c.archived, editedSinceBake = x?.editedSinceBake, edits = x?.changes,
        )
    }
}

@Composable
internal fun EditedConcertsScreen(transport: HttpTransport, onBack: () -> Unit) {
    val scope = rememberCoroutineScope()
    var concerts by remember { mutableStateOf<List<ServerConcert>?>(null) }
    var failed by remember { mutableStateOf(false) }
    var tick by remember { mutableStateOf(0) }
    val baking = remember { mutableStateMapOf<String, String>() } // concert id → live bake line
    val errors = remember { mutableStateMapOf<String, String>() }
    BackHandler { onBack() }
    LaunchedEffect(tick) {
        val all = fetchServerConcertsWithEdits(transport)
        failed = all == null
        if (all != null) concerts = editedConcerts(all)
    }
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize().statusBarsPadding().padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                TextButton(onClick = onBack) { Text("‹  Home") }
                Text("Edited since their last bake", style = MaterialTheme.typography.headlineSmall, modifier = Modifier.weight(1f))
                TextButton(onClick = { tick++ }) { Text("↻ Refresh") }
            }
            val list = concerts
            when {
                failed -> Text("Couldn't reach the server.", style = MaterialTheme.typography.bodyMedium)
                list == null -> Text("Checking…", style = MaterialTheme.typography.bodyMedium)
                list.isEmpty() -> Text("Every concert is baked with its latest edits.", style = MaterialTheme.typography.bodyMedium)
                else -> LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    items(list, key = { it.id }) { c ->
                        Card(Modifier.fillMaxWidth()) {
                            Row(Modifier.fillMaxWidth().padding(16.dp), verticalAlignment = Alignment.CenterVertically) {
                                Column(Modifier.weight(1f)) {
                                    Text(c.name, style = MaterialTheme.typography.titleMedium)
                                    if (c.eventDate.isNotBlank()) Text(c.eventDate.take(10), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                    val what = baking[c.id] ?: errors[c.id] ?: editsLine(c.edits).ifEmpty { "Edited since its last bake" }
                                    Text(what, style = MaterialTheme.typography.bodySmall,
                                        color = if (errors[c.id] != null) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                                Button(enabled = baking[c.id] == null, onClick = {
                                    errors.remove(c.id); baking[c.id] = "Baking…"
                                    scope.launch {
                                        val err = runReBake(transport, c.id) { baking[c.id] = it }
                                        baking.remove(c.id)
                                        if (err != null) errors[c.id] = err else tick++ // re-list: a baked concert leaves on its own
                                    }
                                }) { Text(if (baking[c.id] != null) "Baking…" else "Re-bake") }
                            }
                        }
                    }
                }
            }
        }
    }
}
