package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * A46 (found by A33 drill 2) — reopening a concert must land back where the performer left off, not
 * page 0. [resolveStartPage] maps a saved LOGICAL position (songId + page-in-song) to a global page
 * index, surviving a re-bake that reorders songs/pages (and clamping to the song's last page if a
 * shorter re-bake dropped the exact page — A46 §3); the VM seeds `current` from it. Before this the
 * Stage always started at 0, so a process death / exit mid-set lost your place.
 */
class StagePositionTest {

    private fun page(songId: String, pageInSong: Int) = StagePage(
        songId = songId,
        songName = songId,
        pageInSong = pageInSong,
        rasterRef = "$songId-$pageInSong",
        overlays = emptyList(),
        status = PageStatus.READY,
    )

    private fun state(vararg p: StagePage) = StageState(pages = p.toList())

    @Test
    fun emptySaved_startsAtTop() {
        val s = state(page("a", 0), page("a", 1), page("b", 0))
        assertEquals(0, resolveStartPage(s, "", 0))
    }

    @Test
    fun exactLogicalPage_resolvesToItsGlobalIndex() {
        val s = state(page("a", 0), page("a", 1), page("b", 0), page("b", 1))
        assertEquals(1, resolveStartPage(s, "a", 1))
        assertEquals(3, resolveStartPage(s, "b", 1))
    }

    @Test
    fun beyondEnd_clampsToTheSongsLastPage() {
        // A46 §3: the concert was re-baked shorter — b now has pages 0,1,2 but we saved b#5. Clamp to b's
        // LAST page (index 4), never off the end and not its first (index 2) — discriminating.
        val s = state(page("a", 0), page("a", 1), page("b", 0), page("b", 1), page("b", 2))
        assertEquals(4, resolveStartPage(s, "b", 5))
    }

    @Test
    fun unknownSong_fallsBackToZero() {
        val s = state(page("a", 0), page("b", 0))
        assertEquals(0, resolveStartPage(s, "gone", 0))
    }

    @Test
    fun survivesReorder_findsByLogicalIdNotRawIndex() {
        // Saved "b#0" while b happened to be first; a re-bake moved a to the front. Still lands on b#0,
        // not whatever now sits at the old raw index.
        val s = state(page("a", 0), page("a", 1), page("b", 0))
        assertEquals(2, resolveStartPage(s, "b", 0))
    }

    // --- the VM seeds `current` from the saved position (the wiring, end to end) ---

    private fun bundle(vararg songPages: Pair<String, Int>) = LoadResult.Loaded(
        ConcertBundle(
            concertId = "c1",
            songs = songPages.map { (id, pages) ->
                BakedSong(songId = id, pages = (1..pages).map { PageImages(pageRasterRef = "$id-p$it.png") })
            },
        ),
        emptyList(),
    )

    @Test
    fun vm_seedsCurrentFromInitialPosition() {
        // pages: a0 a1 b0 b1 c0  ⇒ indices 0..4; b#1 is index 3.
        val vm = StageViewModel(bundle("a" to 2, "b" to 2, "c" to 1), initialSongId = "b", initialPageInSong = 1)
        assertEquals(3, vm.state.value.current)
        assertEquals("b", vm.state.value.currentPage?.songId)
        assertEquals(1, vm.state.value.currentPage?.pageInSong)
    }

    @Test
    fun vm_defaultsToTop_withNoInitialPosition() {
        val vm = StageViewModel(bundle("a" to 2, "b" to 1))
        assertEquals(0, vm.state.value.current)
    }

    // --- T153: an intermission is not "Song N" in the title-card position line ---

    private fun withIntermission() = StageState(
        pages = listOf(
            StagePage("a", "A", 0, "a", overlays = emptyList(), status = PageStatus.READY),
            StagePage("", "Entracte", 0, "i", overlays = emptyList(), status = PageStatus.READY),
            StagePage("c", "C", 0, "c", overlays = emptyList(), status = PageStatus.READY),
        ),
        songs = listOf(
            SongInfo("a", "A", firstPage = 0),
            SongInfo("", "Entracte", firstPage = 1, kind = RunningOrderKind.INTERMISSION, label = "Entracte"),
            SongInfo("c", "C", firstPage = 2),
        ),
    )

