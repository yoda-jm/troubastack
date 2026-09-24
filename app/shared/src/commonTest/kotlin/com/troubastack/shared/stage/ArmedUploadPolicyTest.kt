package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** A77 §7 — the pure armed-send policy: first send is non-overwrite (so a note we did not put surfaces as a
 *  conflict), later sends replace our own; the verdict routes OK/CONFLICT/FAILED. */
class ArmedUploadPolicyTest {

    @Test
    fun firstSendDoesNotOverwrite_laterSendsReplaceOurOwn() {
        assertFalse(armedSendOverwrite(alreadyAutoSent = false)) // first send: let a foreign note 409
        assertTrue(armedSendOverwrite(alreadyAutoSent = true))   // ours already: replace it
    }

    @Test
    fun verdict_routesEachOutcome() {
        assertEquals(ArmedSendVerdict.SENT, armedSendVerdict(SendResultKind.OK))
        assertEquals(ArmedSendVerdict.CONFLICT_DISARM, armedSendVerdict(SendResultKind.CONFLICT))
        assertEquals(ArmedSendVerdict.FAILED, armedSendVerdict(SendResultKind.FAILED))
    }

    @Test
    fun aConflictOnFirstSendDisarms_butAFailureDoesNot() {
        // The scenario §7 names: a 409 on the FIRST (non-overwrite) send is a foreign note ⇒ disarm; a network
        // FAILED is transient ⇒ stay armed (A77 is built for the no-network case).
        val overwrite = armedSendOverwrite(alreadyAutoSent = false)
        assertFalse(overwrite)
        assertEquals(ArmedSendVerdict.CONFLICT_DISARM, armedSendVerdict(SendResultKind.CONFLICT))
        assertEquals(ArmedSendVerdict.FAILED, armedSendVerdict(SendResultKind.FAILED))
    }
}
