// Generated proto types (NormPoint, Object, …) come from gen/ — single source of truth is
// proto/ (I1). Geometry on the wire is PDF-relative [0,1], never pixels (I3).
package com.troubastack.shared.seams

/**
 * SEAM 2 of 3 — the low-latency ink overlay. The single irreducibly per-platform perf piece,
 * and one of the ONLY three places native code is allowed (I15).
 *
 * This is also the ONE sanctioned re-implementation of stroke rendering (I8): the single
 * stroke renderer is `web/ink`, and this overlay MUST match it **pixel-for-pixel** — guarded
 * by a golden-image parity test. If native and web disagree, strokes jump at the handoff.
 *
 * Scope is as small as physically possible (I9):
 *  - renders ONLY the in-progress (wet) freehand stroke — nothing committed, no other tool;
 *  - lines, shapes, text, move, select all preview in the **web** layer (not latency-critical);
 *  - on pen-up the stroke COMMITS: it migrates native → web and the native layer is CLEARED.
 *
 * Coordinates: the web layer owns the viewport transform; you don't zoom mid-stroke, so the
 * transform is static for a stroke's lifetime — captured at stroke-start, no per-frame
 * native↔web bridge sync (docs/design/03-rendering-and-ink.md). Points are normalized [0,1] (I3).
 *
 * Feature-detected enhancement only — the in-browser wet path is the canonical fallback (I10).
 *
 *  - Android `actual` → Jetpack Ink (`androidx.ink`) / `GLFrontBufferedRenderer`.
 *  - iOS `actual`     → PencilKit / Metal.  // iOS-later
 */
expect class InkOverlay {

    /** Begin a wet stroke. `viewport` is captured here and held static for the stroke (I3). */
    fun beginStroke(/* viewportTransform, NormPoint start */)

    /** Append coalesced points to the wet stroke (lowest input→photon latency path). */
    fun extendStroke(/* coalesced NormPoints */)

    /**
     * End the stroke: hand the finished freehand to the web layer to commit (I9) and clear the
     * native surface. The committed object then lives in Studio / proto-land (I1, I8).
     */
    fun commitStroke()

    /** Discard the wet stroke without committing (e.g. palm rejection, cancel). */
    fun cancelStroke()
}
