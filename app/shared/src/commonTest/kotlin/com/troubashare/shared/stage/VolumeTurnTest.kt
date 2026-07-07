package com.troubashare.shared.stage

import com.troubashare.shared.bundle.BakedSong
import com.troubashare.shared.bundle.ConcertBundle
import com.troubashare.shared.bundle.LoadResult
import com.troubashare.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * A13 — Android volume keys must turn by a whole spread in two-up (parity with pedals), and by one
 * page in one-up. The volume path can't be reached in a headless commonTest, so this drives the same
 * pieces the composable wires together: [turnTarget] (the shared nav rule) and a fake registrar that
 * forwards a press into a real [StageViewModel], asserting `current` moves the way a live press would.
 */
class VolumeTurnTest {

    private fun vmWithPages(pages: Int): StageViewModel {
        val bundle = ConcertBundle(
            concertId = "c1",
            songs = listOf(BakedSong(songId = "s1", pages = (1..pages).map { PageImages(pageRasterRef = "p$it.png") })),
        )
        return StageViewModel(LoadResult.Loaded(bundle, emptyList()))
    }

    @Test
    fun turnTarget_twoUp_movesByWholeSpread() {
        // From either page of the 0–1 spread, NEXT lands on the next spread's left page (2), not 1.
        assertEquals(2, turnTarget(page = 0, pageCount = 6, twoUp = true, dir = PageTurn.NEXT))
        assertEquals(2, turnTarget(page = 1, pageCount = 6, twoUp = true, dir = PageTurn.NEXT))
        // PREV from the 2–3 spread goes back to the 0–1 spread's left page.
        assertEquals(0, turnTarget(page = 2, pageCount = 6, twoUp = true, dir = PageTurn.PREV))
        assertEquals(0, turnTarget(page = 3, pageCount = 6, twoUp = true, dir = PageTurn.PREV))
    }

    @Test
    fun turnTarget_oneUp_movesByOnePage() {
        assertEquals(2, turnTarget(page = 1, pageCount = 6, twoUp = false, dir = PageTurn.NEXT))
        assertEquals(0, turnTarget(page = 1, pageCount = 6, twoUp = false, dir = PageTurn.PREV))
    }

    /** A fake registrar mirroring androidApp: it just stores the handler StageScreen publishes. */
    private class FakeRegistrar {
        var handler: ((PageTurn) -> Unit)? = null
        val register: (((PageTurn) -> Unit)?) -> Unit = { handler = it }
        fun press(dir: PageTurn) = handler?.invoke(dir)
    }

    @Test
    fun registeredHandler_inTwoUp_advancesCurrentBySpread() {
        val vm = vmWithPages(6)
        val twoUp = true
        val reg = FakeRegistrar()
        // What StageScreen registers: forward the press through the spread-aware turn.
        reg.register { pt -> vm.goToPage(turnTarget(vm.state.value.current, vm.state.value.pageCount, twoUp, pt)) }

        assertEquals(0, vm.state.value.current)
        reg.press(PageTurn.NEXT)               // ONE press ...
        assertEquals(2, vm.state.value.current) // ... advances a whole spread, not turn-by-1
        reg.press(PageTurn.NEXT)
        assertEquals(4, vm.state.value.current)
        reg.press(PageTurn.PREV)
        assertEquals(2, vm.state.value.current)
    }

    @Test
    fun registrar_null_unregisters() {
        val reg = FakeRegistrar()
        reg.register { }
        reg.register(null)
        assertEquals(null, reg.handler) // dispose reverts volume keys to normal (A09 contract)
    }
}
