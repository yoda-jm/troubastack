package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * T153 — the bundle→Stage mapping for an intermission. Core bakes a break as a BakedSong with
 * kind="intermission", a label, and exactly one separator page (slice 2). The mapper must turn that into a
 * SongInfo(kind=INTERMISSION) carrying the label, so the drawer numbers it via the shared T158 rule (no
 * number, no shift) and the musical chrome suppresses. The wire is a STRING (BAKED_KIND_INTERMISSION),
 * additive: absent/"song" ⇒ a normal song, so a pre-T153 bundle is unchanged.
 *
 * This is the mapper seam the vectors cannot cover — RunningOrderNumberingTest pins the rule off hand-built
 * SongInfo; this pins that a baked entry actually BECOMES the right SongInfo.
 */
class IntermissionMapperTest {
    private fun state(vararg songs: BakedSong) =
        StageViewModel(LoadResult.Loaded(ConcertBundle(songs = songs.toList()), emptyList())).state.value

    private fun onePage(ref: String) = listOf(PageImages(pageRasterRef = ref))

    @Test fun an_intermission_entry_maps_to_an_INTERMISSION_song_with_its_label() {
        val s = state(
            BakedSong(songId = "s1", title = "A", pages = onePage("a")),
            BakedSong(songId = "", kind = "intermission", label = "Entracte", pages = onePage("break")),
            BakedSong(songId = "s3", title = "C", pages = onePage("c")),
        )
        assertEquals(3, s.songs.size)
        val brk = s.songs[1]
        assertEquals(RunningOrderKind.INTERMISSION, brk.kind)
        assertEquals("Entracte", brk.name, "the intermission shows its authored label, not a Song-N default")
        assertEquals(RunningOrderKind.SONG, s.songs[0].kind)
        assertEquals(RunningOrderKind.SONG, s.songs[2].kind)

        // End-to-end: the drawer numbers via the shared rule off the mapped kind — the break carries no
        // number and does not shift the song after it (1, null, 2), not (1, 2, 3).
        val rows = drawerRows(s).filterIsInstance<DrawerRow.Song>()
        assertEquals(listOf(1, null, 2), rows.map { it.number })
    }

    @Test fun an_intermission_with_a_blank_label_shows_the_default() {
        val s = state(BakedSong(kind = "intermission", label = "", pages = onePage("break")))
        assertEquals(RunningOrderKind.INTERMISSION, s.songs[0].kind)
        assertEquals(INTERMISSION_DEFAULT_LABEL, s.songs[0].name, "an empty label renders the default, not a blank card")
    }

    @Test fun absent_kind_maps_to_a_song_the_additive_contract() {
        // TEETH: a pre-T153 bundle (kind="") must read every entry as a SONG — if the mapper treated absent
        // as anything but SONG, an old bundle's songs would lose their numbers. Paired with a real break so
        // the two paths cannot be conflated.
        val s = state(
            BakedSong(songId = "s1", title = "A", pages = onePage("a")), // kind absent ⇒ song
            BakedSong(kind = "intermission", label = "Break", pages = onePage("i")),
        )
        assertEquals(RunningOrderKind.SONG, s.songs[0].kind)
        assertEquals("A", s.songs[0].name)
        assertEquals(RunningOrderKind.INTERMISSION, s.songs[1].kind)
    }
}
