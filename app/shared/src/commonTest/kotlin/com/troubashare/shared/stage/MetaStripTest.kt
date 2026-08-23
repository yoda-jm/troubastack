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

    @Test
    fun tempoGlyphFollowsTheMetre() {
        // A35: the beat-note glyph names what the tempo counts — ♩ simple, ♩. compound, ♪ irregular.
        assertEquals("♩=120", metaStripText("", "", 120, "4/4"))
        assertEquals("♩=120", metaStripText("", "", 120, ""), "unset metre ⇒ 4/4 ⇒ quarter (pre-T86 bundles)")
        assertEquals("♩.=120", metaStripText("", "", 120, "6/8"))
        assertEquals("♩.=120", metaStripText("", "", 120, "12/8"))
        assertEquals("♪=120", metaStripText("", "", 120, "3+4/8"))
    }
}
