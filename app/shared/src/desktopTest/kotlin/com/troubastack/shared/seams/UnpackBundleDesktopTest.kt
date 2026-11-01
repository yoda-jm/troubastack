package com.troubastack.shared.seams

import java.io.File
import java.nio.file.Files
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream
import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertTrue

/**
 * WIN02 — the desktop `unpackBundle` runs on plain `java.util.zip`, so it is a JVM unit test (it runs on
 * BOTH JVMs: Windows in the `windows` job, Linux in the `android` job's `:shared:check`). The zip is crafted
 * at runtime — no malicious binary is committed. Mirrors the Android `UnpackBundleTest` and adds the
 * size-cap refusal (driven with a tiny cap so no 512 MB fixture is needed).
 */
class UnpackBundleDesktopTest {

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

    @Test
    fun oversizedArchive_isRejected_bytheSizeCap() {
        // A highly compressible entry: the zip FILE is tiny but its contents decompress past the cap, so
        // the whole-file guard passes and the written-bytes (zip-bomb) guard is the one that fires.
        val dest = Files.createTempDirectory("unpack-big").toFile()
        val payload = "a".repeat(5_000)
        val zip = zipWith(listOf("bundle.json" to payload))
        assertTrue(zip.length() < 1_000, "the compressed fixture must fit under the test cap to reach the bomb guard")

        val result = unpackBundle(zip.path, dest.path, maxArchiveBytes = 1_000L)
        assertIs<UnpackResult.Failed>(result)
        assertTrue((result as UnpackResult.Failed).reason.contains("too large"), "reason: ${result.reason}")
    }
}
