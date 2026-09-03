// A64 — the annotation-colour rule. A page is baked ONCE (neutral raster + transparent overlay PNGs);
// the four reading schemes are draw-time colour transforms (StageColorMode). The naive path applied the
// SAME scheme matrix to the page raster AND the annotation ink, so a colour that MEANS something (a red
// conductor cue) changed meaning with the reading light — red read teal in Night. This module is the
// rule that fixes that, gated on how colourful the ink is (see docs/design/12-annotation-colour.md and
// docs/tasks/A64):
//
//   1. Achromatic (Lab chroma C* < 20): it is INK. Apply the page matrix (it inverts with the paper),
//      then lift lightness until it clears 4.5:1 against the paper.
//   2. Chromatic (C* >= 20): it is a CODE. Preserve hue + saturation, remap lightness only. On light
//      grounds leave it (authored for white paper); on dark grounds solve the lightness closest to the
//      original that clears 4.5:1 vs paper AND deltaE >= 25 vs the printed ink (so it never dissolves
//      into the text it sits on).
//   3. Highlight fills are not text (a highlighter is meant to be low-contrast): on dark grounds reduce
//      alpha 0.55 -> 0.30 instead of touching the hue.
//
// PURE and platform-agnostic (the pure-seam pattern): unit-tested off-device over the real palette. The
// paper and printed-ink colours are DERIVED from the real StageColorMode matrices (white/black through
// the matrix), so the test genuinely exercises the shipped schemes, not invented samples.
package com.troubastack.shared.stage

import androidx.compose.ui.graphics.Color
import kotlin.math.abs
import kotlin.math.cbrt
import kotlin.math.max
import kotlin.math.min
import kotlin.math.pow
import kotlin.math.sqrt

/** Lab-chroma gate: below this a colour is treated as ink (invert it), at/above it is a code (preserve
 *  it). Robust, not tuned: the palette's near-black `#111827` sits at C*≈11.5 (ink) and the next colour
 *  up is Teal at 35.3, so 20 is an empty band. See the spec's warning against HLS saturation. */
const val CHROMA_INK_THRESHOLD = 20f

/** WCAG minimum for a mark against its paper (both a lifted achromatic ink and a remapped code). */
const val MIN_MARK_CONTRAST = 4.5f

/** Minimum Lab deltaE between a remapped code and the printed ink, so a mark never dissolves into text. */
const val MIN_INK_DELTA_E = 25f

/** Highlight fill alpha: baked at 0.55, reduced on dark grounds so printed text still reads through. */
const val HIGHLIGHT_ALPHA_LIGHT = 0.55f
const val HIGHLIGHT_ALPHA_DARK = 0.30f

// --- sRGB companding -------------------------------------------------------------------------------

private fun srgbToLinear(c: Float): Float =
    if (c <= 0.04045f) c / 12.92f else ((c + 0.055f) / 1.055f).pow(2.4f)

// --- WCAG relative luminance + contrast ------------------------------------------------------------

/** WCAG 2.x relative luminance of an opaque colour (alpha ignored — composite first if needed). */
fun relativeLuminance(color: Color): Float {
    val r = srgbToLinear(color.red)
    val g = srgbToLinear(color.green)
    val b = srgbToLinear(color.blue)
    return 0.2126f * r + 0.7152f * g + 0.0722f * b
}

/** WCAG contrast ratio in [1, 21]. */
fun contrastRatio(a: Color, b: Color): Float {
    val la = relativeLuminance(a)
    val lb = relativeLuminance(b)
    val hi = max(la, lb)
    val lo = min(la, lb)
    return (hi + 0.05f) / (lo + 0.05f)
}

// --- CIE Lab (D65) ---------------------------------------------------------------------------------

private fun labOf(color: Color): FloatArray {
    val r = srgbToLinear(color.red)
    val g = srgbToLinear(color.green)
    val b = srgbToLinear(color.blue)
    // sRGB -> XYZ (D65)
    val x = r * 0.4124f + g * 0.3576f + b * 0.1805f
    val y = r * 0.2126f + g * 0.7152f + b * 0.0722f
    val z = r * 0.0193f + g * 0.1192f + b * 0.9505f
    // XYZ -> Lab, D65 reference white
    val xn = 0.95047f
    val yn = 1.00000f
    val zn = 1.08883f
    fun f(t: Float): Float {
        val d = 6f / 29f
        return if (t > d * d * d) cbrt(t) else t / (3 * d * d) + 4f / 29f
    }
    val fx = f(x / xn)
    val fy = f(y / yn)
    val fz = f(z / zn)
    return floatArrayOf(116 * fy - 16, 500 * (fx - fy), 200 * (fy - fz))
}

