package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A87 — ✓ (the only exit from note mode, A73) stays on screen in portrait. Widths measured on VLL's tablet
 * (1200×1920, uiautomator bounds, 2026-10-12): the bar's inner width in portrait is 1200 − 2×21 = 1158 px;
 * "Clear page" at its natural width is 160 px; ✓ is 119 px; the gap is 6 dp = 11 px. The A86 tool row (four
 * zoom controls) was 1114 px — more than the 857 px a packed row left it, which is what pushed ✓ off screen.
 */
class NoteBarLayoutTest {
    private val portrait = 1158
    private val landscape = 1878
    private val clear = 160
    private val done = 119
    private val gap = 11
    private val a86Tools = 1114 // the A86 tool row, wider than portrait leaves it
    private val a87Tools = 853  // A87: one zoom chip — fits

    @Test
    fun done_and_clear_are_inside_the_portrait_width_at_their_natural_size() {
        val s = noteBarSlots(portrait, a86Tools, clear, done, gap)
        assertTrue(s.doneX >= 0 && s.doneX + done <= portrait, "✓ must be on screen: x=${s.doneX}")
        assertTrue(s.clearX + clear <= s.doneX, "Clear page must not overlap ✓")
        assertTrue(s.toolsMax + gap <= s.clearX, "the tools give way, not Clear page or ✓")
        assertEquals(portrait - done, s.doneX)
    }

    @Test
    fun landscape_keeps_the_same_right_edge_and_gives_the_tools_the_rest() {
        val s = noteBarSlots(landscape, a86Tools, clear, done, gap)
        assertEquals(landscape - done, s.doneX)
        assertEquals(a86Tools, s.toolsMax, "landscape has room: the tools keep their natural width")
    }

    @Test
    fun a_degenerate_width_never_goes_negative() {
        val s = noteBarSlots(100, a86Tools, clear, done, gap)
        assertTrue(s.doneX >= 0 && s.clearX >= 0 && s.toolsMax >= 0)
    }

    @Test
    fun the_a87_tool_row_fits_portrait_at_its_natural_width() {
        assertEquals(a87Tools, noteBarSlots(portrait, a87Tools, clear, done, gap).toolsMax, "no scrolling needed in portrait")
    }
}
