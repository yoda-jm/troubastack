package com.troubastack.app

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * A48 — the Stage reading-position string seam (A46's persisted position, read at app open on data from
 * a PREVIOUS install). The load-bearing case is a MALFORMED stored value: the decode must degrade to
 * null (→ start at the top), never throw at composition time on launch. Per-test teeth-check in comments.
 */
class StagePositionRoundTripTest {

    @Test
    fun roundTrip_ordinaryIds() {
        // decode(encode(id, n)) == (id, n) for ordinary ids (song slugs, a real UUID).
        assertEquals("song-1" to 3, decodeStagePosition(encodeStagePosition("song-1", 3)))
        assertEquals("song-1" to 0, decodeStagePosition(encodeStagePosition("song-1", 0)))
        val uuid = "0b4205bb-1909-49cd-bf4e-1e9b44d0cf6e"
        assertEquals(uuid to 12, decodeStagePosition(encodeStagePosition(uuid, 12)))
    }

    @Test
    fun noSeparator_returnsNull_neverThrows() {
        // THE load-bearing case: a truncated/foreign value with no '#'. Teeth: removing the
        // `takeIf { it.size == 2 }` guard makes `[1]` throw IndexOutOfBounds here — at launch, pre-render.
        assertNull(decodeStagePosition("song-1"))
    }

    @Test
    fun nullOrEmpty_returnsNull() {
        assertNull(decodeStagePosition(null))
        assertNull(decodeStagePosition("")) // "".split('#') == [""] (size 1) → guarded out
    }

    @Test
    fun nonNumericPage_degradesToSongsFirstPage() {
        // A decision, not an accident: the song is kept, the page defaults to 0 (its first page).
        // Teeth: `toInt()` instead of `toIntOrNull() ?: 0` throws NumberFormatException here.
        assertEquals("song-1" to 0, decodeStagePosition("song-1#abc"))
    }

    @Test
    fun hashInsideSongId_isASafeDecision() {
        // limit=2 keeps the first '#' as the split point: "ab#cd#3" → songId "ab", page "cd#3" → non-numeric
        // → 0. Downstream resolveStartPage won't find song "ab" and lands at the top. Pinned as a decision.
        assertEquals("ab" to 0, decodeStagePosition("ab#cd#3"))
    }

    @Test
    fun intermission_roundTripsAsItsOrdinal() {
        // A94: a card is the pair ("", k) and is stored as "#k". Teeth: an encode that drops an empty song id, or a
        // decode that rejects it, breaks this.
        assertEquals("#0", encodeStagePosition("", 0))
        assertEquals("#2", encodeStagePosition("", 2))
        assertEquals("" to 0, decodeStagePosition(encodeStagePosition("", 0)))
        assertEquals("" to 2, decodeStagePosition(encodeStagePosition("", 2)))
    }

    @Test
    fun aPreA94HashZero_decodesAsTheFirstIntermission() {
        // Every intermission wrote "#0" before A94; it must still decode (to ("", 0), the first card), not to null.
        assertEquals("" to 0, decodeStagePosition("#0"))
    }

    @Test
    fun anIntermissionPosition_neverEqualsASongPosition() {
        // The constraint "must not collide with any song id": a song position always has a non-empty song id.
        val card = encodeStagePosition("", 1)
        for (song in listOf("1", "a", "0f3c2b9e-0000-4000-8000-000000000001")) {
            for (page in 0..3) {
                val stored = encodeStagePosition(song, page)
                assertTrue(stored != card && decodeStagePosition(stored)!!.first.isNotEmpty(), "song $song#$page vs card $card")
            }
        }
    }
}
