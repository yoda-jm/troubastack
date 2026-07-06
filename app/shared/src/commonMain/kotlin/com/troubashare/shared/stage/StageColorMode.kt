// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubashare.shared.stage

import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.ColorMatrix

/**
 * Stage rendering mode (A10). A pure LOCAL reading preference (like fit/role) — a draw-time color
 * transform only; never mutates or re-encodes the cached bitmaps and never touches the bake (I12).
 */
enum class StageColorMode {
    /** Paper as baked (white). */
    NORMAL,
    /** Inverted for a dark venue — paper near-black, ink light. */
    NIGHT,
    ;

    /** Cycle to the next mode (one chrome button). */
    fun next(): StageColorMode = if (this == NORMAL) NIGHT else NORMAL

    companion object {
        /** Parse a persisted value; anything unknown/absent → NORMAL. */
        fun parse(value: String?): StageColorMode = entries.firstOrNull { it.name == value } ?: NORMAL
    }
}

/**
 * The draw-time [ColorFilter] for a mode, applied identically to the page raster AND its overlays so
 * composites stay coherent (an inverted page inverts its cues too — accepted for v1). NORMAL → none.
 * NIGHT → straight RGB inversion (matrix −1 on the diagonal + 255 offset); alpha untouched so
 * transparent overlay margins stay transparent.
 */
fun StageColorMode.pageColorFilter(): ColorFilter? = when (this) {
    StageColorMode.NORMAL -> null
    StageColorMode.NIGHT -> ColorFilter.colorMatrix(
        ColorMatrix(
            floatArrayOf(
                -1f, 0f, 0f, 0f, 255f,
                0f, -1f, 0f, 0f, 255f,
                0f, 0f, -1f, 0f, 255f,
                0f, 0f, 0f, 1f, 0f,
            ),
        ),
    )
}
