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
        val a = concertRowSubtitle(1uL, 1_700_000_000L, offsetSeconds = 0)
        val b = concertRowSubtitle(10uL, 1_700_086_400L, offsetSeconds = 0)
        assertTrue(a != b, "same-named bakes must render distinguishable subtitles: '$a' vs '$b'")
        assertTrue(a.contains("rev 1"))
        assertTrue(b.contains("rev 10"))
    }

    @Test
    fun subtitle_formats_utc_when_offset_is_zero() {
        // A viewer whose zone IS UTC (offset 0) sees UTC with no label — the label is only for UNRESOLVED.
        assertEquals("rev 7 · 2023-11-14 22:13", concertRowSubtitle(7uL, 1_700_000_000L, offsetSeconds = 0))
    }

    @Test
    fun subtitle_omits_absent_time() {
        assertEquals("rev 0", concertRowSubtitle(0uL, 0L, offsetSeconds = 0))
    }

    // --- T148: the time reads in the musician's own zone, not UTC ---

    @Test
    fun subtitle_renders_local_hour_utc_plus_two() {
        // 1_700_000_000 = 2023-11-14 22:13 UTC. VLL is on UTC+2 → 2023-11-15 00:13 local (and the DATE rolls).
        // On the pre-T148 UTC code this is red by exactly the two-hour gap VLL hit.
        assertEquals("rev 7 · 2023-11-15 00:13", concertRowSubtitle(7uL, 1_700_000_000L, offsetSeconds = 2 * 3600))
    }

    @Test
    fun subtitle_renders_local_hour_zone_behind_utc() {
        // A zone BEHIND UTC (−5h) → 2023-11-14 17:13. A sign error could not pass both this and the +2 case.
        assertEquals("rev 7 · 2023-11-14 17:13", concertRowSubtitle(7uL, 1_700_000_000L, offsetSeconds = -5 * 3600))
    }

    @Test
    fun subtitle_local_date_crosses_midnight() {
        // The date-boundary case: 22:13 on the 14th UTC is 00:13 on the 15th at +2 — the DATE must advance,
        // which a naive offset-on-the-time-only would get wrong (it would keep "11-14").
        val s = concertRowSubtitle(7uL, 1_700_000_000L, offsetSeconds = 2 * 3600)
        assertTrue(s.contains("2023-11-15"), "local date must roll to the 15th, got '$s'")
    }

    @Test
    fun subtitle_labels_utc_when_zone_unresolved() {
        // If the platform seam returns null (zone unresolvable), show UTC and SAY so — never a silent non-local time.
        assertEquals("rev 7 · 2023-11-14 22:13 UTC", concertRowSubtitle(7uL, 1_700_000_000L, offsetSeconds = null))
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
