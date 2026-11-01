package com.troubastack.app

import androidx.compose.ui.graphics.Canvas
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.Paint
import androidx.compose.ui.geometry.Offset
import com.troubastack.shared.stage.notes.NoteEntry
import java.io.ByteArrayOutputStream
import java.io.File
import java.nio.file.Files
import kotlin.test.AfterTest
import kotlin.test.Test
import kotlin.test.assertContentEquals
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/** WIN03 D1 — the desktop's Skia page decoder and note codec keep the Android contracts. */
class SkiaBitmapsDesktopTest {
    private val dir: File = Files.createTempDirectory("skia").toFile()

    @AfterTest
    fun cleanUp() { dir.deleteRecursively() }

    /** A [w]×[h] bitmap, transparent, with one opaque red pixel at (1, 1) unless [blank]. */
    private fun bitmap(w: Int, h: Int, blank: Boolean = false): ImageBitmap {
        val b = ImageBitmap(w, h)
        if (!blank) Canvas(b).drawCircle(Offset(1.5f, 1.5f), 0.5f, Paint().apply { color = Color.Red })
        return b
    }

    private fun png(b: ImageBitmap): ByteArray = ByteArrayOutputStream().also { SkiaNoteBitmapCodec.writePng(b, it) }.toByteArray()

    @Test
    fun sampleSize_isTheLargestPowerOfTwoThatKeepsTheTarget() {
        assertEquals(1, decodeSampleSize(1600, 2560, 1600, 2560))
        assertEquals(2, decodeSampleSize(1600, 2560, 800, 1280))
        assertEquals(2, decodeSampleSize(1600, 2560, 700, 1100), "never below the target")
        assertEquals(4, decodeSampleSize(1600, 2560, 400, 640))
        assertEquals(1, decodeSampleSize(0, 10, 10, 10))
    }

    @Test
    fun theDecoder_downsamplesTowardTheTarget_andFailsCleanlyOnAMissingFile() {
        File(dir, "p.png").writeBytes(png(bitmap(400, 300)))
        val decoder = SkiaImageDecoder(dir)
        val small = decoder.decode("p.png", 100, 75).getOrThrow()
        assertEquals(100 to 75, small.width to small.height)
        val full = decoder.decode("p.png", 400, 300).getOrThrow()
        assertEquals(400 to 300, full.width to full.height)
        assertTrue(decoder.decode("missing.png", 10, 10).isFailure)
    }

    @Test
    fun theCodec_roundTripsAPng_andTheDecodedNoteIsMutable() {
        val f = File(dir, "n.png").apply { writeBytes(png(bitmap(8, 8))) }
        val back = assertNotNull(SkiaNoteBitmapCodec.decode(f.path))
        assertEquals(8 to 8, back.width to back.height)
        assertEquals(false, SkiaNoteBitmapCodec.isAllTransparent(back), "the red pixel survived")
        // The pad draws into the loaded note: drawing must not throw (an immutable bitmap would).
        Canvas(back).drawCircle(Offset(5f, 5f), 1f, Paint().apply { color = Color.Black })
    }

    @Test
    fun allTransparent_isTrueOnlyWhenNothingWasDrawn() {
        assertEquals(true, SkiaNoteBitmapCodec.isAllTransparent(bitmap(16, 16, blank = true)))
        assertEquals(false, SkiaNoteBitmapCodec.isAllTransparent(bitmap(16, 16)))
    }

    @Test
    fun theSharedStore_worksWithTheSkiaCodec() {
        val store = FileRehearsalNotes(dir.path, SkiaNoteBitmapCodec)
        val entry = NoteEntry("s1", "h1", "", 0, "Song", "Band", 1, "m1", 8, 8, 1L)
        store.save("c1", entry, bitmap(8, 8))
        val saved = store.index("c1").single()
        val bytes = assertNotNull(store.pngBytes("c1", saved.key))
        assertContentEquals(png(assertNotNull(store.load("c1", saved.key))), bytes, "load and re-encode gives the same PNG")
        store.save("c1", entry, bitmap(8, 8, blank = true))
        assertFalse(store.index("c1").isNotEmpty(), "an all-transparent save deletes the note")
    }
}