    @Test
    fun positionLabel_dropsTheSongCounter_forAnIntermission() {
        // On the intermission page: the title already reads "Entracte", so the position line must NOT call it
        // "Song 2/3" — it shows only the page position. (Teeth: remove the guard and this flips to "Song 2…".)
        val label = stagePositionLabel(withIntermission().copy(current = 1), topPage = 1, twoUp = false)
        assertTrue("Song" !in label, "an intermission must not be labelled \"Song N\" (got \"$label\")")
    }

    @Test
    fun a_break_presents_as_a_poster_but_a_song_does_not() {
        // T165: the intermission page is fit-to-viewport (a poster); song pages read as documents. TEETH:
        // both sides asserted, so "make everything a poster" or "never a poster" each reddens.
        val s = withIntermission()
        assertTrue(stagePresentsAsPoster(s.copy(current = 1)), "the break page presents as a poster")
        assertFalse(stagePresentsAsPoster(s.copy(current = 0)), "a song does not")
        assertFalse(stagePresentsAsPoster(s.copy(current = 2)), "nor the song after the break")
    }

    @Test
    fun positionLabel_keepsTheSongCounter_forARealSong() {
        // The other side of the guard: a real song still carries its counter.
        val label = stagePositionLabel(withIntermission().copy(current = 0), topPage = 0, twoUp = false)
        assertTrue(label.startsWith("Song 1/3"), "a song keeps its counter (got \"$label\")")
    }

    // --- A94: an intermission card is stored as ("", k), the k-th intermission, and resolves back to it ---

    /** A concert from entries: a song is (id to its page count), an intermission card is `null` (one page). */
    private fun concert(vararg entries: Pair<String, Int>?): StageState {
        val pages = ArrayList<StagePage>()
        val songs = ArrayList<SongInfo>()
        for (e in entries) {
            if (e == null) {
                songs += SongInfo("", "Interval", firstPage = pages.size, kind = RunningOrderKind.INTERMISSION, label = "Interval")
                pages += page("", 0)
            } else {
                songs += SongInfo(e.first, e.first, firstPage = pages.size)
                repeat(e.second) { pages += page(e.first, it) }
            }
        }
        return StageState(pages = pages, songs = songs)
    }

    /** a0 a1 | card | b0 | card | c0 — the cards are at indices 2 and 4. */
    private fun twoCards() = concert("a" to 2, null, "b" to 1, null, "c" to 1)

    @Test
    fun aCard_isStoredAsItsOrdinal_andResolvesBackToIt() {
        // TEETH (the A94 sabotage): put back the empty-songId early return and both cards resolve to the top (0).
        val s = twoCards()
        assertEquals("" to 0, positionAt(s, 2), "the first card is the 0th intermission")
        assertEquals("" to 1, positionAt(s, 4), "the second card is the 1st intermission")
        assertEquals(2, resolveStartPage(s, "", 0))
        assertEquals(4, resolveStartPage(s, "", 1))
    }

    @Test
    fun everyPage_roundTripsThroughItsStoredPosition() {
        // The property, over songs and cards alike: what is stored for a page resolves back to that page.
        val s = twoCards()
        for (i in s.pages.indices) {
            val (songId, at) = positionAt(s, i)!!
            assertEquals(i, resolveStartPage(s, songId, at), "page $i stored as ($songId, $at)")
        }
    }

    @Test
    fun everyPageOfACard_isThatCard() {
        // Fable's guard: a card is decided by the entry that owns the page, so a (hypothetical) two-page card never
        // reports its second page as ("", 1) — another card. TEETH: decide by "is the card's first page" and this reddens.
        val s = twoCards().let { st -> st.copy(pages = st.pages.toMutableList().apply { add(3, page("", 1)) },
            songs = st.songs.mapIndexed { i, e -> if (i >= 2) e.copy(firstPage = e.firstPage + 1) else e }) }
        assertEquals("" to 0, positionAt(s, 2))
        assertEquals("" to 0, positionAt(s, 3), "the card's second page is still the first card")
        assertEquals("" to 1, positionAt(s, 5))
    }

