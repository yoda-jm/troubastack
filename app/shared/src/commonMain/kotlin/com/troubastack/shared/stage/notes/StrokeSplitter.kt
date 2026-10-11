// A85 — a note stroke that leaves its page is SPLIT, not dropped (VLL, 2026-09-09: "un down à l'entrée sur une
// nouvelle page et un up quand ça en sort"). PURE: no Compose, unit-tested off-device.
//
// Fed the owning pointer's points (from StrokeReader) in the gesture surface's space, plus each visible page's
// PAPER rectangle in that same space. Leaving a page ends that page's stroke AT THE EDGE — the crossing is
// interpolated between the last inside sample and the first outside one, never snapped to the last sample —
// and entering a page starts a new stroke seeded with the entry point on its edge. Points on no paper (the
// gutter, a letterbox bar) draw nothing. Each sample-to-sample step is clipped against every page, so a fast
// flick that jumps the gutter, or crosses a whole narrow page, between two samples still splits correctly. A
// segment of a stroke is a straight line, so it meets a (convex) page in at most one interval.
package com.troubastack.shared.stage.notes

/** One page's paper, as an axis-aligned rectangle in the gesture surface's space (screen px). Inclusive. */
data class PaperRect(val left: Float, val top: Float, val right: Float, val bottom: Float) {
    fun contains(x: Float, y: Float): Boolean = x >= left && x <= right && y >= top && y <= bottom
    fun clamp(p: StrokePoint): StrokePoint = StrokePoint(p.x.coerceIn(left, right), p.y.coerceIn(top, bottom))
}

/** What the splitter tells the pages. A page's segment is: one [Begin], any number of [Point], one [End]. */
sealed interface SplitEvent {
    val page: Int

    /** A new stroke starts on [page], seeded with [point] (the touchdown, or the entry point on its edge). */
    data class Begin(override val page: Int, val point: StrokePoint) : SplitEvent

    /** [point] is appended to [page]'s stroke (a sample, or the exit point on its edge). */
    data class Point(override val page: Int, val point: StrokePoint) : SplitEvent

    /** [page]'s stroke is complete: commit it (the pen left the page, or lifted). */
    data class End(override val page: Int) : SplitEvent
}

/** The streaming splitter for ONE gesture. [papers] are fixed for the gesture (layout does not move mid-stroke). */
class StrokeSplitter(private val papers: List<PaperRect>) {
    private var last: StrokePoint? = null
    private var current: Int? = null

    /** The owner went down at [p]: a stroke starts if [p] is on a page (a gutter touchdown starts nothing yet). */
    fun down(p: StrokePoint): List<SplitEvent> {
        last = p
        current = papers.indexOfFirst { it.contains(p.x, p.y) }.takeIf { it >= 0 }
        return current?.let { listOf(SplitEvent.Begin(it, p)) } ?: emptyList()
    }

    /** The owner moved to [b]. Ends the current page's stroke at its edge and begins the next at its edge. */
    fun move(b: StrokePoint): List<SplitEvent> {
        val a = last ?: return down(b)
        last = b
        val out = ArrayList<SplitEvent>(2)
        // Every page this step touches, as the parameter interval [t0, t1] of a→b inside it, in crossing order.
        val spans = papers.indices.mapNotNull { i -> clip(a, b, papers[i])?.let { Span(i, it.first, it.second) } }.sortedBy { it.t0 }
        var tAt = 0f
        val left = current
        current?.let { c ->
            val s = spans.firstOrNull { it.page == c }
            if (s != null && s.t1 >= 1f) {
                out += SplitEvent.Point(c, papers[c].clamp(b)) // still on the same page
                return out
            }
            val tExit = s?.t1 ?: 0f
            out += SplitEvent.Point(c, papers[c].clamp(lerp(a, b, tExit))) // the edge, interpolated
            out += SplitEvent.End(c)
            current = null
            tAt = tExit
        }
        for (s in spans) {
            if (s.page == left || s.t1 < tAt) continue // the page just left, or behind us
            val entry = papers[s.page].clamp(lerp(a, b, s.t0))
            out += SplitEvent.Begin(s.page, entry)
            if (s.t1 >= 1f) {
                if (s.t0 < 1f) out += SplitEvent.Point(s.page, papers[s.page].clamp(b))
                current = s.page
                break
            }
            // crossed this page entirely between two samples: a two-point stroke, edge to edge
            out += SplitEvent.Point(s.page, papers[s.page].clamp(lerp(a, b, s.t1)))
            out += SplitEvent.End(s.page)
            tAt = s.t1
        }
        return out
    }

    /** The owner lifted: the page under the pen (if any) commits. The splitter is then idle. */
    fun up(): List<SplitEvent> {
        val c = current
        current = null
        last = null
        return c?.let { listOf(SplitEvent.End(it)) } ?: emptyList()
    }

    private data class Span(val page: Int, val t0: Float, val t1: Float)

    private fun lerp(a: StrokePoint, b: StrokePoint, t: Float) = StrokePoint(a.x + (b.x - a.x) * t, a.y + (b.y - a.y) * t)

    /** Liang–Barsky: the [t0, t1] ⊂ [0, 1] of a→b inside [r], or null if the step misses it. */
    private fun clip(a: StrokePoint, b: StrokePoint, r: PaperRect): Pair<Float, Float>? {
        val dx = b.x - a.x
        val dy = b.y - a.y
        var t0 = 0f
        var t1 = 1f
        for ((p, q) in listOf(-dx to a.x - r.left, dx to r.right - a.x, -dy to a.y - r.top, dy to r.bottom - a.y)) {
            if (p == 0f) {
                if (q < 0f) return null
            } else {
                val t = q / p
                if (p < 0f) t0 = maxOf(t0, t) else t1 = minOf(t1, t)
            }
        }
        return if (t0 <= t1) t0 to t1 else null
    }
}

/** A whole stroke split at once (the streaming splitter driven over [points]): one point list per page segment,
 *  in drawing order. A convenience for tests and for any caller that has the full polyline. */
data class PageSegment(val page: Int, val points: List<StrokePoint>)

fun splitStroke(points: List<StrokePoint>, papers: List<PaperRect>): List<PageSegment> {
    if (points.isEmpty()) return emptyList()
    val s = StrokeSplitter(papers)
    val events = s.down(points[0]) + points.drop(1).flatMap { s.move(it) } + s.up()
    val out = ArrayList<PageSegment>()
    var open: Pair<Int, MutableList<StrokePoint>>? = null
    for (e in events) when (e) {
        is SplitEvent.Begin -> open = e.page to mutableListOf(e.point)
        is SplitEvent.Point -> open?.second?.add(e.point)
        is SplitEvent.End -> open?.let { out += PageSegment(it.first, it.second.toList()); open = null }
    }
    return out
}
