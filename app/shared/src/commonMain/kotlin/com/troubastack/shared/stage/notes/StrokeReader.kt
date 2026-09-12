// A71 — the note pad reads the finger directly. A PURE touch→stroke state machine (no Compose): the note
// mode owns the touch (nothing else wants it — §2), so there is nothing to disambiguate and therefore no
// reason for a slop threshold. The touchdown is the first point (D1); every move of the owning pointer is
// appended as it arrives; a down-and-up with no move is a one-point stroke — a dot (D2); the FIRST pointer
// owns the stroke and any later pointer is ignored, including for ending it (D3); the wet list and the owner
// id share one lifetime, so a completed stroke leaves NO carried state (D5).
package com.troubastack.shared.stage.notes

/** A single sampled touch point in the reader's input space (screen px; the caller maps to note space). */
data class StrokePoint(val x: Float, val y: Float)

/**
 * Fed raw pointer phases — [down], [move], [up] — keyed by an opaque pointer id (the Compose PointerId's
 * Long). Returns, from [down]/[move], the point IF it belongs to the owning stroke (so the caller can dab an
 * eraser at touchdown and grow a wet preview), or null for an ignored non-owner event. [up] returns the
 * completed point list when the OWNER lifts (and resets), or null for a non-owner lift (a resting palm does
 * not end the stroke). No smoothing, no minimum distance, no minimum count (§6).
 */
class StrokeReader {
    private var owner: Long? = null
    private val wet = ArrayList<StrokePoint>()

    /** True while a stroke is in progress (an owning pointer is down). */
    val drawing: Boolean get() = owner != null

    /** The points committed so far this stroke (a copy — safe for the caller to hold as preview state). */
    val current: List<StrokePoint> get() = wet.toList()

    /** A pointer went down. The first such pointer takes ownership and seeds the stroke with the touchdown
     *  (D1); returns that point. A down while a stroke is already owned is ignored (D3) → null. */
    fun down(pointerId: Long, x: Float, y: Float): StrokePoint? {
        if (owner != null) return null
        owner = pointerId
        val p = StrokePoint(x, y)
        wet.clear()
        wet.add(p)
        return p
    }

    /** A pointer moved. Appended and returned only for the owner (D3); a non-owner move → null. */
    fun move(pointerId: Long, x: Float, y: Float): StrokePoint? {
        if (pointerId != owner) return null
        val p = StrokePoint(x, y)
        wet.add(p)
        return p
    }

    /** A pointer lifted. If it is the owner, the stroke completes: returns its points and resets to idle
     *  (D5). A non-owner lift → null, and the stroke continues (D3). */
    fun up(pointerId: Long): List<StrokePoint>? {
        if (pointerId != owner) return null
        val done = wet.toList()
        owner = null
        wet.clear()
        return done
    }
}
