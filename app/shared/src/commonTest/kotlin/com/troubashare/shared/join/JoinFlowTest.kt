package com.troubashare.shared.join

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class JoinFlowTest {

    // ---- PendingToken: the bearer-credential lifecycle ------------------------------------------

    @Test fun pending_token_starts_empty() {
        val p = PendingToken()
        assertNull(p.value)
        assertTrue(!p.isArmed)
    }

    @Test fun arm_then_clear_leaves_nothing_behind() {
        val p = PendingToken()
        p.arm("secret-token")
        assertEquals("secret-token", p.value)
        assertTrue(p.isArmed)
        p.clear()
        assertNull(p.value)      // cleared on the terminal path — no token outlives the flow
        assertTrue(!p.isArmed)
    }

    @Test fun clear_is_idempotent() {
        val p = PendingToken()
        p.clear(); p.clear()     // safe to call on every terminal path without checking
        assertNull(p.value)
    }

    @Test fun a_new_link_supersedes_an_abandoned_flow() {
        val p = PendingToken()
        p.arm("first"); p.arm("second")
        assertEquals("second", p.value)
    }

    // ---- acceptOutcome: the server's own words --------------------------------------------------

    @Test fun accept_200_joins_with_the_band_name() {
        assertEquals(AcceptOutcome.Joined("The Wildflowers"), acceptOutcome(200, "The Wildflowers", null))
    }

    @Test fun accept_200_blank_band_falls_back_neutrally() {
        assertEquals(AcceptOutcome.Joined("your band"), acceptOutcome(200, "", null))
        assertEquals(AcceptOutcome.Joined("your band"), acceptOutcome(200, null, null))
    }

    @Test fun accept_410_preserves_the_servers_reason_verbatim() {
        // The server distinguishes expired / revoked / exhausted — surface its word, don't flatten it.
        assertEquals(AcceptOutcome.Gone("this invite link has expired"), acceptOutcome(410, null, "this invite link has expired"))
        assertEquals(AcceptOutcome.Gone("this invite link was revoked"), acceptOutcome(410, null, "this invite link was revoked"))
    }

    @Test fun accept_410_without_a_reason_still_says_something_useful() {
        assertEquals(AcceptOutcome.Gone("This invite is no longer usable."), acceptOutcome(410, null, ""))
    }

    @Test fun accept_404_and_401_and_other_route_distinctly() {
        assertEquals(AcceptOutcome.NotFound, acceptOutcome(404, null, null))
        assertEquals(AcceptOutcome.NeedsSignIn, acceptOutcome(401, null, null))
        assertEquals(AcceptOutcome.Failed(500), acceptOutcome(500, null, null))
        assertEquals(AcceptOutcome.Failed(503), acceptOutcome(503, null, null))
    }

    // ---- previewOutcome -------------------------------------------------------------------------

    @Test fun preview_200_valid_is_ready_with_band_and_role() {
        assertEquals(PreviewResult.Ready("The Wildflowers", "member"), previewOutcome(200, "The Wildflowers", "member", valid = true, reason = null))
    }

    @Test fun preview_200_invalid_carries_the_reason() {
        assertEquals(PreviewResult.Unusable("this invite link is used up"), previewOutcome(200, "b", "member", valid = false, reason = "this invite link is used up"))
    }

    @Test fun preview_401_and_404_and_other() {
        assertEquals(PreviewResult.NeedsSignIn, previewOutcome(401, null, null, valid = false, reason = null))
        assertEquals(PreviewResult.NotFound, previewOutcome(404, null, null, valid = false, reason = null))
        assertEquals(PreviewResult.Failed(500), previewOutcome(500, null, null, valid = false, reason = null))
    }
}
