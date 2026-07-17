package com.troubashare.shared.stage

/**
 * A12 / N6 — facing-pages (two-up) spread math. Pure + total: the UI decides WHEN to go two-up (a
 * landscape viewport in FIT_PAGE), and this decides WHICH pages a spread shows and HOW a turn moves.
 *
 * N6 (amends A12's original global pairing): a spread must NEVER contain two songs. Pairing is
 * SONG-ALIGNED — it restarts at each song's first page — so an odd-paged song shows its last page
 * SOLO (blank right half) and the next song always opens a fresh spread on the left (book convention;
 * matches the N2 per-song model — only an explicit turn crosses songs). [songStarts] is the ascending
 * list of each song's first global page index (`StageState.songs.map { it.firstPage }`); an empty list
 * degrades to a single whole-concert song (plain global pairing), so callers without songs still work.
 * The source of truth stays a single "current page"; the left page of its spread is [spreadFor].
 */

/** The `[first, last]` global page range of the song containing [page]. */
private fun songBounds(page: Int, songStarts: List<Int>, pageCount: Int): IntRange {
    if (pageCount <= 0) return IntRange.EMPTY
    val p = page.coerceIn(0, pageCount - 1)
    val first = songStarts.filter { it <= p }.maxOrNull() ?: 0
    val nextStart = songStarts.filter { it > p }.minOrNull() ?: pageCount
    return first..(nextStart - 1)
}

/** The left page index of the spread containing [page] — aligned to an EVEN OFFSET WITHIN its song. */
fun spreadFor(page: Int, songStarts: List<Int>, pageCount: Int): Int {
    val b = songBounds(page, songStarts, pageCount)
    if (b.isEmpty()) return page.coerceAtLeast(0)
    val offset = page.coerceIn(b.first, b.last) - b.first
    return b.first + (offset - (offset and 1))
}

/**
 * The 1 or 2 page indices visible in [page]'s spread. The right page shows only if it's in the SAME
 * song (N6) — an odd-paged song's last page, or a single-page song, shows alone. Empty when no pages.
 */
fun spreadPages(page: Int, songStarts: List<Int>, pageCount: Int): List<Int> {
    if (pageCount <= 0) return emptyList()
    val left = spreadFor(page.coerceIn(0, pageCount - 1), songStarts, pageCount)
    val b = songBounds(left, songStarts, pageCount)
    val right = left + 1
    return if (right <= b.last) listOf(left, right) else listOf(left)
}

/**
 * Next spread's left page (turn-by-spread). Within the song, advance by 2; at the song's last spread,
 * cross to the NEXT song's first page (a fresh spread). At the LAST spread of the LAST song, return
 * this spread's own LEFT page — a true no-op (A22): returning the spread's RIGHT page instead would
 * move `current` within the same spread and make the N4 slide animate a turn that renders the identical
 * page ("extra swipe with animation, same page"). Every result is a spread-left index.
 */
fun nextSpreadPage(page: Int, songStarts: List<Int>, pageCount: Int): Int {
    val left = spreadFor(page, songStarts, pageCount)
    val b = songBounds(left, songStarts, pageCount)
    if (left + 2 <= b.last) return left + 2 // next spread within the song
    val nextSong = b.last + 1               // first page of the next song
    return if (nextSong <= pageCount - 1) nextSong else left // no next song → stay on this spread (no-op)
}

/**
 * Previous spread's left page (turn-by-spread). Within the song, step back by 2; at the song's first
 * spread, cross to the PREVIOUS song's last spread (its left page). Clamped at the very first spread.
 */
fun prevSpreadPage(page: Int, songStarts: List<Int>, pageCount: Int): Int {
    val left = spreadFor(page, songStarts, pageCount)
    val b = songBounds(left, songStarts, pageCount)
    if (left - 2 >= b.first) return left - 2
    val prevSongLast = b.first - 1 // last page of the previous song
    if (prevSongLast < 0) return b.first // already the first song's first spread → stay
    return spreadFor(prevSongLast, songStarts, pageCount)
}

