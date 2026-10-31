package com.troubastack.shared.distribution

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

    // A93 — Reload/Retry reload the page you are on, not the band the screen was opened with.
    private val entry = embeddedUrl("http://h:8080", "/bands/b1")

    @Test
    fun reload_currentPageWins() {
        // band → song inside Studio: the router pushed the song URL (no flag — Studio keeps it in sessionStorage)
        assertEquals("http://h:8080/bands/b1/songs/s1", reloadUrl("http://h:8080/bands/b1/songs/s1", entry))
        // kept exactly as it is: re-appending the flag would make a new history entry on the same page
        assertEquals("http://h:8080/bands/b1/songs/s1?tab=chart#x", reloadUrl("http://h:8080/bands/b1/songs/s1?tab=chart#x", entry))
        // the scheme/host compare is case-insensitive, like the session binding
        assertEquals("HTTP://H:8080/bands/b1/songs/s1", reloadUrl("HTTP://H:8080/bands/b1/songs/s1", entry))
    }

    @Test
    fun reload_entryIsTheFallback() {
        assertEquals(entry, reloadUrl(null, entry))   // nothing committed yet
        assertEquals(entry, reloadUrl("  ", entry))
        assertEquals(entry, reloadUrl("about:blank", entry))
        assertEquals(entry, reloadUrl("chrome-error://chromewebdata/", entry))
        assertEquals(entry, reloadUrl("data:text/html,chromewebdata", entry))
    }

    @Test
    fun reload_serverChangeResetsToTheEntry() {
        // the user typed a new server: the WebView is still on the old one's song
        val newEntry = embeddedUrl("http://other:8080", "/bands/b1")
        assertEquals(newEntry, reloadUrl("http://h:8080/bands/b1/songs/s1", newEntry))
        // same host, another port is another server
        assertEquals(embeddedUrl("http://h:9090", null), reloadUrl("http://h:8080/bands/b1/songs/s1", embeddedUrl("http://h:9090", null)))
    }
}
