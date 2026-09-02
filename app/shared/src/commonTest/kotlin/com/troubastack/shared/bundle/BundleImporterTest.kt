package com.troubastack.shared.bundle

import com.troubastack.shared.seams.UnpackResult
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertTrue

/** What a fake "zip" expands to when unpacked: either a set of files, or a failure. */
private sealed interface Zip {
    data class Files(val files: Map<String, String>) : Zip // relPath -> content
    data class Fail(val reason: String) : Zip
}

/** In-memory [ImportFs]: a path→content map (files only; directories are implicit). */
private class FakeImportFs(private val zips: Map<String, Zip>) : ImportFs {
    val fs = HashMap<String, String>()
    private var tempCounter = 0

    override fun exists(path: String) = fs.containsKey(path) || fs.keys.any { it.startsWith("$path/") }
    override fun readText(path: String) = fs[path]
    override fun sizeOf(path: String) = fs[path]?.length?.toLong() ?: 0L

    override fun makeImportTempDir(): String = "temp/import-${tempCounter++}"

    override fun unpack(zipPath: String, destDir: String): UnpackResult = when (val z = zips[zipPath]) {
        null, is Zip.Fail -> UnpackResult.Failed((z as? Zip.Fail)?.reason ?: "no such archive")
        is Zip.Files -> {
            z.files.forEach { (rel, content) -> fs["$destDir/$rel"] = content }
            UnpackResult.Ok(destDir)
        }
    }

    override fun bundleTargetDir(concertId: String) = "bundles/$concertId"

    override fun move(fromDir: String, toDir: String): Boolean {
        val moved = fs.keys.filter { it == fromDir || it.startsWith("$fromDir/") }
        if (moved.isEmpty()) return false
        moved.forEach { key -> fs[toDir + key.removePrefix(fromDir)] = fs.remove(key)!! }
        return true
    }

    override fun deleteRecursively(path: String) {
        fs.keys.filter { it == path || it.startsWith("$path/") }.forEach { fs.remove(it) }
    }
}

private fun validBundle(concertId: String, marker: String = "r") = Zip.Files(
    mapOf(
        "bundle.json" to """{"concertId":"$concertId","songs":[{"songId":"s1","pages":[{"pageRasterRef":"blobs/p.png","overlays":[]}]}]}""",
        "blobs/p.png" to marker,
    ),
)

class BundleImporterTest {

    @Test
    fun validImport_landsUnderConcertId() {
        val fs = FakeImportFs(mapOf("in.tstage" to validBundle("concert-a")))
        val result = BundleImporter(fs).import("in.tstage")

        assertEquals("concert-a", assertIs<ImportResult.Imported>(result).concertId)
        assertTrue(fs.exists("bundles/concert-a/bundle.json"), "bundle installed under its id")
        assertFalse(fs.fs.keys.any { it.startsWith("temp/") }, "temp cleaned up")
    }

    @Test
    fun invalidBundle_neverAppearsInBundlesDir() {
        val bad = Zip.Files(mapOf("bundle.json" to """{"concertId":"x","songs":[""")) // truncated
        val fs = FakeImportFs(mapOf("in.tstage" to bad))
        val result = BundleImporter(fs).import("in.tstage")

        assertIs<ImportResult.Failed>(result)
        assertFalse(fs.exists("bundles/x"), "a bundle that fails validation must not be installed")
        assertFalse(fs.fs.keys.any { it.startsWith("temp/") }, "temp cleaned up")
    }

    @Test
    fun unpackFailure_isReportedNothingInstalled() {
        val fs = FakeImportFs(mapOf("in.tstage" to Zip.Fail("that file isn't a valid concert archive")))
        val result = BundleImporter(fs).import("in.tstage")
        assertEquals("that file isn't a valid concert archive", assertIs<ImportResult.Failed>(result).reason)
        assertTrue(fs.fs.isEmpty(), "nothing written on unpack failure")
    }

    @Test
    fun reimportSameId_swapsCleanly() {
        val fs = FakeImportFs(mapOf("v1.tstage" to validBundle("c1", "one"), "v2.tstage" to validBundle("c1", "two")))
        val importer = BundleImporter(fs)

        assertIs<ImportResult.Imported>(importer.import("v1.tstage"))
        assertEquals("one", fs.readText("bundles/c1/blobs/p.png"))

        assertIs<ImportResult.Imported>(importer.import("v2.tstage"))
        assertEquals("two", fs.readText("bundles/c1/blobs/p.png"), "re-import replaced the content")
        assertFalse(fs.exists("bundles/c1.replacing"), "no swap-aside leftover")
        assertFalse(fs.fs.keys.any { it.startsWith("temp/") }, "temp cleaned up")
    }
}
