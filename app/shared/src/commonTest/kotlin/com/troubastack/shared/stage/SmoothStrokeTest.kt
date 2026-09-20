package com.troubastack.shared.stage

import androidx.compose.ui.geometry.Offset
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A76 — the pure geometry behind the drawn/erased curve ([smoothStroke]). The eraser sweeps the SAME smoothed
 * path the pencil draws (Fable), so the one property both rely on is: the path STARTS at the first sample and
 * ENDS at the last. If either endpoint drifted, a fast stroke and its erase would no longer cancel.
 */
class SmoothStrokeTest {

    private fun endpointOf(v: StrokeVerb): Offset = when (v) {
        is StrokeVerb.Move -> v.to
        is StrokeVerb.Line -> v.to
        is StrokeVerb.Quad -> v.to
    }

    @Test
    fun empty_isNoVerbs() {
        assertEquals(emptyList(), smoothStroke(emptyList()))
    }

    @Test
    fun onePoint_isJustAMoveToThatPoint() {
        val p = Offset(3f, 4f)
        assertEquals(listOf(StrokeVerb.Move(p)), smoothStroke(listOf(p)))
    }

    @Test
    fun twoPoints_isAStraightLine_noBezier() {
        // Under three samples there is no midpoint to curve through — a plain move+line, so a two-sample stroke
        // is exactly its chord (and the eraser clears exactly that chord).
        val a = Offset(1f, 1f); val b = Offset(9f, 5f)
        assertEquals(listOf(StrokeVerb.Move(a), StrokeVerb.Line(b)), smoothStroke(listOf(a, b)))
    }

    @Test
    fun pathStartsAtFirstSampleAndEndsAtLast() {
        // The property the eraser depends on, exercised with enough points to trigger the Bézier branch.
        val pts = listOf(Offset(0f, 0f), Offset(10f, 20f), Offset(30f, 5f), Offset(50f, 40f), Offset(70f, 0f))
        val verbs = smoothStroke(pts)
        assertTrue(verbs.first() is StrokeVerb.Move)
        assertEquals(pts.first(), endpointOf(verbs.first()))
        assertEquals(pts.last(), endpointOf(verbs.last()))
    }

    @Test
    fun threePlusPoints_curveThroughMidpoints() {
        // First interior step is a Line to the first midpoint (seeds the curve); the rest are Quads whose
        // control point is the ORIGINAL sample and whose endpoint is the next midpoint — the finger-draw scheme.
        val a = Offset(0f, 0f); val b = Offset(10f, 0f); val c = Offset(20f, 0f)
        val verbs = smoothStroke(listOf(a, b, c))
        assertEquals(StrokeVerb.Move(a), verbs[0])
        assertEquals(StrokeVerb.Line(Offset(5f, 0f)), verbs[1])          // midpoint a→b
        assertEquals(StrokeVerb.Quad(b, Offset(15f, 0f)), verbs[2])      // ctrl b, midpoint b→c
        assertEquals(StrokeVerb.Line(c), verbs[3])                       // close on the last sample
    }
}
