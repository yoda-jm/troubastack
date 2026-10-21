// A91 — THE concert list (TroubaStage): every concert of my bands, one row per concert, its state in plain words and
// ONE primary action. The state machine is the pure, table-tested `rowState`/`concertRows` in :shared; this file
// loads the three sources (bundles on this tablet, the server manifest, the Studio concert list), runs the actions,
// and renders. Nothing is announced and nothing installs on its own: "Update" is always a tap (VLL).
package com.troubastack.app

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.troubastack.shared.bundle.ImportResult
import com.troubastack.shared.distribution.Availability
import com.troubastack.shared.distribution.ConcertRow
import com.troubastack.shared.distribution.LocalBundle
import com.troubastack.shared.distribution.Reach
import com.troubastack.shared.distribution.RowAction
import com.troubastack.shared.distribution.RowMenuItem
import com.troubastack.shared.distribution.RowOp
import com.troubastack.shared.distribution.RowSection
import com.troubastack.shared.distribution.ServerConcert
import com.troubastack.shared.distribution.ServerSong
import com.troubastack.shared.distribution.UpdateProgress
import com.troubastack.shared.distribution.UpdatesManager
import com.troubastack.shared.distribution.agoText
import com.troubastack.shared.distribution.concertRows
import com.troubastack.shared.distribution.setlistIdOf
import com.troubastack.shared.seams.Storage
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import java.time.LocalDate

/** A bundle on this tablet as the list sees it: the pure model plus the folder it lives in. */
internal data class LocalCopy(val bundle: LocalBundle, val dir: String)

private const val K_SERVER_CACHE = "concertList.server"
private val cacheJson = Json { ignoreUnknownKeys = true; encodeDefaults = true }

/** The last server answer, kept so an offline list can still say "last checked …" and "v7 seen on the server". */
@Serializable
private data class ServerCache(val checkedAt: Long = 0, val concerts: List<ServerConcert> = emptyList())

