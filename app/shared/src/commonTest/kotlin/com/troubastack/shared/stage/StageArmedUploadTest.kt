package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import com.troubastack.shared.stage.notes.NoteEntry
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * A77 Stage 1 — the armed auto-upload window: arm/disarm, the two independent expiries (the window lapses on
 * an injected clock; leaving Stage disarms), a bake disarms VISIBLY (⟨D4⟩), and the mirror-clear predicate
 * (⟨D5⟩ — armed AND a live page). All time is the injected monotonic clock, so nothing sleeps.
 */
class StageArmedUploadTest {

    private fun loaded(rev: ULong = 2uL, h1: String = "h1", h2: String = "h2") = LoadResult.Loaded(
        ConcertBundle(
            concertId = "c1", concertRev = rev,
            songs = listOf(
                BakedSong(songId = "song-1", pages = listOf(PageImages(pageRasterRef = "s1p1", rasterHash = h1))),
                BakedSong(songId = "song-2", pages = listOf(PageImages(pageRasterRef = "s2p1", rasterHash = h2))),
            ),
        ),
        emptyList(),
    )

    private fun note(song: String, hash: String) = NoteEntry(
        songId = song, rasterHash = hash, file = "$song-$hash.png", pageInSong = 0, songTitle = song,
        bandName = "b", concertRev = 2, takenAs = "m", width = 1600, height = 2000, updatedAt = 0,
    )

    @Test
    fun arm_setsDeadlineOneWindowAhead_andReadsArmed() {
        var clock = 1_000L
        val vm = StageViewModel(loaded(), monotonicNow = { clock })
        assertFalse(vm.isArmed()) // a fresh Stage entry starts disarmed (leaving Stage ⇒ OFF)
        vm.arm()
        assertEquals(1_000L + ARMED_UPLOAD_WINDOW_MS, vm.state.value.armedUntil)
        assertTrue(vm.isArmed())
    }

    @Test
    fun window_lapsesOnItsOwn_atTheDeadline_onInjectedClock() {
        var clock = 0L
        val vm = StageViewModel(loaded(), monotonicNow = { clock })
        vm.arm()
        val deadline = vm.state.value.armedUntil
        clock = deadline - 1
        assertTrue(vm.isArmed()); vm.expireArmIfDue(); assertTrue(vm.isArmed()) // one ms before: still armed
        clock = deadline // the boundary is exclusive (now < armedUntil) — at the deadline it is OFF
        assertFalse(vm.isArmed())
        vm.expireArmIfDue()
        assertEquals(0L, vm.state.value.armedUntil) // and expiry cleared the deadline
    }

    @Test
    fun setArmed_togglesByIntent() {
        val vm = StageViewModel(loaded(), monotonicNow = { 5L })
        vm.setArmed(true); assertTrue(vm.isArmed())
        vm.setArmed(false); assertFalse(vm.isArmed()); assertEquals(0L, vm.state.value.armedUntil)
    }

    @Test
    fun bakeWhileArmed_disarms_andSaysWhy() {
        var clock = 0L
        val vm = StageViewModel(loaded(rev = 2uL), monotonicNow = { clock })
        vm.arm()
        assertTrue(vm.isArmed())
        vm.applyUpdate(loaded(rev = 3uL)) // a bake arrives under the performer
        assertFalse(vm.isArmed())
        assertEquals(0L, vm.state.value.armedUntil)
        val notice = vm.state.value.updateNotice
        assertNotNull(notice)
        assertTrue(notice!!.contains("disarmed"), "the bake notice must say the window ended: $notice")
    }

    @Test
    fun bakeWhileNotArmed_carriesNoDisarmWord() {
        val vm = StageViewModel(loaded(rev = 2uL), monotonicNow = { 0L })
        vm.applyUpdate(loaded(rev = 3uL))
        val notice = vm.state.value.updateNotice
        assertNotNull(notice)
        assertFalse(notice!!.contains("disarmed"), "not armed ⇒ the normal update notice only: $notice")
    }

    @Test
    fun shouldMirrorClear_onlyWhenArmedAndOnALivePage() {
        var clock = 0L
        val vm = StageViewModel(loaded(), monotonicNow = { clock }, initialNotes = listOf(note("song-1", "h1")))
        val s0 = vm.state.value
        val livePage = s0.pages.first { it.songId == "song-1" }   // carries the note
        val barePage = s0.pages.first { it.songId == "song-2" }   // no note
        // Not armed: never mirrors, even on the live page (T170 §7's two lifetimes stand).
        assertFalse(s0.shouldMirrorClear(livePage, clock))
        vm.arm()
        val s1 = vm.state.value
        assertTrue(s1.shouldMirrorClear(livePage, clock))   // armed + live ⇒ clear reaches Studio
        assertFalse(s1.shouldMirrorClear(barePage, clock))  // armed + no live note ⇒ local only (orphans excluded)
    }
}
