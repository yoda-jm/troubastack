package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * N10 — the crossing-swipe lock (VLL: on stage a vertical scroll easily ends in a horizontal swipe that
 * jumps songs). A session VIEW preference; toggling never touches the page/geometry (same contract as the
 * clock toggles). The gesture wiring it drives — `HorizontalPager(userScrollEnabled = !swipeLocked)` and the
 * dropped page/width turn-swipe — is Compose; this pins the load-bearing flag those read.
 */
class SwipeLockTest {
    private fun vm() = StageViewModel(
        LoadResult.Loaded(
            ConcertBundle(
                concertId = "c1",
                songs = listOf(
                    BakedSong(songId = "a", pages = listOf(PageImages(pageRasterRef = "a-p1.png"), PageImages(pageRasterRef = "a-p2.png"))),
                    BakedSong(songId = "b", pages = listOf(PageImages(pageRasterRef = "b-p1.png"))),
                ),
            ),
            emptyList(),
        ),
    )

    @Test
    fun defaults_unlocked() {
        assertFalse(vm().state.value.swipeLocked, "the crossing swipe is enabled by default")
    }

    @Test
    fun toggle_flips_and_never_moves_the_page() {
        val r = vm()
        r.goToSong(1)
        val pageBefore = r.state.value.current
        r.toggleSwipeLock()
        assertTrue(r.state.value.swipeLocked, "toggle locks")
        assertEquals(pageBefore, r.state.value.current, "locking must NOT move the page (view pref only)")
        r.toggleSwipeLock()
        assertFalse(r.state.value.swipeLocked, "toggle unlocks")
        assertEquals(pageBefore, r.state.value.current, "unlocking must NOT move the page")
    }
}