@Composable
internal fun ConcertListPane(
    storage: Storage,
    transport: HttpTransport,
    updates: UpdatesManager,
    connected: Boolean,
    refreshKey: Int,
    listLocal: () -> List<LocalCopy>,
    onOpen: (dir: String) -> Unit,
    onDeleted: (concertId: String) -> Unit,
    onChanged: () -> Unit,
    message: (String) -> Unit,
) {
    val scope = rememberCoroutineScope()
    var cache by remember { mutableStateOf(runCatching { cacheJson.decodeFromString<ServerCache>(storage.getSecret(K_SERVER_CACHE) ?: "") }.getOrDefault(ServerCache())) }
    var online by remember { mutableStateOf(false) }
    var refreshing by remember { mutableStateOf(false) }
    var tick by remember { mutableStateOf(0) } // re-reads the tablet (after a download, delete, freeze…)
    val ops = remember { mutableStateMapOf<String, RowOp>() }
    val jobs = remember { mutableMapOf<String, Job>() }
    var showArchived by remember { mutableStateOf(false) }
    var confirmDelete by remember { mutableStateOf<ConcertRow?>(null) }
    var changesOf by remember { mutableStateOf<ConcertRow?>(null) }

    val locals = remember(tick, refreshKey) { listLocal() }
    val dirOf = locals.associate { it.bundle.id to it.dir }

    fun refresh() {
        if (refreshing) return
        if (!connected) { online = false; return }
        refreshing = true
        scope.launch {
            val manifest = runCatching { transport.fetchManifest() }.getOrNull()
            val studio = runCatching { transport.fetchStudioConcerts() }.getOrNull().orEmpty()
            if (manifest == null) {
                online = false
            } else {
                val bySetlist = studio.associateBy { it.setlistId }
                val servers = manifest.concerts.filter { it.concertId.isNotEmpty() }.map { c ->
                    val s = bySetlist[setlistIdOf(c.concertId)]
                    ServerConcert(
                        id = c.concertId, rev = c.currentRev, name = s?.name ?: c.name, bandId = s?.bandId ?: "", bandName = s?.bandName ?: "",
                        eventDate = s?.eventDate ?: "", venue = s?.venue ?: "", finalLocked = c.finalLocked, archived = c.archived,
                        songs = c.songs.map { ServerSong(it.songId, it.rev) }, isAdmin = s?.isAdmin ?: false,
                        live = s?.liveUntil?.let { runCatching { java.time.Instant.parse(it).isAfter(java.time.Instant.now()) }.getOrDefault(false) } ?: false,
                        // A92/T201: admins-only "edited since its last bake" + what changed (null = unknown).
                        editedSinceBake = transport.lastExtras[c.concertId]?.editedSinceBake, edits = transport.lastExtras[c.concertId]?.changes,
                    )
                }
                cache = ServerCache(System.currentTimeMillis() / 1000, servers)
                storage.putSecret(K_SERVER_CACHE, cacheJson.encodeToString(ServerCache.serializer(), cache))
                online = true
            }
            refreshing = false
        }
    }
    // The list refreshes when it opens, when the connection changes, and on ↻ (spec rule 3).
    LaunchedEffect(connected, refreshKey) { refresh() }

    val nowSec = System.currentTimeMillis() / 1000
    val reach: Reach = if (online) Reach.Online(cache.checkedAt) else Reach.Offline(cache.checkedAt.takeIf { it > 0 })
    val frozen = locals.map { it.bundle.id }.filter { updates.isFrozen(it) }.toSet()
    val rows = concertRows(locals.map { it.bundle }, cache.concerts, reach, frozen, ops.toMap(), nowSec, LocalDate.now().toString())
    val main = rows.filter { it.section == RowSection.MAIN }
    val archived = rows.filter { it.section == RowSection.ARCHIVED }

    fun runFetch(row: ConcertRow, isUpdate: Boolean) {
        val server = cache.concerts.firstOrNull { it.id == row.id }
        val localRev = locals.firstOrNull { it.bundle.id == row.id }?.bundle?.rev ?: 0uL
        val progress: (UpdateProgress) -> Unit = { p ->
            val f = (p as? UpdateProgress.Downloading)?.let { if (it.contentLength > 0) (it.bytesRead.toFloat() / it.contentLength).coerceIn(0f, 1f) else null }
            ops[row.id] = if (isUpdate) RowOp.Updating(f) else RowOp.Downloading(f)
        }
        ops[row.id] = if (isUpdate) RowOp.Updating(0f) else RowOp.Downloading(0f)
        jobs[row.id] = scope.launch {
            try {
                // A damaged copy is re-downloaded as a download; an Update replaces the installed copy atomically.
                val r = if (isUpdate) updates.apply(Availability.UpdateOffered(row.id, localRev, server?.rev ?: 0uL), progress) else updates.download(row.id, progress)
                message(when (r) { is ImportResult.Imported -> "Downloaded «${row.name}»"; is ImportResult.Failed -> r.reason })
            } finally {
                ops.remove(row.id); jobs.remove(row.id); tick++; onChanged()
            }
        }
    }

    fun reBake(row: ConcertRow) {
        ops[row.id] = RowOp.Baking("")
        jobs[row.id] = scope.launch {
            try {
                runReBake(transport, row.id) { ops[row.id] = RowOp.Baking(it.removePrefix("Baking").trim()) }?.let(message)
            } finally {
                ops.remove(row.id); jobs.remove(row.id); refresh() // the row turns "New version" by itself
            }
        }
    }

    fun primary(row: ConcertRow) {
        when (row.primary) {
            RowAction.OPEN -> dirOf[row.id]?.let(onOpen)
            RowAction.UPDATE -> runFetch(row, isUpdate = true)
            RowAction.DOWNLOAD -> runFetch(row, isUpdate = false)
            RowAction.CANCEL -> jobs[row.id]?.cancel()
            RowAction.NONE -> {}
        }
    }

    fun menu(row: ConcertRow, item: RowMenuItem) {
        when (item) {
            RowMenuItem.OPEN_THIS_VERSION -> dirOf[row.id]?.let(onOpen)
            RowMenuItem.FREEZE -> { updates.freeze(row.id); tick++ }
            RowMenuItem.UNFREEZE -> { updates.unfreeze(row.id); tick++ }
            RowMenuItem.REBAKE -> reBake(row)
            RowMenuItem.LIVE_ON, RowMenuItem.LIVE_OFF -> scope.launch {
                val ok = runCatching { transport.setConcertLive(row.bandId, setlistIdOf(row.id), item == RowMenuItem.LIVE_ON) }.getOrDefault(false)
                if (!ok) message("Couldn't change live mode")
                refresh()
            }
            RowMenuItem.DELETE -> confirmDelete = row
        }
    }

    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                when {
                    refreshing -> "Checking the server…"
                    !connected -> "Not connected to a server" + (cache.checkedAt.takeIf { it > 0 }?.let { " · last checked ${agoText(nowSec, it)}" } ?: "")
                    cache.checkedAt > 0 -> (if (online) "Last checked " else "Server unreachable · last checked ") + agoText(nowSec, cache.checkedAt)
                    else -> "Not checked yet"
                },
                Modifier.weight(1f), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            if (connected) TextButton(onClick = { refresh() }, enabled = !refreshing) { Text("↻ Refresh") }
        }
        if (rows.isEmpty()) {
            Text("No concerts yet. Import a .tstage file, or connect to your band's server.", style = MaterialTheme.typography.bodyMedium)
        }
        LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            var lastBand: String? = null
            main.forEach { row ->
                val band = row.bandName.ifBlank { "On this tablet only" }
                if (band != lastBand) {
                    lastBand = band
                    item(key = "band-${row.bandId}-$band") { Text(band, style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(top = 8.dp)) }
                }
                item(key = "row-${row.id}") { ConcertListRow(row, onPrimary = { primary(row) }, onMenu = { menu(row, it) }, onChanges = { changesOf = row }) }
            }
            if (archived.isNotEmpty()) {
                item(key = "archived-header") {
                    Row(Modifier.fillMaxWidth().clickable { showArchived = !showArchived }.padding(top = 16.dp, bottom = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text(if (showArchived) "▾" else "▸", style = MaterialTheme.typography.titleMedium)
                        Spacer(Modifier.width(8.dp))
                        Text("Archived on this tablet (${archived.size})", style = MaterialTheme.typography.titleSmall)
                    }
                }
                if (showArchived) items(archived, key = { "arch-" + it.id }) { row ->
                    ConcertListRow(row, onPrimary = { primary(row) }, onMenu = { menu(row, it) }, onChanges = {})
                }
            }
        }
    }

    confirmDelete?.let { row ->
        AlertDialog(
            onDismissRequest = { confirmDelete = null },
            title = { Text("Delete from this tablet?") },
            text = { Text("“${row.name}” will be removed from this tablet, with its rehearsal notes. " + if (cache.concerts.any { it.id == row.id && !it.archived }) "It stays on the server and can be downloaded again." else "This tablet may hold the only copy.") },
            confirmButton = {
                TextButton(onClick = {
                    confirmDelete = null
                    dirOf[row.id]?.let { File(it).deleteRecursively() }
                    onDeleted(row.id); tick++; onChanged()
                }) { Text("Delete") }
            },
            dismissButton = { TextButton(onClick = { confirmDelete = null }) { Text("Cancel") } },
        )
    }
    changesOf?.let { row ->
        val c = row.changes
        AlertDialog(
            onDismissRequest = { changesOf = null },
            title = { Text("What changed in «${row.name}»") },
            text = {
                Text(buildString {
                    if (c == null || c.isEmpty) append("The songs are the same; the pages or annotations changed.")
                    else {
                        if (c.changed.isNotEmpty()) append("Changed: " + c.changed.joinToString(", ") + "\n")
                        if (c.added > 0) append("Added: ${c.added} song${if (c.added == 1) "" else "s"}\n")
                        if (c.removed.isNotEmpty()) append("Removed: " + c.removed.joinToString(", "))
                    }
                }.trim())
            },
            confirmButton = { TextButton(onClick = { changesOf = null }) { Text("OK") } },
        )
    }
}

