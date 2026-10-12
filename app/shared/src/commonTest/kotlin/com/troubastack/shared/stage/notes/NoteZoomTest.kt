package com.troubastack.shared.stage.notes

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * A86 §6 — the zoomed touch → note mapping, the window clamp and the steps. The surface is VLL's tablet in
 * landscape two-up: 1920×1131 px of surface, two height-bound pages of a 1600×2263 note each.
 */
class NoteZoomTest {
    private val w = 1920f
    private val h = 1131f
    private val half = 960
    private val noteW = 1600
    private val noteH = 2263
    // Each page's paper in its half (box-local), then in surface space.
    private val local = NoteGeometry.paperRect(half, h.toInt(), noteW, noteH, fillWidth = false)!!
    private val left = local
    private val right = PaperRect(local.left + half, local.top, local.right + half, local.bottom)
    private val papers = listOf(left, right)

    /** The whole chain a touch goes through in note mode: zoom → split (here: which page) → paper → note. */
    private fun touchToNoteZoomed(sx: Float, sy: Float, zoom: Float, cx: Float, cy: Float): Pair<Int, Pair<Float, Float>>? {
        val win = NoteZoom.window(zoom, cx, cy, w, h, NoteZoom.paperUnion(papers))
        val s = NoteZoom.screenToSurface(sx, sy, win, zoom)
        val page = papers.indexOfFirst { it.contains(s.x, s.y) }.takeIf { it >= 0 } ?: return null
        val off = if (page == 0) 0f else half.toFloat()
        val n = NoteGeometry.paperToNote(s.x - off, s.y, local, noteW, noteH)
        return page to (n.x to n.y)
    }

    @Test
    fun at_1x_the_chain_is_todays_mapping_in_both_fill_modes() {
        // FIT_PAGE: the zoom term is the identity and the note point is touchToNote's.
        for ((sx, sy) in listOf(300f to 200f, 700f to 1000f, 1300f to 50f, 1500f to 700f)) {
            val got = touchToNoteZoomed(sx, sy, 1f, w / 2, h / 2)!!
            val off = if (sx < half) 0f else half.toFloat()
            val want = NoteGeometry.touchToNote(sx - off, sy, half, h.toInt(), noteW, noteH, fillWidth = false)!!
            assertEquals(want.x, got.second.first, 0.01f); assertEquals(want.y, got.second.second, 0.01f)
        }
        // FIT_WIDTH: the window at 1× is the surface whatever the centre, so the surface point is the touch.
        val win = NoteZoom.window(1f, 10f, 10f, 800f, 600f, NoteGeometry.paperRect(800, 600, noteW, noteH, fillWidth = true))
        assertEquals(PaperRect(0f, 0f, 800f, 600f), win)
        val s = NoteZoom.screenToSurface(400f, 300f, win, 1f)
        assertEquals(NoteGeometry.touchToNote(400f, 300f, 800, 600, noteW, noteH, fillWidth = true), NoteGeometry.touchToNote(s.x, s.y, 800, 600, noteW, noteH, fillWidth = true))
    }

    @Test
    fun at_2x_a_touch_lands_where_the_window_says_at_half_the_size() {
        // Asked to centre on the right page's middle, the 960-wide window would show the page's outer bar: it is
        // clamped so its right edge is the paper's right edge.
        val union = NoteZoom.paperUnion(papers)!!
        val clamped = NoteZoom.window(2f, (right.left + right.right) / 2, h / 2, w, h, union)
        assertEquals(union.right, clamped.right, 0.01f)
        // Centred at x = 1300 the window needs no clamp: 960×565.5 from (820, 282.75).
        val win = NoteZoom.window(2f, 1300f, h / 2, w, h, union)
        assertEquals(PaperRect(820f, 282.75f, 1780f, 848.25f), win)
        // The screen centre is the window centre: surface (1300, 565.5), on the right page.
        val hit = touchToNoteZoomed(960f, 565.5f, 2f, 1300f, h / 2)
        assertNotNull(hit, "the screen centre at 2× must land on the right page, not in the gutter")
        val (page, n) = hit
        assertEquals(1, page)
        val notePerSurfacePx = noteW / (local.right - local.left)
        assertEquals((1300f - half - local.left) * notePerSurfacePx, n.first, 0.5f)
        assertEquals(noteH / 2f, n.second, 0.5f)
        // 200 screen px to the right is 100 surface px: a pen movement lands at half its size on the paper.
        val (_, n2) = touchToNoteZoomed(1160f, 565.5f, 2f, 1300f, h / 2)!!
        assertEquals(100f * notePerSurfacePx, n2.first - n.first, 0.5f)
    }

