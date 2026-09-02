package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LayerImage
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals

/** N10 — the Layers dialog is per-song: [songLayers] scopes to the current song, and [layerLabel]
 *  gives a friendlier interim name (prettified role, else id) until real names ride the bundle (T53). */
class LayersTest {

    private fun layer(id: String, role: String = "", mandatory: Boolean = false, name: String = "") =
        LayerImage(layerId = id, imageRef = "$id.png", roleTag = role, mandatory = mandatory, name = name)

    private fun state(): StageState {
        val bundle = ConcertBundle(
            concertId = "c1",
            songs = listOf(
                BakedSong(songId = "a", pages = listOf(PageImages(pageRasterRef = "a.png", overlays = listOf(
                    layer("la1"), layer("la2", role = "conductor", mandatory = true))))),
                BakedSong(songId = "b", pages = listOf(PageImages(pageRasterRef = "b.png", overlays = listOf(
                    layer("lb1", role = "guitar"))))),
            ),
        )
        return StageViewModel(LoadResult.Loaded(bundle, emptyList())).state.value
    }

    @Test
    fun songLayers_scopesToTheCurrentSong_notTheConcert() {
        val s = state()
        // The concert-wide aggregate has all three; per-song lists only that song's.
        assertEquals(listOf("la1", "la2", "lb1"), s.layers.map { it.layerId }, "aggregate is concert-wide")
        assertEquals(listOf("la1", "la2"), songLayers(s, "a").map { it.layerId })
        assertEquals(listOf("lb1"), songLayers(s, "b").map { it.layerId })
        assertEquals(emptyList(), songLayers(s, "nope").map { it.layerId })
    }

    @Test
    fun songLayerLabels_prettifyRole_elseNumberUntagged() {
        val s = state()
        // Song "a": la1 untagged, la2=conductor. Named layers never consume a number; untagged get "Layer N".
        assertEquals(listOf("Layer 1", "Conductor"), songLayerLabels(s, "a").map { it.second })
        assertEquals(listOf("Guitar"), songLayerLabels(s, "b").map { it.second })
    }

    @Test
    fun songLayerLabels_numberUntaggedInOrder_noHashLeaks() {
        // Two untagged + one role: hashes must never appear; untagged number 1,2 across the named one.
        val bundle = ConcertBundle(
            concertId = "c",
            songs = listOf(BakedSong(songId = "s", pages = listOf(PageImages(pageRasterRef = "s.png", overlays = listOf(
                layer("L-86453186435184"),
                layer("L-conductorhash", role = "conductor"),
                layer("L-99deadbeef00"),
            ))))),
        )
        val s = StageViewModel(LoadResult.Loaded(bundle, emptyList())).state.value
        assertEquals(listOf("Layer 1", "Conductor", "Layer 2"), songLayerLabels(s, "s").map { it.second })
    }

    @Test
    fun songLayerLabels_prefersBakedName_overRoleAndNumber() {
        // T53: once the bundle carries a name, it wins over role/number and never consumes a number.
        val bundle = ConcertBundle(
            concertId = "c",
            songs = listOf(BakedSong(songId = "s", pages = listOf(PageImages(pageRasterRef = "s.png", overlays = listOf(
                layer("L-hash1", name = "Lead vocal"),
                layer("L-hash2", role = "guitar", name = "Marie's guitar"), // name beats role
                layer("L-hash3"), // no name, no role → numbered
            ))))),
        )
        val s = StageViewModel(LoadResult.Loaded(bundle, emptyList())).state.value
        assertEquals(listOf("Lead vocal", "Marie's guitar", "Layer 1"), songLayerLabels(s, "s").map { it.second })
    }
}