/** Lab chroma C* = sqrt(a*^2 + b*^2). The gate in clause 1/2 — stable at low lightness, unlike HLS. */
fun chroma(color: Color): Float {
    val lab = labOf(color)
    return sqrt(lab[1] * lab[1] + lab[2] * lab[2])
}

/** Lab hue angle in degrees [0,360). The identity of a colour code — a red cue must keep roughly this
 *  angle across schemes; today's straight inversion swings it ~180° (red -> teal), which is the defect. */
fun labHue(color: Color): Float {
    val lab = labOf(color)
    val deg = kotlin.math.atan2(lab[2], lab[1]) * 180f / kotlin.math.PI.toFloat()
    return (deg + 360f) % 360f
}

/** Smallest absolute angular distance between two Lab hues, in degrees [0,180]. */
fun hueDistance(a: Color, b: Color): Float {
    val d = abs(labHue(a) - labHue(b)) % 360f
    return if (d > 180f) 360f - d else d
}

/** CIE76 colour difference in Lab. Used against the printed ink so a code never dissolves into text. */
fun deltaE76(a: Color, b: Color): Float {
    val la = labOf(a)
    val lb = labOf(b)
    val dl = la[0] - lb[0]
    val da = la[1] - lb[1]
    val db = la[2] - lb[2]
    return sqrt(dl * dl + da * da + db * db)
}

// --- HSL (hue/saturation preserved; lightness remapped) --------------------------------------------

private data class Hsl(val h: Float, val s: Float, val l: Float)

private fun toHsl(c: Color): Hsl {
    val r = c.red
    val g = c.green
    val b = c.blue
    val mx = max(r, max(g, b))
    val mn = min(r, min(g, b))
    val l = (mx + mn) / 2f
    if (mx == mn) return Hsl(0f, 0f, l) // achromatic
    val d = mx - mn
    val s = if (l > 0.5f) d / (2f - mx - mn) else d / (mx + mn)
    val h = when (mx) {
        r -> (g - b) / d + (if (g < b) 6f else 0f)
        g -> (b - r) / d + 2f
        else -> (r - g) / d + 4f
    } / 6f
    return Hsl(h, s, l)
}

private fun fromHsl(h: Float, s: Float, l: Float): Color {
    if (s == 0f) return Color(l, l, l)
    fun hue2rgb(p: Float, q: Float, tIn: Float): Float {
        var t = tIn
        if (t < 0f) t += 1f
        if (t > 1f) t -= 1f
        return when {
            t < 1f / 6f -> p + (q - p) * 6f * t
            t < 1f / 2f -> q
            t < 2f / 3f -> p + (q - p) * (2f / 3f - t) * 6f
            else -> p
        }
    }
    val q = if (l < 0.5f) l * (1f + s) else l + s - l * s
    val p = 2f * l - q
    return Color(hue2rgb(p, q, h + 1f / 3f), hue2rgb(p, q, h), hue2rgb(p, q, h - 1f / 3f))
}

// --- the scheme matrices, in code (mirrors StageColorMode.pageColorFilter, so paper/ink stay in sync) -

private fun clamp01(v: Float) = v.coerceIn(0f, 1f)

/** Apply a scheme's page matrix to one colour — the same transform pageColorFilter() applies on the GPU
 *  to the raster. Used to derive the paper/ink of a scheme and to invert achromatic ink (clause 1). */
fun applySchemeMatrix(color: Color, scheme: StageColorMode): Color = when (scheme) {
    StageColorMode.NORMAL -> color
    StageColorMode.WARM -> Color(clamp01(color.red * 1.00f), clamp01(color.green * 0.96f), clamp01(color.blue * 0.82f))
    StageColorMode.NIGHT -> Color(clamp01(1f - color.red), clamp01(1f - color.green), clamp01(1f - color.blue))
    StageColorMode.AMBER -> Color(clamp01(1f - color.red), clamp01(0.75f * (1f - color.green)), clamp01(0.45f * (1f - color.blue)))
}

/** The paper a scheme settles to: white through the page matrix. NORMAL/WARM light, NIGHT/AMBER dark. */
fun schemePaper(scheme: StageColorMode): Color = applySchemeMatrix(Color.White, scheme)

/** The printed-ink colour a scheme settles to: black through the page matrix (white in Night, amber in Amber). */
fun schemePrintedInk(scheme: StageColorMode): Color = applySchemeMatrix(Color.Black, scheme)

// --- the rule --------------------------------------------------------------------------------------

/** Search lightness (keeping hue + saturation) for the value closest to [start]'s lightness that
 *  satisfies [feasible]; if nothing does, return the endpoint that comes closest to satisfying it. */
