package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import com.troubastack.shared.bundle.SongCue
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * A15 — the song-jump drawer's two pure inputs: which song a page belongs to (highlight) and the
 * per-song meta line (notes · key · ♩=tempo from the song's first page).
 */
class SongDrawerTest {

    private fun state(vararg songs: BakedSong): StageState {
        val r = StageViewModel(LoadResult.Loaded(ConcertBundle(concertId = "c1", songs = songs.toList()), emptyList()))
        return r.state.value
    }

    private fun song(pages: Int, key: String = "", tempo: Int = 0, notes: String = "", title: String = "", onCall: Boolean = false, cues: List<SongCue> = emptyList()) = BakedSong(
        songId = "s",
        pages = (1..pages).map { PageImages(pageRasterRef = "p$it.png") },
        key = key, tempo = tempo, displayNotes = notes, title = title, onCall = onCall, cues = cues,
    )

    @Test
    fun cues_flowFromBundleToSongInfo_inOrder_defaultEmpty() {
        // A20: the baked-for member's cues ride BakedSong.cues → SongInfo.cues (bake order preserved);
        // a song with none defaults to empty (additive field, old bundles omit it).
        val cues = listOf(SongCue("mic"), SongCue("guitar-electric", "#ff0000"))
        val s = state(song(2, cues = cues), song(1))
        assertEquals(cues, s.songs[0].cues)
        assertEquals(emptyList(), s.songs[1].cues)
    }

    @Test
    fun songName_usesBakedTitle_elseFallsBackToSongN() {
        // T26: the baked title names the song; an empty title falls back to "Song N".
        val s = state(song(1, title = "The Open Road"), song(1), song(1, title = "Amazing Grace"))
        assertEquals(listOf("The Open Road", "Song 2", "Amazing Grace"), s.songs.map { it.name })
    }

    @Test
    fun onCall_flagFlowsFromBundleToSongInfo() {
        // T23: bench songs carry on_call → SongInfo.onCall, which the drawer groups on.
        val s = state(song(2), song(1, onCall = true))
        assertEquals(listOf(false, true), s.songs.map { it.onCall })
    }

    @Test
    fun currentSong_isTheSongOwningTheCurrentPage() {
        // songs of 3 + 2 + 1 pages → firstPages 0, 3, 5.
        val s = state(song(3), song(2), song(1))
        assertEquals(3, s.songs.size)
        assertEquals(listOf(0, 3, 5), s.songs.map { it.firstPage })
        // page 0,1,2 → song 0 ; 3,4 → song 1 ; 5 → song 2
        assertEquals(0, s.copy(current = 0).currentSong)
        assertEquals(0, s.copy(current = 2).currentSong)
        assertEquals(1, s.copy(current = 3).currentSong)
        assertEquals(1, s.copy(current = 4).currentSong)
        assertEquals(2, s.copy(current = 5).currentSong)
    }

    @Test
    fun songMetaLine_readsFirstPageMetadata_andOmitsEmpty() {
        val s = state(
            song(3, key = "Em", tempo = 96, notes = "Acoustic intro"),
            song(2, key = "C"),                 // key only
            song(1),                            // nothing
        )
        assertEquals("Acoustic intro  ·  Em  ·  ♩=96", songMetaLine(s, 0))
        assertEquals("C", songMetaLine(s, 1))
        assertNull(songMetaLine(s, 2))          // no metadata → no line
    }

    @Test
    fun songMetaLine_outOfRangeSong_isNull() {
        val s = state(song(2, key = "G"))
        assertNull(songMetaLine(s, -1))
        assertNull(songMetaLine(s, 5))
    }

    @Test
    fun drawerRows_listsEverySong_numberedFrom1() {
        // A60 P1 (reachability) + P2 (numbering): a setlist longer than any screen still yields a row
        // per song, and the running order is numbered 1..N. The old direct-into-ColumnScope drawer laid
        // rows out with no scroll, so past-the-fold songs were unreachable; this guards the model half.
        val songs = (1..22).map { song(1, title = "S$it") }.toTypedArray()
        val rows = drawerRows(state(*songs))
        val songRows = rows.filterIsInstance<DrawerRow.Song>()
        assertEquals(22, songRows.size)                        // every song is present (reachable)
        assertEquals((1..22).toList(), songRows.map { it.number })    // numbered 1..22 in order
        assertEquals((0..21).toList(), songRows.map { it.songIndex }) // original indices drive the jump
        assertEquals(DrawerRow.Header("Songs"), rows.first())        // header reads as a header, first
    }

    @Test
    fun drawerRows_benchSeparated_unnumbered_afterMain() {
        // T23 + A60 P2/P3: "on call" songs form a second group — a Divider + "On call" header — and are
        // NOT numbered (deliberately not in the running order). Original indices survive the partition.
        val rows = drawerRows(state(song(1, title = "A"), song(1, title = "B", onCall = true), song(1, title = "C")))
        val songRows = rows.filterIsInstance<DrawerRow.Song>()
        assertEquals(listOf(0, 2, 1), songRows.map { it.songIndex }) // main A(0),C(2) then bench B(1)
        assertEquals(listOf(1, 2, null), songRows.map { it.number }) // main 1,2 ; bench unnumbered
        assertEquals(
            listOf(DrawerRow.Header("Songs"), DrawerRow.Header("On call")),
            rows.filterIsInstance<DrawerRow.Header>(),
        )
        assertEquals(1, rows.count { it is DrawerRow.Divider })
    }

    @Test
    fun drawerRows_noBench_hasNoDividerOrSecondHeader() {
        // With no "on call" songs there is exactly one header and no separator — the divider is a group
        // separator, not a per-header decoration (P3).
        val rows = drawerRows(state(song(1), song(1)))
        assertEquals(0, rows.count { it is DrawerRow.Divider })
        assertEquals(1, rows.filterIsInstance<DrawerRow.Header>().size)
        assertEquals(listOf(1, 2), rows.filterIsInstance<DrawerRow.Song>().map { it.number })
    }
}
