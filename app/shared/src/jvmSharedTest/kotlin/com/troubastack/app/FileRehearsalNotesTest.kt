package com.troubastack.app

import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.ImageBitmapConfig
import androidx.compose.ui.graphics.colorspace.ColorSpace
import androidx.compose.ui.graphics.colorspace.ColorSpaces
import com.troubastack.shared.stage.notes.NoteEntry
import com.troubastack.shared.stage.notes.NoteKey
import java.io.File
import java.io.OutputStream
import java.nio.file.Files
import kotlin.test.AfterTest
import kotlin.test.Test
import kotlin.test.assertContentEquals
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlin.test.fail

/**
 * WIN03a — the rehearsal-notes on-disk format, now shared by Android and the desktop ([FileRehearsalNotes]). Its
 * first unit tests. The golden half reads a notes directory that MAIN's APK wrote on the emulator (two notes on the
 * demo concert, synthetic), so a change of format, file naming or index serialisation shows here before it reaches
 * a tablet that already holds notes. A fake codec stands in for the bitmap calls (no graphics runtime in a unit test).
 */
class FileRehearsalNotesTest {

    /** A bitmap the fake codec can write and recognise; nothing here draws it. */
    private class FakeBitmap(val bytes: ByteArray, val blank: Boolean = false) : ImageBitmap {
        override val width = 1
        override val height = 1
        override val colorSpace: ColorSpace = ColorSpaces.Srgb
        override val hasAlpha = true
        override val config = ImageBitmapConfig.Argb8888
        override fun readPixels(buffer: IntArray, startX: Int, startY: Int, width: Int, height: Int, bufferOffset: Int, stride: Int) = Unit
        override fun prepareToDraw() = Unit
    }

    private object FakeCodec : NoteBitmapCodec {
        override fun decode(path: String): ImageBitmap? = runCatching { FakeBitmap(File(path).readBytes()) }.getOrNull()
        override fun isAllTransparent(bitmap: ImageBitmap): Boolean? = (bitmap as? FakeBitmap)?.blank
        override fun writePng(bitmap: ImageBitmap, out: OutputStream) = out.write((bitmap as FakeBitmap).bytes)
    }

    private val root: File = Files.createTempDirectory("notes").toFile()

    @AfterTest
    fun cleanUp() { root.deleteRecursively() }

    private fun store() = FileRehearsalNotes(root.path, FakeCodec)

    private fun entry(songId: String, page: Int, rasterHash: String = "h-$songId-$page") = NoteEntry(
        songId = songId, rasterHash = rasterHash, file = "", pageInSong = page, songTitle = "T $songId", bandName = "B",
        concertRev = 3, takenAs = "m1", width = 10, height = 20, updatedAt = 1000L + page, sentAt = 77L, bakesSinceTouched = 2,
    )

    // --- golden: what MAIN's APK wrote ------------------------------------------------------------------------

    /** Copy the committed fixture (main's notes dir) into the temp root; returns its concert id. */
    private fun goldenConcert(): String {
        val src = fixtureDir("win03a-notes")
        src.copyRecursively(root, overwrite = true)
        return src.listFiles()!!.single { it.isDirectory }.name
    }

    @Test
    fun golden_mainsNotesReadAsTheirEntries() {
        val cid = goldenConcert()
        val notes = store().index(cid)
        assertEquals(GOLDEN_SONGS, notes.map { it.songId to it.pageInSong })
        // The file name rule (sha256(songId \n rasterHash)[..16].png) is the one main used: every entry's PNG is found.
        for (n in notes) {
            assertTrue(File(root, "$cid/${n.file}").isFile, "${n.file} on disk")
            assertContentEquals(File(root, "$cid/${n.file}").readBytes(), store().pngBytes(cid, n.key), "pngBytes is the file verbatim")
        }
    }

    @Test
    fun golden_indexReserialisesByteIdentical() {
        // TEETH: any change to the index's JSON shape (a renamed field, encodeDefaults off, pretty printing) reddens.
        val cid = goldenConcert()
        val before = File(root, "$cid/index.json").readBytes()
        store().markSent(cid, NoteKey("no-such-song", "no-such-hash"), 1L) // rewrites the index with the same entries
        assertContentEquals(before, File(root, "$cid/index.json").readBytes())
    }

    @Test
    fun golden_reconcileDeletesNothing() {
        // The upgrade guarantee: reading a consistent directory written by main changes no file.
        val cid = goldenConcert()
        val before = File(root, cid).listFiles()!!.associate { it.name to it.readBytes().toList() }
        store().index(cid)
        val after = File(root, cid).listFiles()!!.associate { it.name to it.readBytes().toList() }
        assertEquals(before, after)
    }

    // --- §3.3 reconcile ---------------------------------------------------------------------------------------

    @Test
    fun crash_pngWrittenIndexNotYet_isReconciledAway() {
        // PNG-first, index-last: a crash between them leaves a PNG no entry names. It is unaddressable (its name is
        // a hash), so index() deletes it and keeps the notes the index does know.
        val s = store()
        s.save("c1", entry("a", 0), FakeBitmap(byteArrayOf(1)))
        val orphan = File(root, "c1/0123456789abcdef.png").apply { writeBytes(byteArrayOf(9)) }
        val notes = s.index("c1")
        assertEquals(listOf("a"), notes.map { it.songId })
        assertFalse(orphan.exists(), "the orphan PNG is deleted")
        assertTrue(File(root, "c1/${notes.single().file}").exists(), "the known note is kept")
    }

