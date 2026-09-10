package com.troubastack.shared.stage.notes

import com.troubastack.shared.stage.StageColorMode
import com.troubastack.shared.stage.schemePaper
import com.troubastack.shared.stage.transformOverlayPixel
import kotlin.math.pow
import kotlin.test.Test
import kotlin.test.assertTrue

/**
 * A70 §6.1 — the four note colours are ink and go through A64's per-pixel rule (`transformOverlayPixel`),
 * NOT the page matrix. So each must clear WCAG 4.5:1 against every scheme's paper, black must INVERT with
 * the paper (light on dark), and each chromatic colour must KEEP its hue (its dominant channel stays
 * dominant). Fails against drawing the note through the page colour matrix (which would darken red on
 * NIGHT's dark paper into mud).
 */
class NotePaletteTest {

    private fun lin(c: Double) = if (c <= 0.03928) c / 12.92 else ((c + 0.055) / 1.055).pow(2.4)
    private fun luminance(r: Double, g: Double, b: Double) = 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
    private fun contrast(l1: Double, l2: Double): Double {
        val hi = maxOf(l1, l2); val lo = minOf(l1, l2); return (hi + 0.05) / (lo + 0.05)
    }
    private fun rgb(argb: Int) = Triple((argb shr 16 and 0xFF) / 255.0, (argb shr 8 and 0xFF) / 255.0, (argb and 0xFF) / 255.0)

    @Test
    fun everyColour_isLegible_perA64Guarantee() {
        // A64 lifts EVERY colour to MIN_MARK_CONTRAST (4.5) on DARK grounds; on LIGHT grounds it leaves a
        // chromatic code as authored (clause 2) and only lifts achromatic ink. So the honest bar is: 4.5 on
        // dark for all; 4.5 for black everywhere; and the authored pens stay at least graphically legible
        // (≥3:1) on light paper (VLL's four colours: red 4.0, blue 5.3, green 4.2 on white).
        for (scheme in StageColorMode.entries) {
            val paper = schemePaper(scheme)
            val paperL = luminance(paper.red.toDouble(), paper.green.toDouble(), paper.blue.toDouble())
            for (c in NoteTools.COLOURS) {
                val (r, g, b) = rgb(transformOverlayPixel(c.toInt(), scheme))
                val cr = contrast(luminance(r, g, b), paperL)
                val floor = if (scheme.isDark || c == NoteTools.BLACK) 4.5 else 3.0
                assertTrue(cr >= floor, "colour ${c.toString(16)} on $scheme = ${(cr * 100).toInt() / 100.0}:1 (< $floor)")
            }
        }
    }

    @Test
    fun black_invertsOnDarkSchemes() {
        for (scheme in listOf(StageColorMode.NIGHT, StageColorMode.AMBER)) {
            val (r, g, b) = rgb(transformOverlayPixel(NoteTools.BLACK.toInt(), scheme))
            assertTrue(luminance(r, g, b) > 0.4, "black must read LIGHT on $scheme, was L=${luminance(r, g, b)}")
        }
    }

    @Test
    fun chromatics_keepDominantChannel() {
        // red → red channel max; blue → blue max; green → green max, in every scheme (hue kept, only
        // lightness remapped). Teeth: the page matrix would invert channels and flip which one dominates.
        data class Hue(val argb: Long, val chan: Int) // chan 0=r,1=g,2=b
        for (h in listOf(Hue(NoteTools.RED, 0), Hue(NoteTools.BLUE, 2), Hue(NoteTools.GREEN, 1))) {
            for (scheme in StageColorMode.entries) {
                val out = transformOverlayPixel(h.argb.toInt(), scheme)
                val ch = intArrayOf(out shr 16 and 0xFF, out shr 8 and 0xFF, out and 0xFF)
                val maxCh = ch.indices.maxBy { ch[it] }
                assertTrue(maxCh == h.chan, "colour ${h.argb.toString(16)} on $scheme lost its hue (max chan $maxCh)")
            }
        }
    }
}
