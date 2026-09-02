package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * A12 / N6 — facing-pages spread math (pure). Pairing is SONG-ALIGNED: a spread never contains two
 * songs, an odd-paged (or single-page) song shows its last page solo, and each song opens a fresh
 * spread. The layout itself is code-review + device screenshot.
 */
class FacingPagesTest {

    // Concert: song A = 3 pages (0,1,2), B = 2 (3,4), C = 1 (5), D = 4 (6,7,8,9). Total 10.
    // Exercises odd, even, and single-page songs plus every boundary.
    private val starts = listOf(0, 3, 5, 6)
    private val n = 10

    @Test
    fun spreadFor_alignsToEvenOffsetWithinSong() {
        assertEquals(0, spreadFor(0, starts, n))
        assertEquals(0, spreadFor(1, starts, n))
        assertEquals(2, spreadFor(2, starts, n)) // song A's 3rd page → its own (solo) spread
        assertEquals(3, spreadFor(3, starts, n)) // song B opens fresh at 3 (odd global index!)
        assertEquals(3, spreadFor(4, starts, n))
        assertEquals(5, spreadFor(5, starts, n)) // single-page song C
        assertEquals(6, spreadFor(6, starts, n))
        assertEquals(8, spreadFor(9, starts, n))
    }

    @Test
    fun spreadPages_neverStraddlesASong() {
        assertEquals(listOf(0, 1), spreadPages(0, starts, n))
        assertEquals(listOf(2), spreadPages(2, starts, n))       // A's odd tail, solo
        assertEquals(listOf(3, 4), spreadPages(3, starts, n))    // B pairs cleanly, starts fresh
        assertEquals(listOf(5), spreadPages(5, starts, n))       // single-page song, solo
        assertEquals(listOf(6, 7), spreadPages(6, starts, n))
        assertEquals(listOf(8, 9), spreadPages(8, starts, n))
    }

    @Test
    fun nextSpread_stepsWithinSongThenCrossesToFreshSpread() {
        assertEquals(2, nextSpreadPage(0, starts, n))  // A: [0,1] → [2]
        assertEquals(3, nextSpreadPage(2, starts, n))  // A tail → B fresh [3,4]
        assertEquals(5, nextSpreadPage(3, starts, n))  // B → C [5]
        assertEquals(6, nextSpreadPage(5, starts, n))  // C → D [6,7]
        assertEquals(8, nextSpreadPage(6, starts, n))  // D: [6,7] → [8,9]
        // A22: last spread of the last song is a TRUE no-op — returns THIS spread's LEFT page (8), not
        // its right page (9). Returning 9 would move `current` within the same spread and make the N4
        // slide animate a turn that renders the identical page.
        assertEquals(8, nextSpreadPage(8, starts, n)) // from the spread-left, next stays put
        assertEquals(8, nextSpreadPage(9, starts, n)) // even from the right page it normalises to left
    }

    @Test
    fun prevSpread_stepsBackThenCrossesToPriorSongsLastSpread() {
        assertEquals(0, prevSpreadPage(2, starts, n))  // A: [2] → [0,1]
        assertEquals(0, prevSpreadPage(0, starts, n))  // very first spread → stays
        assertEquals(2, prevSpreadPage(3, starts, n))  // B → A's solo last spread [2]
        assertEquals(3, prevSpreadPage(5, starts, n))  // C → B [3,4]
        assertEquals(5, prevSpreadPage(6, starts, n))  // D → C's solo [5]
        assertEquals(6, prevSpreadPage(8, starts, n))  // D: [8,9] → [6,7]
    }

    @Test
    fun turnTarget_oneUpUnchanged_twoUpSongAligned() {
        // one-up: ±1, unclamped (VM clamps) — unchanged by N6.
        assertEquals(6, turnTarget(5, n, twoUp = false, PageTurn.NEXT, starts))
        assertEquals(4, turnTarget(5, n, twoUp = false, PageTurn.PREV, starts))
        // two-up: routes through the song-aligned spread math.
        assertEquals(3, turnTarget(2, n, twoUp = true, PageTurn.NEXT, starts)) // A tail → B fresh
        assertEquals(2, turnTarget(3, n, twoUp = true, PageTurn.PREV, starts)) // B → A solo tail
    }