@Composable
private fun ConcertListRow(row: ConcertRow, onPrimary: () -> Unit, onMenu: (RowMenuItem) -> Unit, onChanges: () -> Unit) {
    var menuOpen by remember { mutableStateOf(false) }
    Card(Modifier.fillMaxWidth()) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(row.name, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                val where = listOf(row.eventDate.take(10), row.venue).filter { it.isNotBlank() }.joinToString(" · ")
                if (where.isNotEmpty()) Text(where, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                val tappable = row.changes != null && !row.changes!!.isEmpty
                Text(
                    row.line, style = MaterialTheme.typography.bodySmall,
                    color = if (row.primary == RowAction.UPDATE) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = if (tappable) Modifier.clickable(onClick = onChanges) else Modifier,
                )
            }
            val label = when (row.primary) {
                RowAction.OPEN -> "Open"; RowAction.UPDATE -> "Update"; RowAction.DOWNLOAD -> "Download"; RowAction.CANCEL -> "Cancel"; RowAction.NONE -> null
            }
            if (label != null) {
                if (row.primary == RowAction.OPEN || row.primary == RowAction.CANCEL) OutlinedButton(onClick = onPrimary, enabled = row.primaryEnabled) { Text(label) }
                else Button(onClick = onPrimary, enabled = row.primaryEnabled) { Text(label) }
            }
            if (row.menu.isNotEmpty()) {
                Box {
                    TextButton(onClick = { menuOpen = true }) { Text("⋯") }
                    DropdownMenu(expanded = menuOpen, onDismissRequest = { menuOpen = false }) {
                        row.menu.forEach { item ->
                            val text = when (item) {
                                RowMenuItem.OPEN_THIS_VERSION -> "Open this version"
                                RowMenuItem.FREEZE -> "Freeze this version"
                                RowMenuItem.UNFREEZE -> "Unfreeze"
                                RowMenuItem.REBAKE -> "Re-bake"
                                RowMenuItem.LIVE_ON -> "Rehearsal live mode · auto-bakes for 3 h"
                                RowMenuItem.LIVE_OFF -> "Stop rehearsal live mode"
                                RowMenuItem.DELETE -> "Delete from tablet…"
                            }
                            DropdownMenuItem(text = { Text(text) }, onClick = { menuOpen = false; onMenu(item) })
                        }
                    }
                }
            }
        }
    }
}
