package com.troubastack.app

import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * A97 — the update's signer check never fails open and never guesses: install only when both signer sets were read and
 * match; "different key" only when both were read and differ; an unreadable set is "could not check". At API 29 the
 * downloaded file's set came back empty and the app told a same-key update it had a different key.
 */
class SignerCheckTest {
    private val ours = setOf("e99675323f86797c5addbafe2d4c0a8c7d855fb1d21e7e00fe9c72d07340a426")
    private val other = setOf("0000000000000000000000000000000000000000000000000000000000000000")

    @Test
    fun theSameSigners_install() {
        assertEquals(SignerVerdict.INSTALL, signerVerdict(ours, ours.toSet()))
        assertEquals(null, signerRefusal(SignerVerdict.INSTALL))
    }

    @Test
    fun otherSigners_areADifferentKey() {
        assertEquals(SignerVerdict.DIFFERENT_KEY, signerVerdict(ours, other))
        assertEquals(SignerVerdict.DIFFERENT_KEY, signerVerdict(ours, ours + other), "an extra signer is not the same set")
        assertEquals(DIFFERENT_KEY_LINE, signerRefusal(SignerVerdict.DIFFERENT_KEY))
    }

    @Test
    fun anUnreadableSet_couldNotBeChecked_neverAMatch_neverADifferentKey() {
        // TEETH: count "unreadable" as a match (two empty sets are equal) → red; report it as "different key" → red.
        for ((mine, theirs) in listOf(ours to null, ours to emptySet(), null to ours, emptySet<String>() to ours, null to null, emptySet<String>() to emptySet())) {
            assertEquals(SignerVerdict.COULD_NOT_CHECK, signerVerdict(mine, theirs), "ours=$mine theirs=$theirs")
        }
        assertEquals(
            "TroubaStage could not check who signed this update, so it did not install it. Your notes are safe; nothing was changed.",
            signerRefusal(SignerVerdict.COULD_NOT_CHECK),
        )
    }
}
