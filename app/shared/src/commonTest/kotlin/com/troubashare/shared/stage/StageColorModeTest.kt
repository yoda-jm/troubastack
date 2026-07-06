package com.troubashare.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals

/** A10 — night-mode cycling + persistence round-trip (pure logic; the ColorFilter is draw-time). */
class StageColorModeTest {

    @Test
    fun cyclesNormalNight() {
        assertEquals(StageColorMode.NIGHT, StageColorMode.NORMAL.next())
        assertEquals(StageColorMode.NORMAL, StageColorMode.NIGHT.next())
    }

    @Test
    fun persistenceRoundTrip() {
        for (m in StageColorMode.entries) {
            assertEquals(m, StageColorMode.parse(m.name))
        }
    }

    @Test
    fun parseUnknownOrNull_defaultsNormal() {
        assertEquals(StageColorMode.NORMAL, StageColorMode.parse(null))
        assertEquals(StageColorMode.NORMAL, StageColorMode.parse(""))
        assertEquals(StageColorMode.NORMAL, StageColorMode.parse("sepia"))
    }
}
