package com.troubastack.shared.distribution

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** A91 — the one-list state table, row by row, in precedence order (spec: docs/tasks/A91-one-concert-list.md). */
class ConcertRowsTest {
    private val now = 1_800_000_000L
    private val online = Reach.Online(now - 60)
    private val offline = Reach.Offline(now - 3 * 3600)
    private fun local(rev: ULong = 5uL, damaged: Boolean = false, songs: List<LocalSong> = emptyList()) =
        LocalBundle("c1", rev, bakedAt = now - 2 * 3600, name = "Sat @ The Anchor", bandId = "b1", bandName = "The Troubadours", damaged = damaged, songs = songs)
    private fun server(rev: ULong = 7uL, finalLocked: Boolean = false, archived: Boolean = false, admin: Boolean = false, songs: List<ServerSong> = emptyList()) =
        ServerConcert("c1", rev, name = "Sat @ The Anchor", bandId = "b1", bandName = "The Troubadours", eventDate = "2026-11-07",
            finalLocked = finalLocked, archived = archived, songs = songs, isAdmin = admin)
    private fun row(local: LocalBundle?, server: ServerConcert?, reach: Reach = online, frozen: Boolean = false, op: RowOp? = null) =
        rowState(local, server, reach, frozen, op, now)

    @Test fun r1_downloading_shows_progress_and_cancel() {
        val r = row(null, server(), op = RowOp.Downloading(0.42f))!!
        assertEquals(RowStateKind.BUSY, r.state); assertEquals("Downloading… 42 %", r.line); assertEquals(RowAction.CANCEL, r.primary)
    }
    @Test fun r1b_baking_has_no_cancel_but_the_copy_stays_openable() {
        val r = row(local(), server(), op = RowOp.Baking("3/12"))!!
        assertEquals(RowAction.NONE, r.primary); assertFalse(r.primaryEnabled); assertEquals(listOf(RowMenuItem.OPEN_THIS_VERSION), r.menu)
    }
    @Test fun r2_archived_with_a_copy_goes_to_the_archived_section_open_and_delete_only() {
        val r = row(local(), server(archived = true))!!
        assertEquals(RowSection.ARCHIVED, r.section); assertEquals("Archived on the server · v5", r.line)
        assertEquals(RowAction.OPEN, r.primary); assertEquals(listOf(RowMenuItem.DELETE), r.menu)
    }
    @Test fun r3_archived_without_a_copy_is_not_listed() {
        assertNull(row(null, server(archived = true)))
    }
    @Test fun archived_is_never_offered_even_damaged_or_behind() {
        val r = row(local(rev = 2uL, damaged = true), server(rev = 9uL, archived = true))!!
        assertEquals(RowSection.ARCHIVED, r.section); assertFalse(r.primaryEnabled); assertEquals("Archived on the server · v2 · can't be opened", r.line)
    }
    @Test fun unarchived_returns_to_its_normal_row() {
        assertEquals(RowStateKind.UPDATE_AVAILABLE, row(local(), server(archived = false))!!.state)
    }
    @Test fun r4_damaged_offers_a_redownload_only_when_online_and_on_the_server() {
        assertEquals(RowAction.DOWNLOAD, row(local(damaged = true), server())!!.primary)
        assertTrue(row(local(damaged = true), server())!!.primaryEnabled)
        assertFalse(row(local(damaged = true), null)!!.primaryEnabled)
        assertFalse(row(local(damaged = true), server(), reach = offline)!!.primaryEnabled)
    }
    @Test fun r5_a90_deleted_once_is_listed_on_the_server_with_download() {
        // A90's case: the copy was deleted (no local), the server lists it — the row stays, with Download.
        val r = row(null, server())!!
        assertEquals(RowStateKind.NOT_ON_TABLET, r.state); assertEquals("On the server · v7", r.line)
        assertEquals(RowAction.DOWNLOAD, r.primary); assertTrue(r.primaryEnabled)
    }
    @Test fun r6_offline_not_on_tablet_is_shown_but_cannot_download() {
        val r = row(null, server(), reach = offline)!!
        assertEquals("On the server · v7 · last checked 3 h ago", r.line); assertFalse(r.primaryEnabled)
    }
    @Test fun r7_only_on_this_tablet() {
        val r = row(local(), null)!!
        assertEquals(RowStateKind.LOCAL_ONLY, r.state); assertEquals("On this tablet only · v5", r.line); assertEquals(RowAction.OPEN, r.primary)
    }
    @Test fun r8_offline_opens_the_copy_and_says_what_was_seen() {
        val r = row(local(), server(), reach = offline)!!
        assertEquals(RowStateKind.OFFLINE, r.state); assertEquals("v5 · last checked 3 h ago · v7 seen on the server", r.line); assertEquals(RowAction.OPEN, r.primary)
        assertEquals("v5 · last checked 3 h ago", row(local(), null, reach = offline)!!.line)
    }
    @Test fun r9_up_to_date() {
        val r = row(local(rev = 7uL), server(rev = 7uL))!!
        assertEquals(RowStateKind.UP_TO_DATE, r.state); assertEquals("Up to date · v7 · baked 2 h ago", r.line); assertEquals(RowAction.OPEN, r.primary)
    }
    @Test fun r10_frozen_offers_no_update() {
        val r = row(local(), server(), frozen = true)!!
        assertEquals(RowStateKind.FROZEN, r.state); assertEquals("Frozen at v5 · v7 on the server", r.line); assertEquals(RowAction.OPEN, r.primary)
        assertTrue(RowMenuItem.UNFREEZE in r.menu)
    }
    @Test fun r11_final_version_is_offered() {
        val r = row(local(), server(finalLocked = true))!!
        assertEquals(RowStateKind.FINAL, r.state); assertEquals("Final version v7 (you have v5)", r.line); assertEquals(RowAction.UPDATE, r.primary)
    }
    @Test fun r12_update_available_is_a_tap_with_what_changed() {
        val l = local(songs = listOf(LocalSong("s1", 1uL, "The Open Road"), LocalSong("s2", 1uL, "Amazing Grace"), LocalSong("s3", 1uL, "Greensleeves")))
        val r = row(l, server(songs = listOf(ServerSong("s1", 2uL), ServerSong("s2", 1uL), ServerSong("s4", 1uL))))!!
        assertEquals(RowStateKind.UPDATE_AVAILABLE, r.state); assertEquals(RowAction.UPDATE, r.primary)
        assertEquals("New version v7 (you have v5) · 1 song changed, 1 added, 1 removed", r.line)
        assertEquals(SongChanges(changed = listOf("The Open Road"), added = 1, removed = listOf("Greensleeves")), r.changes)
        assertTrue(RowMenuItem.OPEN_THIS_VERSION in r.menu, "Open stays one tap away, on the old version")
    }
    @Test fun admin_items_need_the_band_role_and_the_server() {
        assertTrue(RowMenuItem.REBAKE in row(local(rev = 7uL), server(admin = true))!!.menu)
        assertFalse(RowMenuItem.REBAKE in row(local(rev = 7uL), server(admin = false))!!.menu)
        assertFalse(RowMenuItem.REBAKE in row(local(rev = 7uL), server(admin = true), reach = offline)!!.menu)
    }
    @Test fun deleting_with_the_server_online_keeps_the_row_as_on_the_server() {
        val before = concertRows(listOf(local()), listOf(server()), online, emptySet(), emptyMap(), now, "2026-10-21")
        val after = concertRows(emptyList(), listOf(server()), online, emptySet(), emptyMap(), now, "2026-10-21")
        assertEquals(1, before.size); assertEquals(1, after.size)
        assertEquals(RowStateKind.NOT_ON_TABLET, after.single().state)
    }
    @Test fun bands_then_upcoming_ascending_then_past_descending_then_undated() {
        fun s(id: String, date: String, band: String = "The Troubadours") = ServerConcert(id, 1uL, name = id, bandId = band, bandName = band, eventDate = date)
        val rows = concertRows(
            locals = listOf(LocalBundle("mine", 1uL, now, "mine", "", "")),
            servers = listOf(s("past-old", "2026-01-10"), s("undated", ""), s("soon", "2026-10-25"), s("later", "2026-12-01"), s("past-recent", "2026-09-30"), s("other", "2026-11-01", band = "A Band")),
            reach = online, frozenIds = emptySet(), inFlight = emptyMap(), nowSec = now, today = "2026-10-21",
        )
        assertEquals(listOf("other", "soon", "later", "past-recent", "past-old", "undated", "mine"), rows.map { it.id })
    }
    @Test fun what_changed_counts() {
        val c = songChanges(listOf(LocalSong("a", 1uL, "A"), LocalSong("b", 3uL, "B")), listOf(ServerSong("a", 1uL), ServerSong("b", 4uL), ServerSong("c", 1uL), ServerSong("d", 1uL)))
        assertEquals(SongChanges(listOf("B"), added = 2, removed = emptyList()), c); assertEquals("1 song changed, 2 added", c.summary())
        assertTrue(songChanges(listOf(LocalSong("a", 1uL, "A")), listOf(ServerSong("a", 1uL))).isEmpty)
    }
}
