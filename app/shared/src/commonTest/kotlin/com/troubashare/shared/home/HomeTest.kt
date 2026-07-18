package com.troubashare.shared.home

import kotlin.test.Test
import kotlin.test.assertEquals

/** A27 — the Home identity line + trailing action, per the 2026-07-18 ruling: one clean line, the raw
 *  server IP:port never here (it's behind Manage), name/band fill in as they become cheaply known. */
class HomeTest {

    @Test
    fun identityLine_disconnected_invitesConnect() {
        assertEquals("Connect to your band", identityLine(Identity.Disconnected))
    }

    @Test
    fun identityLine_connected_isCleanNoServer() {
        assertEquals("Connected ✓", identityLine(Identity.Connected())) // no raw IP:port
        assertEquals("Connected · syncing…", identityLine(Identity.Connected(synced = false)))
    }

    @Test
    fun identityLine_connected_withNameAndBand_readsPerformingAs() {
        // The line P205 Stage 3a fills in.
        assertEquals(
            "Performing as Marie · The Troubadours ✓",
            identityLine(Identity.Connected(name = "Marie", band = "The Troubadours", synced = true)),
        )
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
    fun identityAction_connectWhenDisconnected_elseManage() {
        assertEquals("Connect", identityAction(Identity.Disconnected))
        assertEquals("Manage", identityAction(Identity.Connected()))
        assertEquals("Manage", identityAction(Identity.Offline()))
    }
}
