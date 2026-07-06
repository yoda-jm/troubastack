package com.troubashare.shared.seams

import java.io.ByteArrayOutputStream
import java.util.zip.Deflater
import kotlin.test.Test
import kotlin.test.assertContentEquals
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

/**
 * Exercises the [rawInflate] seam (Android actual) — round-trip + the fail-closed length checks
 * added in the iOS seam-hardening batch (a stream that doesn't end exactly at the declared size is
 * rejected, in either direction). Raw DEFLATE (nowrap) crafted with `Deflater`.
 */
class RawInflateTest {

    private fun deflateRaw(data: ByteArray): ByteArray {
        val d = Deflater(Deflater.DEFAULT_COMPRESSION, /* nowrap = */ true)
        d.setInput(data); d.finish()
        val out = ByteArrayOutputStream()
        val buf = ByteArray(1024)
        while (!d.finished()) out.write(buf, 0, d.deflate(buf))
        d.end()
        return out.toByteArray()
    }

    @Test
    fun exactDeclaredSize_roundTrips() {
        val data = "the quick brown fox ".repeat(20).toByteArray()
        assertContentEquals(data, rawInflate(deflateRaw(data), data.size))
    }

    @Test
    fun declaredTooSmall_failsClosed() {
        val data = ByteArray(500) { (it % 97).toByte() }
        // Declaring fewer bytes than the stream yields must be rejected (was silently truncated).
        assertFailsWith<ZipFormatException> { rawInflate(deflateRaw(data), data.size - 1) }
    }

    @Test
    fun declaredTooBig_failsClosed() {
        val data = ByteArray(500) { (it % 97).toByte() }
        assertFailsWith<ZipFormatException> { rawInflate(deflateRaw(data), data.size + 1) }
    }

    @Test
    fun zeroSize_isEmpty() {
        assertEquals(0, rawInflate(ByteArray(0), 0).size)
    }
}