    @Test
    fun label_songAlignedSpreads() {
        assertEquals("3/10", pagerLabel(2, n, twoUp = true, starts))   // A's solo tail
        assertEquals("1–2/10", pagerLabel(0, n, twoUp = true, starts))
        assertEquals("4–5/10", pagerLabel(3, n, twoUp = true, starts)) // B, fresh from an odd index
        assertEquals("6/10", pagerLabel(5, n, twoUp = true, starts))   // single-page song
        assertEquals("6/10", pagerLabel(5, n, twoUp = false, starts))  // one-up label unaffected
    }

    @Test
    fun isBlockedTurn_trueOnlyAtTheConcertEdge() {
        // N7: a turn is "blocked" (no-op) only at the very first (PREV) / last (NEXT) page or spread.
        // two-up (song-aligned): last spread of song D is [8,9], left 8; first spread [0,1], left 0.
        assertTrue(isBlockedTurn(8, n, twoUp = true, PageTurn.NEXT, starts))  // at the end → blocked
        assertTrue(isBlockedTurn(0, n, twoUp = true, PageTurn.PREV, starts))  // at the start → blocked
        assertFalse(isBlockedTurn(0, n, twoUp = true, PageTurn.NEXT, starts)) // mid-concert → not
        assertFalse(isBlockedTurn(8, n, twoUp = true, PageTurn.PREV, starts))
        assertFalse(isBlockedTurn(2, n, twoUp = true, PageTurn.NEXT, starts)) // crossing a song is a real turn
        // one-up: last page 9, first page 0.
        assertTrue(isBlockedTurn(9, n, twoUp = false, PageTurn.NEXT, starts))
        assertTrue(isBlockedTurn(0, n, twoUp = false, PageTurn.PREV, starts))
        assertFalse(isBlockedTurn(5, n, twoUp = false, PageTurn.NEXT, starts))
        // degenerate bundle ⇒ blocked (nothing to turn to).
        assertTrue(isBlockedTurn(0, 0, twoUp = true, PageTurn.NEXT, starts))
    }

    @Test
    fun prefetchTargets_areTheNeighbourTurnsWouldShow() {
        // N9: prefetch exactly what a next/prev turn displays, minus the current spread; song-aligned.
        // one-up: neighbours are prev/next page (blocked direction drops out at the ends).
        assertEquals(listOf(1), prefetchTargets(0, n, twoUp = false, starts))        // first page → only next
        assertEquals(listOf(4, 6), prefetchTargets(5, n, twoUp = false, starts))     // mid → both sides
        assertEquals(listOf(8), prefetchTargets(9, n, twoUp = false, starts))        // last page → only prev
        // two-up: whole adjacent spreads, song-aligned, current spread excluded.
        assertEquals(listOf(2), prefetchTargets(0, n, twoUp = true, starts))         // [0,1] shown → next solo [2]
        assertEquals(listOf(0, 1, 3, 4), prefetchTargets(2, n, twoUp = true, starts)) // [2] shown → prev [0,1] + next [3,4]
        // degenerate
        assertEquals(emptyList(), prefetchTargets(0, 0, twoUp = false, starts))
    }

    @Test
    fun emptyBundle_andNoSongs_degradeToGlobalPairing() {
        assertEquals(emptyList(), spreadPages(0, starts, 0))
        // No songStarts ⇒ one whole-concert song ⇒ plain global pairing (backward-compatible).
        assertEquals(listOf(0, 1), spreadPages(1, emptyList(), 22))
        assertEquals(listOf(20, 21), spreadPages(21, emptyList(), 22))
        assertEquals(listOf(22), spreadPages(22, emptyList(), 23)) // odd total, lone last page
        assertEquals("21–22/22", pagerLabel(21, 22, twoUp = true, emptyList()))
        assertEquals(2, nextSpreadPage(0, emptyList(), 22))
        assertEquals(0, prevSpreadPage(3, emptyList(), 22))
    }
}
