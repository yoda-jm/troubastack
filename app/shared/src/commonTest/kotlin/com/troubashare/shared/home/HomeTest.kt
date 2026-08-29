package com.troubashare.shared.home

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * A27/A38 — the Home connection row: status label + primary action, both pure and table-tested. A38
 * splits the old single Disconnected into **Signed out** (Guest, server known → Sign in) and **Not set
 * up** (Guest, nothing configured → Connect); the status word is the same ("Guest") but the action
 * differs. Raw server IP:port is never here (it's behind Manage).
 */
class HomeTest {

    @Test
    fun identityLine_recognized_readsPerformingAs() {
        assertEquals("Connected ✓", identityLine(Identity.Connected())) // no name yet, no raw IP:port
        assertEquals("Connected · syncing…", identityLine(Identity.Connected(synced = false)))
        assertEquals(
            "Performing as Marie · The Troubadours ✓",
            identityLine(Identity.Connected(name = "Marie", band = "The Troubadours", synced = true)),
        )
    }

    @Test
    fun identityLine_checking_isTransientProbeLabel() {
        assertEquals("Checking…", identityLine(Identity.Checking))
    }

    @Test
    fun identityLine_offline_isReassurance_notError() {
        assertEquals("Offline · concerts on device still work", identityLine(Identity.Offline()))
        assertEquals(
            "Offline · The Troubadours · concerts on device still work",
            identityLine(Identity.Offline(band = "The Troubadours")),
        )
    }

    @Test
    fun identityLine_guest_bothVariants_sayGuest() {
        // A38: the status word is "Guest" for BOTH an expired session and a never-set-up device.
        assertTrue(identityLine(Identity.SignedOut()).startsWith("Guest"))
        assertEquals("Guest · The Troubadours", identityLine(Identity.SignedOut(band = "The Troubadours")))
        assertTrue(identityLine(Identity.NotSetUp).startsWith("Guest"))
    }

    @Test
    fun identityAction_matchesTheStatus() {
        // A38: the action must match the state — the old "always Manage" was the bug.
        assertEquals("Disconnect", identityAction(Identity.Connected()))
        assertEquals("Retry", identityAction(Identity.Offline()))
        // A57: both Guest states say "Join or sign in" — the door must not read as sign-in-only to someone
        // holding an invite (the Connect modal leads with paste/Scan/Join).
        assertEquals("Join or sign in", identityAction(Identity.SignedOut(band = "The Troubadours"))) // server known
        assertEquals("Join or sign in", identityAction(Identity.NotSetUp)) // nothing set up
        assertEquals("", identityAction(Identity.Checking)) // no action word; the row renders it disabled, not hidden
    }

    @Test
    fun bandLabel_saysOnlyWhatIsTrue() {
        // A38 multi-band ruling: nothing / the name / a COUNT — never an arbitrary firstOrNull().
        assertEquals("", bandLabel(emptyList()))
        assertEquals("Good Vibes Only", bandLabel(listOf("Good Vibes Only")))
        assertEquals("2 bands", bandLabel(listOf("Good Vibes Only", "The Troubadours")))
        assertEquals("3 bands", bandLabel(listOf("A", "B", "C")))
    }

    @Test
    fun updateSummary_namesOneCountsMany() {
        // A39: one concert names it; several are counted (detail lives in Manage).
        assertEquals("", updateSummary(emptyList()))
        assertEquals("Sat @ The Anchor — new version", updateSummary(listOf("Sat @ The Anchor")))
        assertEquals("2 concerts to update", updateSummary(listOf("Sat @ The Anchor", "Spring Concert")))
    }

    @Test
    fun manage_shownExceptFreshInstallAndProbe() {
        assertTrue(identityHasManage(Identity.Connected()))
        assertTrue(identityHasManage(Identity.Offline()))
        assertTrue(identityHasManage(Identity.SignedOut()))
        assertFalse(identityHasManage(Identity.NotSetUp)) // nothing to manage yet
        assertFalse(identityHasManage(Identity.Checking))
    }
}
