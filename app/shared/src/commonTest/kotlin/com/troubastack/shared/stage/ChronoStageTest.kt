package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.BundleMember
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * T147 — the chronometer wired into the Stage VM. It times the SESSION, so it must survive song
 * navigation, a bundle update (T143's viewport-preserving swap) and an identity switch; and toggling the
 * clock overlay must never disturb the page. Uses an injected `now` — no sleeping.
 */
class ChronoStageTest {
    private var now = 0L

    private fun newVm(roster: List<BundleMember> = emptyList()): StageViewModel {
        val bundle = ConcertBundle(
            concertId = "c1",
            roster = roster,
            songs = listOf(
                BakedSong(songId = "song-1", pages = listOf(PageImages(pageRasterRef = "a"), PageImages(pageRasterRef = "b"))),
                BakedSong(songId = "song-2", pages = listOf(PageImages(pageRasterRef = "c"), PageImages(pageRasterRef = "d"))),
            ),
        )
        return StageViewModel(LoadResult.Loaded(bundle, emptyList()), monotonicNow = { now })
    }

    private fun rebaked() = LoadResult.Loaded(
        ConcertBundle(
            concertId = "c1",
            songs = listOf(BakedSong(songId = "song-1", pages = listOf(PageImages(pageRasterRef = "a2"), PageImages(pageRasterRef = "b")))),
        ),
        emptyList(),
    )

    @Test
    fun chrono_runs_and_pauses_via_the_vm_with_injected_clock() {
        now = 1_000
        val vm = newVm().also { it.startChrono() }
        now = 4_000
        assertEquals(3_000, vm.chronoElapsedMs())
        vm.pauseChrono()
        now = 100_000
        assertEquals(3_000, vm.chronoElapsedMs()) // paused ⇒ frozen even as the clock races on
    }

    @Test
    fun chrono_survives_song_navigation() {
        now = 0
        val vm = newVm().also { it.startChrono() }
        vm.goToSong(1)
        assertEquals(1, vm.state.value.currentSong)
        now = 5_000
        assertTrue(vm.state.value.chrono.running)
        assertEquals(5_000, vm.chronoElapsedMs()) // kept counting across the song change
    }

    @Test
    fun chrono_survives_a_bundle_update() {
        now = 0
        val vm = newVm().also { it.startChrono() }
        now = 6_000
        vm.applyUpdate(rebaked())
        assertTrue(vm.state.value.chrono.running, "an auto-update must not stop the chrono")
        now = 10_000
        assertEquals(10_000, vm.chronoElapsedMs()) // and must not reset it
    }

    @Test
    fun chrono_survives_an_identity_switch() {
        now = 0
        val vm = newVm(roster = listOf(BundleMember(memberId = "m1", displayName = "A"), BundleMember(memberId = "m2", displayName = "B")))
        vm.startChrono()
        now = 2_000
        vm.setIdentity("m1")
        assertTrue(vm.state.value.chrono.running)
        now = 7_000
        assertEquals(7_000, vm.chronoElapsedMs())
    }

    @Test
    fun toggling_the_clock_does_not_move_the_page_or_change_geometry() {
        val vm = newVm().also { it.goToPage(1) }
        val before = vm.state.value
        vm.setClockVisible(true)
        val after = vm.state.value
        assertTrue(after.clockVisible)
        assertEquals(before.current, after.current) // page index unchanged
        assertEquals(before.pages, after.pages)     // rendered pages / geometry unchanged
        assertEquals(before.fitMode, after.fitMode)
    }
}
