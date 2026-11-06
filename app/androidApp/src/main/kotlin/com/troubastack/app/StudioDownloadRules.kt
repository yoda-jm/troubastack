// A99 — the pure rules behind Studio's downloads in the tablet app: what the saved file is called, and putting a
// blob back together from the chunks the page sends. No Android API here, so both are unit-tested off the device.
package com.troubastack.app

import com.troubastack.shared.distribution.originOf
import java.io.OutputStream
import java.net.URI
import java.net.URLDecoder

/**
 * The saved file's name: the server's `Content-Disposition` (its `filename*` first, then `filename`), else the link's
 * `download` attribute, else the URL's last path segment — always reduced to one safe file name ([safeFileName]).
 */
internal fun downloadFileName(contentDisposition: String?, downloadAttribute: String?, url: String): String {
    val fromHeader = contentDisposition?.let(::dispositionFileName)
    val fromLink = downloadAttribute?.trim()?.takeIf { it.isNotEmpty() }
    val path = runCatching { URI(url).rawPath }.getOrNull().orEmpty()
    val fromUrl = path.substringAfterLast('/').let { runCatching { URLDecoder.decode(it.replace("+", "%2B"), "UTF-8") }.getOrDefault(it) }
    return safeFileName(fromHeader ?: fromLink ?: fromUrl)
}

/** `filename*=UTF-8''…` (RFC 5987, percent-decoded) if present, else `filename="…"` / `filename=…`; null if neither. */
internal fun dispositionFileName(header: String): String? {
    val star = Regex("""filename\*\s*=\s*([^;]+)""", RegexOption.IGNORE_CASE).find(header)?.groupValues?.get(1)?.trim()
    if (star != null) {
        val value = star.substringAfter("''", star).trim('"')
        val charset = star.substringBefore("''", "UTF-8").ifEmpty { "UTF-8" }
        runCatching { URLDecoder.decode(value.replace("+", "%2B"), charset) }.getOrNull()?.takeIf { it.isNotBlank() }?.let { return it }
    }
    val plain = Regex("""filename\s*=\s*("([^"]*)"|[^;]+)""", RegexOption.IGNORE_CASE).find(header) ?: return null
    return (plain.groupValues[2].ifEmpty { plain.groupValues[1] }).trim().trim('"').takeIf { it.isNotBlank() }
}

/**
 * One file name, never a path (A99 R1): only the last component of anything with separators (`../../x.pdf` → `x.pdf`),
 * no control characters or `:`, no leading dots (so never `.` or `..`), at most 120 characters, else `download`.
 */
internal fun safeFileName(raw: String): String {
    val base = raw.substringAfterLast('/').substringAfterLast('\\')
    val cleaned = base.map { c -> if (c == ':' || c.code < 0x20 || c == '\u007f') '_' else c }
        .joinToString("").trim().trimStart('.').trim()
    return cleaned.take(120).ifEmpty { "download" }
}

/**
 * Puts a blob back together from the page's chunks, written to [out] as they arrive. A chunk out of order, or more
 * bytes than announced, fails the whole file; [complete] is true only when every byte of [expectedSize] arrived.
 */
internal class ChunkAssembler(private val expectedSize: Long, private val out: OutputStream) {
    private var next = 0
    private var written = 0L
    var failed = false; private set

    fun accept(index: Int, bytes: ByteArray) {
        if (failed) return
        if (index != next || written + bytes.size > expectedSize) { failed = true; return }
        out.write(bytes)
        written += bytes.size
        next++
    }

    fun complete(): Boolean = !failed && written == expectedSize
}

/** R1 — the app saves a download only while the WebView's top-level page is on the band server's origin. */
internal fun downloadAllowed(topLevelUrl: String?, serverUrl: String): Boolean {
    val top = topLevelUrl?.let(::originOf).orEmpty()
    return top.isNotEmpty() && top == originOf(serverUrl)
}

/** DownloadManager fetches only http(s) URLs (its Request throws on anything else, which ended the app): a `data:`
 *  link, or a `blob:` the capture missed, is refused with "Couldn't save" instead. */
internal fun downloadManagerCanFetch(url: String): Boolean =
    url.substringBefore(':', "").lowercase().let { it == "http" || it == "https" }

/** R1 — what DownloadManager sends: the WebView's User-Agent, and the session cookie only for the origin that issued
 *  it ([sessionCookieFor] refuses any other), so a download from elsewhere never carries the session. */
internal fun downloadRequestHeaders(getSecret: (String) -> String?, url: String, userAgent: String): Map<String, String> =
    buildMap {
        put("User-Agent", userAgent)
        sessionCookieFor(getSecret, url)?.let { put("Cookie", it) }
    }

/** The largest blob one save accepts (a band export with every chart in it stays far below). */
internal const val MAX_BLOB_BYTES: Long = 512L * 1024 * 1024

/**
 * R1 — the page's blob transfers: one at a time, at most [maxBytes], only while [allowed] (the top-level page is on the
 * server's origin, checked at every call), named by [safeFileName], each written through [open] to its own part (a
 * temp file on the device), numbered here and not by the page, so a save still waiting keeps its bytes.
 */
internal class BlobTransfers(
    private val allowed: () -> Boolean,
    private val open: (part: Int) -> OutputStream,
    private val maxBytes: Long = MAX_BLOB_BYTES,
) {
    /** A transfer that ended: [ok] when every byte arrived from an allowed page, in [part]. */
    data class Ended(val name: String, val mime: String, val ok: Boolean, val part: Int)

    private class Current(val id: String, val name: String, val mime: String, val part: Int, val out: OutputStream, val assembler: ChunkAssembler)

    private var current: Current? = null
    private var nextPart = 0

    @Synchronized
    fun begin(id: String, name: String, mime: String, size: Long): Boolean {
        if (current != null || size < 0 || size > maxBytes || !allowed()) return false
        val part = nextPart++
        val out = open(part)
        current = Current(id, safeFileName(name), mime, part, out, ChunkAssembler(size, out))
        return true
    }

    @Synchronized
    fun chunk(id: String, index: Int, base64: String): Boolean {
        val c = current?.takeIf { it.id == id } ?: return false
        val bytes = runCatching { java.util.Base64.getDecoder().decode(base64) }.getOrNull()
        if (bytes == null || !allowed()) { c.assembler.accept(-1, ByteArray(0)); return false }
        c.assembler.accept(index, bytes)
        return !c.assembler.failed
    }

    @Synchronized
    fun end(id: String): Ended? {
        val c = current?.takeIf { it.id == id } ?: return null
        current = null
        runCatching { c.out.close() }
        return Ended(c.name, c.mime, ok = c.assembler.complete() && allowed(), part = c.part)
    }
}
