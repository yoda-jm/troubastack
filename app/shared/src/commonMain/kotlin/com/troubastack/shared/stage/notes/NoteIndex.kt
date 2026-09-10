// A70 §3.4/§4.1 — the PURE reconciliation of stored notes against the current bundle. Keyed by raster hash
// so a note stays across a bake when its page did not change, orphans (kept, labelled) when it did, and
// nags after a few bakes. No I/O; the host loads the entries and the bundle supplies the live page keys.
package com.troubastack.shared.stage.notes

/** The result of attaching stored notes to a bundle: which are still live, which orphaned, and per-song
 *  LIVE counts (an orphan has no live page, so it drives the Notes tab, not the per-song layer row §3.8). */
data class AttachedNotes(
    val live: List<NoteEntry>,
    val orphaned: List<NoteEntry>,
    val countsBySong: Map<String, Int>,
)

object NoteIndex {
    /**
     * §3.4 — partition [notes] against the [present] page keys of the current bundle. A note whose
     * `(songId, rasterHash)` still exists is live; one whose page is gone is orphaned (kept, never
     * re-placed — the T145 rule). Matching is by the FULL key: the same raster under another song is NOT
     * a match (an intermission poster shared by two songs must not leak a note across them).
     */
    fun attach(notes: List<NoteEntry>, present: Set<NoteKey>): AttachedNotes {
        val live = ArrayList<NoteEntry>()
        val orphaned = ArrayList<NoteEntry>()
        for (n in notes) if (n.key in present) live += n else orphaned += n
        val counts = live.groupingBy { it.songId }.eachCount()
        return AttachedNotes(live, orphaned, counts)
    }

    /** The one note on a page, or null. Notes hold at most one entry per key (§3.3). */
    fun forPage(notes: List<NoteEntry>, key: NoteKey): NoteEntry? = notes.firstOrNull { it.key == key }

    /**
     * §3.4 — on every bake, count one bake against each note NOT sent since its last save; a sent note never
     * ages (recopying is what clears the nag, and a sent note has been recopied's-worth). Pure; the host
     * persists the result.
     */
    fun bumpBakes(notes: List<NoteEntry>): List<NoteEntry> =
        notes.map { if (it.sentAt == null) it.copy(bakesSinceTouched = it.bakesSinceTouched + 1) else it }

    /** True once a note is "old" — unsent and past [NoteTools.NAG_BAKES] bakes. Sent notes are never old. */
    fun isOld(n: NoteEntry): Boolean = n.sentAt == null && n.bakesSinceTouched >= NoteTools.NAG_BAKES

    /**
     * §3.4 — the `Notes ⚠` tab-label state across ALL notes: NONE when none is old, ORANGE when at least one
     * is old, RED once any is past `2 × NAG_BAKES`. Sent notes never count (they cannot be old).
     */
    fun warning(notes: List<NoteEntry>): NotesWarning {
        var worst = NotesWarning.NONE
        for (n in notes) {
            if (!isOld(n)) continue
            if (n.bakesSinceTouched >= 2 * NoteTools.NAG_BAKES) return NotesWarning.RED
            worst = NotesWarning.ORANGE
        }
        return worst
    }
}