    @Test
    fun the_window_stays_on_the_paper_at_every_step() {
        val union = NoteZoom.paperUnion(papers)!!
        for (z in NoteZoom.STEPS + 3f) for (c in listOf(-500f to -500f, 0f to 0f, 960f to 565f, 1919f to 1130f, 5000f to 5000f)) {
            val win = NoteZoom.window(z, c.first, c.second, w, h, union)
            assertEquals(w / z, win.right - win.left, 0.01f)
            assertEquals(h / z, win.bottom - win.top, 0.01f)
            assertTrue(win.left >= 0f && win.top >= 0f && win.right <= w + 0.01f && win.bottom <= h + 0.01f, "inside the surface at $z×: $win")
            if (w / z <= union.right - union.left) assertTrue(win.left >= union.left - 0.01f && win.right <= union.right + 0.01f, "on the paper at $z×: $win")
        }
        // A window wider than the paper (one portrait page, 1.5×) is centred on the paper, not on the request.
        val one = listOf(PaperRect(560f, 0f, 1360f, 1131f))
        val win = NoteZoom.window(1.5f, 1800f, 565f, w, h, NoteZoom.paperUnion(one))
        assertEquals(960f, (win.left + win.right) / 2, 0.01f)
    }

    @Test
    fun a_split_still_yields_two_segments_while_zoomed_across_the_seam() {
        // At 3× centred on the seam, a pen movement across the middle of the screen crosses the gutter.
        val win = NoteZoom.window(3f, w / 2, h / 2, w, h, NoteZoom.paperUnion(papers))
        // The window is surface x 640..1280; the gutter between the two papers is 879.8..1040.2 (two 80 px bars).
        val pts = listOf(300f, 600f, 1300f, 1600f).map { NoteZoom.screenToSurface(it, 565f, win, 3f) }
        val segs = splitStroke(pts, papers)
        assertEquals(listOf(0, 1), segs.map { it.page })
        assertEquals(left.right, segs[0].points.last().x, 0.01f)
        assertEquals(right.left, segs[1].points.first().x, 0.01f)
    }

    @Test
    fun steps_walk_1_15_2_and_label_as_the_bar_shows_them() {
        // Capped at 2× by the §4 measurement (see STEPS); the label still reads any factor the way the bar would.
        assertEquals(listOf(1.5f, 2f, 2f), listOf(1f, 1.5f, 2f).map(NoteZoom::zoomIn))
        assertEquals(listOf(1f, 1f, 1.5f), listOf(1f, 1.5f, 2f).map(NoteZoom::zoomOut))
        assertEquals(listOf("1×", "1.5×", "2×"), NoteZoom.STEPS.map(NoteZoom::label))
        assertEquals("3×", NoteZoom.label(3f))
    }

    @Test
    fun a87_the_window_frames_what_shows_above_the_note_bar() {
        // Fit-width portrait: one page fills the width, its paper taller than the surface. The note bar covers the
        // bottom 120 px of a 1131 px surface, so 1011 px show.
        val w = 1200f; val h = 1131f; val bar = 120f
        val paper = PaperRect(0f, 0f, w, 1700f)
        for (z in listOf(1.5f, 2f)) {
            val win = NoteZoom.window(z, w / 2, h / 2, w, h, paper, visibleH = h - bar)
            assertEquals((h - bar) / z, win.bottom - win.top, 0.01f, "window height at $z×")
            assertEquals(w / z, win.right - win.left, 0.01f)
            // The window's bottom edge is the last surface row visible above the bar.
            assertEquals(h - bar, (win.bottom - win.top) * z, 0.01f)
        }
        // The bottom of the page can be brought above the bar: centred at the very bottom, the window ends at
        // the paper's bottom (clamped there, not beyond).
        val low = NoteZoom.window(2f, w / 2, 1700f, w, 1700f, paper, visibleH = 1700f - bar)
        assertEquals(paper.bottom, low.bottom, 0.01f)
        // At 1× nothing moves, bar or not.
        assertEquals(PaperRect(0f, 0f, w, h), NoteZoom.window(1f, 10f, 10f, w, h, paper, visibleH = h - bar))
    }
}
