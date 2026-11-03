package com.troubastack.shared.ui

import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.size
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.LocalTextStyle
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.takeOrElse
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.drawText
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.unit.DpSize

/**
 * A control's glyph. When the control is [labelled], the glyph is DRAWN (the same text layout a Text would make, painted
 * with drawText) instead of composed as a Text, so it is no semantics node at all: a screen reader does not read "☰"
 * after the name, and the name stays ON the clickable. Any semantics child — even a Text with its semantics cleared —
 * makes Android expose the clickable's name and role on separate "fake" child nodes (Compose's emitFakeNodes), which
 * the emulator's accessibility dump showed. Unlabelled, it is a plain Text. Stage's glyph controls and the dialogs' ✕
 * use it.
 */
@Composable
internal fun GlyphText(
    glyph: String,
    labelled: Boolean,
    style: TextStyle = LocalTextStyle.current,
    modifier: Modifier = Modifier,
    color: Color = Color.Unspecified,
) {
    if (!labelled) {
        Text(glyph, modifier, color = color, style = style)
        return
    }
    val measurer = rememberTextMeasurer()
    val ink = color.takeOrElse { style.color.takeOrElse { LocalContentColor.current } }
    val layout = remember(glyph, style, ink, measurer) { measurer.measure(glyph, style.copy(color = ink)) }
    val density = LocalDensity.current
    Spacer(modifier.size(with(density) { DpSize(layout.size.width.toDp(), layout.size.height.toDp()) }).drawBehind { drawText(layout) })
}
