package com.troubastack.app

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** WIN03 (gate addition) — the desktop Back keeps Android's rule: the newest handler wins; disposing it hands Back on. */
class BackDispatcherTest {

    @Test
    fun nested_theInnerHandlerWins_thenTheOuterAfterTheInnerIsDisposed() {
        // TEETH: a dispatcher that runs the OLDEST handler (or all of them) reddens the first assertion.
        val d = BackDispatcher()
        val calls = ArrayList<String>()
        val outer = d.register { calls += "outer" }
        val inner = d.register { calls += "inner" }
        assertTrue(d.dispatch())
        assertEquals(listOf("inner"), calls)
        inner.dispose()
        assertTrue(d.dispatch())
        assertEquals(listOf("inner", "outer"), calls)
        outer.dispose()
        assertFalse(d.dispatch(), "no handler: nothing happens")
    }

    @Test
    fun disposingAnOlderHandler_leavesTheNewestInCharge() {
        val d = BackDispatcher()
        val calls = ArrayList<String>()
        val first = d.register { calls += "first" }
        d.register { calls += "second" }
        first.dispose()
        d.dispatch()
        assertEquals(listOf("second"), calls)
    }
}
