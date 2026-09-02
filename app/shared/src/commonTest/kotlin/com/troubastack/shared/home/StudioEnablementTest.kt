package com.troubastack.shared.home

import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * A55 — the TroubaStudio tile's enabled-state, derived from the Home Identity. Every row of the spec's
 * table by name; the `Checking` row matters most because it is the one a person sees on every resume.
 */
class StudioEnablementTest {

    @Test fun connected_enables_the_tile() {
        assertEquals(StudioTile.Enabled, studioEnablement(Identity.Connected(name = "Marie", band = "The Wildflowers")))
    }

    @Test fun signed_out_disables_with_sign_in_reason() {
        assertEquals(StudioTile.Disabled("Sign in to manage concerts"), studioEnablement(Identity.SignedOut(band = "The Wildflowers")))
    }

    @Test fun not_set_up_disables_with_sign_in_reason() {
        assertEquals(StudioTile.Disabled("Sign in to manage concerts"), studioEnablement(Identity.NotSetUp))
    }

    @Test fun offline_disables_with_no_connection_reason() {
        assertEquals(StudioTile.Disabled("No connection"), studioEnablement(Identity.Offline(band = "The Wildflowers")))
    }

    @Test fun checking_is_disabled_with_a_neutral_caption() {
        // Chosen: disabled + empty caption while Checking, so the per-resume probe never flashes a wrong
        // reason ("Sign in…") for a beat before enabling. Disabled — but quietly, no misleading text.
        assertEquals(StudioTile.Disabled(""), studioEnablement(Identity.Checking))
    }
}
