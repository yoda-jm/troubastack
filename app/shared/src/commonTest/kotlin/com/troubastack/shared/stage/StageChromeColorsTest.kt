package com.troubastack.shared.stage

import androidx.compose.ui.graphics.Color
import kotlin.math.pow
import kotlin.test.Test
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * A69 ⟨R1⟩ — the chrome palette is PURE (Ruling 1), so its legibility is unit-tested here rather than left
 * to a screenshot. NORMAL returns null (the M3 light baseline is used verbatim ⇒ day pixel-identical); the
 * three reading schemes return a palette whose text clears WCAG AA on its own ground, and whose blackout
 * schemes are actually dark — the whole reason A69 exists (A37: a mistimed tap must not flood white).
 */
class StageChromeColorsTest {
    private fun lin(c: Float): Double {
        val s = c.toDouble()
        return if (s <= 0.03928) s / 12.92 else ((s + 0.055) / 1.055).pow(2.4)
    }
    private fun lum(c: Color): Double = 0.2126 * lin(c.red) + 0.7152 * lin(c.green) + 0.0722 * lin(c.blue)
    private fun contrast(a: Color, b: Color): Double {
        val hi = maxOf(lum(a), lum(b)); val lo = minOf(lum(a), lum(b))
        return (hi + 0.05) / (lo + 0.05)
    }

    @Test fun normal_delegates_to_the_baseline() {
        assertNull(stageChromePalette(StageColorMode.NORMAL), "NORMAL must have no palette ⇒ the M3 baseline is used verbatim")
    }

    @Test fun every_reading_scheme_chrome_is_legible() {
        for (mode in listOf(StageColorMode.WARM, StageColorMode.NIGHT, StageColorMode.AMBER)) {
            val c = stageChromePalette(mode)!!
            assertTrue(contrast(c.onSurface, c.surface) >= 4.5, "$mode primary text ${contrast(c.onSurface, c.surface)} < 4.5:1")
            assertTrue(contrast(c.onSurfaceVariant, c.surface) >= 3.0, "$mode secondary text ${contrast(c.onSurfaceVariant, c.surface)} < 3.0:1")
            assertTrue(contrast(c.onContainer, c.container) >= 4.5, "$mode container text ${contrast(c.onContainer, c.container)} < 4.5:1")
        }
    }

    @Test fun the_blackout_schemes_are_actually_dark() {
        // TEETH: A69 finishes A37's rule that a mistimed tap can't flood a blackout with white. NIGHT and
        // AMBER chrome MUST be a dark surface; "fix" it by returning a light one and this reddens.
        for (mode in listOf(StageColorMode.NIGHT, StageColorMode.AMBER)) {
            assertTrue(lum(stageChromePalette(mode)!!.surface) < 0.08, "$mode chrome surface must be dark (blackout-safe)")
        }
    }

    @Test fun amber_ink_stays_warm_never_white() {
        // AMBER preserves dark-adapted vision: its ink is amber, not a cold white (red must lead blue).
        val ink = stageChromePalette(StageColorMode.AMBER)!!.onSurface
        assertTrue(ink.red > ink.blue + 0.2f, "AMBER ink must be warm amber, not white")
    }
}
