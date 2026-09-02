// Generated proto types come from gen/ — single source of truth is proto/ (I1). Points [0,1] (I3).
package com.troubastack.shared.seams

/**
 * SEAM 2 `actual` (iOS) — low-latency wet ink overlay (I15, I9, I8). ⚠️ iOS-LATER: TODO.
 * Concrete API: PencilKit (`PKCanvasView`) or a custom Metal front buffer.
 *
 * Renders ONLY the in-progress freehand stroke; MUST match `web/ink` pixel-for-pixel — to be
 * guarded by a golden-image parity test (not yet written), I8. On pen-up, hand the stroke to
 * web and clear (I9).
 */
actual class InkOverlay {
    actual fun beginStroke() { TODO("iOS-later: PencilKit / Metal stroke begin; capture viewport") }
    actual fun extendStroke() { TODO("iOS-later: append coalesced touches") }
    actual fun commitStroke() { TODO("iOS-later: finish; hand geometry to web (I9); clear surface") }
    actual fun cancelStroke() { TODO("iOS-later: discard wet stroke") }
}
