package com.troubashare.shared.stage

import com.troubashare.shared.bundle.BakedSong
import com.troubashare.shared.bundle.ConcertBundle
import com.troubashare.shared.bundle.LoadResult
import com.troubashare.shared.bundle.PageImages
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

    private fun song(pages: Int, key: String = "", tempo: Int = 0, notes: String = "") = BakedSong(
        songId = "s",
        pages = (1..pages).map { PageImages(pageRasterRef = "p$it.png") },
        key = key, tempo = tempo, displayNotes = notes,
    )

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
