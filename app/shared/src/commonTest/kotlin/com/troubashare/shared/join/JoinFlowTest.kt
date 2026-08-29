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

    // ---- serverIdentity: the T123 probe that gates the password field ---------------------------

    @Test fun identity_troubastack_current_contract_is_safe() {
        assertEquals(ServerIdentity.TroubaStack, serverIdentity(200, "troubastack", 1, clientMax = 1))
        // An OLDER server (contract below what we know) is still fine — the client is backward compatible.
        assertEquals(ServerIdentity.TroubaStack, serverIdentity(200, "troubastack", 1, clientMax = 2))
    }

    @Test fun identity_foreign_host_is_refused() {
        // The adversarial case: a QR pointed at a host that answers but isn't ours. Must NOT pass.
        assertEquals(ServerIdentity.Foreign, serverIdentity(200, "grafana", 1))
        assertEquals(ServerIdentity.Foreign, serverIdentity(200, "", 1))
        assertEquals(ServerIdentity.Foreign, serverIdentity(200, null, 1))
    }

    @Test fun identity_troubastack_but_no_contract_version_is_refused() {
        // Claims to be ours but stamps no apiVersion — a real server always does; treat silence as not-trust.
        assertEquals(ServerIdentity.Foreign, serverIdentity(200, "troubastack", null))
    }

    @Test fun identity_newer_contract_is_refused_as_too_new() {
        assertEquals(ServerIdentity.TooNew(2, 1), serverIdentity(200, "troubastack", 2, clientMax = 1))
    }

    @Test fun identity_non_200_is_unreachable() {
        assertEquals(ServerIdentity.Unreachable, serverIdentity(404, "troubastack", 1))
        assertEquals(ServerIdentity.Unreachable, serverIdentity(0, "troubastack", 1))   // our network-failure sentinel
        assertEquals(ServerIdentity.Unreachable, serverIdentity(500, "troubastack", 1))
    }

    // ---- registerOutcome: A57 sign-up from an invite ---------------------------------------------

    @Test fun register_201_or_200_is_created() {
        assertEquals(RegisterOutcome.Created, registerOutcome(201)) // the server's actual success status
        assertEquals(RegisterOutcome.Created, registerOutcome(200))
    }

    @Test fun register_409_name_taken_is_its_own_recoverable_outcome() {
        // The common real failure — must be distinct so the sheet says "that name is taken" and keeps the
        // person in the form. (This is teeth-check territory.)
        assertEquals(RegisterOutcome.NameTaken, registerOutcome(409))
    }

    @Test fun register_other_statuses_fail_with_the_code() {
        assertEquals(RegisterOutcome.Failed(500), registerOutcome(500))
        assertEquals(RegisterOutcome.Failed(400), registerOutcome(400))
        assertEquals(RegisterOutcome.Failed(0), registerOutcome(0)) // network-failure sentinel
    }
}
