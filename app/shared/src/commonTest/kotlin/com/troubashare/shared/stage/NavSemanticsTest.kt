package com.troubashare.shared.stage

import com.troubashare.shared.bundle.BakedSong
import com.troubashare.shared.bundle.ConcertBundle
import com.troubashare.shared.bundle.LoadResult
import com.troubashare.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * N1/N2 — the pure nav semantics behind the reworked Stage navigation: whether a move crossed a song
 * boundary (N1's cue trigger) and the per-song page range that the scroll column and its edge-crossings
 * read (N2). The gesture wiring on top is Compose; these are the load-bearing decisions.
 */
class NavSemanticsTest {

    private fun state(vararg songs: BakedSong): StageState {
        val r = StageViewModel(LoadResult.Loaded(ConcertBundle(concertId = "c1", songs = songs.toList()), emptyList()))
        return r.state.value
    }

    private fun song(id: String, pages: Int) = BakedSong(
        songId = id,
        pages = (1..pages).map { PageImages(pageRasterRef = "$id-p$it.png") },
    )

    // songs of 3 + 2 + 1 pages → firstPages 0, 3, 5; global pages 0..5.
    private fun sample() = state(song("a", 3), song("b", 2), song("c", 1))

    @Test
    fun crossedSongBoundary_trueOnlyAcrossSongs() {
        val pages = sample().pages
        assertFalse(crossedSongBoundary(pages, 0, 1), "within song a")
        assertFalse(crossedSongBoundary(pages, 1, 2), "within song a")
        assertTrue(crossedSongBoundary(pages, 2, 3), "a → b")
        assertTrue(crossedSongBoundary(pages, 4, 5), "b → c")
        assertTrue(crossedSongBoundary(pages, 5, 4), "c → b (backwards still crosses)")
    }

    @Test
    fun crossedSongBoundary_outOfRange_isFalse() {
        val pages = sample().pages
        assertFalse(crossedSongBoundary(pages, -1, 0))
        assertFalse(crossedSongBoundary(pages, 5, 6))
        assertFalse(crossedSongBoundary(emptyList(), 0, 1))
    }

    @Test
    fun songPageRange_spansOnlyTheContainingSong() {
        val s = sample()
        // song a: pages 0..2
        assertEquals(0..2, songPageRange(s, 0))
        assertEquals(0..2, songPageRange(s, 2))
        // song b: pages 3..4
        assertEquals(3..4, songPageRange(s, 3))
        assertEquals(3..4, songPageRange(s, 4))
        // song c: page 5 (single-page song → 5..5)
        assertEquals(5..5, songPageRange(s, 5))
    }

    @Test
    fun songPageRange_clampsOutOfRangePage() {
        val s = sample()
        assertEquals(0..2, songPageRange(s, -3)) // clamps into song a
        assertEquals(5..5, songPageRange(s, 99)) // clamps into song c
    }

    @Test
    fun songPageRange_emptyBundle_isEmptyRange() {
        val s = StageState() // no pages
        assertTrue(songPageRange(s, 0).isEmpty())
    }

    @Test
    fun isBlockedSongCross_atTheEndsOnly() {
        // N8: a horizontal scroll-mode swipe crosses songs; blocked only at the first (backward) or
        // last (forward) song — where it must fire the N7 cue instead of a silent no-op. 3 songs (0..2).
        assertTrue(isBlockedSongCross(currentSong = 2, songCount = 3, forward = true))   // last → next blocked
        assertTrue(isBlockedSongCross(currentSong = 0, songCount = 3, forward = false))  // first → prev blocked
        assertFalse(isBlockedSongCross(currentSong = 0, songCount = 3, forward = true))  // first → next ok
        assertFalse(isBlockedSongCross(currentSong = 2, songCount = 3, forward = false)) // last → prev ok
        assertFalse(isBlockedSongCross(currentSong = 1, songCount = 3, forward = true))  // middle either way
        assertFalse(isBlockedSongCross(currentSong = 1, songCount = 3, forward = false))
        // single-song concert: both directions blocked; empty concert: blocked.
        assertTrue(isBlockedSongCross(currentSong = 0, songCount = 1, forward = true))
        assertTrue(isBlockedSongCross(currentSong = 0, songCount = 1, forward = false))
        assertTrue(isBlockedSongCross(currentSong = 0, songCount = 0, forward = true))
    }
}
