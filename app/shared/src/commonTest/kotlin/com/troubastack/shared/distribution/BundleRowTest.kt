package com.troubastack.shared.distribution

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** T143 — the concert row must distinguish same-named bakes, and Delete must be reachable in manage but
 *  never in perform (lean). Pure seams, RED-FIRST. */
class BundleRowTest {

    @Test
    fun subtitle_distinguishes_two_same_named_bakes() {
        // Same concert NAME, two bakes: the subtitle (rev + time) must differ — the whole point of T143 §1.
        val a = concertRowSubtitle(1uL, 1_700_000_000L)
        val b = concertRowSubtitle(10uL, 1_700_086_400L)
        assertTrue(a != b, "same-named bakes must render distinguishable subtitles: '$a' vs '$b'")
        assertTrue(a.contains("rev 1"))
        assertTrue(b.contains("rev 10"))
    }

    @Test
    fun subtitle_formats_utc_minute() {
        assertEquals("rev 7 · 2023-11-14 22:13", concertRowSubtitle(7uL, 1_700_000_000L))
    }

    @Test
    fun subtitle_omits_absent_time() {
        assertEquals("rev 0", concertRowSubtitle(0uL, 0L))
    }

    @Test
    fun menu_perform_lean_offers_nothing() {
        assertTrue(bundleMenuActions(lean = true, damaged = false).isEmpty())
        assertTrue(bundleMenuActions(lean = true, damaged = true).isEmpty()) // still lean, even if damaged
    }

    @Test
    fun menu_manage_healthy_offers_delete_alongside_the_rest() {
        val actions = bundleMenuActions(lean = false, damaged = false)
        assertTrue(BundleAction.Delete in actions, "a healthy duplicate must be deletable (VLL's case)")
        assertTrue(BundleAction.Freeze in actions && BundleAction.Pin in actions)
    }

    @Test
    fun menu_damaged_is_delete_only() {
        assertEquals(listOf(BundleAction.Delete), bundleMenuActions(lean = false, damaged = true))
    }

    // --- T143 accordion: group the library by band ---

    private data class Row(val id: String, val band: String, val bandName: String)

    @Test
    fun group_by_band_collects_and_labels() {
        val rows = listOf(
            Row("c1", "b-zulu", "Zulu Choir"),
            Row("c2", "b-alpha", "Alpha Band"),
            Row("c3", "b-zulu", "Zulu Choir"),
        )
        val groups = groupByBand(rows, { it.band }, { it.bandName })
        // Alphabetical by name: Alpha before Zulu.
        assertEquals(listOf("Alpha Band", "Zulu Choir"), groups.map { it.bandName })
        assertEquals(listOf("c2"), groups[0].items.map { it.id })
        assertEquals(listOf("c1", "c3"), groups[1].items.map { it.id }) // both zulu, incoming order kept
    }

    @Test
    fun group_by_band_puts_unknown_last_and_never_drops() {
        val rows = listOf(
            Row("c1", "", ""),               // pre-T143 / old bundle: no band identity
            Row("c2", "b-alpha", "Alpha Band"),
        )
        val groups = groupByBand(rows, { it.band }, { it.bandName })
        assertEquals(listOf("Alpha Band", UNKNOWN_BAND_LABEL), groups.map { it.bandName })
        assertEquals(2, groups.sumOf { it.items.size }) // no bundle dropped
        assertEquals("", groups.last().bandId)
    }

    @Test
    fun group_by_band_merges_same_id_takes_first_nonblank_name() {
        // Same band id, one bake stored a blank name (older baker): still one group, real name wins.
        val rows = listOf(Row("c1", "b-x", ""), Row("c2", "b-x", "Real Name"))
        val groups = groupByBand(rows, { it.band }, { it.bandName })
        assertEquals(1, groups.size)
        assertEquals("Real Name", groups[0].bandName)
        assertEquals(2, groups[0].items.size)
    }

    // --- T143: setlist id in the ⋮ ---

    @Test
    fun setlist_id_of_band_wide_concert_is_the_concert_id() {
        assertEquals("setlist-123", setlistIdOf("setlist-123"))
    }

    @Test
    fun setlist_id_of_legacy_member_variant_strips_the_owner() {
        assertEquals("setlist-123", setlistIdOf("setlist-123~user-456"))
    }
}
