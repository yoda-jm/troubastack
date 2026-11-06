package com.troubastack.app

import java.io.ByteArrayOutputStream
import kotlin.test.Test
import kotlin.test.assertContentEquals
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** A99 — what a Studio download is saved as, and a blob put back together from its chunks without losing a byte. */
class StudioDownloadRulesTest {
    @Test
    fun theName_isTheServers_thenTheLinks_thenTheUrls() {
        // Measured headers (A99, the scratch core): an encoded filename* wins over the plain one.
        assertEquals(
            "Sat @ The Anchor — Marie.pdf",
            downloadFileName("attachment; filename=\"Sat @ The Anchor  Marie.pdf\"; filename*=UTF-8''Sat%20%40%20The%20Anchor%20%E2%80%94%20Marie.pdf", "ignored.pdf", "http://h/x"),
        )
        assertEquals("The-Troubadours-2026-11-05.tband.zip", downloadFileName("attachment; filename=\"The-Troubadours-2026-11-05.tband.zip\"", null, "http://h/x"))
        assertEquals("concert.tstage", downloadFileName("attachment; filename=concert.tstage", null, "http://h/x"))
        // No Content-Disposition (the .tstage link makes no request first): the <a download> attribute.
        assertEquals("Sat @ The Anchor.tstage", downloadFileName(null, "Sat @ The Anchor.tstage", "http://10.0.2.2:18097/api/bands/b/concerts/c/bundle"))
        assertEquals("Sat @ The Anchor.tstage", downloadFileName("attachment", "Sat @ The Anchor.tstage", "http://h/bundle"))
        // Neither: the URL's last segment, decoded, without query or fragment.
        assertEquals("My Song.pdf", downloadFileName(null, "  ", "http://h/files/My%20Song.pdf?x=1#y"))
    }

    @Test
    fun theName_isAlwaysOneSafeFileName() {
        // R1: a name is never a path.
        assertEquals("x.pdf", safeFileName("../../x.pdf"))
        assertEquals("x.pdf", downloadFileName("attachment; filename=\"../../x.pdf\"", null, "http://h/y"))
        assertEquals("passwd", downloadFileName(null, "/../etc/passwd", "http://h/y"))
        assertEquals("c", safeFileName("a\\b\\c"))
        assertEquals("a_b", safeFileName("a:b"))
        assertEquals("download", safeFileName(".."))
        assertEquals("download", safeFileName("a/"))
        assertEquals("hidden", safeFileName("..hidden"))
        assertEquals("download", safeFileName("   "))
        assertEquals("download", downloadFileName(null, null, "http://h/"))
        assertEquals(120, safeFileName("x".repeat(300)).length)
    }

    @Test
    fun chunksInOrder_withEveryByte_makeTheFile() {
        val bytes = ByteArray(1_000) { (it % 251).toByte() }
        val out = ByteArrayOutputStream()
        val a = ChunkAssembler(bytes.size.toLong(), out)
        bytes.toList().chunked(384).forEachIndexed { i, part -> a.accept(i, part.toByteArray()) }
        assertTrue(a.complete())
        assertContentEquals(bytes, out.toByteArray())
    }

    @Test
    fun aChunkOutOfOrder_orAMissingOne_orTooMany_failsTheFile() {
        val parts = ByteArray(1_000).toList().chunked(384).map { it.toByteArray() }
        ChunkAssembler(1_000, ByteArrayOutputStream()).apply { accept(0, parts[0]); accept(2, parts[2]); accept(1, parts[1]) }
            .let { assertFalse(it.complete(), "out of order"); assertTrue(it.failed) }
        // TEETH: drop the size check and a truncated file (the last chunk never came) passes.
        ChunkAssembler(1_000, ByteArrayOutputStream()).apply { accept(0, parts[0]); accept(1, parts[1]) }
            .let { assertFalse(it.complete(), "truncated: 768 of 1000 bytes") }
        ChunkAssembler(700, ByteArrayOutputStream()).apply { parts.forEachIndexed { i, p -> accept(i, p) } }
            .let { assertFalse(it.complete(), "more bytes than announced"); assertTrue(it.failed) }
    }

    // ---- R1: the interface is visible to every page the WebView loads ----

    private val server = "http://10.0.2.2:18097"
    private fun b64(b: ByteArray) = java.util.Base64.getEncoder().encodeToString(b)

    @Test
    fun anOffOriginCaller_writesNothing() {
        // TEETH: drop the origin check (allowed = { true }) and begin opens a file for the evil page → red.
        var opened = 0
        val t = BlobTransfers(allowed = { downloadAllowed("https://evil.example/page", server) }, open = { opened++; ByteArrayOutputStream() })
        assertFalse(t.begin("1", "x.pdf", "application/pdf", 3))
        assertFalse(t.chunk("1", 0, b64(byteArrayOf(1, 2, 3))))
        assertEquals(null, t.end("1"))
        assertEquals(0, opened, "no file was opened for an off-origin page")
    }

