package com.troubastack.shared.stage.notes

import kotlin.math.abs
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** A70 §6.1 — pure geometry: the touch→note mapping (letterbox-aware for FIT_PAGE, scroll-aware for
 *  FIT_WIDTH) and the canonical note size. Off-device (A58). */
class NoteGeometryTest {

    private fun near(a: Float, b: Float, eps: Float = 1.5f) = assertTrue(abs(a - b) <= eps, "$a vs $b")

    @Test
    fun fitPage_fourCorners_mapToFourBitmapCorners_andLetterboxIsNull() {
        // A portrait page (aspect 0.75) in a LANDSCAPE box (2000×1000): height-bound, bars left/right.
        // rh=1000, rw=750, ox=(2000-750)/2=625. Teeth: the naive `x*noteW/boxW` ignores the 625px bar and
        // would map the top-left bar into the page.
        val noteW = 1600; val noteH = (1600.0 * 4 / 3).toInt() // 2133, aspect 0.75
        val boxW = 2000; val boxH = 1000
        val ox = 625f
        // top-left of the RASTER (ox, 0) → (0,0)
        val tl = NoteGeometry.touchToNote(ox, 0f, boxW, boxH, noteW, noteH, fillWidth = false)
        assertNotNull(tl); near(tl.x, 0f); near(tl.y, 0f)
        // bottom-right of the raster (ox+750, 1000) → (noteW, noteH)
        val br = NoteGeometry.touchToNote(ox + 750f, 1000f, boxW, boxH, noteW, noteH, fillWidth = false)
        assertNotNull(br); near(br.x, noteW.toFloat()); near(br.y, noteH.toFloat())
        // centre → centre
        val c = NoteGeometry.touchToNote(1000f, 500f, boxW, boxH, noteW, noteH, fillWidth = false)
        assertNotNull(c); near(c.x, noteW / 2f); near(c.y, noteH / 2f)
        // a touch in the LEFT BAR (x < 625) is off the page → null
        assertNull(NoteGeometry.touchToNote(100f, 500f, boxW, boxH, noteW, noteH, fillWidth = false))
    }

    @Test
    fun fitPage_twoUpHalf_mapsWithinItsHalfBox() {
        // A two-up half is just a narrower box; the same math holds. Landscape page (aspect ~1.5) in a
        // near-square half (1000×1000): width-bound, bars top/bottom (~167px each).
        val noteW = 1600; val noteH = (1600.0 / 1.5).toInt() // ~1066, aspect ~1.5
        val boxW = 1000; val boxH = 1000
        // centre → centre (robust to the exact bar height)
        val c = NoteGeometry.touchToNote(500f, 500f, boxW, boxH, noteW, noteH, fillWidth = false)
        assertNotNull(c); near(c.x, noteW / 2f); near(c.y, noteH / 2f, 4f)
        // a touch well inside the top bar is off the page → null
        assertNull(NoteGeometry.touchToNote(500f, 20f, boxW, boxH, noteW, noteH, fillWidth = false))
        // a touch well inside the bottom bar → null
        assertNull(NoteGeometry.touchToNote(500f, 980f, boxW, boxH, noteW, noteH, fillWidth = false))
    }

    @Test
    fun fillWidth_touchBelowVisibleBand_mapsToLowerNoteRow_viaScrollOffset() {
        // FIT_WIDTH: the raster fills width; a portrait page is taller than the box, scrolled. noteW/boxW is
        // the right scale here (both axes). A touch at the box top with a big scroll offset lands LOW in the
        // note. Teeth against ignoring scrollOffsetY (would map to the top row).
        val noteW = 1600; val noteH = 3200 // 2× taller than wide
        val boxW = 800; val boxH = 800 // note renders at 800 wide → content 1600 tall on screen
        val scroll = 800 // scrolled a full box down
        val hit = NoteGeometry.touchToNote(400f, 0f, boxW, boxH, noteW, noteH, fillWidth = true, scrollOffsetY = scroll)
        assertNotNull(hit)
        near(hit.x, noteW / 2f) // centre x
        // y: (0 + 800) * (1600/800) = 1600 = the note's vertical midpoint
        near(hit.y, 1600f, 2f)
        // without the scroll it would be the very top
        val top = NoteGeometry.touchToNote(400f, 0f, boxW, boxH, noteW, noteH, fillWidth = true, scrollOffsetY = 0)
        assertNotNull(top); near(top.y, 0f)
    }

    @Test
    fun noteSize_keepsAspectWithin1px_forDownsampledRasters() {
        // A 3:4 page decoded at 2× (1224×1632) and 4× (612×816) must yield the same note aspect within 1 px.
        val full = NoteGeometry.noteSize(2448, 3264)
        val half = NoteGeometry.noteSize(1224, 1632)
        val quarter = NoteGeometry.noteSize(612, 816)
        assertEquals(NoteTools.NOTE_W, full.width)
        assertTrue(abs(full.height - half.height) <= 1)
        assertTrue(abs(full.height - quarter.height) <= 1)
        // degenerate input is (0,0), never a divide-by-zero
        assertEquals(NoteDim(0, 0), NoteGeometry.noteSize(0, 100))
    }

    @Test
    fun tools_eraserWidth_andPaletteShape() {
        assertEquals(NoteTools.MEDIUM, NoteTools.eraserWidth(NoteTools.FINE)) // 3×3=9 == MEDIUM floor
        assertEquals(NoteTools.WIDE * 3, NoteTools.eraserWidth(NoteTools.WIDE))
        assertEquals(3, NoteTools.WIDTHS.size)
        assertEquals(4, NoteTools.COLOURS.size)
        assertEquals("~notes", NoteTools.RESERVED_LAYER_ID)
    }
}
