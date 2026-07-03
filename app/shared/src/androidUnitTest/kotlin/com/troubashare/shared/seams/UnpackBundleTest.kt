package com.troubashare.shared.seams

import java.io.File
import java.nio.file.Files
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream
import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertTrue

/**
 * unpackBundle runs on plain java.util.zip (no Android framework), so it's testable as a JVM unit
 * test. Zip is crafted at runtime with ZipOutputStream — no malicious binary is committed.
 */
class UnpackBundleTest {

    private fun zipWith(entries: List<Pair<String, String>>): File {
        val zip = File.createTempFile("tstage", ".zip")
        ZipOutputStream(zip.outputStream()).use { zos ->
            for ((name, content) in entries) {
                zos.putNextEntry(ZipEntry(name))
                zos.write(content.toByteArray())
                zos.closeEntry()
            }
        }
        return zip
    }

    @Test
    fun zipSlipEntry_isRejected_andWritesNothingOutside() {
        val dest = Files.createTempDirectory("unpack-dest").toFile()
        val outside = File(dest.parentFile, "evil.txt")
        outside.delete()

        val zip = zipWith(listOf("bundle.json" to "{}", "../evil.txt" to "pwned"))
        val result = unpackBundle(zip.path, dest.path)

        assertIs<UnpackResult.Failed>(result)
        assertFalse(outside.exists(), "zip-slip target must NOT be written outside destDir")
    }

    @Test
    fun validZip_unpacksIntoDest() {
        val dest = Files.createTempDirectory("unpack-ok").toFile()
        val zip = zipWith(listOf("bundle.json" to "{\"concertId\":\"c1\"}", "blobs/p.png" to "raster"))

        assertIs<UnpackResult.Ok>(unpackBundle(zip.path, dest.path))
        assertTrue(File(dest, "bundle.json").isFile)
        assertTrue(File(dest, "blobs/p.png").isFile)
    }
}
