// T50/A20 — rendering the shared cue glyph contract (CueGlyphData, from web/ink/glyphs.json) as a
// tintable Compose icon, plus the "#rrggbb" tint parse. No SVG parser at runtime (I1 contract).
package com.troubastack.shared.stage

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/**
 * Parse a cue tint. Accepts "#rrggbb" (or "rrggbb"); anything empty/invalid falls back to [neutral]
 * (the untinted case), so a bad color never crashes or blanks the glyph. Pure/testable.
 */
fun parseCueColor(hex: String, neutral: Color): Color {
    val h = hex.trim().removePrefix("#")
    if (h.length != 6) return neutral
    val v = h.toLongOrNull(16) ?: return neutral
    return Color(0xFF000000L or v)
}

/**
 * A single tinted cue glyph (T50). Renders the shared polyline geometry ([cueGlyph], unknown id →
 * `note` fallback) into a [size]×[size] box: fills filled, strokes stroked with round caps/joins at
 * `strokeWidth×size`. [tint] is the cue color already resolved (see [parseCueColor]); everything draws
 * in that one color, so a "red electric guitar" is just a red tint — exactly the studio's rendering.
 */
@Composable
fun CueGlyphIcon(icon: String, tint: Color, size: Dp = 20.dp, modifier: Modifier = Modifier) {
    val glyph = remember(icon) { cueGlyph(icon) }
    Canvas(modifier.size(size)) {
        val s = this.size.minDimension
        fun path(poly: List<Offset>, close: Boolean) = Path().apply {
            poly.forEachIndexed { i, o ->
                val x = o.x * s
                val y = o.y * s
                if (i == 0) moveTo(x, y) else lineTo(x, y)
            }
            if (close) close()
        }
        glyph.fills.forEach { drawPath(path(it, close = true), tint) }
        val strokePx = glyph.strokeWidth * s
        glyph.strokes.forEach {
            drawPath(path(it, close = false), tint, style = Stroke(width = strokePx, cap = StrokeCap.Round, join = StrokeJoin.Round))
        }
    }
}
