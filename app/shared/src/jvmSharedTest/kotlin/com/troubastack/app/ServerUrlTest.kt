package com.troubastack.app

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** WIN03 (Fable's ruling) — the server in use: the saved one, else the platform's default, else none. */
class ServerUrlTest {
    private val android = "http://10.0.2.2:8080"

    @Test
    fun aSavedServer_wins_onEveryPlatform() {
        assertEquals("http://band.lan:8080", resolveServerUrl("http://band.lan:8080/", android))
        assertEquals("http://band.lan:8080", resolveServerUrl("http://band.lan:8080", null))
    }

    @Test
    fun nothingSaved_isThePlatformDefault_onAndroid_andNoServerAtAll_onTheDesktop() {
        // TEETH: a shared fallback to the emulator alias makes the desktop contact 10.0.2.2 (somebody's machine on a LAN).
        assertEquals(android, resolveServerUrl(null, android))
        assertNull(resolveServerUrl(null, null))
        assertNull(resolveServerUrl("   ", null), "a blank saved value is nothing saved")
    }
}
