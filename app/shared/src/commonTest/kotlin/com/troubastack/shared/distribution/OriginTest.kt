package com.troubastack.shared.distribution

import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * The session-binding origin match (device-QA fix): a session must never be replayed to a different
 * server than the one that issued it. These pin the parse that the guard compares on.
 */
class OriginTest {

    @Test
    fun originOf_normalizesSchemeHostPort() {
        assertEquals("http://192.168.1.23:8080", originOf("http://192.168.1.23:8080"))
        assertEquals("http://192.168.1.23:8080", originOf("http://192.168.1.23:8080/"))
        assertEquals("http://192.168.1.23:8080", originOf("http://192.168.1.23:8080/api/bands?x=1#f"))
        assertEquals("https://band.example.com", originOf("https://band.example.com"))
    }

    @Test
    fun originOf_isCaseInsensitiveOnSchemeAndHost() {
        assertEquals("http://host.local:8080", originOf("HTTP://Host.Local:8080/Path"))
    }

    @Test
    fun originOf_stripsUserinfo() {
        assertEquals("http://host:8080", originOf("http://user:pw@host:8080/x"))
    }

    @Test
    fun originOf_differsByPortAndHostAndScheme() {
        // The whole point: these must NOT be treated as the same origin.
        val a = originOf("http://192.168.1.23:8080")
        assertEquals(false, a == originOf("http://192.168.1.23:9000"), "different port")
        assertEquals(false, a == originOf("http://192.168.1.24:8080"), "different host")
        assertEquals(false, a == originOf("https://192.168.1.23:8080"), "different scheme")
    }

    @Test
    fun originOf_emptyForMissingSchemeOrHost() {
        assertEquals("", originOf("192.168.1.23:8080")) // no scheme → not a usable origin
        assertEquals("", originOf(""))
        assertEquals("", originOf("   "))
        assertEquals("", originOf("http://")) // no host
    }
}
