package com.troubastack.desktop.studio

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.wrapContentHeight
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.awt.ComposePanel
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.unit.dp
import com.troubastack.shared.ui.TroubaTheme
import java.awt.Dimension

/** S2.7: one row-height constant used by BOTH the layout (each row is this tall) and the window's preferredSize, so the
 *  window can't be a few px short of what it draws (Fable: a row draws 48 px; 44+8 clipped the third of three rows). */
private const val ROW_HEIGHT = 48

/**
 * S2.7 — the ⋮ menu's content as a [ComposePanel] (Compose-in-Swing), themed with [TroubaTheme] (Material3) so it
 * matches the rest of the app, replacing stage 1's plain `JMenuItem`s. It lives in the SAME owned, non-focusable,
 * never-topmost `JWindow` as before, so non-activation is unchanged (a `ComposePanel` renders and handles MOUSE in a
 * non-focusable window — a menu needs no keyboard focus, so the frame keeps activation and `menu-no-activation-steal`
 * still holds).
 *
 * [dark] is the APP's resolved theme (themePref.resolveDark()), not the OS setting, so a Parameters theme that differs
 * from Windows' is honoured. Each row is a **named, pressable node in the in-process accessibility tree** (a `Role.Button`
 * `clickable` whose `Text` merges into its name), which both smokes read and press through `AccessibleContext`. (That
 * in-process tree is NOT by itself external-screen-reader support: a Windows reader needs the Java Access Bridge, and
 * `jdk.accessibility` isn't in the runtime — tracked as an S2.10 open item.)
 *
 * Sabotages: [useSystemTheme] (menusystheme) uses `isSystemInDarkTheme()` instead of [dark]; [short] (menushort) sizes
 * the window 44 px/row so the last row is clipped; [noA11y] (menunoa11y) clears each row's semantics.
 */
internal fun studioMenuPanel(
    items: List<Pair<String, () -> Unit>>,
    dark: Boolean,
    noA11y: Boolean = false,
    useSystemTheme: Boolean = false,
    short: Boolean = false,
    lightEdge: Boolean = false,
): ComposePanel {
    val panel = ComposePanel()
    val rowH = if (short) 44 else ROW_HEIGHT
    // Exactly rows×ROW_HEIGHT: no extra strip (Compose's border draws INSIDE its bounds, so a +2 left bare opaque-white
    // JWindow under the Column). SQUARE corners (the surface fills every pixel — rounded corners left white wedges where
    // the opaque JWindow showed through). Window rounding, if wanted, is the OS's (DWM) job, not a transparent clip.
    // Normal: exactly rows×ROW_HEIGHT. menulightedge is the OLD code — the +2 bare strip under the Column, which the
    // opaque JWindow showed as white on a dark page (the defect menu-edges-match-theme guards against).
    panel.preferredSize = Dimension(240, items.size * rowH + if (lightEdge) 2 else 0)
    panel.setContent {
        val content = @androidx.compose.runtime.Composable {
            // SQUARE corners (rounded revealed the opaque JWindow at the corner wedges). menulightedge is the old rounded clip.
            val shape = if (lightEdge) RoundedCornerShape(6.dp) else androidx.compose.ui.graphics.RectangleShape
            Column(
                Modifier
                    .background(MaterialTheme.colorScheme.surface, shape)
                    .border(1.dp, MaterialTheme.colorScheme.outlineVariant, shape),
            ) {
                items.forEach { (label, onClick) ->
                    val rowMod = Modifier
                        .fillMaxWidth()
                        .height(ROW_HEIGHT.dp)
                        .clickable(role = Role.Button, onClick = onClick)
                    Text(
                        text = label,
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurface,
                        modifier = (if (noA11y) rowMod.clearAndSetSemantics {} else rowMod)
                            .wrapContentHeight(Alignment.CenterVertically)
                            .padding(horizontal = 16.dp),
                    )
                }
            }
        }
        if (useSystemTheme) TroubaTheme(content = content) else TroubaTheme(dark = dark, content = content)
    }
    return panel
}
