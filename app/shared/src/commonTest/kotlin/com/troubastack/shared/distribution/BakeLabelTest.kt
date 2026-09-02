package com.troubastack.shared.distribution

import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * A42 ② — the pure Home bake line. The load-bearing case is the "Finishing…" tail: `done == total`
 * with no song must NEVER render a frozen "N of N", which is the failure T99 exists to prevent.
 */
class BakeLabelTest {

    @Test
    fun running_withSong_namesItAndCounts() {
        assertEquals("Baking House of the Rising Sun — 3 of 25", bakeLabel(BakeProgress(state = "running", done = 3, total = 25, song = "House of the Rising Sun")))
    }

    @Test
    fun doneEqualsTotal_noSong_isFinishing_notNofN() {
        assertEquals("Finishing…", bakeLabel(BakeProgress(state = "running", done = 25, total = 25, song = "")))
        assertEquals("Finishing…", bakeLabel(BakeProgress(state = "running", done = 4, total = 4)))
    }

    @Test
    fun noSnapshotYet_degradesToBaking() {
        assertEquals("Baking…", bakeLabel(null))
        assertEquals("Baking…", bakeLabel(BakeProgress(state = "running", done = 0, total = 0)))
    }

    @Test
    fun running_noSong_butCounting_showsTheCount() {
        assertEquals("Baking 2 of 25", bakeLabel(BakeProgress(state = "running", done = 2, total = 25, song = "")))
    }
}
