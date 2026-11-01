package com.troubastack.app

import com.troubastack.shared.ui.LocalMaxContentWidth
import com.troubastack.shared.ui.readableWidth
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.foundation.Image
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.TabRowDefaults
import androidx.compose.material3.TabRowDefaults.tabIndicatorOffset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.painter.BitmapPainter
import com.troubastack.shared.stage.notes.NoteIndex
import com.troubastack.shared.stage.notes.NotesWarning
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import com.troubastack.shared.stage.notes.NoteEntry
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.troubastack.shared.bundle.BundleImporter
import com.troubastack.shared.bundle.BundleLoader
import com.troubastack.shared.bundle.ImportResult
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.distribution.UpdatesManager
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.ui.LocalBrandAccents
import kotlinx.coroutines.launch
import java.io.File

@Composable
internal fun ConcertsScreen(
    platform: AppPlatform,
    storage: Storage,
    transport: HttpTransport,
    updates: UpdatesManager,
    onOpen: (String) -> Unit,
    onHome: () -> Unit,
) {
    var refresh by remember { mutableStateOf(0) }
    var message by remember { mutableStateOf<String?>(null) }
    val connected = transport.isConnected
    // A70 §5.3 — the Stage section is two tabs: Concerts (A91's one list) | Notes (rehearsal-note management).
    var stageTab by remember { mutableStateOf(0) }
    // A78 (Fable conditional GO): the Notes tab's "Send all (N)" lives in THIS title row's empty right side,
    // not on a row of its own (≈ 56 dp back for the list). NotesTab publishes it here; see NotesSendAllHandle.
    val notesSendAll = remember { NotesSendAllHandle() }

    // Import a .tstage through the platform's picker (none ⇒ no Import button): it hands over a temp copy.
    val pickBundle = platform.bundlePicker?.rememberLauncher { temp ->
        message = when (val r = BundleImporter(AndroidImportFs(storage)).import(temp)) {
            // A81: name the concert, not its id — the id is a UUID nobody can read.
            is ImportResult.Imported ->
                "Imported «${listConcerts(storage).firstOrNull { it.concertId == r.concertId }?.label ?: r.concertId}»"
            is ImportResult.Failed -> r.reason
        }
        File(temp).delete()
        refresh++
    }

    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize().readableWidth(LocalMaxContentWidth.current).statusBarsPadding().padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            // A31: a visible Home affordance on every sub-screen (placement consistent with the Edit
            // bar's "‹ Back"), so returning to the landing page never depends on the system-back gesture.
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                TextButton(onClick = onHome) { Text("‹  Home") }
                // A65 (VLL): the Stage native page wears the Stage brand — two-tone "TroubaStage", gold.
                Text(
                    brandTitle("Stage", accent = LocalBrandAccents.current.stage, ink = MaterialTheme.colorScheme.onSurface),
                    style = MaterialTheme.typography.headlineMedium,
                    modifier = Modifier.weight(1f),
                )
                // A78 ⟨D1⟩ — top-level "Send all (N)", only while the Notes tab is selected.
                val line = notesSendAll.line
                if (stageTab == 1 && line is SendAllLine.Shown) {
                    Button(onClick = { notesSendAll.onClick() }, enabled = line.enabled) {
                        Text(if (line.sending) "Sending…" else "Send all (${line.count})")
                    }
                }
                // A81/A91: Import lives once, in the list's top bar (works with no server).
                if (stageTab == 0 && pickBundle != null) {
                    TextButton(onClick = pickBundle) { Text("Import a .tstage…") }
                }
            }
            // A70 §5.3 — Concerts | Notes tabs, Stage accent. The Notes tab label carries the whole nag.
            val stageAccent = LocalBrandAccents.current.stage
            val notesWarn = remember(refresh) { allNotesWarning(platform, storage) }
            TabRow(
                selectedTabIndex = stageTab, contentColor = stageAccent,
                indicator = { pos -> TabRowDefaults.SecondaryIndicator(Modifier.tabIndicatorOffset(pos[stageTab]), color = stageAccent) },
            ) {
                Tab(selected = stageTab == 0, onClick = { stageTab = 0 }, text = { Text("Concerts") }, selectedContentColor = stageAccent, unselectedContentColor = MaterialTheme.colorScheme.onSurfaceVariant)
                Tab(
                    selected = stageTab == 1, onClick = { stageTab = 1 },
                    text = { Text(if (notesWarn == NotesWarning.NONE) "Notes" else "Notes ⚠") },
                    selectedContentColor = when (notesWarn) { NotesWarning.RED -> Color(0xFFD32F2F); NotesWarning.ORANGE -> Color(0xFFF57C00); else -> stageAccent },
                    unselectedContentColor = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            message?.let { Text(it, style = MaterialTheme.typography.bodyMedium) }
            if (stageTab == 0) {
                // A91 — the ONE concert list (server ∪ tablet), state + one action per row.
                ConcertListPane(
                    storage = storage, transport = transport, updates = updates, connected = connected, refreshKey = refresh,
                    listLocal = { listConcerts(storage).map { e -> LocalCopy(e.toLocalBundle(), e.dir) } },
                    onOpen = onOpen,
                    // A70 §3.3: deleting a bundle removes its rehearsal notes too (they live beside it).
                    onDeleted = { cid -> if (cid.isNotEmpty() && !cid.startsWith(DAMAGED_PREFIX)) platform.rehearsalNotes(storage.notesDir()).deleteAll(cid) },
                    onChanged = { refresh++ },
                    message = { message = it },
                )
            } else {
                NotesTab(platform, storage, transport, connected, onChanged = { refresh++ }, sendAllHandle = notesSendAll)
            }
        }
    }
}

