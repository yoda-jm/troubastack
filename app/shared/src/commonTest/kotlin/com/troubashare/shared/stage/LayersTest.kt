package com.troubashare.shared.stage

import com.troubashare.shared.bundle.BakedSong
import com.troubashare.shared.bundle.ConcertBundle
import com.troubashare.shared.bundle.LayerImage
import com.troubashare.shared.bundle.LoadResult
import com.troubashare.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals

/** N10 — the Layers dialog is per-song: [songLayers] scopes to the current song, and [layerLabel]
 *  gives a friendlier interim name (prettified role, else id) until real names ride the bundle (T53). */
class LayersTest {

    private fun layer(id: String, role: String = "", mandatory: Boolean = false) =
        LayerImage(layerId = id, imageRef = "$id.png", roleTag = role, mandatory = mandatory)

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
    fun layerLabel_prettifiesRole_elseId() {
        assertEquals("Conductor", layerLabel(LayerInfo("la2", mandatory = true, roleTag = "conductor")))
        assertEquals("Guitar", layerLabel(LayerInfo("lb1", mandatory = false, roleTag = "guitar")))
        assertEquals("la1", layerLabel(LayerInfo("la1", mandatory = false, roleTag = ""))) // untagged → id (interim, T53 names it)
    }
}
