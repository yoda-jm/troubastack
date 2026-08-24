package com.troubashare.shared.home

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * A45 — the account chip + sheet as pure state-mapping (T58's "one account trigger", phone-shaped as a
 * bottom sheet). Every identity state must yield a correct glanceable chip label AND a correct sheet
 * menu, and the update affordance must stay coupled to Recognized exactly as before — the regression this
 * refactor could most easily cause.
 */
class AccountTriggerTest {

    @Test
    fun chipLabel_isGlanceable_perState() {
        assertEquals("The Troubadours", accountChipLabel(Identity.Connected(name = "Marie", band = "The Troubadours")))
        assertEquals("Marie", accountChipLabel(Identity.Connected(name = "Marie", band = "")), "no band ⇒ fall back to the name")
        assertEquals("Connected", accountChipLabel(Identity.Connected()))
        assertEquals("Offline", accountChipLabel(Identity.Offline()))
        assertEquals("Guest", accountChipLabel(Identity.SignedOut()))
        assertEquals("Guest", accountChipLabel(Identity.NotSetUp))
        assertEquals("…", accountChipLabel(Identity.Checking))
    }

    @Test
    fun menu_offersTheRightActions_perState() {
        assertEquals(AccountMenu(manage = true, primaryAction = "Disconnect"), accountMenu(Identity.Connected(name = "Marie")))
        assertEquals(AccountMenu(manage = true, primaryAction = "Retry"), accountMenu(Identity.Offline()))
        assertEquals(AccountMenu(manage = true, primaryAction = "Sign in"), accountMenu(Identity.SignedOut()))
        assertEquals(AccountMenu(manage = false, primaryAction = "Connect"), accountMenu(Identity.NotSetUp))
        // Parameters is always available (chrome, not account).
        assertTrue(accountMenu(Identity.NotSetUp).settings)
    }

    @Test
    fun menu_offersNoLiveActionWhileChecking() {
        // A38: mid-probe shows the action disabled, never live. primaryAction is blank; Manage is withheld.
        val m = accountMenu(Identity.Checking)
        assertEquals("", m.primaryAction)
        assertFalse(m.manage)
    }

    @Test
    fun chip_collapsesToDotOnly_atNarrowWidth() {
        assertFalse(accountChipShowsLabel(320), "at 320 dp the chip must collapse to the dot (no overflow)")
        assertTrue(accountChipShowsLabel(400), "with room, show the label")
    }

    @Test
    fun updateAffordance_onlyWhenRecognized_regressionGuard() {
        // The coupling A45 must not break: the update row is eligible ONLY when Connected. A Guest or an
        // Offline player is never offered an update (the host forces Hidden; this pins it).
        assertTrue(updateRowEligible(Identity.Connected(name = "Marie")))
        for (guest in listOf(Identity.Offline(), Identity.SignedOut(), Identity.NotSetUp, Identity.Checking)) {
            assertFalse(updateRowEligible(guest), "update must not be offered for $guest")
        }
    }
}
