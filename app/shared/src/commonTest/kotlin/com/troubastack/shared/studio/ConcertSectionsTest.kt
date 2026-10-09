package com.troubastack.shared.studio

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** A83 §6 — the Concerts tab's grouping and order. */
class ConcertSectionsTest {

    private data class C(val bandId: String, val band: String, val name: String, val date: String = "")

    private fun group(list: List<C>) = groupConcerts(list, { it.bandId }, { it.band }, { it.name }, { it.date })

    @Test
    fun bandsInNaturalOrder_concertsGroupedFromInterleavedInput() {
        // Interleaved across bands, as the server returns them band by band but mixed here on purpose:
        // a function that only SORTS (and never groups) cannot pass.
        val input = listOf(
            C("b10", "Band 10", "Late gig", "2026-05-01"),
            C("b2", "band 2", "Session 10"),
            C("b10", "Band 10", "Early gig", "2026-01-01"),
            C("b2", "band 2", "Spring show", "2026-03-01"),
            C("b2", "band 2", "Session 2"),
            C("b2", "band 2", "Autumn show", "2026-09-12"),
        )
        val sections = group(input)
        assertEquals(listOf("band 2", "Band 10"), sections.map { it.bandName }, "natural, case ignored: 2 before 10")
        assertTrue(sections.all { it.showHeader })
        assertEquals(
            listOf("Autumn show", "Spring show", "Session 2", "Session 10"),
            sections[0].concerts.map { it.name },
            "dated newest first (2026-09-12 before 2026-03-01), both before undated; undated Session 2 before Session 10",
        )
        assertEquals(listOf("Late gig", "Early gig"), sections[1].concerts.map { it.name })
    }

    @Test
    fun exactlyOneBand_oneSectionWithoutHeader() {
        val sections = group(listOf(C("b1", "The Band", "B", "2026-01-02"), C("b1", "The Band", "A")))
        assertEquals(1, sections.size)
        assertFalse(sections[0].showHeader)
        assertEquals(listOf("B", "A"), sections[0].concerts.map { it.name })
    }

    @Test
    fun noConcerts_noSections() {
        assertEquals(emptyList(), group(emptyList()))
    }

    @Test
    fun naturalCompare_ignoresCaseAndAccents_andComparesNumbersByValue() {
        assertTrue(naturalCompare("Session 2", "Session 10") < 0)
        assertTrue(naturalCompare("band 2", "Band 10") < 0)
        assertTrue(naturalCompare("Électro", "electro 2") < 0)
        assertTrue(naturalCompare("Zoé", "zoe") != 0, "a total order: equal folds still differ by the raw string")
        assertEquals(0, naturalCompare("Same", "Same"))
    }
}