    @Test
    fun anEntryWhosePngIsGone_isDropped_andTheIndexRewritten() {
        val s = store()
        s.save("c1", entry("a", 0), FakeBitmap(byteArrayOf(1)))
        s.save("c1", entry("b", 1), FakeBitmap(byteArrayOf(2)))
        File(root, "c1/${s.index("c1").first { it.songId == "a" }.file}").delete()
        assertEquals(listOf("b"), s.index("c1").map { it.songId })
        assertFalse(File(root, "c1/index.json").readText().contains("\"a\""), "the index no longer names it")
    }

    @Test
    fun noDirectory_isNoNotes() = assertEquals(emptyList(), store().index("never"))

    // --- save / delete / send ---------------------------------------------------------------------------------

    @Test
    fun save_writesThePng_andResetsTheNag() {
        val s = store()
        s.save("c1", entry("a", 0), FakeBitmap(byteArrayOf(4, 2)))
        val n = s.index("c1").single()
        assertEquals(0, n.bakesSinceTouched, "a fresh save is untouched")
        assertNull(n.sentAt, "and unsent")
        assertContentEquals(byteArrayOf(4, 2), s.pngBytes("c1", n.key))
        assertFalse(File(root, "c1/${n.file}.tmp").exists(), "no tmp left behind")
    }

    @Test
    fun savingTheSameNoteTwice_theSecondPngWins() {
        // Every edit after a note's first save re-saves it over its own file. TEETH (on Windows): File.renameTo
        // refuses an existing target, so the second save was dropped and the first PNG stayed.
        val s = store()
        s.save("c1", entry("a", 0), FakeBitmap(byteArrayOf(1, 1)))
        s.save("c1", entry("a", 0), FakeBitmap(byteArrayOf(2, 2, 2)))
        val n = s.index("c1").single()
        assertContentEquals(byteArrayOf(2, 2, 2), s.pngBytes("c1", n.key))
        assertNoTmp()
    }

    @Test
    fun noWriteLeavesATmpBehind() {
        // Every write is tmp + move into place; a move that fails (renameTo onto an existing file on Windows) leaves the
        // tmp. Checked after each kind of write, and on main's golden so its byte-identical check cannot pass vacuously.
        val s = store()
        s.save("c1", entry("a", 0), FakeBitmap(byteArrayOf(1))); assertNoTmp()
        s.save("c1", entry("b", 1), FakeBitmap(byteArrayOf(2))); assertNoTmp()
        s.markSent("c1", s.index("c1").first().key, 9L); assertNoTmp()
        s.delete("c1", s.index("c1").first().key); assertNoTmp()
        val cid = goldenConcert()
        store().markSent(cid, NoteKey("no-such-song", "no-such-hash"), 1L); assertNoTmp()
    }

    private fun assertNoTmp() {
        val left = root.walkTopDown().filter { it.isFile && it.name.endsWith(".tmp") }.map { it.name }.toList()
        assertEquals(emptyList(), left, "no tmp file is left behind")
    }

    @Test
    fun save_ofAnAllTransparentBitmap_deletesTheNote() {
        val s = store()
        s.save("c1", entry("a", 0), FakeBitmap(byteArrayOf(1)))
        s.save("c1", entry("a", 0), FakeBitmap(byteArrayOf(), blank = true))
        assertEquals(emptyList(), s.index("c1"))
    }

    @Test
    fun save_ofABitmapTheCodecCannotRead_isANoOp() {
        val s = store()
        val other = object : ImageBitmap by FakeBitmap(byteArrayOf()) {}
        s.save("c1", entry("a", 0), other)
        assertFalse(File(root, "c1").exists(), "nothing written")
    }

    @Test
    fun markSent_delete_deleteAll() {
        val s = store()
        s.save("c1", entry("a", 0), FakeBitmap(byteArrayOf(1)))
        s.save("c1", entry("b", 1), FakeBitmap(byteArrayOf(2)))
        val a = s.index("c1").first { it.songId == "a" }
        s.markSent("c1", a.key, 4242L)
        assertEquals(4242L, s.index("c1").first { it.songId == "a" }.sentAt)
        s.delete("c1", a.key)
        assertEquals(listOf("b"), s.index("c1").map { it.songId })
        assertFalse(File(root, "c1/${a.file}").exists())
        s.deleteAll("c1")
        assertFalse(File(root, "c1").exists())
    }

    @Test
    fun load_decodesTheStoredFile() {
        val s = store()
        s.save("c1", entry("a", 0), FakeBitmap(byteArrayOf(7, 7)))
        val key = s.index("c1").single().key
        assertContentEquals(byteArrayOf(7, 7), (s.load("c1", key) as FakeBitmap).bytes)
        assertNull(s.load("c1", NoteKey("x", "y")))
    }

    /** A committed fixture dir: the test classpath first, then the source tree (as FixtureBundleTest does). */
    private fun fixtureDir(rel: String): File {
        val res = javaClass.classLoader?.getResource("fixtures/$rel")
        if (res != null && res.protocol == "file") return File(res.toURI())
        val cwd = System.getProperty("user.dir")
        for (base in listOf("src/jvmSharedTest/resources", "shared/src/jvmSharedTest/resources", "app/shared/src/jvmSharedTest/resources")) {
            val f = File(cwd, "$base/fixtures/$rel")
            if (f.isDirectory) return f
        }
        fail("fixture '$rel' not found (cwd=$cwd)")
    }

    private companion object {
        /** The (songId, pageInSong) of the two notes main's APK wrote: the demo's first two pages. */
        val GOLDEN_SONGS = listOf(
            "591f5bdc-a151-58ec-ba4c-e8f590ea00d3" to 0, // House of the Rising Sun (demo), page 1
            "a7eb81c8-f078-545e-bd79-8dd9ed62a544" to 0, // Amazing Grace (demo), page 1
        )
    }
}
