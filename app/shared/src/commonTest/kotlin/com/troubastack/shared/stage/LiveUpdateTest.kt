package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LayerImage
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

// A bundle where page rasters carry explicit hashes so R10 remapping can be exercised.
// `hashes[songIndex][pageIndex]` = the raster hash for that page (empty ⇒ no hash).
private fun hashedBundle(hashes: List<List<String>>): LoadResult.Loaded {
    val bundle = ConcertBundle(
        concertId = "c1",
        songs = hashes.mapIndexed { s, pageHashes ->
            BakedSong(
                songId = "song-${s + 1}",
                pages = pageHashes.mapIndexed { p, h ->
                    PageImages(pageRasterRef = "blobs/s${s + 1}-p${p + 1}.png", rasterHash = h)
                },
            )
        },
    )
    return LoadResult.Loaded(bundle, emptyList())
}

class LiveUpdateTest {

    // ---- The transient toggle (I13) ----

    @Test
    fun autoUpdate_startsOff_andIsTransientPerEntry() {
        val vm = StageViewModel(hashedBundle(listOf(listOf("a", "b"))))
        assertFalse(vm.state.value.autoUpdate, "a fresh Stage entry must start with auto-update OFF")
        vm.setAutoUpdate(true)
        assertTrue(vm.state.value.autoUpdate)
        // "Leaving Stage" = the ViewModel is discarded; a NEW one is off again (nothing persisted).
        val reentered = StageViewModel(hashedBundle(listOf(listOf("a", "b"))))
        assertFalse(reentered.state.value.autoUpdate, "re-entering Stage must reset auto-update to OFF")
    }

    // ---- R10: viewport-preserving swap ----

    @Test
    fun applyUpdate_unchangedCurrentPage_staysExactlyThere() {
        // One song, 3 pages; sit on page index 1 (hash "b").
        val vm = StageViewModel(hashedBundle(listOf(listOf("a", "b", "c"))))
        vm.setAutoUpdate(true)
        vm.goToPage(1)
        // Re-bake: page "b" is UNCHANGED (same hash) but a page was inserted before it.
        vm.applyUpdate(hashedBundle(listOf(listOf("a", "a2", "b", "c"))))
        assertEquals(2, vm.state.value.current, "the unchanged page (hash b) moved to index 2; follow it")
        assertTrue(vm.state.value.autoUpdate, "auto-update survives the swap")
    }

    // ---- T143 §3: an applied auto-update says a word, without moving the page ----

    @Test
    fun applyUpdate_setsASelfDismissingNotice_withoutMovingThePage() {
        val vm = StageViewModel(hashedBundle(listOf(listOf("a", "b", "c"))))
        vm.setAutoUpdate(true)
        vm.goToPage(1) // sit on page "b"
        assertEquals(null, vm.state.value.updateNotice, "no notice before an update arrives")
        // A re-bake at a named rev; page "b" is unchanged, only inserted before.
        val next = LoadResult.Loaded(
            ConcertBundle(
                concertId = "c1",
                concertRev = 5uL,
                songs = listOf(
                    BakedSong(
                        songId = "song-1",
                        pages = listOf("a", "a2", "b", "c").mapIndexed { p, h ->
                            PageImages(pageRasterRef = "blobs/s1-p${p + 1}.png", rasterHash = h)
                        },
                    ),
                ),
            ),
            emptyList(),
        )
        vm.applyUpdate(next)
        assertEquals("Updated to rev 5", vm.state.value.updateNotice, "the performer is told what arrived")
        assertEquals(2, vm.state.value.current, "the notice must NOT move the page (R10 remap still holds)")
        // Self-dismissing: once the view has shown it, it clears and does not re-appear.
        vm.clearUpdateNotice()
        assertEquals(null, vm.state.value.updateNotice)
    }

    @Test
    fun applyUpdate_changedCurrentPage_keepsLogicalPosition() {
        val vm = StageViewModel(hashedBundle(listOf(listOf("a", "b", "c"))))
        vm.goToPage(1) // song-1, pageInSong 1
        // Re-bake: same structure but page 1's CONTENT changed (new hash) — no hash match.
        vm.applyUpdate(hashedBundle(listOf(listOf("a", "b2", "c"))))
        assertEquals(1, vm.state.value.current, "no hash match → same (song, pageInSong) = index 1")
    }

