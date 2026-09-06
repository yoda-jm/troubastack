package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * T149 (mobile half) — in SCROLL mode, a song's LAST page is trimmed to the baked content bottom (plus a
 * breathing margin) so the performer doesn't scroll through blank paper before the next song. Pure seam;
 * asserts the composed height, not a screenshot.
 */
class ScrollTrimTest {
    private val margin = SCROLL_TRIM_BREATHING_PERMILLE / 1000.0 // 0.04

    @Test
    fun last_page_with_short_content_is_trimmed_to_content_plus_margin() {
        // last page, text ends ~5% down → draw 5% + breathing margin, not the full page.
        assertEquals(0.05 + margin, scrollTrimFraction(isLastPageOfSong = true, contentBottomPermille = 50), 1e-9)
    }

    @Test
    fun mark_below_the_text_stays_visible() {
        // THE protect-the-annotation case: text ends at 0.05 but an overlay reaches 0.42, so the baker wrote
        // contentBottom=420 (max of raster+overlays). Obeying it draws to 0.42+margin — the mark is shown,
        // NOT cropped. (Trimming to the text alone, ~0.05, would hide it.)
        val f = scrollTrimFraction(isLastPageOfSong = true, contentBottomPermille = 420)
        assertEquals(0.42 + margin, f, 1e-9)
        assertTrue(f > 0.42, "the mark at 0.42 must be within the drawn region")
    }

    @Test
    fun intermediate_page_keeps_full_height() {
        // A non-last page with a short tail must NOT be trimmed (trimming mid-song makes the page lie).
        assertEquals(1.0, scrollTrimFraction(isLastPageOfSong = false, contentBottomPermille = 50), 1e-9)
    }

    @Test
    fun old_bundle_without_content_bottom_renders_full() {
        // Absent/0 ⇒ full page (the T143 "never distort what predates the field" lesson).
        assertEquals(1.0, scrollTrimFraction(isLastPageOfSong = true, contentBottomPermille = 0), 1e-9)
    }

    @Test
    fun ink_reaching_the_bottom_is_not_trimmed() {
        assertEquals(1.0, scrollTrimFraction(isLastPageOfSong = true, contentBottomPermille = 1000), 1e-9)
        // and a near-full page never grows past full even with the margin added
        assertEquals(1.0, scrollTrimFraction(isLastPageOfSong = true, contentBottomPermille = 980), 1e-9)
    }

    @Test
    fun composed_scroll_height_drops_when_the_last_page_is_short() {
        // A 2-page song at a fixed page height; page 0 full (non-last), page 1's content ends ~5%.
        val fullPageH = 1000.0
        val pages = listOf(0 to false, 50 to true) // (contentBottomPermille, isLast)
        val composed = pages.sumOf { (cb, last) -> fullPageH * scrollTrimFraction(last, cb) }
        val untrimmed = pages.size * fullPageH
        assertTrue(composed < untrimmed, "the short last page must shorten the column: $composed vs $untrimmed")
        assertEquals(fullPageH + (0.05 + margin) * fullPageH, composed, 1e-6)
    }
}
