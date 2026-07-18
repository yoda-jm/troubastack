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
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** P205 Stage 3a/3b — view-time identity: roster in state, member_cues by identity (field-10 fallback),
 *  default-visibility precedence, and Stage-3b filtering (other members' personal layers dropped). */
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

    // Build a LayerInfo directly so the precedence test is independent of Stage-3b filtering (which
    // would remove another member's layer from state.layers before we could assert on it).
    private fun li(id: String, owner: String = "", role: String = "", mandatory: Boolean = false, defaultOn: Boolean? = null) =
        LayerInfo(id, mandatory = mandatory, roleTag = role, owner = owner, defaultOn = defaultOn)

    @Test
    fun defaultVisible_precedence() {
        // mandatory: always on, for anyone.
        assertEquals(true, defaultVisible(li("cond", role = "conductor", mandatory = true), role = "", identity = ""))
        // my personal layer on for me; another member's personal layer NOT for me.
        assertEquals(true, defaultVisible(li("mm", owner = MARIE), role = "", identity = MARIE))
        assertEquals(false, defaultVisible(li("mm", owner = MARIE), role = "", identity = LEO))
        assertEquals(false, defaultVisible(li("ml", owner = LEO), role = "", identity = MARIE))
        // shared layer baked OFF stays off even though its roleTag is empty (default_on overrides legacy).
        assertEquals(false, defaultVisible(li("form", defaultOn = false), role = "", identity = MARIE))
        // role-scoped shared layer: off for the empty role, on when the role matches.
        assertEquals(false, defaultVisible(li("guitar", role = "guitar"), role = "", identity = MARIE))
        assertEquals(true, defaultVisible(li("guitar", role = "guitar"), role = "guitar", identity = MARIE))
    }

    @Test
    fun stage3b_dropsOtherMembersLayers_fromModelAndComposite() {
        val marie = state(MARIE)
        // Leo's personal layer is gone entirely — not listed, not composited; Marie's is kept.
        assertNull(marie.layers.find { it.layerId == "mine-leo" })
        assertTrue(marie.layers.any { it.layerId == "mine-marie" })
        val overlayIds = marie.pages.first().overlays.map { it.layerId }
        assertTrue("mine-leo" !in overlayIds, "Leo's overlay must not composite for Marie")
        assertTrue("mine-marie" in overlayIds)
        // Anonymous sees NEITHER personal layer (only shared ones).
        val anon = state("")
        assertTrue(anon.layers.none { it.layerId == "mine-marie" || it.layerId == "mine-leo" })
        assertTrue(anon.layers.any { it.layerId == "cond" }) // shared layers remain
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
