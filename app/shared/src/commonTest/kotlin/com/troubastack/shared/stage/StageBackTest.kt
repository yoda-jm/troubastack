package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals

/** A79 — system Back never leaves a concert that is being performed; it still leaves the failure/empty screens. */
class StageBackTest {

    private fun loaded(songs: List<BakedSong>) = LoadResult.Loaded(ConcertBundle(concertId = "c1", concertRev = 1uL, songs = songs), emptyList())
    private val twoSongs = listOf(
        BakedSong(songId = "s1", pages = listOf(PageImages(pageRasterRef = "a", rasterHash = "h1"), PageImages(pageRasterRef = "b", rasterHash = "h2"))),
        BakedSong(songId = "s2", pages = listOf(PageImages(pageRasterRef = "c", rasterHash = "h3"))),
    )

    @Test
    fun performing_backStays_onTheSamePage() {
        val vm = StageViewModel(loaded(twoSongs))
        vm.goToPage(2) // mid-concert, not page 0 — so "same page" is a real assertion
        val before = vm.state.value.current
        assertEquals(StageBack.STAY, stageBackAction(vm.state.value))
        assertEquals(before, vm.state.value.current) // deciding Back moves nothing
        assertEquals(2, vm.state.value.current)
    }

    @Test
    fun noteMode_backStays_theNoteIsNotDiscarded() {
        val vm = StageViewModel(loaded(twoSongs))
        vm.requestNoteMode(); vm.confirmNoteMode()
        assertEquals(StageBack.STAY, stageBackAction(vm.state.value))
        assertEquals(true, vm.state.value.noteMode) // still drawing
    }

    @Test
    fun failureScreen_backStillLeaves() {
        assertEquals(StageBack.LEAVE, stageBackAction(StageViewModel(LoadResult.Failed("bundle.json is missing")).state.value))
    }

    @Test
    fun emptyConcert_backStillLeaves() {
        assertEquals(StageBack.LEAVE, stageBackAction(StageViewModel(loaded(emptyList())).state.value))
    }

    @Test
    fun drawerOpen_backClosesTheDrawer_andTheConcertStays() {
        val vm = StageViewModel(loaded(twoSongs))
        vm.goToPage(2)
        assertEquals(StageBack.CLOSE_DRAWER, stageBackAction(vm.state.value, drawerOpen = true))
        assertEquals(2, vm.state.value.current) // deciding moves nothing
        // once closed, Back is consumed again (the concert stays) — never LEAVE while performing
        assertEquals(StageBack.STAY, stageBackAction(vm.state.value, drawerOpen = false))
    }

    @Test
    fun drawerFlag_neverTurnsAFailureScreenIntoSomethingElse() {
        // There is no drawer on the failure screen; a stray flag must not stop Back from leaving it.
        assertEquals(StageBack.LEAVE, stageBackAction(StageViewModel(LoadResult.Failed("x")).state.value, drawerOpen = true))
    }
}