    @Test
    fun applyUpdate_songShrank_staysInSongNearestPage() {
        // Two songs, 3 + 2 pages; sit on song-1 page 2 (index 2, hash "c").
        val vm = StageViewModel(hashedBundle(listOf(listOf("a", "b", "c"), listOf("d", "e"))))
        vm.goToPage(2)
        // Re-bake: song-1 shrank to 2 pages, and page "c" is gone (removed) — no hash, no (song,page2).
        vm.applyUpdate(hashedBundle(listOf(listOf("a", "b"), listOf("d", "e"))))
        assertEquals(1, vm.state.value.current, "song-1 shrank; stay in song-1 at its nearest page (index 1)")
        assertEquals("song-1", vm.state.value.currentPage?.songId)
    }

    @Test
    fun applyUpdate_songVanished_clampsIntoRange() {
        val vm = StageViewModel(hashedBundle(listOf(listOf("a", "b"), listOf("c", "d"))))
        vm.goToPage(3) // song-2, last page
        // Re-bake: song-2 removed entirely; only song-1 remains (2 pages).
        vm.applyUpdate(hashedBundle(listOf(listOf("a", "b"))))
        assertTrue(vm.state.value.current in 0..1, "the old index clamps into the new, smaller range")
    }

    @Test
    fun applyUpdate_preservesFitAndLayers() {
        val vm = StageViewModel(hashedBundle(listOf(listOf("a", "b"))))
        vm.toggleFit() // FIT_PAGE → FIT_WIDTH
        val fit = vm.state.value.fitMode
        vm.applyUpdate(hashedBundle(listOf(listOf("a", "b"))))
        assertEquals(fit, vm.state.value.fitMode, "fit mode survives an auto-update swap")
    }

    // ---- A1: per-song layer overrides survive an auto-update swap ----

    private fun overlayBundle(mandatory: Boolean = false): LoadResult.Loaded {
        val notes = LayerImage(layerId = "notes", imageRef = "n.png", order = 0, mandatory = mandatory)
        val bundle = ConcertBundle(
            concertId = "c1",
            songs = listOf("song-1", "song-2").map { id ->
                BakedSong(songId = id, pages = listOf(PageImages(pageRasterRef = "$id.png", overlays = listOf(notes))))
            },
        )
        return LoadResult.Loaded(bundle, emptyList())
    }

    @Test
    fun applyUpdate_preservesPerSongLayerOverride() {
        // A1: an auto-update mid-rehearsal must not clobber a per-song layer choice.
        val vm = StageViewModel(overlayBundle())
        vm.setLayerVisible("notes", false) // hide on song-1 only (current page)
        assertFalse("notes" in vm.state.value.visibleFor("song-1"))
        vm.applyUpdate(overlayBundle())
        assertFalse("notes" in vm.state.value.visibleFor("song-1"), "song-1 override survives the swap")
        assertTrue("notes" in vm.state.value.visibleFor("song-2"), "song-2 default is unchanged")
    }

    @Test
    fun applyUpdate_layerBecomingMandatory_isForcedVisible() {
        // A18 conditional (I12): a performer legally hides an OPTIONAL layer, then a re-bake marks it
        // mandatory. After auto-update the merged kept-set still excludes it — but mandatory is ALWAYS
        // visible, enforced at READ in visibleFor, so it must show regardless of the stored override.
        val vm = StageViewModel(overlayBundle(mandatory = false))
        vm.setLayerVisible("notes", false)
        assertFalse("notes" in vm.state.value.visibleFor("song-1"))
        vm.applyUpdate(overlayBundle(mandatory = true)) // same layer, now mandatory
        assertTrue("notes" in vm.state.value.visibleFor("song-1"), "a now-mandatory layer can never stay hidden")
        assertTrue("notes" in vm.state.value.visibleFor("song-2"))
    }
}
