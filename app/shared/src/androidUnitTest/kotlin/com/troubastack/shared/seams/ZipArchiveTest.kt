package com.troubastack.shared.seams

import java.io.ByteArrayOutputStream
import java.util.zip.CRC32
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream
import kotlin.test.Test
import kotlin.test.assertContentEquals
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * Exercises the *common* [ZipArchive] parser + [rawInflate] seam on the JVM (via the Android
 * `rawInflate` actual). This is the code that ships on iOS, where there is no zip API — testing it
 * here keeps it verifiable off-device (IOS01 acceptance). Zips are crafted at runtime with
 * `ZipOutputStream`; nothing binary is committed.
 */
class ZipArchiveTest {

    /** DEFLATE entry so [rawInflate] is exercised; pad content so it actually compresses. */
    private fun deflatedZip(entries: List<Pair<String, ByteArray>>): ByteArray {
        val bos = ByteArrayOutputStream()
        ZipOutputStream(bos).use { zos ->
            zos.setMethod(ZipOutputStream.DEFLATED)
            for ((name, content) in entries) {
                zos.putNextEntry(ZipEntry(name))
                zos.write(content)
                zos.closeEntry()
            }
        }
        return bos.toByteArray()
    }

    @Test
    fun parsesDirectory_andInflatesDeflatedEntries() {
        val json = """{"concertId":"c1"}""".repeat(50).toByteArray()   // repetitive → compresses
        val png = ByteArray(4096) { (it % 251).toByte() }
        val bytes = deflatedZip(listOf("bundle.json" to json, "blobs/" to ByteArray(0), "blobs/p.png" to png))

        val archive = ZipArchive.parse(bytes)
        val byName = archive.entries.associateBy { it.name }

        assertTrue(byName.containsKey("bundle.json"))
        assertTrue(byName.getValue("blobs/").isDirectory)
        assertContentEquals(json, archive.readEntry(byName.getValue("bundle.json")))
        assertContentEquals(png, archive.readEntry(byName.getValue("blobs/p.png")))
    }

    @Test
    fun storedEntry_isCopiedVerbatim() {
        // A STORE (method 0) entry requires size + CRC set up front.
        val content = "no compression here".toByteArray()
        val bos = ByteArrayOutputStream()
        ZipOutputStream(bos).use { zos ->
            zos.setMethod(ZipOutputStream.STORED)
            val e = ZipEntry("raw.txt").apply {
                size = content.size.toLong()
                compressedSize = content.size.toLong()
                crc = CRC32().apply { update(content) }.value
            }
            zos.putNextEntry(e)
            zos.write(content)
            zos.closeEntry()
        }
        val archive = ZipArchive.parse(bos.toByteArray())
        val entry = archive.entries.single { it.name == "raw.txt" }
        assertEquals(0, entry.method)
        assertContentEquals(content, archive.readEntry(entry))
    }

    @Test
    fun garbageBytes_throwFormatException() {
        assertFailsWith<ZipFormatException> { ZipArchive.parse(ByteArray(8) { 0x7F }) }
        assertFailsWith<ZipFormatException> { ZipArchive.parse("not a zip at all, no EOCD".toByteArray()) }
    }

    @Test
    fun zipSlipNameGuard_rejectsTraversal_allowsPlainPaths() {
        assertTrue(isSafeZipEntryName("bundle.json"))
        assertTrue(isSafeZipEntryName("blobs/page-01.png"))
        assertTrue(isSafeZipEntryName("blobs/"))          // directory, trailing slash
        assertTrue(isSafeZipEntryName("a/./b.txt"))       // "." segment is benign

        assertFalse(isSafeZipEntryName(""))
        assertFalse(isSafeZipEntryName("../evil.txt"))
        assertFalse(isSafeZipEntryName("blobs/../../evil"))
        assertFalse(isSafeZipEntryName("/etc/passwd"))
        assertFalse(isSafeZipEntryName("C:\\windows"))
        assertFalse(isSafeZipEntryName("dir\\evil.txt"))
    }

    @Test
    fun sizeCapGuard_flagsCumulativeExcess_andOverflowedNegativeSizes() {
        val cap = 1000L
        assertFalse(exceedsSizeCap(written = 0, uncompressedSize = 500, cap = cap))
        assertFalse(exceedsSizeCap(written = 500, uncompressedSize = 500, cap = cap)) // exactly at cap is OK
        assertTrue(exceedsSizeCap(written = 501, uncompressedSize = 500, cap = cap))  // cumulative overflow
        // A >2 GiB uncompressed size overflows the 32-bit field to a negative Int — must NOT slip under.
        assertTrue(exceedsSizeCap(written = 0, uncompressedSize = -1, cap = cap))
        assertTrue(exceedsSizeCap(written = 0, uncompressedSize = Int.MIN_VALUE, cap = cap))
    }
}
