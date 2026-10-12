// A86 — zoom in note mode: write big under the pen, land small on the page (VLL, 2026-10-09: "des contrôles
// zoom +/− 1:1 et une sorte d'overview dans un coin pour bouger la fenêtre de zoom"). PURE: no Compose.
//
// The zoom magnifies the whole note-mode SURFACE (the single page or the two-up spread) about a WINDOW: the
// part of the surface shown on screen. A touch on screen is first taken back to the surface by
// [NoteZoom.screenToSurface]; from there A85's splitter and NoteGeometry's paper map do the rest. So there is
// still ONE chain from a touch to a note pixel for the pencil, the eraser, the split and the wet preview —
// the zoom is a term in it, not a second copy of it.
package com.troubastack.shared.stage.notes

import kotlin.math.roundToInt

object NoteZoom {
    /** ⟨D1⟩ the zoom steps: `+` and `−` walk them, `1:1` returns to the first. VLL's design is 1×–3×; §4 caps them
     *  at the largest factor whose committed ink is as crisp as its wet preview. Measured 2026-10-12 (two-up
     *  landscape, 1200×1920 panel): at 2× one note pixel is ~one screen pixel and the pair matches; at 3× the 1600 px
     *  note is upscaled ~1.6× and the committed stroke is visibly softer than the wet one. 3× returns with a higher
     *  NOTE_W, which is a format change brought to the gate as a proposal — not raised here. */
    val STEPS = listOf(1f, 1.5f, 2f)
    private const val EPS = 1e-3f

    fun zoomIn(z: Float): Float = STEPS.firstOrNull { it > z + EPS } ?: STEPS.last()
    fun zoomOut(z: Float): Float = STEPS.lastOrNull { it < z - EPS } ?: STEPS.first()
    fun canZoomIn(z: Float): Boolean = z < STEPS.last() - EPS
    fun canZoomOut(z: Float): Boolean = z > STEPS.first() + EPS

    /** The factor as the note bar shows it: `1×`, `1.5×`, `2×`, `3×`. */
    fun label(z: Float): String {
        val tenths = (z * 10).roundToInt()
        return if (tenths % 10 == 0) "${tenths / 10}×" else "${tenths / 10}.${tenths % 10}×"
    }

    /**
     * The window shown at [zoom] over a [w]×[h] surface: [w]/[zoom] × [h]/[zoom], centred as near the surface
     * point ([cx], [cy]) as the paper allows. ⟨D2⟩ it never shows beyond the paper: it is clamped inside
     * [paper] (the union of the visible pages, surface px), and on an axis where the window is larger than the
     * paper it is centred on the paper. It always stays inside the surface, so at 1× it IS the surface. A87:
     * [visibleH] is the surface height the writer can actually see (the surface minus the note bar over it).
     */
    fun window(zoom: Float, cx: Float, cy: Float, w: Float, h: Float, paper: PaperRect?, visibleH: Float = h): PaperRect {
        val z = zoom.coerceAtLeast(1f)
        // A87 — at 1× the view is never moved. Zoomed, the window is what the writer can SEE: the note bar covers
        // the bottom of the surface, so only [visibleH] of it shows; the window is visibleH / zoom tall, and the
        // paper clamp then lets the bottom of the page come up above the bar.
        if (z <= 1f) return PaperRect(0f, 0f, w, h)
        val ww = w / z
        val wh = visibleH.coerceIn(1f, h) / z
        val b = paper ?: PaperRect(0f, 0f, w, h)
        fun start(c: Float, size: Float, lo: Float, hi: Float, max: Float): Float {
            val s = if (size >= hi - lo) (lo + hi) / 2f - size / 2f else (c - size / 2f).coerceIn(lo, hi - size)
            return s.coerceIn(0f, (max - size).coerceAtLeast(0f))
        }
        val left = start(cx, ww, b.left, b.right, w)
        val top = start(cy, wh, b.top, b.bottom, h)
        return PaperRect(left, top, left + ww, top + wh)
    }

    /** A point on screen (the surface's own box, px) → the surface point under it, through [window] at [zoom]. */
    fun screenToSurface(x: Float, y: Float, window: PaperRect, zoom: Float): StrokePoint =
        StrokePoint(window.left + x / zoom, window.top + y / zoom)

    /** The union of the visible pages' paper — what the window is clamped to. Null when there is no page. */
    fun paperUnion(papers: List<PaperRect>): PaperRect? =
        if (papers.isEmpty()) null
        else PaperRect(papers.minOf { it.left }, papers.minOf { it.top }, papers.maxOf { it.right }, papers.maxOf { it.bottom })
}
