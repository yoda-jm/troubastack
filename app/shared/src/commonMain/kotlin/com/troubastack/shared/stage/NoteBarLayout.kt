// A87 — where the note bar's three groups go, as a pure rule (no Compose), so a unit test can pin it. In portrait the
// single row used to pack left to right: the tools, then "Clear page", then ✓ — wider than the 1200 px panel, so
// "Clear page" was squeezed to one letter per line (the bar grew ~3× taller) and ✓, the only way out of note mode
// (A73), was pushed off screen. Now ✓ and "Clear page" are placed from the RIGHT edge at their natural width, and the
// tools get whatever is left (they scroll sideways if that is ever less than they need).
package com.troubastack.shared.stage

/** The bar's horizontal placement, in px: the tools' maximum width, then each group's x. */
internal data class NoteBarSlots(val toolsMax: Int, val toolsX: Int, val clearX: Int, val doneX: Int)

/**
 * Place the bar's groups in [width] px: ✓ ([done] px wide) flush right, "Clear page" ([clear] px) [gap] before it,
 * and the tools ([tools] px at their natural width) from the left edge — at that width when it fits, else in what
 * remains (they scroll). ✓ is inside [width] whenever it fits at all; only the tools ever give way.
 */
internal fun noteBarSlots(width: Int, tools: Int, clear: Int, done: Int, gap: Int): NoteBarSlots {
    val doneX = (width - done).coerceAtLeast(0)
    val clearX = (doneX - gap - clear).coerceAtLeast(0)
    return NoteBarSlots(toolsMax = minOf(tools, (clearX - gap).coerceAtLeast(0)), toolsX = 0, clearX = clearX, doneX = doneX)
}