    @Test
    fun songPages_storeWhatTheyStoredBefore() {
        // A46/A48 untouched: a song page is still (songId, pageInSong).
        val s = twoCards()
        assertEquals("a" to 1, positionAt(s, 1))
        assertEquals("b" to 0, positionAt(s, 3))
        assertEquals("c" to 0, positionAt(s, 5))
    }

    @Test
    fun aRemovedCard_landsOnTheLastCardLeft_neverTheTop() {
        // Left on the second card; the re-bake now has only one. Its card, not the top (and not off the end).
        val oneCard = concert("a" to 2, null, "b" to 1, "c" to 1)
        assertEquals(2, resolveStartPage(oneCard, "", 1))
    }

    @Test
    fun aMovedCard_isFoundWhereItIsNow() {
        // Songs were added before the first card: the card moved from index 2 to 4, and is still found.
        val moved = concert("a" to 2, "x" to 1, "y" to 1, null, "b" to 1, null, "c" to 1)
        assertEquals(4, resolveStartPage(moved, "", 0))
        assertEquals(6, resolveStartPage(moved, "", 1))
    }

    @Test
    fun noCardLeft_landsAtTheTop() {
        val noCards = concert("a" to 2, "b" to 1)
        assertEquals(0, resolveStartPage(noCards, "", 0))
        assertEquals(0, resolveStartPage(noCards, "", 1))
    }

    @Test
    fun theOldHashZero_meansTheFirstCard() {
        // Before A94 every intermission stored "#0" (decoded as ("", 0)). Ruled: the first card, or the top with none.
        assertEquals(2, resolveStartPage(concert("a" to 2, null, "b" to 1), "", 0), "one card")
        assertEquals(2, resolveStartPage(twoCards(), "", 0), "two cards: the first")
        assertEquals(0, resolveStartPage(concert("a" to 2, "b" to 1), "", 0), "no card: the top")
    }

    @Test
    fun aNegativeOrdinal_isTheFirstCard() {
        // decodeStagePosition parses "#-3" as -3; never an exception, never off the start.
        assertEquals(2, resolveStartPage(twoCards(), "", -3))
    }

    // --- A94 wiring: the VM seeds the card, and an identity switch on a card keeps it ---

    private fun bundleWithCards() = LoadResult.Loaded(
        ConcertBundle(
            concertId = "c1",
            songs = listOf(
                BakedSong(songId = "a", pages = listOf(PageImages(pageRasterRef = "a0.png"), PageImages(pageRasterRef = "a1.png"))),
                BakedSong(kind = "intermission", label = "Interval", pages = listOf(PageImages(pageRasterRef = "i1.png"))),
                BakedSong(songId = "b", pages = listOf(PageImages(pageRasterRef = "b0.png"))),
                BakedSong(kind = "intermission", label = "Interval", pages = listOf(PageImages(pageRasterRef = "i2.png"))),
                BakedSong(songId = "c", pages = listOf(PageImages(pageRasterRef = "c0.png"))),
            ),
        ),
        emptyList(),
    )

    @Test
    fun vm_withNothingSaved_startsAtTheTop_evenWithACard() {
        // A concert opened for the first time has no stored position: the top, not the first card. Found on the device
        // pass: the host passed "" for "nothing saved", which A94 reads as the first intermission. TEETH: default
        // initialSongId back to "" and this lands on 2.
        assertEquals(0, StageViewModel(bundleWithCards()).state.value.current)
    }

    @Test
    fun vm_seedsTheSecondCard_fromItsStoredPosition() {
        val vm = StageViewModel(bundleWithCards(), initialSongId = "", initialPageInSong = 1)
        assertEquals(4, vm.state.value.current)
        assertEquals("" to 1, positionAt(vm.state.value, vm.state.value.current))
    }

    @Test
    fun vm_identitySwitchOnACard_keepsThatCard() {
        // T137 lands a SONG on its first page after a switch; a card has no song id, so before A94 it went to the top.
        // TEETH: resolve the switch with ("", 0) (the first card) and this lands on 2, not 4.
        val vm = StageViewModel(bundleWithCards(), initialSongId = "", initialPageInSong = 1)
        vm.setIdentity("someone")
        assertEquals(4, vm.state.value.current)
    }
}
