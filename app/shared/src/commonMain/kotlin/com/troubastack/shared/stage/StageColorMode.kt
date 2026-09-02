// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.stage

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.ColorMatrix

/**
 * Stage reading colour scheme (A10 → A37). A pure LOCAL reading preference (like fit/role) — a
 * draw-time colour transform only; never mutates or re-encodes the cached bitmaps and never touches the
 * bake (I12). A10 shipped NORMAL + NIGHT; A37 grows it to four reading conditions.
 *
 * **The declaration order IS the ping-pong path** (Normal → Warm → Night → Amber, dark increasing with
 * the ordinal). [stageSchemeStep] walks up to the darkest then back down and never jumps dark→white,
 * so a mistimed on-stage tap in a pit blackout can't flood the player with a full-white page (A37
 * Ruling 1/1b).
 */
enum class StageColorMode {
    /** Paper as baked (white). Lit stage, daylight practice. */
    NORMAL,

    /** Cream paper, warm-dark ink — glare / blue-light comfort for long practice. Blacks stay black. */
    WARM,

    /** Inverted for a dark venue — paper near-black, ink light. */
    NIGHT,

    /** Black paper, amber ink — pit / blackout; preserves dark-adapted vision (stand-light amber). */
    AMBER,
    ;

    /** A short human label for the chrome cycle button and the Parameters selector. */
    fun label(): String = when (this) {
        NORMAL -> "Normal"
        WARM -> "Warm"
        NIGHT -> "Night"
        AMBER -> "Amber"
    }

    /** True for the schemes whose ink ground is dark — the ones a full-white page would blind (A37). */
    val isDark: Boolean get() = this == NIGHT || this == AMBER

    companion object {
        /** Parse a persisted value; anything unknown/absent → NORMAL. */
        fun parse(value: String?): StageColorMode = entries.firstOrNull { it.name == value } ?: NORMAL
    }
}

/** Which way the on-stage cycle is currently walking. UP steps toward the darkest scheme (Amber). This
 *  is momentary walk-state, NOT a preference: it is never persisted and resets to [INITIAL] on a cold
 *  start and on a direct pick from Parameters (A37 Ruling 1b). */
enum class SchemeCycleDirection {
    UP,
    DOWN,
    ;

    companion object {
        /** The direction every walk starts from — toward darker. The first tap after any restart is
         *  therefore fully determined by the scheme on screen (predictable-from-what-you-see, Ruling 1b). */
        val INITIAL = UP
    }
}

/** One ping-pong step's result: the scheme to show and the direction to remember for the next tap. */
data class SchemeStep(val mode: StageColorMode, val direction: SchemeCycleDirection)

/**
 * The on-stage cycle step — PURE (a function of its arguments, no hidden field), so all of Ruling 1b is
 * table-testable, the A34/T85 precedent. Ping-pong: walk up to Amber then back down. At either endpoint
 * the direction flips and steps, so a tap is never a no-op (Ruling 3) and a dark scheme (Night/Amber)
 * never steps straight to Normal (Ruling 1).
 */
fun stageSchemeStep(mode: StageColorMode, direction: SchemeCycleDirection): SchemeStep {
    val order = StageColorMode.entries
    val i = mode.ordinal
    val dir = when {
        direction == SchemeCycleDirection.UP && i == order.lastIndex -> SchemeCycleDirection.DOWN
        direction == SchemeCycleDirection.DOWN && i == 0 -> SchemeCycleDirection.UP
        else -> direction
    }
    val next = if (dir == SchemeCycleDirection.UP) i + 1 else i - 1
    return SchemeStep(order[next], dir)
}

/** A DIRECT pick (from the A36 Parameters selector) is a fresh start, not a continuation of a walk, so
 *  it resets the direction to [SchemeCycleDirection.INITIAL] — set Amber, walk down to Night, pick Warm
 *  here, and the next on-stage tap goes to Night, not back to Normal (A37 Ruling 1b, criterion 2). */
fun stageSchemeSelect(mode: StageColorMode): SchemeStep = SchemeStep(mode, SchemeCycleDirection.INITIAL)

/**
 * The draw-time [ColorFilter] for a scheme, applied identically to the page raster AND its overlays so
 * composites stay coherent (an inverted page inverts its cues too — accepted for v1, A10). Values live
 * here, NOT in the A36 theme — they are performance decisions about a dark room, not brand decisions.
 * NORMAL → none. Matrices are row-major RGBA + a 0..255 offset column; alpha row is identity so
 * transparent overlay margins stay transparent.
 */
fun StageColorMode.pageColorFilter(): ColorFilter? = when (this) {
    StageColorMode.NORMAL -> null
    // Diagonal tint, no offset → blacks stay black, white → ≈#FFF5D1 (warm cream). Glare/blue-light comfort.
    StageColorMode.WARM -> ColorFilter.colorMatrix(
        ColorMatrix(
            floatArrayOf(
                1.00f, 0f, 0f, 0f, 0f,
                0f, 0.96f, 0f, 0f, 0f,
                0f, 0f, 0.82f, 0f, 0f,
                0f, 0f, 0f, 1f, 0f,
            ),
        ),
    )
    // Straight RGB inversion (−1 diagonal + 255 offset). Near-black paper, light ink.
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
    // Invert THEN warm: R'=255−R · G'=(255−G)×0.75 · B'=(255−B)×0.45. Black → amber ink ≈#FFBF73,
    // white → black paper. Preserves dark-adapted vision in a pit/blackout.
    StageColorMode.AMBER -> ColorFilter.colorMatrix(
        ColorMatrix(
            floatArrayOf(
                -1.00f, 0f, 0f, 0f, 255.00f,
                0f, -0.75f, 0f, 0f, 191.25f,
                0f, 0f, -0.45f, 0f, 114.75f,
                0f, 0f, 0f, 1f, 0f,
            ),
        ),
    )
}

/**
 * N9 — the placeholder tint shown behind a page while its bitmap is still decoding, so a turn never
 * flashes a BLACK void mid-slide (and a cold start into a dark scheme never flashes a WHITE one). Each
 * matches the paper the page will settle to: near-paper in NORMAL, cream in WARM, a dark-but-distinct
 * tint in NIGHT/AMBER (warm for AMBER).
 */
fun StageColorMode.pagePlaceholder(): Color = when (this) {
    StageColorMode.NORMAL -> Color(0xFFEDEDED) // near-paper
    StageColorMode.WARM -> Color(0xFFEDE4C2)   // dimmed cream (the warm paper, one notch down)
    StageColorMode.NIGHT -> Color(0xFF1A1A1A)  // dark, still distinct from the pure-black canvas
    StageColorMode.AMBER -> Color(0xFF1A1710)  // dark, faintly warm — no cool flash before amber ink
}
