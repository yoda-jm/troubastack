package com.troubastack.app

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * A98/WIN08 — the session cookie is seeded into a web view AS THE SERVER SET IT: `HttpOnly` + `SameSite=Lax`, `Secure`
 * only on https, the `name=value` unchanged, and NO `Expires`/`Max-Age` (re-seeded on every Studio open). Both hosts
 * (Android's `seedSessionCookie`, WIN08's `Browser.setCookie`) seed through [sessionCookieSeed], so they match.
 */
class SessionCookieSeedTest {
    private fun parts(s: String) = s.split("; ")

    @Test
    fun http_httpOnly_pathRoot_sameSiteLax_notSecure_noExpiry() {
        val p = parts(sessionCookieSeed("trouba_session=abc123", "http://127.0.0.1:8080"))
        assertEquals("trouba_session=abc123", p.first()) // name=value stays first, unchanged
        val attrs = p.drop(1).toSet()
        assertTrue("Path=/" in attrs)
        assertTrue("HttpOnly" in attrs) // TEETH: drop HttpOnly from sessionCookieSeed → this reds (JS could read the session)
        assertTrue("SameSite=Lax" in attrs)
        assertFalse("Secure" in attrs)
        assertFalse(p.any { it.startsWith("Expires", ignoreCase = true) || it.startsWith("Max-Age", ignoreCase = true) })
    }

    @Test
    fun https_addsSecure() {
        val p = parts(sessionCookieSeed("trouba_session=abc123", "https://band.example:8443"))
        assertEquals("trouba_session=abc123", p.first())
        val attrs = p.drop(1).toSet()
        assertTrue("Path=/" in attrs)
        assertTrue("HttpOnly" in attrs)
        assertTrue("SameSite=Lax" in attrs)
        assertTrue("Secure" in attrs)
        assertFalse(p.any { it.startsWith("Expires", ignoreCase = true) || it.startsWith("Max-Age", ignoreCase = true) })
    }
}
