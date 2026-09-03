package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * P207 stage 2 — the artist flows from the bundle's [BakedSong] into the drawer's [SongInfo], and an
 * absent artist stays empty (the compatibility property: an old bundle from before the field, or any
 * song with no artist, must behave exactly as before — no drawer dash). The drawer line itself is thin
 * Compose (Title — grey Artist, artist clipped first); this pins the model wiring underneath it.
 */
class P207ArtistTest {

    private fun state(vararg songs: BakedSong): StageState =
        StageViewModel(LoadResult.Loaded(ConcertBundle(concertId = "c1", songs = songs.toList()), emptyList())).state.value

    private fun song(id: String, title: String, artist: String = "") = BakedSong(
        songId = id,
        title = title,
        artist = artist,
        pages = listOf(PageImages(pageRasterRef = "$id-p1.png")),
    )

    @Test
    fun songInfo_carriesArtist_whenPresent() {
        val s = state(song("a", "Song A", artist = "The Artist"))
        assertEquals("The Artist", s.songs[0].artist)
    }

    @Test
    fun songInfo_artistEmpty_whenAbsent_isTheCompatCase() {
        // A song with no artist (an old bundle, or just a song without one) → empty, so the drawer draws
        // no dash and no grey suffix. This is the whole additive-compatibility guarantee.
        val s = state(song("a", "Song A"))
        assertEquals("", s.songs[0].artist)
    }
}