/** A91 — a damaged bundle has no readable concert id; its row is keyed by its folder so Delete still finds it. */
private const val DAMAGED_PREFIX = "damaged:"

/** A91 — a listed bundle as the pure list model sees it. */
private fun ConcertEntry.toLocalBundle() = com.troubastack.shared.distribution.LocalBundle(
    id = if (damaged || concertId.isEmpty()) DAMAGED_PREFIX + dir else concertId,
    rev = concertRev, bakedAt = bakedAt, name = label, bandId = bandId, bandName = bandName, damaged = damaged, songs = songs,
)

// A70 §5 — every stored rehearsal note across all installed concerts, paired with its concert id.
private fun allNoteEntries(platform: AppPlatform, storage: Storage): List<Pair<String, com.troubastack.shared.stage.notes.NoteEntry>> {
    val port = platform.rehearsalNotes(storage.notesDir())
    return listConcerts(storage).flatMap { c -> port.index(c.concertId).map { c.concertId to it } }
}

private fun allNotesWarning(platform: AppPlatform, storage: Storage): NotesWarning =
    NoteIndex.warning(allNoteEntries(platform, storage).map { it.second })

/**
 * A70 §5.3 / T170 §6 — the Notes tab: every rehearsal note grouped band → concert → song → page, each with
 * See / Delete / Send. Send (Part B) uploads the note's PNG to Studio, where it prints as a reference
 * underlay beneath the annotation layers (T170 §1 — never a bakeable object). A concert node can BULK-send
 * every note under it in one tap. The tab LABEL carries the `⚠` nag (computed by the caller); each old note
 * also spells it out. This is the only place a note can be deleted, viewed off the page, or sent.
 */
