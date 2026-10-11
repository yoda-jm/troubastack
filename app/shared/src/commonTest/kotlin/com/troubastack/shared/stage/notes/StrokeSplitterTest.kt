package com.troubastack.shared.stage.notes

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A85 §5 — the pure split. Two-up spread: page A = x 0..100, page B = x 120..220, a 20 px gutter between them
 * (the shape VLL wrote across on 10-09). Coordinates are chosen so the interpolated edge point differs from
 * every sample: a snap-to-last-sample implementation lands on 90 or 130, never on the edge.
 */
class StrokeSplitterTest {
    private val a = PaperRect(0f, 0f, 100f, 100f)
    private val b = PaperRect(120f, 0f, 220f, 100f)
    private fun p(x: Float, y: Float) = StrokePoint(x, y)

    @Test
    fun a_word_across_the_gutter_is_two_strokes_ending_and_starting_on_the_edges() {
        val segs = splitStroke(listOf(p(50f, 50f), p(90f, 50f), p(110f, 50f), p(130f, 50f), p(170f, 50f)), listOf(a, b))
        assertEquals(
            listOf(
                PageSegment(0, listOf(p(50f, 50f), p(90f, 50f), p(100f, 50f))),
                PageSegment(1, listOf(p(120f, 50f), p(130f, 50f), p(170f, 50f))),
            ),
            segs,
        )
    }

    @Test
    fun the_edge_point_is_interpolated_not_snapped_to_the_last_sample() {
        // (90,40) → (110,60) crosses x = 100 at y = 50; (110,60) → (130,80) enters B at x = 120, y = 70.
        val segs = splitStroke(listOf(p(90f, 40f), p(110f, 60f), p(130f, 80f)), listOf(a, b))
        assertEquals(listOf(p(90f, 40f), p(100f, 50f)), segs[0].points)
        assertEquals(listOf(p(120f, 70f), p(130f, 80f)), segs[1].points)
    }

    @Test
    fun a_fast_flick_over_the_gutter_between_two_samples_still_splits_on_both_edges() {
        val segs = splitStroke(listOf(p(90f, 50f), p(130f, 50f)), listOf(a, b))
        assertEquals(listOf(PageSegment(0, listOf(p(90f, 50f), p(100f, 50f))), PageSegment(1, listOf(p(120f, 50f), p(130f, 50f)))), segs)
    }

    @Test
    fun leaving_into_a_bar_and_coming_back_is_two_strokes_with_no_chord() {
        // Single page: out through the right bar and back in lower down. The old code joined (90,50) to (90,60)
        // with a straight chord; now the first stroke ends at the edge and the second starts there.
        val segs = splitStroke(listOf(p(50f, 50f), p(90f, 50f), p(110f, 50f), p(110f, 60f), p(90f, 60f), p(50f, 60f)), listOf(a))
        assertEquals(
            listOf(
                PageSegment(0, listOf(p(50f, 50f), p(90f, 50f), p(100f, 50f))),
                PageSegment(0, listOf(p(100f, 60f), p(90f, 60f), p(50f, 60f))),
            ),
            segs,
        )
    }

    @Test
    fun a_stroke_inside_one_page_is_unchanged() {
        val pts = listOf(p(10f, 10f), p(12f, 11f), p(40f, 70f), p(99f, 99f))
        assertEquals(listOf(PageSegment(0, pts)), splitStroke(pts, listOf(a, b)))
    }

    @Test
    fun a_stroke_starting_in_the_gutter_starts_on_bs_edge() {
        assertEquals(listOf(PageSegment(1, listOf(p(120f, 50f), p(130f, 50f)))), splitStroke(listOf(p(110f, 50f), p(130f, 50f)), listOf(a, b)))
    }

    @Test
    fun a_dot_is_a_dot_and_a_gutter_tap_is_nothing() {
        assertEquals(listOf(PageSegment(0, listOf(p(30f, 30f)))), splitStroke(listOf(p(30f, 30f)), listOf(a, b)))
        assertEquals(emptyList(), splitStroke(listOf(p(110f, 30f)), listOf(a, b)))
        assertEquals(emptyList(), splitStroke(listOf(p(105f, 30f), p(115f, 40f)), listOf(a, b))) // all gutter
    }

    @Test
    fun touching_pages_split_on_the_shared_edge() {
        val b0 = PaperRect(100f, 0f, 200f, 100f)
        val segs = splitStroke(listOf(p(80f, 50f), p(120f, 50f)), listOf(a, b0))
        assertEquals(listOf(PageSegment(0, listOf(p(80f, 50f), p(100f, 50f))), PageSegment(1, listOf(p(100f, 50f), p(120f, 50f)))), segs)
    }

    @Test
    fun a_page_crossed_entirely_between_two_samples_gets_an_edge_to_edge_stroke() {
        // Three narrow pages; one step from the first to the third passes straight over the middle one.
        val narrow = listOf(PaperRect(0f, 0f, 10f, 100f), PaperRect(20f, 0f, 30f, 100f), PaperRect(40f, 0f, 50f, 100f))
        val segs = splitStroke(listOf(p(5f, 50f), p(45f, 50f)), narrow)
        assertEquals(
            listOf(
                PageSegment(0, listOf(p(5f, 50f), p(10f, 50f))),
                PageSegment(1, listOf(p(20f, 50f), p(30f, 50f))),
                PageSegment(2, listOf(p(40f, 50f), p(45f, 50f))),
            ),
            segs,
        )
    }

    @Test
    fun streaming_events_are_the_ones_the_pages_act_on() {
        // The wet preview and the commit read the SAME events: B's stroke begins in the same move that ends A's.
        val s = StrokeSplitter(listOf(a, b))
        assertEquals(listOf(SplitEvent.Begin(0, p(90f, 50f))), s.down(p(90f, 50f)))
        assertEquals(
            listOf(SplitEvent.Point(0, p(100f, 50f)), SplitEvent.End(0), SplitEvent.Begin(1, p(120f, 50f)), SplitEvent.Point(1, p(130f, 50f))),
            s.move(p(130f, 50f)),
        )
        assertEquals(listOf(SplitEvent.End(1)), s.up())
        assertTrue(s.up().isEmpty(), "an idle splitter carries nothing over (A71 D5)")
    }
}
