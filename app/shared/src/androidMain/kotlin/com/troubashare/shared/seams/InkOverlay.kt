// Generated proto types come from gen/ — single source of truth is proto/ (I1). Points [0,1] (I3).
package com.troubashare.shared.seams

/**
 * SEAM 2 `actual` (Android) — low-latency wet ink overlay (I15, I9, I8).
 * Concrete API: Jetpack Ink (`androidx.ink`: `InProgressStrokesView` / `androidx.ink.rendering`)
 * over a `GLFrontBufferedRenderer` front buffer for lowest input→photon latency.
 *
 * Renders ONLY the in-progress freehand stroke; must match `web/ink` PIXEL-FOR-PIXEL (golden
 * parity test, I8). On pen-up, hand the stroke to web and clear (I9).
 */
actual class InkOverlay {
    actual fun beginStroke() { TODO("Android: androidx.ink InProgressStrokesView.startStroke; capture viewport") }
    actual fun extendStroke() { TODO("Android: addToStroke with getHistoricalAxisValue / coalesced points") }
    actual fun commitStroke() { TODO("Android: finishStroke; hand geometry to web (I9); clear front buffer") }
    actual fun cancelStroke() { TODO("Android: cancelStroke; clear front buffer") }
}
