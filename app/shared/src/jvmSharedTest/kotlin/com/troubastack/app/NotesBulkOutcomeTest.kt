package com.troubastack.app

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * T170 §6 / A75 — the bulk "Send all" decision, the one that must never regress (Fable A75 review). The
 * load-bearing case is a 409 on a note that is NOT locally sent: it is a CONFLICT and must write no `sentAt`
 * marker, or the note would drop into the collapsed "Sent" group as if delivered when Studio kept an older
 * version. With A75's Sent group collapsed-by-default, a false marker HIDES the note, so this is teeth-checked.
 */
class NotesBulkOutcomeTest {

    @Test
    fun alreadySent_isSkipped_withoutContactingServer() {
        // sentAt set ⇒ SKIPPED regardless of any result (⟨D1⟩ R2: never re-send an already-sent note).
        assertEquals(BulkOutcome.SKIPPED, bulkNoteOutcome(alreadySent = true, result = null))
        assertEquals(BulkOutcome.SKIPPED, bulkNoteOutcome(alreadySent = true, result = NoteSendResult.Ok))
    }

    @Test
    fun ok_isSent_andMarks() {
        assertEquals(BulkOutcome.SENT, bulkNoteOutcome(alreadySent = false, result = NoteSendResult.Ok))
        assertTrue(BulkOutcome.SENT.marksSent())
    }

    @Test
    fun conflict_onUnsentNote_isConflict_andDoesNotMark() {
        // THE guard: a 409 on an unsent note is a conflict, not a delivery. No marker.
        assertEquals(BulkOutcome.CONFLICT, bulkNoteOutcome(alreadySent = false, result = NoteSendResult.Exists))
        assertFalse(BulkOutcome.CONFLICT.marksSent())
    }

    @Test
    fun failed_isFailed_andDoesNotMark() {
        assertEquals(BulkOutcome.FAILED, bulkNoteOutcome(alreadySent = false, result = NoteSendResult.Failed("boom")))
        assertFalse(BulkOutcome.FAILED.marksSent())
    }

    @Test
    fun onlyASendEverWritesTheMarker() {
        // The invariant stated once more, across every outcome: SENT and nothing else.
        assertTrue(BulkOutcome.SENT.marksSent())
        assertFalse(BulkOutcome.SKIPPED.marksSent())
        assertFalse(BulkOutcome.CONFLICT.marksSent())
        assertFalse(BulkOutcome.FAILED.marksSent())
    }
}
