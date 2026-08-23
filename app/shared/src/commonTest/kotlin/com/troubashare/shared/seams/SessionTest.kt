package com.troubashare.shared.seams

import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * A38 — the I12 promise as a test, not a claim: signing out clears the session but **keeps the server
 * address** (so Sign in resumes with a password) and **touches nothing else** (concerts stay on the
 * device). [clearSession] is the storage half; it's a put-only function, so it structurally cannot
 * reach the concerts on disk — this asserts it also leaves every unrelated key alone.
 */
class SessionTest {

    @Test
    fun clearSession_emptiesOnlyTheSessionKeys() {
        val kv = mutableMapOf(
            SESSION_COOKIE_KEY to "s=abc123",
            SESSION_ORIGIN_KEY to "http://192.168.2.8:8080",
            "coreUrl" to "http://192.168.2.8:8080", // the server address — must survive (Sign in resumes)
            "lastUsername" to "vincent", // A41: the remembered username — must survive sign-out
            "home.lastConcertDir" to "/bundles/sat-at-the-anchor", // an unrelated app key — must survive
        )

        clearSession { k, v -> kv[k] = v }

        assertEquals("", kv[SESSION_COOKIE_KEY], "session cookie is cleared")
        assertEquals("", kv[SESSION_ORIGIN_KEY], "session origin is cleared")
        assertEquals("http://192.168.2.8:8080", kv["coreUrl"], "server address is KEPT — Sign in needs only a password")
        assertEquals("vincent", kv["lastUsername"], "the remembered username is KEPT across sign-out (A41)")
        assertEquals("/bundles/sat-at-the-anchor", kv["home.lastConcertDir"], "no unrelated key is touched (concerts stay, I12)")
    }

    @Test
    fun clearSession_writesExactlyTwoKeys() {
        // It can only ever put the two session keys — so it can never name coreUrl or the bundles dir.
        val written = mutableListOf<String>()
        clearSession { k, _ -> written += k }
        assertEquals(listOf(SESSION_COOKIE_KEY, SESSION_ORIGIN_KEY), written)
    }
}