/**
 * The target page for a page turn from [page], spread-aware when [twoUp] (A13 + N6 song-aligned). This
 * is the single navigation rule every input shares — keys/pedals, taps, swipes, pager buttons AND
 * Android volume keys — so two-up always turns by a whole (song-aligned) spread and one-up by one
 * page. The result is fed to [StageViewModel.goToPage], which clamps; NEXT/PREV in one-up are left
 * unclamped here (page±1) to match that clamp-at-the-VM contract. [songStarts] is ignored in one-up.
 */
fun turnTarget(page: Int, pageCount: Int, twoUp: Boolean, dir: PageTurn, songStarts: List<Int>): Int = when (dir) {
    PageTurn.NEXT -> if (twoUp) nextSpreadPage(page, songStarts, pageCount) else page + 1
    PageTurn.PREV -> if (twoUp) prevSpreadPage(page, songStarts, pageCount) else page - 1
}

/**
 * N7 — would a turn from [page] in [dir] be a no-op blocked at the concert EDGE? True when the shared
 * [turnTarget] rule, after the VM's clamp, lands back on the same [page] — i.e. NEXT at the last
 * page/spread or PREV at the first. Drives the end-of-bounds cue so a dead swipe reads as "you're at
 * the edge" instead of a broken turn. Pure; works for one-up (page±1 clamped) and two-up (song-aligned
 * spread). Empty/degenerate bundle ⇒ blocked.
 */
fun isBlockedTurn(page: Int, pageCount: Int, twoUp: Boolean, dir: PageTurn, songStarts: List<Int>): Boolean {
    if (pageCount <= 0) return true
    return turnTarget(page, pageCount, twoUp, dir, songStarts).coerceIn(0, pageCount - 1) == page
}

/**
 * N9 — the page indices a NEXT or PREV turn from [page] would DISPLAY, so they can be pre-decoded
 * before the slide (killing the blank-slide-in). Derived from the SAME turn rule + spread math the
 * navigation uses (song-aligned, N6) — never hand-rolled adjacency — so prefetch always matches what a
 * turn actually shows: both pages of the adjacent spread in two-up, the single next/prev page
 * otherwise. Excludes the currently-displayed spread (already resident/pinned) and, at the concert
 * ends, the blocked direction (its target == current). Distinct, in ascending order. Pure.
 */
fun prefetchTargets(page: Int, pageCount: Int, twoUp: Boolean, songStarts: List<Int>): List<Int> {
    if (pageCount <= 0) return emptyList()
    val displayed = if (twoUp) spreadPages(page, songStarts, pageCount).toSet() else setOf(page.coerceIn(0, pageCount - 1))
    val neighbours = listOf(PageTurn.NEXT, PageTurn.PREV).flatMap { dir ->
        val target = turnTarget(page, pageCount, twoUp, dir, songStarts).coerceIn(0, pageCount - 1)
        if (twoUp) spreadPages(target, songStarts, pageCount) else listOf(target)
    }
    return (neighbours.toSet() - displayed).sorted()
}

/**
 * The pager label: "3–4/22" for a two-up spread, "22/22" for a lone last page, "5/22" one-up.
 * [twoUp] is the live layout decision; [page] is the source-of-truth current page.
 */
fun pagerLabel(page: Int, pageCount: Int, twoUp: Boolean, songStarts: List<Int>): String {
    if (pageCount <= 0) return "0/0"
    if (!twoUp) return "${page.coerceIn(0, pageCount - 1) + 1}/$pageCount"
    val pages = spreadPages(page, songStarts, pageCount)
    return if (pages.size == 2) "${pages[0] + 1}–${pages[1] + 1}/$pageCount"
    else "${pages[0] + 1}/$pageCount"
}
