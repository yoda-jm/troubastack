package com.troubashare.shared.seams

import java.io.IOException
import java.security.GeneralSecurityException
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * A54 — the crash-recovery seam. This path (`EncryptedSharedPreferences.create` throwing on an unreadable
 * KeyStore) was unreachable from any test before A54 — no test touched it, which is exactly why the crash
 * shipped. The creator is injectable here so a throwing one can be handed in; the model mirrors reality:
 * a locked store fails `create` UNTIL the corrupt file+key are wiped, then succeeds.
 */
class StorageHealTest {

    @Test fun heals_once_when_the_store_is_unreadable_until_wiped() {
        // create fails while the corrupt store is present, succeeds only after wipe() — as in the field.
        var wiped = false
        var calls = 0
        val result = openOrHeal(
            create = { calls++; if (!wiped) throw GeneralSecurityException("keystore locked"); "prefs" },
            wipe = { wiped = true },
        )
        assertEquals(PrefsResult.Opened("prefs", healed = true), result)
        assertTrue(wiped, "the corrupt store must be wiped to recover") // teeth: retry-without-wipe reddens here
        assertEquals(2, calls) // one failed open, one successful retry
    }

    @Test fun total_failure_degrades_to_Failed_no_exception_escapes() {
        // create throws even after a wipe (e.g. the KeyStore itself is wedged) — must NOT crash the app.
        var wipes = 0
        val result = openOrHeal<String>(
            create = { throw GeneralSecurityException("keystore wedged") },
            wipe = { wipes++ },
        )
        assertEquals(PrefsResult.Failed, result)
        assertEquals(1, wipes) // healed exactly once before giving up
    }

    @Test fun io_failure_is_also_recoverable() {
        var wiped = false
        val result = openOrHeal(
            create = { if (!wiped) throw IOException("prefs file corrupt"); "prefs" },
            wipe = { wiped = true },
        )
        assertEquals(PrefsResult.Opened("prefs", healed = true), result)
    }

    @Test fun happy_path_opens_without_wiping() {
        var wipes = 0
        val result = openOrHeal(create = { "prefs" }, wipe = { wipes++ })
        assertEquals(PrefsResult.Opened("prefs", healed = false), result)
        assertFalse(wipes > 0, "a healthy store is never wiped")
    }

    @Test fun a_non_recoverable_bug_is_not_swallowed() {
        // Only the KeyStore/IO shapes heal; a real programming error must surface, not be masked as a reset.
        assertFailsWith<IllegalStateException> {
            openOrHeal<String>(create = { throw IllegalStateException("real bug") }, wipe = {})
        }
    }
}
