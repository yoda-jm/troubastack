package com.troubastack.shared.stage.notes

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * A71 §5.1 — the pure stroke reader. The DISCRIMINATING coordinates are SMALLER than any touch slop (single
 * digits): the whole point of the fix is that a sub-slop stroke is a real two-point stroke, not a dot at the
 * finger-up position. A test using a big displacement would pass against the OLD (slop-based) code too and
 * prove nothing.
 */
class StrokeReaderTest {

    @Test
    fun down_then_up_is_a_one_point_dot() {
        val r = StrokeReader()
        assertEquals(StrokePoint(3f, 4f), r.down(1, 3f, 4f)) // the touchdown IS reported (D1)
        assertEquals(listOf(StrokePoint(3f, 4f)), r.up(1))   // one-point stroke → the dot branch (D2)
    }

    @Test
    fun sub_slop_move_commits_both_points_not_a_dot_at_the_end() {
        // The row the old code failed: |P1-P0| = 2px, far below the ~8dp slop. Must be [P0, P1].
        val r = StrokeReader()
        r.down(1, 10f, 10f)
        r.move(1, 12f, 11f)
        assertEquals(listOf(StrokePoint(10f, 10f), StrokePoint(12f, 11f)), r.up(1))
    }

    @Test
    fun first_committed_point_is_the_touchdown() {
        val r = StrokeReader()
        r.down(1, 5f, 5f); r.move(1, 6f, 5f); r.move(1, 7f, 6f)
        assertEquals(listOf(StrokePoint(5f, 5f), StrokePoint(6f, 5f), StrokePoint(7f, 6f)), r.up(1))
    }

    @Test
    fun first_pointer_owns_the_stroke_second_is_ignored_and_owner_ends_it() {
        val r = StrokeReader()
        assertEquals(StrokePoint(0f, 0f), r.down(1, 0f, 0f))
        assertNull(r.down(2, 9f, 9f))   // second pointer down mid-stroke: ignored (D3)
        assertNull(r.move(2, 8f, 8f))   // its moves: ignored
        assertEquals(StrokePoint(1f, 1f), r.move(1, 1f, 1f))
        val done = assertNotNull(r.up(1)) // owner lifts while id=2 is still down → stroke completes now (D3)
        assertEquals(listOf(StrokePoint(0f, 0f), StrokePoint(1f, 1f)), done)
        assertTrue(StrokePoint(9f, 9f) !in done && StrokePoint(8f, 8f) !in done) // Q* appear nowhere
    }

    @Test
    fun a_non_owner_lift_does_not_end_the_stroke() {
        val r = StrokeReader()
        r.down(1, 0f, 0f); r.down(2, 5f, 5f)
        assertNull(r.up(2))      // the palm (id=2) lifting is not the end
        assertTrue(r.drawing)    // still drawing
        assertEquals(listOf(StrokePoint(0f, 0f)), r.up(1)) // the owner ends it
    }

    @Test
    fun reader_carries_no_state_between_strokes() {
        val r = StrokeReader()
        r.down(1, 0f, 0f); r.move(1, 1f, 1f); r.up(1)
        assertTrue(!r.drawing)
        assertEquals(emptyList(), r.current)
        // A fresh gesture (even a new pointer id) starts clean — no leftover points, no stuck owner (D5).
        assertEquals(StrokePoint(2f, 2f), r.down(2, 2f, 2f))
        assertEquals(listOf(StrokePoint(2f, 2f)), r.up(2))
    }

    @Test
    fun down_reports_an_immediate_point_for_the_eraser_dab() {
        // The eraser dabs at touchdown, not only on the first move — so down() must return the point.
        val r = StrokeReader()
        assertEquals(StrokePoint(4f, 7f), r.down(1, 4f, 7f))
    }
}