@Composable
private fun NotesTab(platform: AppPlatform, storage: Storage, transport: HttpTransport, connected: Boolean, onChanged: () -> Unit = {}, sendAllHandle: NotesSendAllHandle? = null) {
    var refresh by remember { mutableStateOf(0) }
    val notes = remember(refresh) { allNoteEntries(platform, storage) }
    val port = remember { platform.rehearsalNotes(storage.notesDir()) }
    val labels = remember(refresh) { listConcerts(storage).associate { it.concertId to it.label } }
    var see by remember { mutableStateOf<Pair<String, com.troubastack.shared.stage.notes.NoteEntry>?>(null) }
    // A70 §5 (VLL): a collapsible band → concert → song → page tree, so a page-heavy concert folds away in one
    // tap instead of a flat wall of cards. A key in [collapsed] is folded; default is everything expanded.
    var collapsed by remember { mutableStateOf(emptySet<String>()) }
    fun toggle(k: String) { collapsed = if (k in collapsed) collapsed - k else collapsed + k }
    // A75 ⟨D3⟩: sent notes step back into a "Sent (N)" group, COLLAPSED by default (revises A74 ⟨D2⟩'s
    // "dimmed" — on a 686 dp panel a dimmed row costs the same as a bright one).
    var sentCollapsed by remember { mutableStateOf(true) }
    // T170 §6 — send state. [busy] holds the ids of notes currently uploading (per-note spinner + disable
    // + a concert node's "Sending…"); [status] is the one-line outcome. [myId] is the signed-in user, for
    // the identity prompt; [owAsk]/[idAsk] are the two confirmations the spec requires before a send.
    val scope = rememberCoroutineScope()
    var busy by remember { mutableStateOf(emptySet<String>()) }
    var status by remember { mutableStateOf<String?>(null) }
    var myId by remember { mutableStateOf<String?>(null) }
    var owAsk by remember { mutableStateOf<Pair<String, NoteEntry>?>(null) }
    // idAsk: the items to send once the "under your account" identity prompt is confirmed, + whether it was a
    // bulk request (so the confirm resumes the right path). §6 step 1.
    var idAsk by remember { mutableStateOf<Pair<List<Pair<String, NoteEntry>>, Boolean>?>(null) }
    // A78 ⟨D3⟩ — the sent notes a "Clear" would remove, awaiting the confirmation that states the consequence.
    var clearAsk by remember { mutableStateOf<List<NoteItem>?>(null) }
    LaunchedEffect(connected) { if (connected) myId = transport.myUserId() }
    fun noteId(cid: String, n: NoteEntry) = "$cid:${n.file}"

    // One note. [overwrite] off for a first send so a 409 surfaces the overwrite prompt; a Re-send passes it.
    fun launchSend(cid: String, n: NoteEntry, overwrite: Boolean) {
        val id = noteId(cid, n); busy = busy + id; status = "Sending…"
        scope.launch {
            val png = port.pngBytes(cid, n.key)
            val res = if (png == null) NoteSendResult.Failed("Couldn't read the note") else transport.sendRehearsalNote(cid, n, png, overwrite)
            busy = busy - id
            when (res) {
                NoteSendResult.Ok -> { port.markSent(cid, n.key, System.currentTimeMillis()); status = "Sent to Studio ✓"; refresh++; onChanged() }
                NoteSendResult.Exists -> { status = null; owAsk = cid to n } // §3.2 409 → ask before clobbering
                is NoteSendResult.Failed -> status = res.message
            }
        }
    }

    // A whole node. ⟨D1⟩ R2: a note already SENT (sentAt set) is skipped and the skip is reported. A 409 on a
    // note that is NOT locally sent is a CONFLICT, not a skip: Studio holds SOME note for this (owner, song,
    // page), but not necessarily THIS one — a re-drawn note resets sentAt to null, so bulk would ship the newer
    // drawing, get a 409, and (the old bug) mark it "sent" while Studio kept the old version. So a conflict must
    // NEVER write sentAt; it is reported as still outstanding and stays individually retryable (per-note Send →
    // the overwrite prompt). (Fable T170 §6 review — the once-per-batch overwrite ask is deferred to the redesign.)
    fun launchBulk(items: List<Pair<String, NoteEntry>>) {
        val ids = items.map { (c, n) -> noteId(c, n) }.toSet(); busy = busy + ids; status = "Sending…"
        scope.launch {
            var ok = 0; var skipped = 0; var conflict = 0; var fail = 0
            for ((cid, n) in items) {
                val already = n.sentAt != null
                // Never upload an already-sent note; otherwise ask the server. The result → outcome mapping is
                // the pure, tested [bulkNoteOutcome] — the one place the "409 ⇒ conflict, no marker" rule lives.
                val result = if (already) null else {
                    val png = port.pngBytes(cid, n.key)
                    if (png == null) NoteSendResult.Failed("Couldn't read the note") else transport.sendRehearsalNote(cid, n, png, overwrite = false)
                }
                when (bulkNoteOutcome(already, result)) {
                    BulkOutcome.SKIPPED -> skipped++
                    BulkOutcome.SENT -> { port.markSent(cid, n.key, System.currentTimeMillis()); ok++ }
                    BulkOutcome.CONFLICT -> conflict++ // no markSent — the note stays outstanding + retryable
                    BulkOutcome.FAILED -> fail++
                }
            }
            busy = busy - ids
            val parts = buildList {
                if (ok > 0) add("$ok sent")
                if (skipped > 0) add("$skipped already sent")
                if (conflict > 0) add("$conflict need overwrite")
                if (fail > 0) add("$fail failed")
            }
            status = if (parts.isEmpty()) "Nothing to send" else parts.joinToString(" · ") + if (fail == 0 && conflict == 0) " ✓" else ""
            refresh++; onChanged()
        }
    }

    // Entry points — gate each on the identity prompt first (§6 step 1), then run.
    fun onSendTap(cid: String, n: NoteEntry) {
        if (!connected) return
        if (needsIdentityPrompt(listOf(cid to n), myId)) idAsk = listOf(cid to n) to false else launchSend(cid, n, overwrite = n.sentAt != null)
    }
    fun onSendAll(items: List<Pair<String, NoteEntry>>) {
        if (!connected || items.isEmpty()) return
        if (needsIdentityPrompt(items, myId)) idAsk = items to true else launchBulk(items) // one rule, every entry point
    }
    if (notes.isEmpty()) {
        Text("No rehearsal notes yet. Open a concert and tap ✎ to take one.", style = MaterialTheme.typography.bodyMedium)
        return
    }
    // A75/A78 — render one set (unsent, or the Sent group) from the pure [noteRows] plan (fold rules tested
    // off-device). A78 ⟨D2⟩: the concert header is ALWAYS drawn — it is what collapses and what carries the
    // node action; the band header still folds with one band (its name rides on the concert header). The node
    // action is "Send all" on the unsent side and "Clear" on the Sent side (⟨D3⟩) — never Clear on unsent notes.
    fun nodeAction(items: List<NoteItem>, sentSide: Boolean): HeaderAction? = when (nodeActionKind(sentSide, connected)) {
        NodeActionKind.CLEAR -> HeaderAction("Clear", enabled = true) { clearAsk = sentToClear(items) }
        NodeActionKind.SEND_ALL -> {
            val sending = items.any { (c, n) -> noteId(c, n) in busy }
            HeaderAction(if (sending) "Sending…" else "Send all", enabled = !sending) { onSendAll(items) }
        }
        NodeActionKind.NONE -> null
    }
    fun renderSection(ls: LazyListScope, items: List<NoteItem>, kp: String, sentSide: Boolean) {
        noteRows(items, kp, labels, collapsed).forEach { row ->
            when (row) {
                is NoteRow.Band -> ls.item(key = row.key) {
                    GroupHeader(row.label, row.items.size, indent = 4.dp, collapsed = row.key in collapsed, action = nodeAction(row.items, sentSide)) { toggle(row.key) }
                }
                is NoteRow.Concert -> ls.item(key = row.key) {
                    GroupHeader(row.label, row.items.size, indent = row.indentDp.dp, collapsed = row.key in collapsed, action = nodeAction(row.items, sentSide)) { toggle(row.key) }
                }
                is NoteRow.Song -> ls.item(key = row.key) {
                    Text(row.label, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(start = 32.dp, top = 4.dp))
                }
                is NoteRow.Leaf -> ls.item(key = row.key) {
                    NoteLeaf(row.label, old = NoteIndex.isOld(row.item.second), indent = row.indentDp.dp) { see = row.item }
                }
            }
        }
    }
    val unsent = notes.filter { it.second.sentAt == null }
    val sent = notes.filter { it.second.sentAt != null }
    // Status + offline hint live ABOVE the list (a plain composable, not a lazy item — a conditional
    // item() driven by a coroutine-written state didn't reliably re-emit, so the "already sent"/"Sent ✓"
    // line — which ⟨D1⟩ R2 requires be SAID — never showed).
    Column(Modifier.fillMaxSize()) {
        status?.let { Text(it, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.primary, modifier = Modifier.padding(vertical = 4.dp)) }
        // A78 ⟨D1⟩ — the top-level "Send all (N)" over every band and concert, through the SAME bulk path as a
        // node (identity check → launchBulk). Rendered in the screen's title row (no row of its own); this
        // only publishes its state + action. The Connect hint below stays in the tab.
        val sendAll = sendAllLine(unsent.size, connected, sending = unsent.any { (c, n) -> noteId(c, n) in busy })
        if (sendAllHandle != null) {
            SideEffect { sendAllHandle.line = sendAll; sendAllHandle.onClick = { onSendAll(unsent) } }
            DisposableEffect(sendAllHandle) { onDispose { sendAllHandle.line = SendAllLine.Hidden } }
        }
        if (!connected) Text("Connect (Bakes tab) to send notes to Studio.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(bottom = 4.dp))
        LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            renderSection(this, unsent, "u", sentSide = false)
            // ⟨D3⟩ — the Sent group, collapsed by default; ⟨D1⟩ applies (no header when there are no sent notes).
            if (sent.isNotEmpty()) {
                item(key = "sent-header") {
                    // A78 ⟨D3⟩ — "Clear sent (N)": every sent note on this tablet, after a confirmation.
                    GroupHeader("Sent", sent.size, indent = 4.dp, collapsed = sentCollapsed,
                        action = HeaderAction("Clear sent (${sent.size})", enabled = true) { clearAsk = sentToClear(sent) }) { sentCollapsed = !sentCollapsed }
                }
                if (!sentCollapsed) renderSection(this, sent, "s", sentSide = true)
            }
        }
    }
    // A75 ⟨D2⟩ — the leaf's resting state has no button; tapping it opens this view, which is where Send /
    // Re-send / Delete live. Looking at the note is the ordinary act; the actions are one level in.
    see?.let { (cid, n) ->
        val bmp = remember(cid, n.file) { port.load(cid, n.key) }
        val sending = noteId(cid, n) in busy
        AlertDialog(
            onDismissRequest = { see = null },
            title = { Text("${n.songTitle.ifEmpty { "Untitled" }} · page ${n.pageInSong + 1}") },
            text = {
                if (bmp != null) Image(BitmapPainter(bmp), contentDescription = null, modifier = Modifier.fillMaxWidth(), colorFilter = null)
                else Text("Couldn't load this note.")
            },
            confirmButton = {
                val label = if (sending) "Sending…" else if (n.sentAt != null) "Re-send" else "Send"
                TextButton(onClick = { see = null; onSendTap(cid, n) }, enabled = connected && !sending) { Text(label) }
            },
            dismissButton = {
                Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    TextButton(onClick = { see = null; port.delete(cid, n.key); refresh++; onChanged() }) { Text("Delete") }
                    TextButton(onClick = { see = null }) { Text("Close") }
                }
            },
        )
    }
    // A78 ⟨D3⟩ — Clear sent: LOCAL only (Studio keeps its copies, in every state including an A77 window), and
    // only after a confirmation that says the consequence in numbers.
    clearAsk?.let { targets ->
        AlertDialog(
            onDismissRequest = { clearAsk = null },
            text = { Text(clearSentQuestion(targets.size)) },
            confirmButton = {
                TextButton(onClick = {
                    clearAsk = null
                    val n = clearSentLocally(targets) { cid, e -> port.delete(cid, e.key) }
                    status = clearSentDone(n); refresh++; onChanged()
                }) { Text("Remove") }
            },
            dismissButton = { TextButton(onClick = { clearAsk = null }) { Text("Cancel") } },
        )
    }
    // T170 §3.2 — a first send hit a 409: a note is already in Studio for this page. Ask before clobbering it.
    owAsk?.let { (cid, n) ->
        AlertDialog(
            onDismissRequest = { owAsk = null },
            title = { Text("Overwrite the note in Studio?") },
            text = { Text("A note for page ${n.pageInSong + 1} of “${n.songTitle.ifEmpty { "this song" }}” is already in Studio. Replace it with this one?") },
            confirmButton = { TextButton(onClick = { owAsk = null; launchSend(cid, n, overwrite = true) }) { Text("Overwrite") } },
            dismissButton = { TextButton(onClick = { owAsk = null; status = "Left the note in Studio unchanged" }) { Text("Cancel") } },
        )
    }
    // T170 §6 step 1 — the note was taken under another member's name; confirm before sending it as yours.
    idAsk?.let { (items, bulk) ->
        AlertDialog(
            onDismissRequest = { idAsk = null },
            title = { Text("Send under your account?") },
            text = { Text(if (bulk) "Some of these notes were taken as another member. Send them to Studio under your account?" else "This note was taken as another member. Send it to Studio under your account?") },
            confirmButton = {
                TextButton(onClick = {
                    idAsk = null
                    if (bulk) launchBulk(items) else items.first().let { (c, n) -> launchSend(c, n, overwrite = n.sentAt != null) }
                }) { Text("Send") }
            },
            dismissButton = { TextButton(onClick = { idAsk = null }) { Text("Cancel") } },
        )
    }
}

