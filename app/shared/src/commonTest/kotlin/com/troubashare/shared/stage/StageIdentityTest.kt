package com.troubashare.shared.stage

import com.troubashare.shared.bundle.BakedSong
import com.troubashare.shared.bundle.BundleMember
import com.troubashare.shared.bundle.ConcertBundle
import com.troubashare.shared.bundle.LayerImage
import com.troubashare.shared.bundle.LoadResult
import com.troubashare.shared.bundle.MemberCues
import com.troubashare.shared.bundle.PageImages
import com.troubashare.shared.bundle.SongCue
import kotlin.test.Test
import kotlin.test.assertEquals

/** P205 Stage 3a — view-time identity: roster in state, member_cues by identity (field-10 fallback),
 *  and the default-visibility precedence (mandatory > mine > others'-off > default_on∧role > role). */
class StageIdentityTest {

    private val MARIE = "m-marie"
    private val LEO = "m-leo"

    private fun layer(id: String, owner: String = "", role: String = "", mandatory: Boolean = false, defaultOn: Boolean? = null) =
        LayerImage(layerId = id, imageRef = "$id.png", roleTag = role, mandatory = mandatory, owner = owner, defaultOn = defaultOn)

    private fun bundle() = ConcertBundle(
        concertId = "c1",
        roster = listOf(
            BundleMember(memberId = MARIE, displayName = "Marie", role = "admin"),
            BundleMember(memberId = LEO, displayName = "Leo", role = "member"),
        ),
        songs = listOf(
            BakedSong(
                songId = "a",
                pages = listOf(PageImages(pageRasterRef = "a.png", overlays = listOf(
                    layer("cond", role = "conductor", mandatory = true), // mandatory shared
                    layer("mine-marie", owner = MARIE),                   // Marie's personal
                    layer("mine-leo", owner = LEO),                       // Leo's personal
                    layer("form", defaultOn = false),                     // shared but baked OFF
                    layer("guitar", role = "guitar"),                     // role-scoped, no default_on
                ))),
                cues = emptyList(), // band-wide bundle: field-10 empty…
                memberCues = listOf(                                     // …cues ride per member
                    MemberCues(memberId = MARIE, cues = listOf(SongCue(icon = "mic"))),
                    MemberCues(memberId = LEO, cues = listOf(SongCue(icon = "tambourine"))),
                ),
            ),
        ),
    )

    private fun state(identity: String) =
        StageViewModel(LoadResult.Loaded(bundle(), emptyList()), identity = identity).state.value

    @Test
    fun defaultVisible_precedence() {
        val layers = state(MARIE).layers
        fun vis(id: String, identity: String) =
            defaultVisible(layers.first { it.layerId == id }, role = "", identity = identity)

        // mandatory: always on, for anyone.
        assertEquals(true, vis("cond", ""))
        // my personal layer on for me; another member's personal layer NOT for me.
        assertEquals(true, vis("mine-marie", MARIE))
        assertEquals(false, vis("mine-marie", LEO))
        assertEquals(false, vis("mine-leo", MARIE))
        // shared layer baked OFF stays off even though its roleTag is empty (default_on overrides legacy).
        assertEquals(false, vis("form", MARIE))
        // role-scoped shared layer: off for the empty role, on when the role matches.
        assertEquals(false, vis("guitar", MARIE))
        assertEquals(true, defaultVisible(layers.first { it.layerId == "guitar" }, role = "guitar", identity = MARIE))
    }

    @Test
    fun identity_seedsMyLayers_andMyCues() {
        val marie = state(MARIE)
        assertEquals(listOf(MARIE, LEO), marie.roster.map { it.memberId })
        assertEquals(setOf("cond", "mine-marie"), marie.visibleFor("a"))
        assertEquals(listOf("mic"), marie.songs.single().cues.map { it.icon })

        val leo = state(LEO)
        assertEquals(setOf("cond", "mine-leo"), leo.visibleFor("a"))
        assertEquals(listOf("tambourine"), leo.songs.single().cues.map { it.icon })

        // Anonymous: only mandatory rides; no personal cues (field-10 is empty here).
        val anon = state("")
        assertEquals(setOf("cond"), anon.visibleFor("a"))
        assertEquals(emptyList(), anon.songs.single().cues)
    }

    @Test
    fun setIdentity_reseedsCuesAndVisibility() {
        val vm = StageViewModel(LoadResult.Loaded(bundle(), emptyList()), identity = MARIE)
        vm.setIdentity(LEO)
        val s = vm.state.value
        assertEquals(LEO, s.identity)
        assertEquals(setOf("cond", "mine-leo"), s.visibleFor("a"))
        assertEquals(listOf("tambourine"), s.songs.single().cues.map { it.icon })
    }

    @Test
    fun cuesForIdentity_fallsBackToField10_forOldBundles() {
        // An old / -mine bake: no member_cues, cues in field 10 → shown regardless of identity.
        val old = BakedSong(songId = "s", cues = listOf(SongCue(icon = "guitar-electric", color = "#e11d48")))
        assertEquals(listOf("guitar-electric"), cuesForIdentity(old, MARIE).map { it.icon })
        assertEquals(listOf("guitar-electric"), cuesForIdentity(old, "").map { it.icon })
    }
}