private fun closestLightnessMeeting(start: Color, feasible: (Color) -> Boolean): Color {
    val hsl = toHsl(start)
    if (feasible(start)) return start
    val steps = 200
    var best: Color? = null
    var bestDist = Float.MAX_VALUE
    for (i in 0..steps) {
        val l = i / steps.toFloat()
        val cand = fromHsl(hsl.h, hsl.s, l)
        if (feasible(cand)) {
            val dist = abs(l - hsl.l)
            if (dist < bestDist) {
                bestDist = dist
                best = cand
            }
        }
    }
    // Fallback: push to the paper-opposite extreme (max legibility) if no lightness satisfied both.
    return best ?: fromHsl(hsl.h, hsl.s, if (hsl.l < 0.5f) 1f else 0f)
}

/**
 * The annotation-ink transform for one solid colour under [scheme] (clauses 1 & 2). Handwriting inverts
 * with the paper; a colour code keeps its identity. Returns the colour to draw the mark (or glyph) in.
 * Alpha is preserved. Highlight FILLS use [highlightFillAlpha] instead of a colour change (clause 3).
 */
fun annotationInk(color: Color, scheme: StageColorMode): Color {
    val paper = schemePaper(scheme)
    val result = if (chroma(color) < CHROMA_INK_THRESHOLD) {
        // Clause 1 — achromatic ink: invert with the paper, then lift until it clears 4.5:1.
        val inverted = applySchemeMatrix(color, scheme)
        closestLightnessMeeting(inverted) { contrastRatio(it, paper) >= MIN_MARK_CONTRAST }
    } else if (!scheme.isDark) {
        // Clause 2, light grounds: the code was authored for white paper — leave it.
        color
    } else {
        // Clause 2, dark grounds: preserve hue + saturation, remap lightness for contrast AND deltaE.
        val ink = schemePrintedInk(scheme)
        closestLightnessMeeting(color) {
            contrastRatio(it, paper) >= MIN_MARK_CONTRAST && deltaE76(it, ink) >= MIN_INK_DELTA_E
        }
    }
    return result.copy(alpha = color.alpha)
}

/** Clause 3 — the highlight fill alpha under [scheme]: unchanged on light grounds, reduced on dark so
 *  printed text still reads THROUGH the band (rotating the hue scores worse and destroys the code).
 *
 *  NOTE (A64 part 2 finding): this can only be applied where the caller KNOWS a mark is a highlight.
 *  On a flattened per-layer overlay PNG a highlight fill and a stroke are indistinguishable per-pixel
 *  (both are just coloured pixels; anti-aliased stroke edges also carry partial alpha), so clause 3
 *  cannot be honoured by [transformOverlayPixel] alone — it needs per-object form metadata the bake
 *  does not carry today. See the gate note; do NOT scale all overlay alpha (it would dim strokes). */
fun highlightFillAlpha(scheme: StageColorMode, baseAlpha: Float = HIGHLIGHT_ALPHA_LIGHT): Float =
    if (scheme.isDark) HIGHLIGHT_ALPHA_DARK else baseAlpha

// --- A64 part 2: the per-pixel overlay transform (pure core, mechanism-agnostic) -------------------

/**
 * Transform one straight-alpha packed `0xAARRGGBB` overlay pixel under [scheme] (clauses 1 & 2). This
 * is the reference logic ANY overlay mechanism encodes — a CPU pixel walk (`expect`/`actual` over the
 * decoded [androidx.compose.ui.graphics.ImageBitmap], recommended) or a GPU shader. Kept PURE and
 * Int-in/Int-out so it is fully unit-testable off-device, and so the SAME [annotationInk] rule drives
 * both the overlay ink and the cue glyph — they cannot diverge.
 *
 * Fully-transparent pixels pass through untouched (they must stay transparent — overlay margins). Alpha
 * is preserved verbatim: clause 3 (highlight alpha) is deliberately NOT applied here (see
 * [highlightFillAlpha] — it needs per-object form metadata, absent on a flattened PNG).
 */
fun transformOverlayPixel(argb: Int, scheme: StageColorMode): Int {
    val a = (argb ushr 24) and 0xFF
    if (a == 0) return argb // transparent — leave it (do not tint the invisible)
    val src = Color(
        red = ((argb ushr 16) and 0xFF) / 255f,
        green = ((argb ushr 8) and 0xFF) / 255f,
        blue = (argb and 0xFF) / 255f,
    )
    val out = annotationInk(src, scheme)
    val r = (out.red * 255f + 0.5f).toInt().coerceIn(0, 255)
    val g = (out.green * 255f + 0.5f).toInt().coerceIn(0, 255)
    val b = (out.blue * 255f + 0.5f).toInt().coerceIn(0, 255)
    return (a shl 24) or (r shl 16) or (g shl 8) or b
}
