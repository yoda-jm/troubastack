// Generated proto types come from gen/ — single source of truth is proto/ (I1). Points [0,1] (I3).
package com.troubastack.shared.seams

/**
 * SEAM 2 `actual` (desktop) — low-latency wet ink overlay (I15, I9, I8). `TODO`, exactly as on Android
 * and iOS: A07 never landed, and desktop ink is out of scope for the WIN track (WIN03 ports the Stage
 * screens; the wet-ink seam is its own later piece). Present only so `:shared` compiles for desktop.
 */
actual class InkOverlay {
    actual fun beginStroke() { TODO("Desktop: wet ink overlay not implemented (A07-blocked; out of WIN scope)") }
    actual fun extendStroke() { TODO("Desktop: wet ink overlay not implemented (A07-blocked; out of WIN scope)") }
    actual fun commitStroke() { TODO("Desktop: wet ink overlay not implemented (A07-blocked; out of WIN scope)") }
    actual fun cancelStroke() { TODO("Desktop: wet ink overlay not implemented (A07-blocked; out of WIN scope)") }
}
