package com.troubastack.shared.distribution

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** A93 — the Studio WebView's page and load failure, folded from the platform callbacks. */
class EditorPageTest {

    private val entry = embeddedUrl("http://h:8080", "/bands/b1")
    private val song = "http://h:8080/bands/b1/setlists/l1/songs/s1"
    private val refused = "net::ERR_CONNECTION_REFUSED"

    @Test
    fun failedLoad_staysFailedThroughTheErrorPageFinish() {
        // the sequence logged on the emulator: started → error → finished (WebView's own error page)
        val p = EditorPage().apply { load(song) }
        assertTrue(p.started(song)) // a real start: shown as loading
        assertTrue(p.failed(song, refused))
        assertFalse(p.finished())
        assertEquals(song, p.currentUrl) // what Retry reloads
    }

    @Test
    fun failedLoad_staysFailed_whenTheErrorComesBeforeTheStart() {
        val p = EditorPage().apply { load(song) }
        assertTrue(p.failed(song, refused))
        assertFalse(p.started(song)) // its error page starting must not replace the cover with "Loading…"
        assertFalse(p.finished())
    }

    @Test
    fun errorPagePseudoUrls_neitherClearTheFailureNorMoveTheCurrentPage() {
        val p = EditorPage().apply { load(song); started(song); failed(song, refused) }
        assertFalse(p.started("chrome-error://chromewebdata/"))
        p.visited("data:text/html,chromewebdata")
        assertFalse(p.started("about:blank"))
        assertFalse(p.finished())
        assertEquals(song, p.currentUrl)
    }

    @Test
    fun retryOrLeaving_clearsTheFailure() {
        val retry = EditorPage().apply { load(song); started(song); failed(song, refused) }
        retry.load(song)
        assertTrue(retry.started(song))
        assertTrue(retry.finished())

        val back = EditorPage().apply { load(song); started(song); failed(song, refused) }
        assertTrue(back.started(entry)) // Back from the error page loads another page
        assertTrue(back.finished())
        assertEquals(entry, back.currentUrl)
    }

    @Test
    fun interruptedLoad_isNotAFailure() {
        // Reload pressed mid-load: the first load is aborted, the second one finishes
        val p = EditorPage().apply { load(entry); started(entry); visited(song) }
        assertFalse(p.failed(song, "net::ERR_ABORTED"))
        assertEquals(song, p.currentUrl)
        p.load(song); p.started(song)
        assertTrue(p.finished())
    }

    @Test
    fun studioNavigation_movesTheCurrentPage_andReloadKeepsIt() {
        val p = EditorPage().apply { load(entry); started(entry) }
        assertTrue(p.finished())
        p.visited(song) // band → song is a pushState inside Studio: no page load
        assertEquals(song, p.currentUrl)
        assertEquals(song, reloadUrl(p.currentUrl, entry))
    }
}