/** A78 — carries the Notes tab's top-level "Send all" up to the screen's title row. [line] is observable (an
 *  equal value does not recompose); [onClick] is a plain var read only on tap, so re-publishing it every
 *  composition cannot loop. */
class NotesSendAllHandle {
    var line by mutableStateOf<SendAllLine>(SendAllLine.Hidden)
    var onClick: () -> Unit = {}
}

/** A78 — a header's trailing action: "Send all" on the unsent side, "Clear" on the Sent side. */
private class HeaderAction(val label: String, val enabled: Boolean, val onClick: () -> Unit)

/** A75 — a collapsible grouping header (band or concert, or the "Sent" group). It is a clickable row, so it
 *  holds the 48 dp touch floor. [onSendAll], when present, is a height-free trailing bulk-send that lives
 *  inside the header row (never on a leaf). */
@Composable
private fun GroupHeader(
    label: String, count: Int, indent: androidx.compose.ui.unit.Dp, collapsed: Boolean,
    action: HeaderAction?, onToggle: () -> Unit,
) {
    Row(
        Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable(onClick = onToggle).padding(start = indent, end = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(if (collapsed) "▸" else "▾", style = MaterialTheme.typography.bodyMedium)
        Spacer(Modifier.width(8.dp))
        Text(label, style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
        if (action != null) TextButton(onClick = action.onClick, enabled = action.enabled, contentPadding = PaddingValues(horizontal = 8.dp)) {
            Text(action.label, style = MaterialTheme.typography.labelMedium)
        }
        Text("$count", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

/** A75 ⟨D2⟩ — a rehearsal-note leaf: a single tappable line (tap = See; Send / Delete live in that dialog),
 *  so the RESTING row holds no button. Clickable, so it meets the 48 dp touch floor. The unsent "older than
 *  the bake" nag rides under it (⟨D5⟩ keeps the nag scoped to unsent notes). */
@Composable
private fun NoteLeaf(label: String, old: Boolean, indent: androidx.compose.ui.unit.Dp, onTap: () -> Unit) {
    Column(
        Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable(onClick = onTap).padding(start = indent, end = 8.dp),
        verticalArrangement = Arrangement.Center,
    ) {
        Text(label, style = MaterialTheme.typography.bodyMedium)
        if (old) Text("Older than the current bake — recopy in Studio, then delete.", style = MaterialTheme.typography.bodySmall, color = Color(0xFFF57C00))
    }
}

/** List installed bundles; a directory that fails to load is shown as damaged (with a delete action). */
internal fun listConcerts(storage: Storage): List<ConcertEntry> {
    val dirs = File(storage.bundlesDir()).listFiles()?.filter { it.isDirectory }?.sortedBy { it.name } ?: emptyList()
    return dirs.map { d ->
        when (val r = BundleLoader().load(d.path, FileBundleFiles())) {
            is LoadResult.Loaded -> ConcertEntry(
                d.path, r.bundle.concertId, r.bundle.concertRev, r.bundle.bakedAt,
                r.bundle.name.ifEmpty { r.bundle.concertId.ifEmpty { d.name } },
                bandId = r.bundle.bandId, bandName = r.bundle.bandName, damaged = false,
                // A91: compare like with like — the server's per-song `rev` is the song's SOURCE revision at bake
                // (bakeapi viewOf: s.SourceRevision), so "what changed" reads the bundle's sourceRevision, not songRev.
                songs = r.bundle.songs.map { com.troubastack.shared.distribution.LocalSong(it.songId, it.sourceRevision, it.title.ifBlank { it.label }) },
            )
            is LoadResult.Failed -> ConcertEntry(d.path, "", 0uL, 0L, "Damaged bundle (${d.name})", bandId = "", bandName = "", damaged = true)
        }
    }
}
