package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import com.troubastack.shared.stage.notes.NoteEntry
import com.troubastack.shared.stage.notes.NoteTool
import com.troubastack.shared.stage.notes.NoteTools
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** A70 §6.1 — the note-mode state machine: request/confirm/cancel/exit never move the page, scroll mode is
 *  refused, entering un-hides the note layer, and applyUpdate leaves note mode, ages bakes, and reports
 *  orphans. */
class StageNoteModeTest {

    /** Two songs, one page each, with controllable raster hashes so notes can match by key. */
    private fun loaded(h1: String = "h1", h2: String = "h2") = LoadResult.Loaded(
        ConcertBundle(
            concertId = "c1", concertRev = 2uL,
            songs = listOf(
                BakedSong(songId = "song-1", pages = listOf(PageImages(pageRasterRef = "s1p1", rasterHash = h1))),
                BakedSong(songId = "song-2", pages = listOf(PageImages(pageRasterRef = "s2p1", rasterHash = h2))),
            ),
        ),
        emptyList(),
    )

    private fun note(song: String, hash: String, sent: Long? = null, bakes: Int = 0) = NoteEntry(
        songId = song, rasterHash = hash, file = "$song-$hash.png", pageInSong = 0, songTitle = song,
        bandName = "b", concertRev = 2, takenAs = "m", width = 1600, height = 2000, updatedAt = 0,
        sentAt = sent, bakesSinceTouched = bakes,
    )

    @Test
    fun requestNoteMode_refusedInScroll_withReason_noStateChange() {
        val vm = StageViewModel(loaded(), initialFit = FitMode.SCROLL)
        val before = vm.state.value.current
        assertFalse(vm.requestNoteMode())
        val s = vm.state.value
        assertEquals("Notes: switch to page mode", s.noteModeRefusal)
        assertFalse(s.noteMode); assertFalse(s.noteModePending)
        assertEquals(before, s.current)
    }

    @Test
    fun requestNoteMode_setsPendingOnly_thenConfirmEnters_neitherMovesThePage() {
        val vm = StageViewModel(loaded())
        vm.goToSong(1) // current = song-2's page
        val page = vm.state.value.current
        assertTrue(vm.requestNoteMode())
        vm.state.value.let {
            assertTrue(it.noteModePending); assertFalse(it.noteMode) // pending ONLY
            assertEquals(page, it.current)
        }
        vm.confirmNoteMode()
        vm.state.value.let {
            assertTrue(it.noteMode); assertFalse(it.noteModePending)
            assertEquals(page, it.current) // entering never moves the page
        }
    }

    @Test
    fun cancelNoteMode_dismisses_withoutEntering() {
        val vm = StageViewModel(loaded())
        vm.requestNoteMode()
        vm.cancelNoteMode()
        vm.state.value.let { assertFalse(it.noteMode); assertFalse(it.noteModePending) }
    }

    @Test
    fun exitNoteMode_leaves() {
        val vm = StageViewModel(loaded())
        vm.requestNoteMode(); vm.confirmNoteMode()
        assertTrue(vm.state.value.noteMode)
        vm.exitNoteMode()
        assertFalse(vm.state.value.noteMode)
    }

    @Test
    fun noteLayerRow_absentWithoutANote_presentWithOne_andToggles() {
        val vm = StageViewModel(loaded(), initialNotes = listOf(note("song-1", "h1")))
        // song-1 has a live note; song-2 does not.
        assertTrue(vm.state.value.hasLiveNote("song-1"))
        assertTrue(vm.state.value.noteVisibleFor("song-1")) // default ON
        assertFalse(vm.state.value.hasLiveNote("song-2"))
        assertFalse(vm.state.value.noteVisibleFor("song-2")) // no note → no row → not visible
        // toggle it off for the current song (song-1)
        vm.setNoteLayerVisible(false)
        assertFalse(vm.state.value.noteVisibleFor("song-1"))
        vm.setNoteLayerVisible(true)
        assertTrue(vm.state.value.noteVisibleFor("song-1"))
    }

    @Test
    fun confirmNoteMode_unhidesTheLayerForCurrentSong() {
        val vm = StageViewModel(loaded(), initialNotes = listOf(note("song-1", "h1")))
        vm.setNoteLayerVisible(false) // hidden on song-1
        assertFalse(vm.state.value.noteVisibleFor("song-1"))
        vm.requestNoteMode(); vm.confirmNoteMode()
        assertTrue(vm.state.value.noteVisibleFor("song-1")) // entering forces it back on
    }

    @Test
    fun setToolWidthColour() {
        val vm = StageViewModel(loaded())
        vm.setNoteTool(NoteTool.ERASER); assertEquals(NoteTool.ERASER, vm.state.value.noteTool)
        vm.setNoteWidth(NoteTools.WIDE); assertEquals(NoteTools.WIDE, vm.state.value.noteWidth)
        vm.setNoteColour(NoteTools.RED); assertEquals(NoteTools.RED, vm.state.value.noteColour)
        val n0 = vm.state.value.noteRevision
        vm.bumpNoteRevision(); assertEquals(n0 + 1, vm.state.value.noteRevision)
    }

    @Test
    fun applyUpdate_leavesNoteMode_bumpsBakes_reportsOrphans_inTheNotice() {
        val vm = StageViewModel(loaded(h1 = "h1"), initialNotes = listOf(note("song-1", "h1", bakes = 0)))
        vm.requestNoteMode(); vm.confirmNoteMode()
        assertTrue(vm.state.value.noteMode)
        // Re-bake: song-1's page raster CHANGED (h1 → h1-new) → the note orphans; song-2 unchanged.
        vm.applyUpdate(loaded(h1 = "h1-new"))
        val s = vm.state.value
        assertFalse(s.noteMode, "applyUpdate leaves note mode")
        assertEquals(1, s.orphanedNoteCount)
        assertEquals(1, s.notes.first().bakesSinceTouched, "the unsent note aged one bake")
        assertTrue(s.updateNotice?.contains("1 pages of notes are from the previous bake") == true, "notice: ${s.updateNotice}")
    }

    @Test
    fun applyUpdate_unchangedPage_keepsNoteLive_noOrphan() {
        val vm = StageViewModel(loaded(h1 = "h1"), initialNotes = listOf(note("song-1", "h1")))
        vm.applyUpdate(loaded(h1 = "h1")) // same hash → still live
        assertEquals(0, vm.state.value.orphanedNoteCount)
        assertTrue(vm.state.value.hasLiveNote("song-1"))
    }

    @Test
    fun noteForPage_findsTheStoredNote() {
        val vm = StageViewModel(loaded(), initialNotes = listOf(note("song-1", "h1")))
        val page = vm.state.value.pages.first { it.songId == "song-1" }
        assertEquals("song-1-h1.png", vm.state.value.noteForPage(page)?.file)
        val page2 = vm.state.value.pages.first { it.songId == "song-2" }
        assertNull(vm.state.value.noteForPage(page2))
    }
}
