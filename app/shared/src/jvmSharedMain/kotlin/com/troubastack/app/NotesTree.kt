package com.troubastack.app

import com.troubastack.shared.stage.notes.NoteEntry

/** One note in the Notes tab: the concert it belongs to + its index entry. */
typealias NoteItem = Pair<String, NoteEntry>

/**
 * A75/A78 — the rows of one section (unsent, or the Sent group) as a PURE plan, so the fold rules are tested
 * off-device; the composable only maps rows to widgets. [items] are the notes of the section, [labels] the
 * concert labels by id, [collapsed] the folded keys (a folded header hides its children).
 */
sealed interface NoteRow {
    val key: String
    /** A band header — only with 2+ bands (A75 ⟨D1⟩ still holds at the band level). */
    data class Band(override val key: String, val label: String, val items: List<NoteItem>) : NoteRow
    /** A78 ⟨D2⟩ — the concert header is ALWAYS drawn: it collapses and carries the node-level action. When the
     *  band header is folded away, [label] carries the band too ("[band] · [concert]"). */
    data class Concert(override val key: String, val label: String, val items: List<NoteItem>, val indentDp: Int) : NoteRow
    /** A song sub-label — only with 2+ songs in the concert. */
    data class Song(override val key: String, val label: String) : NoteRow
    data class Leaf(override val key: String, val item: NoteItem, val label: String, val indentDp: Int) : NoteRow
}

fun noteRows(items: List<NoteItem>, keyPrefix: String, labels: Map<String, String>, collapsed: Set<String>): List<NoteRow> {
    val rows = mutableListOf<NoteRow>()
    val byBand = items.groupBy { it.second.bandName }
    val multiBand = byBand.size >= 2
    val concertIndent = if (multiBand) 20 else 4
    byBand.forEach { (band, bandItems) ->
        val bandKey = "$keyPrefix:band:$band"
        if (multiBand) rows += NoteRow.Band(bandKey, band.ifEmpty { "Notes" }, bandItems)
        if (multiBand && bandKey in collapsed) return@forEach
        bandItems.groupBy { it.first }.forEach { (cid, concertItems) ->
            val concertKey = "$keyPrefix:c:$band:$cid"
            val concertName = labels[cid] ?: "Concert"
            // ⟨D2⟩: with the band header folded away, keep the context on the concert header.
            val label = if (!multiBand && band.isNotEmpty()) "$band · $concertName" else concertName
            rows += NoteRow.Concert(concertKey, label, concertItems, concertIndent)
            if (concertKey in collapsed) return@forEach
            val bySong = concertItems.groupBy { it.second.songTitle }
            val multiSong = bySong.size >= 2
            val leafIndent = concertIndent + if (multiSong) 40 else 28
            bySong.entries.sortedBy { it.key }.forEach { (song, songItems) ->
                if (multiSong) rows += NoteRow.Song("$keyPrefix:s:$concertKey:$song", song.ifEmpty { "Untitled" })
                songItems.sortedBy { it.second.pageInSong }.forEach { (cid2, n) ->
                    // Key on the note's identity (concert + songId + rasterHash): unique even if two entries
                    // ever shared a filename — a duplicate LazyColumn key crashes the list.
                    val lbl = if (multiSong) "page ${n.pageInSong + 1}" else "${n.songTitle.ifEmpty { "Untitled" }} · page ${n.pageInSong + 1}"
                    rows += NoteRow.Leaf("$keyPrefix:$cid2:${n.songId}:${n.rasterHash}", cid2 to n, lbl, leafIndent)
                }
            }
        }
    }
    return rows
}

/** A78 ⟨D1⟩ — the top-level "Send all (N)" line: hidden with nothing to send; present but disabled offline (a
 *  control that vanishes offline reads as the same bug); busy while a send runs. */
sealed interface SendAllLine {
    data object Hidden : SendAllLine
    data class Shown(val count: Int, val enabled: Boolean, val sending: Boolean) : SendAllLine
}

fun sendAllLine(unsent: Int, connected: Boolean, sending: Boolean): SendAllLine =
    if (unsent <= 0) SendAllLine.Hidden else SendAllLine.Shown(unsent, enabled = connected && !sending, sending = sending)

/** T170 §6 step 1 — a bulk send asks "Send under your account?" when any note was taken as another member. One
 *  function for every entry point (node header AND the top-level line), so they cannot diverge. An unknown
 *  [myId] (offline / failed /api/me) never prompts. */
fun needsIdentityPrompt(items: List<NoteItem>, myId: String?): Boolean =
    myId != null && items.any { (_, n) -> n.takenAs.isNotEmpty() && n.takenAs != myId }

/** A78 ⟨D3⟩ — which notes a "Clear" removes: only SENT ones (an unsent note exists nowhere else, so it is never
 *  bulk-cleared), and with [concertId] only that concert's. */
fun sentToClear(items: List<NoteItem>, concertId: String? = null): List<NoteItem> =
    items.filter { (cid, n) -> n.sentAt != null && (concertId == null || cid == concertId) }

/**
 * A78 ⟨D3⟩ — clear sent notes from THIS TABLET. Local, always — including inside an A77 auto-upload window:
 * Studio keeps its copies. It takes only a LOCAL deleter and has no route to the server or to ArmedUploader
 * (whose mirror-delete belongs to clearing a live page in Stage, not to tidying this list). Returns the count.
 */
fun clearSentLocally(targets: List<NoteItem>, deleteLocal: (concertId: String, n: NoteEntry) -> Unit): Int {
    val sent = targets.filter { it.second.sentAt != null } // defence in depth: never an unsent note
    sent.forEach { (cid, n) -> deleteLocal(cid, n) }
    return sent.size
}

/** A78 ⟨D3⟩ — the confirmation, with the consequence in numbers (A74's reason, said to the person who can judge it). */
fun clearSentQuestion(n: Int): String =
    "Remove $n sent ${if (n == 1) "note" else "notes"} from this tablet? Studio keeps its copies. On Stage, " +
        "${if (n == 1) "this page" else "these pages"} will no longer show ${if (n == 1) "it" else "them"}."

fun clearSentDone(n: Int): String = "Removed $n ${if (n == 1) "note" else "notes"} from this tablet"

/** A78 — which action a grouping header carries. Sent side ⇒ CLEAR (always — clearing is local, no network
 *  needed); unsent side ⇒ SEND_ALL when connected, else none. Never CLEAR on the unsent side: an unsent note
 *  exists nowhere else, and a bulk clear there would destroy the only copy. */
enum class NodeActionKind { SEND_ALL, CLEAR, NONE }

fun nodeActionKind(sentSide: Boolean, connected: Boolean): NodeActionKind = when {
    sentSide -> NodeActionKind.CLEAR
    connected -> NodeActionKind.SEND_ALL
    else -> NodeActionKind.NONE
}
