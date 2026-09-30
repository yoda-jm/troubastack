package com.troubastack.app

import com.troubastack.shared.stage.notes.NoteEntry
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * A78 — the Notes tab's fold rules and bulk actions. The FIRST fixture is VLL's ordinary case — one band, one
 * concert, two unsent notes — because that is exactly the case A75 ⟨D1⟩ broke (no header ⇒ no Send all, nothing
 * to collapse). A multi-concert fixture would have passed against the broken code.
 */
class NotesTreeTest {

    private fun note(band: String, song: String, page: Int, sent: Long? = null, takenAs: String = "me") = NoteEntry(
        songId = "id-$song", rasterHash = "h-$song-$page", file = "$song-$page.png", pageInSong = page, songTitle = song,
        bandName = band, concertRev = 1, takenAs = takenAs, width = 10, height = 10, updatedAt = 0, sentAt = sent,
    )
    private val labels = mapOf("c1" to "Rehearsal A", "c2" to "Rehearsal B", "c3" to "Other band gig")

    @Test
    fun oneBandOneConcertTwoUnsent_hasTopLevelSendAll_andACollapsibleConcertHeader() {
        val items = listOf("c1" to note("Band X", "Song", 0), "c1" to note("Band X", "Song", 1))
        // ⟨D1⟩ the top-level line exists and counts both.
        assertEquals(SendAllLine.Shown(2, enabled = true, sending = false), sendAllLine(items.size, connected = true, sending = false))
        // ⟨D2⟩ a concert header IS drawn (the case A75 folded away), carrying the band since the band header folds.
        val rows = noteRows(items, "u", labels, collapsed = emptySet())
        assertTrue(rows.none { it is NoteRow.Band }, "one band ⇒ the band header still folds")
        val concert = rows.filterIsInstance<NoteRow.Concert>().single()
        assertEquals("Band X · Rehearsal A", concert.label)
        assertEquals(2, concert.items.size)
        assertEquals(NodeActionKind.SEND_ALL, nodeActionKind(sentSide = false, connected = true)) // node-level Send all
        assertEquals(2, rows.count { it is NoteRow.Leaf })
        // …and it collapses: folding the concert hides its notes.
        val folded = noteRows(items, "u", labels, collapsed = setOf(concert.key))
        assertEquals(listOf<NoteRow>(concert), folded)
    }

    @Test
    fun twoBands_topLevelCountsBoth_andEachBandHasAHeader() {
        val items = listOf("c1" to note("Band X", "Song", 0), "c3" to note("Band Y", "Tune", 0), "c3" to note("Band Y", "Tune", 1))
        assertEquals(3, (sendAllLine(items.size, connected = true, sending = false) as SendAllLine.Shown).count)
        val rows = noteRows(items, "u", labels, emptySet())
        assertEquals(listOf("Band X", "Band Y"), rows.filterIsInstance<NoteRow.Band>().map { it.label })
        // with a band header present, the concert header carries the concert name alone
        assertEquals(listOf("Rehearsal A", "Other band gig"), rows.filterIsInstance<NoteRow.Concert>().map { it.label })
    }

    @Test
    fun topLevelSend_goesThroughTheSameIdentityDecision() {
        val mine = "c1" to note("Band X", "Song", 0, takenAs = "me")
        val foreign = "c1" to note("Band X", "Song", 1, takenAs = "someone-else")
        // The top-level line sends ALL unsent notes through onSendAll → needsIdentityPrompt, the same rule a
        // node header uses — so a foreign-identity note raises "Send under your account?" from the top line too.
        assertTrue(needsIdentityPrompt(listOf(mine, foreign), myId = "me"))
        assertFalse(needsIdentityPrompt(listOf(mine), myId = "me"))
        assertFalse(needsIdentityPrompt(listOf(mine, foreign), myId = null), "unknown account (offline) never prompts")
    }

    @Test
    fun offline_theLineIsPresentButDisabled_andNothingToSendHidesIt() {
        assertEquals(SendAllLine.Shown(2, enabled = false, sending = false), sendAllLine(2, connected = false, sending = false))
        assertEquals(SendAllLine.Hidden, sendAllLine(0, connected = true, sending = false))
        assertEquals(SendAllLine.Shown(2, enabled = false, sending = true), sendAllLine(2, connected = true, sending = true))
    }

    @Test
    fun clearSent_isLocal_andAConcertClearSparesUnsentAndOtherConcerts() {
        val sentA1 = "c1" to note("Band X", "Song", 0, sent = 5)
        val sentA2 = "c1" to note("Band X", "Song", 1, sent = 6)
        val unsentA = "c1" to note("Band X", "Song", 2) // same concert, NOT sent — the note the test exists for
        val sentB = "c2" to note("Band X", "Tune", 0, sent = 7)
        val all = listOf(sentA1, sentA2, unsentA, sentB)
        // concert-level Clear: only concert c1's SENT notes
        assertEquals(listOf(sentA1, sentA2), sentToClear(all, concertId = "c1"))
        // the top-level "Clear sent": every sent note, never the unsent one
        assertEquals(listOf(sentA1, sentA2, sentB), sentToClear(all))
        // clearSentLocally takes ONLY a local deleter — it has no route to the server or ArmedUploader, so a
        // clear inside an A77 window cannot reach Studio. It also refuses an unsent note even if handed one.
        val deleted = mutableListOf<NoteEntry>()
        val n = clearSentLocally(listOf(sentA1, sentA2, unsentA)) { _, e -> deleted += e }
        assertEquals(2, n)
        assertEquals(listOf(sentA1.second, sentA2.second), deleted)
    }

    @Test
    fun noClearOnTheUnsentSide_ever() {
        for (connected in listOf(true, false)) {
            assertFalse(nodeActionKind(sentSide = false, connected = connected) == NodeActionKind.CLEAR)
            assertEquals(NodeActionKind.CLEAR, nodeActionKind(sentSide = true, connected = connected)) // local ⇒ works offline
        }
    }

    @Test
    fun clearConfirmation_statesTheConsequenceInNumbers() {
        assertEquals("Remove 7 sent notes from this tablet? Studio keeps its copies. On Stage, these pages will no longer show them.", clearSentQuestion(7))
        assertEquals("Removed 7 notes from this tablet", clearSentDone(7))
        assertEquals("Remove 1 sent note from this tablet? Studio keeps its copies. On Stage, this page will no longer show it.", clearSentQuestion(1))
    }
}
