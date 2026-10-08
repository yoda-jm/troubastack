package com.troubastack.shared.stage

/** A73 ⟨D6⟩ — what the note bar's "Clear page" acts on. */
sealed interface ClearPlan {
    /** Nothing visible to clear (two-up, neither page carries a note): the button is disabled — a dialog that
     *  then does nothing is the silent failure ⟨D6⟩ removes. */
    data object Disabled : ClearPlan
    /** Clear this page (an index into the bundle's pages). */
    data class One(val page: Int) : ClearPlan
    /** Both pages of the spread carry a note: ask which. One at a time — the action cannot be undone. */
    data class Choose(val pages: List<Int>) : ClearPlan
}

/**
 * A73 ⟨D6⟩ — in two-up BOTH pages are editable, but "Clear page" used to clear `state.current`, which may be the
 * page WITHOUT the note (measured on VLL's tablet: the dialog closed and the note stayed). It now acts on a
 * visible page that carries a note. Single-page view: the current page, and — Fable's one line — disabled when it
 * has no note (an enabled button whose dialog then does nothing is the same silent failure, in single view).
 *
 * [visible] — the page indices on screen (the spread in two-up, `[current]` otherwise); [hasNote] — whether a
 * page carries a live note (`state.noteForPage(...) != null`).
 */
fun clearPlan(visible: List<Int>, twoUp: Boolean, hasNote: (Int) -> Boolean): ClearPlan {
    if (!twoUp || visible.size < 2) return visible.firstOrNull()?.takeIf(hasNote)?.let { ClearPlan.One(it) } ?: ClearPlan.Disabled
    val withNote = visible.filter(hasNote)
    return when (withNote.size) {
        0 -> ClearPlan.Disabled
        1 -> ClearPlan.One(withNote.single())
        else -> ClearPlan.Choose(withNote)
    }
}
