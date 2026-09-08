package com.troubastack.shared.stage

import com.troubastack.shared.bundle.PageJump
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * P206 Stage 4 — the PURE jump-mark seams (hit-test, target resolution, landing offset). The spec makes
 * these the correctness core precisely because there are no instrumented tests (A58): the activation
 * decision, the layer/owner filter and the "land near the top" math are all guarded here, off-device.
 */
class JumpMarkTest {
    private fun jump(
        x0: Int, y0: Int, x1: Int, y1: Int,
        layer: String = "L", owner: String = "", target: Int = 1, anchor: Int = 0,
    ) = PageJump(
        x0Permille = x0, y0Permille = y0, x1Permille = x1, y1Permille = y1,
        targetPage = target, targetAnchorYPermille = anchor, layerId = layer, owner = owner,
    )

    private fun page(vararg jumps: PageJump) = StagePage(
        songId = "a", songName = "A", pageInSong = 0, rasterRef = "r",
        overlays = emptyList(), status = PageStatus.READY, jumps = jumps.toList(),
    )

    // ── jumpAt: activation + the §4.4 bypass guard ──

    @Test
    fun tap_inside_a_visible_mark_hits_it() {
        val j = jump(200, 200, 400, 400)
        assertEquals(j, jumpAt(page(j), 300, 300, setOf("L"), identity = ""))
    }

    @Test
    fun tap_outside_the_bbox_misses() {
        val j = jump(200, 200, 400, 400)
        assertNull(jumpAt(page(j), 500, 300, setOf("L"), ""), "right of the box")
        assertNull(jumpAt(page(j), 300, 100, setOf("L"), ""), "above the box")
    }

    @Test
    fun a_hidden_layer_is_NOT_hit_tested() {
        // §4.4: the real failure — hide the mark's layer, tap where it was, nothing happens (not merely
        // "not drawn"). A test that only checked rendering would pass while the hotspot stayed live.
        val j = jump(200, 200, 400, 400, layer = "hidden")
        assertNull(jumpAt(page(j), 300, 300, visibleLayers = setOf("other"), identity = ""))
    }

    @Test
    fun another_members_personal_mark_is_not_hit_tested() {
        val mine = jump(200, 200, 400, 400, owner = "me")
        val theirs = jump(200, 200, 400, 400, owner = "bob")
        assertEquals(mine, jumpAt(page(mine), 300, 300, setOf("L"), identity = "me"))
        assertNull(jumpAt(page(theirs), 300, 300, setOf("L"), identity = "me"), "bob's private mark")
        // shared ("") is visible to anyone, including the anonymous reader
        assertEquals(
            jump(200, 200, 400, 400, owner = ""),
            jumpAt(page(jump(200, 200, 400, 400, owner = "")), 300, 300, setOf("L"), identity = ""),
        )
    }

    @Test
    fun on_overlap_the_topmost_drawn_mark_wins() {
        val under = jump(100, 100, 900, 900, target = 1)
        val over = jump(200, 200, 400, 400, target = 7)
        // both contain (300,300); `over` is drawn last → wins
        assertEquals(7, jumpAt(page(under, over), 300, 300, setOf("L"), "")?.targetPage)
    }

    // ── jumpTargetGlobalPage: within-song resolution + A46 clamp ──

    private fun state() = StageState(
        pages = List(5) { i -> page().copy(pageInSong = i) },
        songs = listOf(
            SongInfo(songId = "a", name = "A", firstPage = 0),
            SongInfo(songId = "b", name = "B", firstPage = 3),
        ),
    )

    @Test
    fun target_resolves_within_the_songs_own_pages() {
        // song a spans global 0..2; a jump on page 0 targeting within-song page 2 → global 2
        assertEquals(2, jumpTargetGlobalPage(state(), fromGlobalPage = 0, jump(0, 0, 0, 0, target = 2)))
        // song b spans global 3..4; a jump on b targeting within-song page 1 → global 4
        assertEquals(4, jumpTargetGlobalPage(state(), fromGlobalPage = 3, jump(0, 0, 0, 0, target = 1)))
    }

    @Test
    fun target_past_the_last_page_clamps_never_dangles() {
        // a shorter re-bake removed the target: clamp to the song's last page (A46), never cross into b
        assertEquals(2, jumpTargetGlobalPage(state(), fromGlobalPage = 0, jump(0, 0, 0, 0, target = 9)))
        assertEquals(4, jumpTargetGlobalPage(state(), fromGlobalPage = 3, jump(0, 0, 0, 0, target = 9)))
    }

    // ── jumpLandOffsetPx: "maybe at the top, not the top of the page" ──

    @Test
    fun anchor_zero_lands_at_the_page_top() {
        assertEquals(0, jumpLandOffsetPx(anchorYPermille = 0, pageHeightPx = 3000, leadInPx = 200, maxScrollPx = 2000))
    }

    @Test
    fun a_mid_page_anchor_lands_with_lead_in_above_it() {
        // 0.5 * 3000 = 1500, minus 200 lead-in = 1300 (within range)
        assertEquals(1300, jumpLandOffsetPx(500, 3000, 200, 5000))
    }

    @Test
    fun an_anchor_near_the_end_clamps_and_stays_visible() {
        // 0.9 * 3000 = 2700 - 200 = 2500, but the column can only scroll 2000 → clamp, not bounce
        assertEquals(2000, jumpLandOffsetPx(900, 3000, 200, 2000))
        // a page shorter than the viewport cannot scroll at all
        assertEquals(0, jumpLandOffsetPx(900, 3000, 200, 0))
    }
}