    @Test
    fun aPageThatLeavesTheOriginMidTransfer_doesNotSave() {
        var top = "$server/bands/b"
        val t = BlobTransfers(allowed = { downloadAllowed(top, server) }, open = { ByteArrayOutputStream() })
        assertTrue(t.begin("1", "x.pdf", "application/pdf", 6))
        assertTrue(t.chunk("1", 0, b64(byteArrayOf(1, 2, 3))))
        top = "https://evil.example/"
        assertFalse(t.chunk("1", 1, b64(byteArrayOf(4, 5, 6))))
        assertEquals(false, t.end("1")?.ok)
    }

    @Test
    fun oneTransferAtATime_withinTheCap_underASafeName() {
        val t = BlobTransfers(allowed = { true }, open = { ByteArrayOutputStream() }, maxBytes = 10)
        assertFalse(t.begin("big", "x.pdf", "application/pdf", 11), "over the cap")
        assertTrue(t.begin("1", "../../x.pdf", "application/pdf", 3))
        assertFalse(t.begin("2", "y.pdf", "application/pdf", 3), "a second while one is in flight")
        assertTrue(t.chunk("1", 0, b64(byteArrayOf(1, 2, 3))))
        assertEquals(BlobTransfers.Ended("x.pdf", "application/pdf", ok = true, part = 0), t.end("1"))
        assertTrue(t.begin("2", "y.pdf", "application/pdf", 3), "the next one once the first ended")
    }

    @Test
    fun eachTransfer_writesItsOwnPart() {
        // A save still waiting (the API 26–28 storage answer) keeps its bytes while the next transfer runs.
        // TEETH: one shared part (open always gets the same number) and the first file holds both.
        val parts = mutableMapOf<Int, ByteArrayOutputStream>()
        val t = BlobTransfers(allowed = { true }, open = { part -> parts.getOrPut(part) { ByteArrayOutputStream() } })
        assertTrue(t.begin("a", "a.pdf", "application/pdf", 3)); assertTrue(t.chunk("a", 0, b64(byteArrayOf(1, 2, 3))))
        val a = t.end("a")!!
        assertTrue(t.begin("b", "b.pdf", "application/pdf", 2)); assertTrue(t.chunk("b", 0, b64(byteArrayOf(9, 8))))
        val b = t.end("b")!!
        assertTrue(a.ok && b.ok)
        assertTrue(a.part != b.part, "two transfers, two parts")
        assertContentEquals(byteArrayOf(1, 2, 3), parts.getValue(a.part).toByteArray())
        assertContentEquals(byteArrayOf(9, 8), parts.getValue(b.part).toByteArray())
    }

    @Test
    fun onlyHttpAndHttps_goToDownloadManager() {
        // Its Request throws on anything else, on the main thread (measured: a data: <a download> ended the app).
        // TEETH: let every scheme through and the data: and blob: vectors go red.
        assertTrue(downloadManagerCanFetch("$server/api/bands/b/concerts/c/bundle"))
        assertTrue(downloadManagerCanFetch("HTTPS://h.example/x.pdf"))
        for (url in listOf("data:text/plain;base64,aGVsbG8=", "blob:$server/2b1c", "javascript:alert(1)", "file:///sdcard/x", "content://x/y", "about:blank", "x.pdf", ""))
            assertFalse(downloadManagerCanFetch(url), url)
    }

    @Test
    fun theOriginRule() {
        assertTrue(downloadAllowed("$server/bands/b/settings?embedded=1", server))
        assertTrue(downloadAllowed("http://10.0.2.2:18097/", "http://10.0.2.2:18097/"))
        assertFalse(downloadAllowed("http://10.0.2.2:8080/bands", server), "another port is another origin")
        assertFalse(downloadAllowed("about:blank", server))
        assertFalse(downloadAllowed(null, server))
    }

    @Test
    fun aCrossOriginDownload_getsNoCookie() {
        val secrets = mapOf(
            com.troubastack.shared.seams.SESSION_COOKIE_KEY to "trouba_session=abc",
            com.troubastack.shared.seams.SESSION_ORIGIN_KEY to server,
        )
        val same = downloadRequestHeaders(secrets::get, "$server/api/bands/b/concerts/c/bundle", "UA/1")
        assertEquals(mapOf("User-Agent" to "UA/1", "Cookie" to "trouba_session=abc"), same)
        val other = downloadRequestHeaders(secrets::get, "https://elsewhere.example/file.pdf", "UA/1")
        assertEquals(mapOf("User-Agent" to "UA/1"), other, "no Cookie header off the session's origin")
    }
}
