package com.troubashare.shared.distribution

import kotlin.test.Test
import kotlin.test.assertEquals

/** A16 — the embedded-editor URL contract (the `?embedded=1` signal web-core T46 reads). */
class EditorUrlTest {

    @Test
    fun noPath_loadsRootEmbedded() {
        assertEquals("http://192.168.1.23:8080/?embedded=1", embeddedUrl("http://192.168.1.23:8080", null))
        assertEquals("http://192.168.1.23:8080/?embedded=1", embeddedUrl("http://192.168.1.23:8080/", ""))
    }

    @Test
    fun deepLinkPath_isNormalizedAndEmbedded() {
        val expected = "http://h:8080/bands/b1/songs/s1?embedded=1"
        assertEquals(expected, embeddedUrl("http://h:8080", "/bands/b1/songs/s1"))
        assertEquals(expected, embeddedUrl("http://h:8080/", "bands/b1/songs/s1")) // missing leading slash
    }

    @Test
    fun pathWithExistingQuery_appendsWithAmpersand() {
        assertEquals("http://h:8080/x?a=1&embedded=1", embeddedUrl("http://h:8080", "/x?a=1"))
    }
}
