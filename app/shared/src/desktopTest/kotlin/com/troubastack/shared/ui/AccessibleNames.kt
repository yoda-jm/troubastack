package com.troubastack.shared.ui

import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import kotlin.test.assertEquals

/**
 * One node carries [label] as its name AND can be clicked — in the UNMERGED tree, so the name sits on the clickable
 * itself — and that node has no semantics children: with any (even a Text whose semantics are cleared), Android's
 * accessibility tree moves the name and the role onto separate "fake" child nodes (Compose's emitFakeNodes), and the
 * node a screen reader focuses then carries the action but not the name. Measured on the emulator.
 */
@OptIn(ExperimentalTestApi::class)
internal fun ComposeUiTest.assertNamedClickable(label: String) {
    val nodes = onAllNodes(hasContentDescription(label) and hasClickAction(), useUnmergedTree = true).fetchSemanticsNodes()
    assertEquals(1, nodes.size, "one clickable named '$label'")
    assertEquals(emptyList(), nodes[0].children.map { it.config.toString() }, "'$label' has no semantics children (Android would split its name off)")
}

@OptIn(ExperimentalTestApi::class)
internal fun ComposeUiTest.assertGlyphsNotRead(vararg glyphs: String) {
    for (g in glyphs) onAllNodes(hasText(g), useUnmergedTree = true).assertCountEquals(0)
}
