package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals

/** A73 ⟨D6⟩ — "Clear page" acts on a visible page that carries a note (the device failure: the note on the LEFT page
 *  while `current` was the RIGHT one). VLL, 2026-10-12: in two-up it always ASKS which page — never silently picks. */
class ClearPlanTest {

    @Test
    fun twoUp_oneNote_stillAsksWhich_withTheOtherPageDisabled() {
        // spread [124, 125]; only 124 (the left page) carries a note — offered, never silently chosen
        assertEquals(ClearPlan.Choose(listOf(124, 125), setOf(124)), clearPlan(listOf(124, 125), twoUp = true) { it == 124 })
        assertEquals(ClearPlan.Choose(listOf(124, 125), setOf(125)), clearPlan(listOf(124, 125), twoUp = true) { it == 125 })
    }

    @Test
    fun twoUp_bothPagesHaveANote_asksWhich() {
        assertEquals(ClearPlan.Choose(listOf(124, 125), setOf(124, 125)), clearPlan(listOf(124, 125), twoUp = true) { true })
    }

    @Test
    fun twoUp_neitherPageHasANote_disablesTheButton() {
        assertEquals(ClearPlan.Disabled, clearPlan(listOf(124, 125), twoUp = true) { false })
    }

    @Test
    fun singlePage_isTheCurrentPage_andDisabledWithoutANote() {
        assertEquals(ClearPlan.One(7), clearPlan(listOf(7), twoUp = false) { true })
        assertEquals(ClearPlan.Disabled, clearPlan(listOf(7), twoUp = false) { false }) // no silent no-op dialog
        // a one-page spread (a song's odd last page) in two-up behaves like single page
        assertEquals(ClearPlan.One(9), clearPlan(listOf(9), twoUp = true) { true })
        assertEquals(ClearPlan.Disabled, clearPlan(listOf(9), twoUp = true) { false })
    }
}
