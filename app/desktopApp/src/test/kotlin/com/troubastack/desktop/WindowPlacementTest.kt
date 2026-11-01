package com.troubastack.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** WIN03 D2 — the window opens where it was, unless that place is no longer on a connected screen. */
class WindowPlacementTest {
    private val laptop = Bounds(0, 0, 1920, 1080)
    private val monitorOnTheRight = Bounds(1920, 0, 2560, 1440)

    @Test
    fun nothingSaved_opensAtTheDefaultSize_centredOnThePrimaryScreen() {
        assertEquals(SavedPlacement(Bounds(320, 140, 1280, 800), false), openingPlacement(null, listOf(laptop), laptop))
    }

    @Test
    fun aPlaceStillOnAScreen_isKept_withItsMaximizedState() {
        val saved = SavedPlacement(Bounds(2100, 200, 1400, 900), maximized = true)
        assertEquals(saved, openingPlacement(saved, listOf(laptop, monitorOnTheRight), laptop))
    }

    @Test
    fun theMonitorWasUnplugged_opensCentredOnThePrimary_keepingSizeAndMaximized() {
        // TEETH: keeping the saved place regardless opens the window where no screen is.
        val saved = SavedPlacement(Bounds(2100, 200, 1400, 900), maximized = true)
        assertEquals(SavedPlacement(Bounds(260, 90, 1400, 900), true), openingPlacement(saved, listOf(laptop), laptop))
    }

    @Test
    fun onlyTheTitleBarCounts_aWindowHangingOffTheBottomStays_oneAboveTheTopDoesNot() {
        val low = SavedPlacement(Bounds(100, 900, 1000, 700), false) // its title bar is on screen, the rest below
        assertEquals(low, openingPlacement(low, listOf(laptop), laptop))
        val high = SavedPlacement(Bounds(100, -40, 1000, 700), false) // its title bar is above the screen
        assertEquals(Bounds(460, 190, 1000, 700), openingPlacement(high, listOf(laptop), laptop).bounds)
    }

    @Test
    fun theSize_staysWithinTheMinimumAndTheScreen() {
        val tiny = SavedPlacement(Bounds(10, 10, 300, 200), false)
        assertEquals(Bounds(10, 10, 800, 600), openingPlacement(tiny, listOf(laptop), laptop).bounds)
        val huge = SavedPlacement(Bounds(4000, 0, 3000, 2000), false) // off-screen AND bigger than the laptop
        assertEquals(Bounds(0, 0, 1920, 1080), openingPlacement(huge, listOf(laptop), laptop).bounds)
    }

    @Test
    fun theStoredForm_roundTrips_andGarbageReadsAsNothing() {
        val p = SavedPlacement(Bounds(-1280, 12, 1000, 700), true)
        assertEquals(p, parsePlacement(p.encode()))
        assertNull(parsePlacement(null))
        assertNull(parsePlacement("1,2,3"))
        assertNull(parsePlacement("a,2,3,4,true"))
        assertNull(parsePlacement("1,2,3,4,yes"))
    }
}
