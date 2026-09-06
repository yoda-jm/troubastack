package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * T158 — the Stage surface running the shared running-order-numbering vectors. These cases mirror
 * docs/contracts/running-order-numbering.vectors.json 1:1 (CI diffs the resources copy against the
 * canonical file); Go (the export) and TS (Studio) run the same table. THE rule: a number belongs only to
 * a main-order song — on-call and intermission carry none and never shift the count.
 */
class RunningOrderNumberingTest {
    private fun song(onCall: Boolean = false) = RunningOrderEntry(RunningOrderKind.SONG, onCall)
    private val intermission = RunningOrderEntry(RunningOrderKind.INTERMISSION, false)

    private fun check(name: String, entries: List<RunningOrderEntry>, expected: List<Int?>) {
        assertEquals(expected, runningOrderNumbers(entries), name)
    }

    @Test fun all_main_songs_number_1_to_n() =
        check("all main songs", listOf(song(), song(), song()), listOf(1, 2, 3))

    @Test fun trailing_on_call_is_unnumbered() =
        check("trailing on-call", listOf(song(), song(), song(onCall = true)), listOf(1, 2, null))

    @Test fun on_call_mid_list_does_not_shift_the_next_song() =
        // the discriminating case: a naive "number every entry" would give [1,2,3], not [1,null,2].
        check("on-call mid-list", listOf(song(), song(onCall = true), song()), listOf(1, null, 2))

    @Test fun intermission_between_2_and_3_leaves_the_next_song_reading_3() =
        check("intermission between", listOf(song(), song(), intermission, song()), listOf(1, 2, null, 3))

    @Test fun leading_intermission_does_not_consume_number_1() =
        check("leading intermission", listOf(intermission, song()), listOf(null, 1))

    @Test fun intermission_and_on_call_both_unnumbered() =
        check("intermission + on-call", listOf(song(), intermission, song(onCall = true)), listOf(1, null, null))

    @Test fun empty_setlist_numbers_nothing() =
        check("empty", emptyList(), emptyList())

    @Test fun the_drawer_numbers_via_the_shared_rule() {
        // The mobile SURFACE (drawerRows) must obey the same rule: a main song numbered, a bench song not.
        val state = StageState(
            pages = listOf(StagePage("s1", "A", 0, "a", overlays = emptyList(), status = PageStatus.READY)),
            songs = listOf(
                SongInfo("s1", "A", firstPage = 0),
                SongInfo("s2", "B", firstPage = 0, onCall = true),
                SongInfo("s3", "C", firstPage = 0),
            ),
        )
        val numbered = drawerRows(state).filterIsInstance<DrawerRow.Song>().associate { it.info.songId to it.number }
        assertEquals(1, numbered["s1"])
        assertEquals(null, numbered["s2"]) // on-call → no number
        assertEquals(2, numbered["s3"]) // the bench song did not shift it to 3
    }

    @Test fun the_drawer_shows_an_intermission_as_an_unnumbered_labelled_row() {
        // T153: a break between songs 1 and 2 is a MAIN-order entry (onCall=false) that carries kind
        // INTERMISSION, so it appears in the drawer, takes no number, and does NOT shift the song after it —
        // the T158 rule, now driven off SongInfo.kind through the Stage drawer surface. A naive "number every
        // main entry" would number the break and push the next song to 3; this asserts it reads 2.
        val state = StageState(
            pages = listOf(StagePage("s1", "A", 0, "a", overlays = emptyList(), status = PageStatus.READY)),
            songs = listOf(
                SongInfo("s1", "A", firstPage = 0),
                SongInfo("", "Entracte", firstPage = 0, kind = RunningOrderKind.INTERMISSION, label = "Entracte"),
                SongInfo("s3", "C", firstPage = 0),
            ),
        )
        val songRows = drawerRows(state).filterIsInstance<DrawerRow.Song>()
        val brk = songRows.first { it.info.kind == RunningOrderKind.INTERMISSION }
        assertEquals(null, brk.number, "an intermission carries no running-order number")
        assertEquals("Entracte", brk.info.name, "the intermission renders its label")
        val byId = songRows.associate { it.info.songId to it.number }
        assertEquals(1, byId["s1"])
        assertEquals(2, byId["s3"]) // the break did not shift the next song to 3
    }
}
