package com.troubashare.shared.stage

import com.troubashare.shared.bundle.BundleMember
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** P205 Stage 3a-ii — resolving/prompting the viewer's identity for a concert. */
class IdentityPickerTest {

    private val roster = listOf(
        BundleMember("m-marie", "Marie", "admin"),
        BundleMember("m-leo", "Leo", "member"),
    )

    @Test
    fun resolveIdentity_storedPickWins_ifStillInRoster() {
        assertEquals("m-leo", resolveIdentity(roster, stored = "m-leo"))
        // stored member no longer in the roster ⇒ ignore it
        assertEquals("", resolveIdentity(roster, stored = "m-gone"))
    }

    @Test
    fun resolveIdentity_autoMatchesLoggedInUser_elseEmpty() {
        assertEquals("m-marie", resolveIdentity(roster, stored = null, autoUserId = "m-marie"))
        assertEquals("", resolveIdentity(roster, stored = null, autoUserId = "someone-else"))
        assertEquals("", resolveIdentity(roster, stored = "", autoUserId = ""))
        // a stored pick beats auto-match
        assertEquals("m-leo", resolveIdentity(roster, stored = "m-leo", autoUserId = "m-marie"))
    }

    @Test
    fun needsIdentityPick_onlyWhenRosterPresentAndUnresolved() {
        assertTrue(needsIdentityPick(roster, resolved = ""))
        assertFalse(needsIdentityPick(roster, resolved = "m-leo"))
        assertFalse(needsIdentityPick(emptyList(), resolved = "")) // no roster (old/-mine bundle) ⇒ never prompt
    }

    @Test
    fun memberLabel_appendsRole_onlyForNonPlainMembers() {
        assertEquals("Marie · admin", memberLabel(BundleMember("m-marie", "Marie", "admin")))
        assertEquals("Leo", memberLabel(BundleMember("m-leo", "Leo", "member")))
        assertEquals("Sam", memberLabel(BundleMember("m-sam", "Sam", "")))
    }
}
