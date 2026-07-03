package com.troubashare.shared.stage

import com.troubashare.shared.bundle.BakedSong
import com.troubashare.shared.bundle.BundleIssue
import com.troubashare.shared.bundle.ConcertBundle
import com.troubashare.shared.bundle.LayerImage
import com.troubashare.shared.bundle.LoadResult
import com.troubashare.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** Build a Loaded result with the given songs×pages and optional per-page overlays. */
private fun loaded(
    songs: Int = 2,
    pagesPerSong: Int = 3,
    overlays: List<LayerImage> = emptyList(),
    issues: List<BundleIssue> = emptyList(),
): LoadResult.Loaded {
    val bundle = ConcertBundle(
        concertId = "c1",
        songs = (1..songs).map { s ->
            BakedSong(
                songId = "song-$s",
                pages = (1..pagesPerSong).map { p ->
                    PageImages(pageRasterRef = "blobs/s$s-p$p.png", overlays = overlays)
                },
            )
        },
    )
    return LoadResult.Loaded(bundle, issues)
}

class StageViewModelTest {

    @Test
    fun navigation_clampsAtBothEnds() {
        val vm = StageViewModel(loaded(songs = 1, pagesPerSong = 3))
        assertEquals(0, vm.state.value.current)
        vm.previous() // already at start
        assertEquals(0, vm.state.value.current)
        vm.next(); vm.next(); vm.next(); vm.next() // past the end
        assertEquals(2, vm.state.value.current, "clamps to last page (index 2 of 3)")
        vm.goToPage(999)
        assertEquals(2, vm.state.value.current)
        vm.goToPage(-5)
        assertEquals(0, vm.state.value.current)
    }

    @Test
    fun songJump_landsOnFirstPageOfSong() {
        val vm = StageViewModel(loaded(songs = 2, pagesPerSong = 3)) // pages 0..2 = song1, 3..5 = song2
        vm.goToSong(1)
        assertEquals(3, vm.state.value.current)
        assertEquals(1, vm.state.value.currentSong)
        vm.goToSong(99) // out of range → ignored
        assertEquals(3, vm.state.value.current)
    }

    @Test
    fun emptyBundle_isCalmEmptyState_noThrow() {
        val vm = StageViewModel(loaded(songs = 0, pagesPerSong = 0))
        assertTrue(vm.state.value.pages.isEmpty())
        assertNull(vm.state.value.failure)
        vm.next(); vm.previous(); vm.goToPage(3); vm.goToSong(0) // all no-ops, no crash
        assertEquals(0, vm.state.value.current)
    }

    @Test
    fun failedLoad_becomesFailureState() {
        val vm = StageViewModel(LoadResult.Failed("bundle.json is missing"))
        assertEquals("bundle.json is missing", vm.state.value.failure)
        assertTrue(vm.state.value.pages.isEmpty())
    }

    @Test
    fun rasterIssue_marksOnlyThatPageUnavailable_restReady() {
        // Flag the raster of song-1 page index 1 (global page 1) as missing.
        val issue = BundleIssue("song-1", 1, "blobs/s1-p2.png", BundleIssue.Kind.MISSING_BLOB)
        val vm = StageViewModel(loaded(songs = 1, pagesPerSong = 3, issues = listOf(issue)))
        val pages = vm.state.value.pages
        assertEquals(PageStatus.READY, pages[0].status)
        assertEquals(PageStatus.UNAVAILABLE, pages[1].status)
        assertEquals(PageStatus.READY, pages[2].status)
    }

    @Test
    fun mandatoryLayer_cannotBeToggledOff() {
        val overlays = listOf(
            LayerImage(layerId = "marks", imageRef = "m.png", order = 0, mandatory = true),
            LayerImage(layerId = "notes", imageRef = "n.png", order = 1),
        )
        val vm = StageViewModel(loaded(songs = 1, pagesPerSong = 1, overlays = overlays))
        assertTrue("marks" in vm.state.value.visibleLayers)
        vm.setLayerVisible("marks", false) // ignored — mandatory
        assertTrue("marks" in vm.state.value.visibleLayers)

        // Non-mandatory toggles freely and changes only the visible set.
        vm.setLayerVisible("notes", false)
        assertFalse("notes" in vm.state.value.visibleLayers)
        vm.setLayerVisible("notes", true)
        assertTrue("notes" in vm.state.value.visibleLayers)
    }

    @Test
    fun roleTag_defaultVisibilityRule() {
        val mandatory = LayerInfo("marks", mandatory = true, roleTag = "")
        val untagged = LayerInfo("all", mandatory = false, roleTag = "")
        val conductor = LayerInfo("cond", mandatory = false, roleTag = "conductor")

        // mandatory ⇒ always on (even when its roleTag would mismatch is moot — mandatory wins)
        assertTrue(defaultVisible(mandatory, role = "violin"))
        // empty roleTag ⇒ on regardless of role
        assertTrue(defaultVisible(untagged, role = "violin"))
        // non-empty roleTag ⇒ on only when it matches the local role
        assertFalse(defaultVisible(conductor, role = "violin"))
        assertTrue(defaultVisible(conductor, role = "conductor"))
    }

    @Test
    fun setRole_reseedsDefaultVisibility() {
        val overlays = listOf(
            LayerImage(layerId = "marks", imageRef = "m.png", order = 0, mandatory = true),
            LayerImage(layerId = "cond", imageRef = "c.png", order = 1, roleTag = "conductor"),
        )
        val vm = StageViewModel(loaded(songs = 1, pagesPerSong = 1, overlays = overlays), role = "")
        assertFalse("cond" in vm.state.value.visibleLayers, "conductor layer hidden by default for empty role")
        vm.setRole("conductor")
        assertTrue("cond" in vm.state.value.visibleLayers)
        assertTrue("marks" in vm.state.value.visibleLayers, "mandatory stays on across role change")
    }

    @Test
    fun fitMode_toggles() {
        val vm = StageViewModel(loaded(songs = 1, pagesPerSong = 1))
        assertEquals(FitMode.FIT_PAGE, vm.state.value.fitMode)
        vm.toggleFit()
        assertEquals(FitMode.FIT_WIDTH, vm.state.value.fitMode)
        vm.toggleFit()
        assertEquals(FitMode.FIT_PAGE, vm.state.value.fitMode)
    }
}
