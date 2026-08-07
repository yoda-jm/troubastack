package com.troubashare.shared.stage

import androidx.compose.ui.graphics.Color
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertSame
import kotlin.test.assertTrue

/** T50/A20 — the pure cue logic: glyph resolution (unknown → `note` fallback) and tint parsing. */
class CueTest {

    @Test
    fun cueGlyph_resolvesKnownAndFallsBackToNote() {
        // Every curated id resolves to its own glyph.
        for (id in listOf("mic", "guitar-electric", "bass", "cajon", "note")) {
            assertSame(CUE_GLYPHS.getValue(id), cueGlyph(id), "known id $id resolves to itself")
        }
        // Unknown / future ids resolve to the pinned `note` fallback — never null, never a crash.
        assertSame(CUE_GLYPHS.getValue(CUE_FALLBACK_ID), cueGlyph("triangle-of-doom"))
        assertSame(CUE_GLYPHS.getValue(CUE_FALLBACK_ID), cueGlyph(""))
    }

    @Test
    fun cueGlyphSet_coversTheCuratedContract() {
        // The curated ids (T50; + `warning` for the T51 stamp set), with the fallback present.
        assertEquals(19, CUE_GLYPHS.size)
        assertTrue(CUE_FALLBACK_ID in CUE_GLYPHS)
        for (id in listOf("guitar-electric", "guitar-acoustic", "guitar-classical", "bass", "ukulele",
                "autoharp", "melodica", "keys", "cajon", "bongo", "djembe", "guiro", "cuica",
                "shaker", "egg-shaker", "tambourine", "mic", "warning", "note")) {
            assertTrue(id in CUE_GLYPHS, "curated id present: $id")
        }
    }

    @Test
    fun parseCueColor_hexOrNeutral() {
        val neutral = Color.White
        assertEquals(Color(0xFFFF0000), parseCueColor("#ff0000", neutral))
        assertEquals(Color(0xFF00FF00), parseCueColor("00ff00", neutral))       // no leading #
        assertEquals(Color(0xFF123ABC), parseCueColor("  #123abc ", neutral))   // trimmed
        // Empty / malformed → the neutral (untinted) fallback, never a crash.
        assertEquals(neutral, parseCueColor("", neutral))
        assertEquals(neutral, parseCueColor("#fff", neutral))                   // wrong length
        assertEquals(neutral, parseCueColor("#gggggg", neutral))                // non-hex
    }
}
