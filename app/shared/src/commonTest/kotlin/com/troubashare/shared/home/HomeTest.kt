package com.troubashare.shared.home

import kotlin.test.Test
import kotlin.test.assertEquals

/** A27 — the Home identity line: pure, testable, degrades gracefully when the name is unknown
 *  (Slice 1 has no user name until P205 Stage 3a resolves "performing as <name>"). */
class HomeTest {

    @Test
    fun identityLine_disconnected_invitesConnect() {
        assertEquals("Connect to your band", identityLine(Identity.Disconnected))
    }

    @Test
    fun identityLine_connected_nameless_showsServer() {
        assertEquals("demo:8080 ✓", identityLine(Identity.Connected(name = "", server = "demo:8080", synced = true)))
    }

    @Test
    fun identityLine_connected_namedAndSyncing() {
        assertEquals("Marie · s.example ✓", identityLine(Identity.Connected("Marie", "s.example", synced = true)))
        assertEquals("Marie · s.example · syncing…", identityLine(Identity.Connected("Marie", "s.example", synced = false)))
    }

    @Test
    fun identityLine_connected_noNameNoServer_saysConnected() {
        assertEquals("Connected ✓", identityLine(Identity.Connected(name = "", server = "", synced = true)))
    }

    @Test
    fun identityLine_offline_rendersLastSynced() {
        assertEquals("Marie · offline · last synced 2m ago", identityLine(Identity.Offline("Marie", "2m ago")))
        assertEquals("offline", identityLine(Identity.Offline(name = "", lastSynced = "")))
    }
}
