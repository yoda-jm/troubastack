package com.troubastack.desktop.studio

import javax.accessibility.Accessible
import javax.accessibility.AccessibleContext

/**
 * S2.7 — read and press an open ⋮ menu window through its ACCESSIBILITY tree, the one path both smokes use. It measures
 * the REAL rendered menu (a model list could pass while the render dropped a row), and works for both stage-1 Swing
 * `JMenuItem`s (AccessibleJMenuItem) and the S2.7 Compose rows (ComposeAccessibleComponent) — Compose Desktop exposes
 * its nodes through the AWT accessibility tree on the runner with no assistive tech running.
 */

/** The accessible names of the ACTIONABLE nodes under [node] — the labels a person sees and can press in the menu. */
internal fun accessibleActionNames(node: Accessible, out: MutableList<String> = mutableListOf(), depth: Int = 0): List<String> {
    val ctx = node.accessibleContext ?: return out
    if ((ctx.accessibleAction?.accessibleActionCount ?: 0) > 0) ctx.accessibleName?.takeIf { it.isNotBlank() }?.let { out += it }
    if (depth < 60) for (i in 0 until ctx.accessibleChildrenCount) ctx.getAccessibleChild(i)?.let { accessibleActionNames(it, out, depth + 1) }
    return out
}

/** The first ACTIONABLE AccessibleContext under [node] whose accessibleName == [name], or null. Requiring an action
 *  (not just the name) means that if a row ever exposed a plain label with the same text beside its clickable node, we
 *  still find the pressable one — so a press can't fail for the wrong reason (1d's note; matches S2.8's AboutSmoke). */
internal fun findAccessibleNamed(node: Accessible, name: String, depth: Int = 0): AccessibleContext? {
    val ctx = node.accessibleContext ?: return null
    if (ctx.accessibleName == name && (ctx.accessibleAction?.accessibleActionCount ?: 0) > 0) return ctx
    if (depth >= 60) return null
    for (i in 0 until ctx.accessibleChildrenCount) findAccessibleNamed(ctx.getAccessibleChild(i) ?: continue, name, depth + 1)?.let { return it }
    return null
}

/** Press the node named [name] under [node] via its first accessible action (as a click does). Caller runs it on the EDT. */
internal fun pressAccessibleNamed(node: Accessible, name: String): Boolean =
    findAccessibleNamed(node, name)?.accessibleAction?.let { it.accessibleActionCount > 0 && it.doAccessibleAction(0) } ?: false

/** The on-SCREEN rectangles of the actionable nodes under [node] (S2.7 menu-rows-fit: every row must lie inside the
 *  window; a too-short window pushes the last row's rect past the window's bottom). Skips any node that can't locate. */
internal fun accessibleActionRects(node: Accessible, out: MutableList<java.awt.Rectangle> = mutableListOf(), depth: Int = 0): List<java.awt.Rectangle> {
    val ctx = node.accessibleContext ?: return out
    if ((ctx.accessibleAction?.accessibleActionCount ?: 0) > 0) ctx.accessibleComponent?.let { comp ->
        runCatching { out += java.awt.Rectangle(comp.locationOnScreen, comp.size) }
    }
    if (depth < 60) for (i in 0 until ctx.accessibleChildrenCount) ctx.getAccessibleChild(i)?.let { accessibleActionRects(it, out, depth + 1) }
    return out
}
