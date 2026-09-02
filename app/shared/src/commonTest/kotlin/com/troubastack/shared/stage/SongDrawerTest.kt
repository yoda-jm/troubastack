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
}
