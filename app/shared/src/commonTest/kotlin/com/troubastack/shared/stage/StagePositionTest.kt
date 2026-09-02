package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * A46 (found by A33 drill 2) — reopening a concert must land back where the performer left off, not
 * page 0. [resolveStartPage] maps a saved LOGICAL position (songId + page-in-song) to a global page
 * index, surviving a re-bake that reorders songs/pages (and clamping to the song's last page if a
 * shorter re-bake dropped the exact page — A46 §3); the VM seeds `current` from it. Before this the
 * Stage always started at 0, so a process death / exit mid-set lost your place.
 */
class StagePositionTest {

    private fun page(songId: String, pageInSong: Int) = StagePage(
        songId = songId,
        songName = songId,
        pageInSong = pageInSong,
        rasterRef = "$songId-$pageInSong",
        overlays = emptyList(),
        status = PageStatus.READY,
    )

    private fun state(vararg p: StagePage) = StageState(pages = p.toList())

    @Test
    fun emptySaved_startsAtTop() {
        val s = state(page("a", 0), page("a", 1), page("b", 0))
        assertEquals(0, resolveStartPage(s, "", 0))
    }

    @Test
    fun exactLogicalPage_resolvesToItsGlobalIndex() {
        val s = state(page("a", 0), page("a", 1), page("b", 0), page("b", 1))
        assertEquals(1, resolveStartPage(s, "a", 1))
        assertEquals(3, resolveStartPage(s, "b", 1))
    }

    @Test
    fun beyondEnd_clampsToTheSongsLastPage() {
        // A46 §3: the concert was re-baked shorter — b now has pages 0,1,2 but we saved b#5. Clamp to b's
        // LAST page (index 4), never off the end and not its first (index 2) — discriminating.
        val s = state(page("a", 0), page("a", 1), page("b", 0), page("b", 1), page("b", 2))
        assertEquals(4, resolveStartPage(s, "b", 5))
    }

    @Test
    fun unknownSong_fallsBackToZero() {
        val s = state(page("a", 0), page("b", 0))
        assertEquals(0, resolveStartPage(s, "gone", 0))
    }

    @Test
    fun survivesReorder_findsByLogicalIdNotRawIndex() {
        // Saved "b#0" while b happened to be first; a re-bake moved a to the front. Still lands on b#0,
        // not whatever now sits at the old raw index.
        val s = state(page("a", 0), page("a", 1), page("b", 0))
        assertEquals(2, resolveStartPage(s, "b", 0))
    }

    // --- the VM seeds `current` from the saved position (the wiring, end to end) ---

    private fun bundle(vararg songPages: Pair<String, Int>) = LoadResult.Loaded(
        ConcertBundle(
            concertId = "c1",
            songs = songPages.map { (id, pages) ->
                BakedSong(songId = id, pages = (1..pages).map { PageImages(pageRasterRef = "$id-p$it.png") })
            },
        ),
        emptyList(),
    )

    @Test
    fun vm_seedsCurrentFromInitialPosition() {
        // pages: a0 a1 b0 b1 c0  ⇒ indices 0..4; b#1 is index 3.
        val vm = StageViewModel(bundle("a" to 2, "b" to 2, "c" to 1), initialSongId = "b", initialPageInSong = 1)
        assertEquals(3, vm.state.value.current)
        assertEquals("b", vm.state.value.currentPage?.songId)
        assertEquals(1, vm.state.value.currentPage?.pageInSong)
    }

    @Test
    fun vm_defaultsToTop_withNoInitialPosition() {
        val vm = StageViewModel(bundle("a" to 2, "b" to 1))
        assertEquals(0, vm.state.value.current)
    }
}
