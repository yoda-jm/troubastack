package com.troubastack.shared.home

import com.troubastack.shared.distribution.Availability
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A43 — the Home landing must not claim currency it can't back up. Three states used to render
 * "Up to date": a deleted concert, an EMPTY device, and a manifest that couldn't load. The load-bearing
 * cases here are the empty device (walk on stage with no charts) and the unreadable manifest (it didn't
 * check). Pure state-mapping — no device, per the spec. `nameOf` is trivial here; wording is covered too.
 */
class LandingUpdateTest {

    private val name: (String) -> String = { id -> "Concert $id" }

    @Test
    fun emptyDevice_nonEmptyManifest_offersDownload() {
        // The band lists two concerts and NONE are installed — the pre-gig failure. Must offer download,
        // never a reassurance.
        val r = landingUpdate(
            manifestSize = 2,
            offered = emptyList(),
            newlyAvailable = listOf(Availability.NewlyAvailable("a"), Availability.NewlyAvailable("b")),
            nameOf = name,
        )
        assertTrue(r.status is UpdateStatus.Available, "empty device must offer, was ${r.status}")
        assertEquals("Download", (r.status as UpdateStatus.Available).action)
        assertEquals("2 concerts to download", r.status.summary)
        assertEquals(2, r.offers.size)
    }

    @Test
    fun emptyDevice_oneConcert_namesIt() {
        val r = landingUpdate(1, emptyList(), listOf(Availability.NewlyAvailable("solo")), name)
        assertEquals(UpdateStatus.Available("Concert solo — not on this device", action = "Download"), r.status)
    }

    @Test
    fun manifestUnavailable_makesNoCurrencyClaim() {
        // Couldn't check ⇒ silence, NEVER a green light. The bug returned UpToDate here.
        val r = landingUpdate(manifestSize = null, offered = emptyList(), newlyAvailable = emptyList(), nameOf = name)
        assertEquals(UpdateStatus.Hidden, r.status)
        assertTrue(r.status !is UpdateStatus.UpToDate, "an unchecked manifest must not read as current")
    }

    @Test
    fun oneDeleted_othersCurrent_staysQuiet_notNagware() {
        // 3 listed, 1 not on device (deliberately deleted), the other 2 installed & current. This must
        // stay quiet — re-offering the one you deleted while you hold the rest is nagware. Teeth: a
        // blanket "any NewlyAvailable ⇒ offer" would turn this into Available.
        val r = landingUpdate(
            manifestSize = 3,
            offered = emptyList(),
            newlyAvailable = listOf(Availability.NewlyAvailable("deleted")),
            nameOf = name,
        )
        assertEquals(UpdateStatus.UpToDate, r.status)
        assertTrue(r.offers.isEmpty(), "a deleted-one set must not surface a download on the landing")
    }

    @Test
    fun staleInstalled_offersUpdate_a39Unregressed() {
        val r = landingUpdate(
            manifestSize = 2,
            offered = listOf(Availability.UpdateOffered("x", localRev = 1uL, serverRev = 2uL)),
            newlyAvailable = emptyList(),
            nameOf = name,
        )
        assertEquals(UpdateStatus.Available("Concert x — new version", action = "Update"), r.status)
        assertEquals(1, r.offers.size)
    }

    @Test
    fun emptyManifest_bandHasNothing_isQuiet() {
        // Band lists no concerts and nothing installed: not the empty-download case (nothing to offer).
        val r = landingUpdate(manifestSize = 0, offered = emptyList(), newlyAvailable = emptyList(), nameOf = name)
        assertEquals(UpdateStatus.UpToDate, r.status)
    }
}
