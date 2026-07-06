package com.troubashare.shared.stage

/**
 * A12 — facing-pages (two-up) spread math. Pure + total: the UI decides WHEN to go two-up (a
 * landscape viewport in FIT_PAGE), and this decides WHICH pages a spread shows and HOW a turn moves.
 * Spreads align to the global page index — pages 2k / 2k+1, no book-parity in v1 (I12; read-only).
 * The source of truth stays a single "current page"; the left page of its spread is [spreadFor].
 */

/** The left (even) page index of the spread that contains [page]. */
fun spreadFor(page: Int): Int = page - (page and 1)

/**
 * The 1 or 2 page indices visible in [page]'s spread, clamped to `[0, pageCount)`. A last odd page —
 * whose right neighbour is past the end — shows alone. Empty when there are no pages.
 */
fun spreadPages(page: Int, pageCount: Int): List<Int> {
    if (pageCount <= 0) return emptyList()
    val left = spreadFor(page.coerceIn(0, pageCount - 1))
    val right = left + 1
    return if (right < pageCount) listOf(left, right) else listOf(left)
}

/** Next spread's left page (turn-by-2 in two-up), clamped so a turn never runs off the end. */
fun nextSpreadPage(page: Int, pageCount: Int): Int =
    (spreadFor(page) + 2).coerceIn(0, (pageCount - 1).coerceAtLeast(0))

/** Previous spread's left page (turn-by-2 in two-up), clamped at the first spread. */
fun prevSpreadPage(page: Int): Int = (spreadFor(page) - 2).coerceAtLeast(0)

/**
 * The pager label: "3–4/22" for a two-up spread, "22/22" for a lone last page, "5/22" one-up.
 * [twoUp] is the live layout decision; [page] is the source-of-truth current page.
 */
fun pagerLabel(page: Int, pageCount: Int, twoUp: Boolean): String {
    if (pageCount <= 0) return "0/0"
    if (!twoUp) return "${page.coerceIn(0, pageCount - 1) + 1}/$pageCount"
    val pages = spreadPages(page, pageCount)
    return if (pages.size == 2) "${pages[0] + 1}–${pages[1] + 1}/$pageCount"
    else "${pages[0] + 1}/$pageCount"
}
