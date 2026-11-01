package com.troubastack.desktop

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.TooltipPlacement
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.PointerIcon
import androidx.compose.ui.unit.DpOffset
import androidx.compose.ui.unit.dp
import com.troubastack.shared.stage.StagePointerIcons
import java.awt.Point
import java.awt.Toolkit
import java.awt.image.BufferedImage

/** WIN03 D3 — Stage's pointer on the desktop: a fully transparent cursor when it hides, a crosshair in note mode. */
fun desktopStagePointerIcons(): StagePointerIcons = StagePointerIcons(
    hidden = PointerIcon(
        Toolkit.getDefaultToolkit().createCustomCursor(BufferedImage(16, 16, BufferedImage.TYPE_INT_ARGB), Point(0, 0), "TroubaStage hidden pointer"),
    ),
)

/** WIN03 D3 — a glyph-only Stage button shows its label on hover, half a second after the pointer settles on it. */
@OptIn(ExperimentalFoundationApi::class)
val desktopGlyphTooltip: @Composable (label: String, content: @Composable () -> Unit) -> Unit = { label, content ->
    TooltipArea(
        tooltip = {
            Surface(color = Color(0xE6202020), contentColor = Color.White, shape = RoundedCornerShape(6.dp)) {
                Text(label, Modifier.padding(horizontal = 8.dp, vertical = 4.dp), style = MaterialTheme.typography.bodySmall)
            }
        },
        delayMillis = 500,
        tooltipPlacement = TooltipPlacement.CursorPoint(offset = DpOffset(0.dp, 20.dp)),
    ) { content() }
}
