package com.troubashare.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** A08 — the setlist-metadata strip formatter: omit empties, tempo 0 omitted, all-empty → null. */
class MetaStripTest {

    @Test
    fun allEmpty_isNull() {
        assertNull(metaStripText("", "", 0))
        assertNull(metaStripText("   ", "  ", 0)) // blanks count as empty
    }

    @Test
    fun singleFields() {
        assertEquals("Acoustic intro, capo 2.", metaStripText("Acoustic intro, capo 2.", "", 0))
        assertEquals("Em", metaStripText("", "Em", 0))
        assertEquals("♩=98", metaStripText("", "", 98))
    }

    @Test
    fun tempoZeroIsOmitted() {
        assertEquals("Acoustic intro, capo 2.  ·  Em", metaStripText("Acoustic intro, capo 2.", "Em", 0))
    }

    @Test
    fun allThree_joinedInOrder() {
        assertEquals("intro  ·  Em  ·  ♩=120", metaStripText("intro", "Em", 120))
    }

    @Test
    fun negativeTempoOmitted() {
        assertEquals("Em", metaStripText("", "Em", -1))
    }
}
