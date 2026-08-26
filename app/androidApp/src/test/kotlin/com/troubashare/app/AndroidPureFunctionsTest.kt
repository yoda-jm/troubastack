package com.troubashare.app

import com.troubashare.shared.distribution.originOf
import com.troubashare.shared.seams.SESSION_COOKIE_KEY
import com.troubashare.shared.seams.SESSION_ORIGIN_KEY
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * A47 — the FIRST unit tests for :androidApp. Before this the module had no `test` source set, so these
 * Android-only pure functions had never been executed by a test on any machine. Chosen the T110 way:
 * the ones whose wrong answer is SILENT — a cross-origin session leak, or a path-traversal filename.
 *
 * Per-test teeth-check (the wrong impl tried, that it reddens) is in each test's comment.
 */
class AndroidPureFunctionsTest {

    private fun secrets(vararg pairs: Pair<String, String>): (String) -> String? {
        val m = pairs.toMap()
        return { m[it] }
    }

    // ---- sessionCookieFor: the guard that a session is NEVER replayed to another origin ----

    @Test
    fun sessionCookie_noCookieStored_isNull() {
        // Teeth: an impl that returns "" or a constant instead of null would redden this.
        assertNull(sessionCookieFor(secrets(), "http://host:8080"))
    }

    @Test
    fun sessionCookie_cookieButNoRecordedOrigin_isNull() {
        // Old install: a cookie exists but the origin was never recorded ⇒ refuse (force one re-login).
        // Teeth: dropping the `origin.isNotEmpty()` clause returns the cookie here — reddens.
        assertNull(sessionCookieFor(secrets(SESSION_COOKIE_KEY to "sid=abc"), "http://host:8080"))
    }

    @Test
    fun sessionCookie_matchingOrigin_returnsIt() {
        val url = "http://192.168.2.8:18080/api/me"
        val s = secrets(SESSION_COOKIE_KEY to "sid=abc", SESSION_ORIGIN_KEY to originOf(url))
        assertEquals("sid=abc", sessionCookieFor(s, url))
    }

    @Test
    fun sessionCookie_crossOrigin_isRefused_neverLeaked() {
        // THE security guard: a cookie issued by server A must not be handed to server B. Teeth: replacing
        // the `origin == originOf(url)` check with `true` (or removing it) returns "sid=A" here — reddens.
        val s = secrets(SESSION_COOKIE_KEY to "sid=A", SESSION_ORIGIN_KEY to originOf("http://a.example:8080/x"))
        assertNull(sessionCookieFor(s, "http://b.example:8080/x"), "a session from A must never leak to B")
    }

    // ---- safeSegment: reduce an arbitrary id to ONE safe filename segment (test what it REJECTS) ----

    @Test
    fun safeSegment_rejectsPathTraversal_stripsSeparatorsAndDots() {
        // The dangerous input: a traversal that, unsanitised, escapes the bundles dir. The expected output
        // differs from the naive-wrong `return input` (which keeps '/' and '.'). Teeth: `return id` reddens
        // on the assertions below (the '/' and '.' would survive).
        val out = safeSegment("../../etc/passwd")
        assertEquals("______etc_passwd", out)
        assertTrue('/' !in out && '.' !in out, "a path-traversal id must not survive as a filename: $out")
    }

    @Test
    fun safeSegment_rejectsBackslashColonAndSlash() {
        // Windows/Unix separators + a drive/scheme colon all collapse to '_' — none can start a new path.
        assertEquals("a_b_c_d", safeSegment("a/b\\c:d"))
    }

    @Test
    fun safeSegment_emptyId_fallsBackToBundle() {
        // Never an empty filename. Teeth: `return cleaned` (no ifBlank) yields "" — reddens.
        assertEquals("bundle", safeSegment(""))
    }

    @Test
    fun safeSegment_validConcertId_isUnchanged() {
        // A real concert UUID is letters/digits/'-' only, so a correct sanitiser is a no-op — it must not
        // corrupt legitimate ids. Teeth: an over-eager sanitiser (e.g. stripping '-') reddens.
        assertEquals("0b4205bb-1909-49cd-bf4e-1e9b44d0cf6e", safeSegment("0b4205bb-1909-49cd-bf4e-1e9b44d0cf6e"))
    }
}
