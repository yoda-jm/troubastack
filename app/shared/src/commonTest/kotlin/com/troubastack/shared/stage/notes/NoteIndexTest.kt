package com.troubastack.shared.stage.notes

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** A70 §6.1 — reconciliation of stored notes against a bundle: live vs orphaned by RASTER HASH, per-song
 *  counts, the bake nag, and the tab-warning predicate. */
class NoteIndexTest {

    private fun note(song: String, hash: String, sent: Long? = null, bakes: Int = 0) = NoteEntry(
        songId = song, rasterHash = hash, file = "$song-$hash.png", pageInSong = 0, songTitle = song,
        bandName = "b", concertRev = 1, takenAs = "m", width = 1600, height = 2000, updatedAt = 0,
        sentAt = sent, bakesSinceTouched = bakes,
    )

    @Test
    fun attach_matchesByFullKey_orphansMissing_neverAcrossSongs() {
        val notes = listOf(note("s1", "hA"), note("s2", "hB"), note("s3", "hGONE"))
        // Present keys: s1/hA still there; hB now belongs to a DIFFERENT song (s9) — must NOT match s2's note.
        val present = setOf(NoteKey("s1", "hA"), NoteKey("s9", "hB"))
        val a = NoteIndex.attach(notes, present)
        assertEquals(listOf(NoteKey("s1", "hA")), a.live.map { it.key })
        assertEquals(setOf(NoteKey("s2", "hB"), NoteKey("s3", "hGONE")), a.orphaned.map { it.key }.toSet())
        assertEquals(mapOf("s1" to 1), a.countsBySong)
    }

    @Test
    fun attach_swappedRasters_orphanBoth_neverSwap() {
        // Teeth (T145): page 1 and 2 of one song swapped rasters between bakes. Keyed by index, a note would
        // silently jump to the other page; keyed by hash, both orphan.
        val notes = listOf(note("s", "hash-of-page1"), note("s", "hash-of-page2"))
        val present = setOf(NoteKey("s", "hash-of-page2-NEW"), NoteKey("s", "hash-of-page1-NEW"))
        val a = NoteIndex.attach(notes, present)
        assertTrue(a.live.isEmpty(), "both orphan — neither is re-placed onto the other page")
        assertEquals(2, a.orphaned.size)
    }

    @Test
    fun bumpBakes_incrementsUnsent_leavesSent() {
        val out = NoteIndex.bumpBakes(listOf(note("s", "h1", bakes = 1), note("s", "h2", sent = 5L, bakes = 0)))
        assertEquals(2, out.first { it.rasterHash == "h1" }.bakesSinceTouched)
        assertEquals(0, out.first { it.rasterHash == "h2" }.bakesSinceTouched)
    }

    @Test
    fun nagPredicate_trueAt3_falseAt2_sentNeverOld() {
        assertTrue(!NoteIndex.isOld(note("s", "h", bakes = 2)), "2 bakes is not yet old")
        assertTrue(NoteIndex.isOld(note("s", "h", bakes = 3)), "3 bakes is old")
        assertTrue(!NoteIndex.isOld(note("s", "h", sent = 9L, bakes = 99)), "a sent note is never old")
    }

    @Test
    fun tabWarning_noneOrangeRed_sentNeverCounts() {
        assertEquals(NotesWarning.NONE, NoteIndex.warning(listOf(note("s", "h", bakes = 2))))
        assertEquals(NotesWarning.ORANGE, NoteIndex.warning(listOf(note("s", "h", bakes = 3))))
        assertEquals(NotesWarning.RED, NoteIndex.warning(listOf(note("s", "h", bakes = 6)))) // 2×NAG
        assertEquals(NotesWarning.NONE, NoteIndex.warning(listOf(note("s", "h", sent = 1L, bakes = 99))))
        // worst wins across the set
        assertEquals(NotesWarning.RED, NoteIndex.warning(listOf(note("a", "h", bakes = 3), note("b", "h", bakes = 7))))
    }
}
