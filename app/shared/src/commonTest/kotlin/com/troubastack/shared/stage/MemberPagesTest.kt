package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.BundleMember
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.MemberPages
import com.troubastack.shared.bundle.PageImages
import kotlinx.serialization.json.Json
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * T137 Stage 3 — per-member reading SEQUENCES. `BakedSong.pages` is a shared pool; each member reads its
 * own ordered indices into it (member_pages), resolving identity → "" default → all-pages-in-order.
 */
class MemberPagesTest {

    private val MARIE = "m-marie"
    private val LEO = "m-leo"

    private fun page(n: Int) = PageImages(pageRasterRef = "p$n.png", rasterHash = "h$n")

    /** One song, a 4-page pool. Marie reads [1,0] (her file, and note: order preserved, ≠ pool order),
     *  Leo reads [2,3], and the "" default reads [0,1]. */
    private fun divergentSong() = BakedSong(
        songId = "s",
        title = "Song",
        pages = listOf(page(0), page(1), page(2), page(3)),
        memberPages = listOf(
            MemberPages(memberId = "", page = listOf(0, 1)),
            MemberPages(memberId = MARIE, page = listOf(1, 0)),
            MemberPages(memberId = LEO, page = listOf(2, 3)),
        ),
    )

    /** A pre-T137 / undivergent song: a plain pool, no member_pages. */
    private fun plainSong(id: String = "s", n: Int = 3) =
        BakedSong(songId = id, title = id, pages = (0 until n).map { page(it) })

    private fun bundleOf(vararg songs: BakedSong) = ConcertBundle(
        concertId = "c",
        roster = listOf(
            BundleMember(memberId = MARIE, displayName = "Marie", role = "member"),
            BundleMember(memberId = LEO, displayName = "Leo", role = "member"),
        ),
        songs = songs.toList(),
    )

    private fun refsFor(identity: String, vararg songs: BakedSong): List<String> =
        StageViewModel(LoadResult.Loaded(bundleOf(*songs), emptyList()), identity = identity)
            .state.value.pages.map { it.rasterRef }

    // ---- resolvePageSequence (the pure seam) ----

    @Test
    fun resolve_identity_then_default_then_all() {
        val s = divergentSong()
        assertEquals(listOf(1, 0), resolvePageSequence(s, MARIE)) // own entry, order preserved
        assertEquals(listOf(2, 3), resolvePageSequence(s, LEO))
        assertEquals(listOf(0, 1), resolvePageSequence(s, ""))            // "" matches the default entry
        assertEquals(listOf(0, 1), resolvePageSequence(s, "m-unknown"))    // no own entry → "" default
        assertEquals(listOf(0, 1, 2), resolvePageSequence(plainSong(n = 3), MARIE)) // no member_pages → all
    }

    @Test
    fun resolve_drops_out_of_range_indices() {
        val s = BakedSong(
            songId = "s", pages = listOf(page(0), page(1)),
            memberPages = listOf(MemberPages(memberId = MARIE, page = listOf(1, 5, 0, -1))),
        )
        assertEquals(listOf(1, 0), resolvePageSequence(s, MARIE)) // 5 and -1 dropped, never crashes
    }

    // ---- buildLoaded: two members get different sequences from ONE bundle ----

    @Test
    fun twoMembersDifferentSequencesFromOneBundle() {
        val s = divergentSong()
        assertEquals(listOf("p1.png", "p0.png"), refsFor(MARIE, s))
        assertEquals(listOf("p2.png", "p3.png"), refsFor(LEO, s))
    }

    @Test
    fun noSelectionAndAnonymousReadTheDefault() {
        val s = divergentSong()
        assertEquals(listOf("p0.png", "p1.png"), refsFor("", s))          // anonymous
        assertEquals(listOf("p0.png", "p1.png"), refsFor("m-unknown", s)) // logged-in, no selection
    }

    @Test
    fun bundleWithoutMemberPagesPlaysUnchanged() {
        // Acceptance: an undivergent / old bundle reads the whole pool in order, for anyone.
        val s = plainSong(n = 3)
        assertEquals(listOf("p0.png", "p1.png", "p2.png"), refsFor(MARIE, s))
        assertEquals(listOf("p0.png", "p1.png", "p2.png"), refsFor("", s))
    }

    @Test
    fun pageInSongCountsWithinTheResolvedSequence() {
        // Marie reads [1,0] → pageInSong is 0,1 within HER sequence (not the pool index).
        val pages = StageViewModel(LoadResult.Loaded(bundleOf(divergentSong()), emptyList()), identity = MARIE)
            .state.value.pages
        assertEquals(listOf(0, 1), pages.map { it.pageInSong })
        assertEquals(listOf("p1.png", "p0.png"), pages.map { it.rasterRef })
    }

    @Test
    fun songStartsDeriveFromResolvedSequence() {
        // Song A: Leo reads 2 pages; Song B: a plain 3-page song. Leo's second song starts at global 2.
        val a = divergentSong()
        val b = plainSong(id = "b", n = 3)
        val st = StageViewModel(LoadResult.Loaded(bundleOf(a, b), emptyList()), identity = LEO).state.value
        assertEquals(listOf(0, 2), st.songs.map { it.firstPage })
        assertEquals(5, st.pages.size) // 2 (Leo's A) + 3 (B)
    }

    // ---- the runtime link: the baker-emitted JSON shape deserializes into the model ----

    @Test
    fun bakerJsonShape_deserializesAndResolves() {
        // The exact keys the Go baker writes (bundle_gen.go json tags: memberPages / memberId / page),
        // decoded through the SAME serializer BundleLoader uses. Guards against a silent key mismatch that
        // would drop member_pages at runtime while the in-memory tests above kept passing.
        val jsonText = """
            {"concertId":"c","songs":[
              {"songId":"s","pages":[
                {"pageRasterRef":"p0.png"},{"pageRasterRef":"p1.png"},{"pageRasterRef":"p2.png"}],
               "memberPages":[
                {"memberId":"","page":[0]},
                {"memberId":"m-leo","page":[2,1]}]}]}
        """.trimIndent()
        val bundle = Json { ignoreUnknownKeys = true }.decodeFromString(ConcertBundle.serializer(), jsonText)
        val song = bundle.songs.single()
        assertEquals(2, song.memberPages.size)
        assertEquals(listOf(2, 1), resolvePageSequence(song, "m-leo"))
        assertEquals(listOf(0), resolvePageSequence(song, ""))
    }

    // ---- identity change invalidates the position (per-identity sequences don't share an index) ----

    @Test
    fun setIdentity_landsAtTheSongsFirstPageInTheNewSequence() {
        val vm = StageViewModel(LoadResult.Loaded(bundleOf(divergentSong()), emptyList()), identity = MARIE)
        vm.goToPage(1) // Marie's page-in-song 1 (pool page 0)
        assertEquals(1, vm.state.value.current)
        vm.setIdentity(LEO)
        // Leo's sequence is unrelated; the position invalidates to the song's first page, not a stale index.
        assertEquals(0, vm.state.value.current)
        assertEquals("p2.png", vm.state.value.currentPage?.rasterRef)
    }
}
