package com.troubastack.app

import com.troubastack.shared.seams.PlainCodec
import com.troubastack.shared.seams.SESSION_COOKIE_KEY
import com.troubastack.shared.seams.SESSION_ORIGIN_KEY
import com.troubastack.shared.seams.Storage
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * WIN08 S2.3 — when the server origin changes or the user signs out, the clearWebCookies hook must be handed the OLD
 * origin and the OLD cookie's name, read from storage BEFORE the session is cleared, so the Windows embedded Studio can
 * expire that exact WebView2 cookie (keyed by name+domain+path). The failure this guards is silent: capture the values
 * AFTER clearSession and the hook gets ("" / null), so it would clear nothing and the old session would still ride in
 * the WebView2 request header. Teeth: moving either capture below clearSession reddens these.
 */
class ClearCookiesSeamTest {
    private fun storageWithSession(origin: String, cookie: String): Storage {
        val s = Storage(Files.createTempDirectory("clearcookies").toFile(), PlainCodec)
        s.putSecret(SESSION_ORIGIN_KEY, origin)
        s.putSecret(SESSION_COOKIE_KEY, cookie)
        return s
    }

    @Test
    fun originChange_handsHookThePreClearOriginAndName() {
        val storage = storageWithSession("http://old.example:8080", "win08sess=abc123")
        var seen: Pair<String?, String?>? = null
        dropSessionIfOriginChanged(storage, "http://new.example:9090") { o, n -> seen = o to n }
        assertEquals("http://old.example:8080" to "win08sess", seen, "the hook gets the OLD origin + cookie name, pre-clear")
        assertEquals("", storage.getSecret(SESSION_ORIGIN_KEY), "and the session is cleared afterwards")
    }

    @Test
    fun sameOrigin_doesNotClearCookies() {
        val storage = storageWithSession("http://same.example:8080", "win08sess=abc")
        var called = false
        dropSessionIfOriginChanged(storage, "http://same.example:8080/api/me") { _, _ -> called = true }
        assertEquals(false, called, "no origin change ⇒ no cookie clear, and the session stays")
        assertEquals("win08sess=abc", storage.getSecret(SESSION_COOKIE_KEY))
    }

    @Test
    fun signOut_handsHookThePreClearOriginAndName() {
        val storage = storageWithSession("http://old.example:8080", "win08sess=abc123")
        var seen: Pair<String?, String?>? = null
        val transport = HttpTransport(storage, defaultServerUrl = null, clearWebCookies = { o, n -> seen = o to n })
        transport.signOut()
        assertEquals("http://old.example:8080" to "win08sess", seen, "sign-out hands the hook the pre-clear origin + name")
    }
}
