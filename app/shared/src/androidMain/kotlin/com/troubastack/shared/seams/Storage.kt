// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.seams

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import java.io.File
import java.io.IOException
import java.security.GeneralSecurityException
import java.security.KeyStore

/**
 * SEAM 3 `actual` (Android) — storage (I15). `Context.filesDir` / `cacheDir` for bundle + temp dirs
 * (atomic swap, I13). Constructed with the application Context by androidApp.
 */
actual class Storage(private val context: Context) {
    actual fun bundlesDir(): String = File(context.filesDir, "bundles").apply { mkdirs() }.path

    actual fun tempDir(): String = context.cacheDir.path

    // A70 — rehearsal notes live BESIDE bundles/, never inside a bundle (a re-import destroys bundles/<id>).
    actual fun notesDir(): String = File(context.filesDir, "notes").apply { mkdirs() }.path

    // Secrets: EncryptedSharedPreferences (androidx.security.crypto) — hardened for B03, which is the
    // first to store an auth session cookie here (A05 flagged this as due before any token lands).
    // Keys are AES256-SIV, values AES256-GCM, under a MasterKey in the Android Keystore. This is a
    // NEW store ("…secrets.enc"); the old plaintext "troubastage.secrets" held nothing sensitive, so
    // no migration is needed. Also serves as the small KV for non-secret distribution policy JSON.
    //
    // A54: open-or-heal. EncryptedSharedPreferences.create THROWS when the KeyStore master key can't
    // decrypt an existing secrets.enc — restored from backup onto a device with no matching key, or the
    // OEM invalidated the key on a lock-screen change. That exception used to reach the FIRST getSecret
    // (the theme read at MainActivity.kt:122, during composition) and crash-loop the app with no way out.
    // Now [openOrHeal] wipes the corrupt file + key and retries once; on total failure we degrade to a
    // non-persistent in-memory map so the app still OPENS. What's discarded is a session + settings —
    // concerts are files on disk, not in this store.
    private val memFallback = HashMap<String, String>()
    private val opened: PrefsResult<SharedPreferences> by lazy {
        openOrHeal(create = ::createEncryptedPrefs, wipe = ::wipeSecrets).also {
            if ((it is PrefsResult.Opened && it.healed) || it is PrefsResult.Failed) settingsWereReset = true
        }
    }
    private val prefs: SharedPreferences? get() = (opened as? PrefsResult.Opened)?.prefs

    private fun createEncryptedPrefs(): SharedPreferences {
        val masterKey = MasterKey.Builder(context)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build()
        return EncryptedSharedPreferences.create(
            context,
            SECRETS_FILE,
            masterKey,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        )
    }

    /** Delete the unreadable encrypted store AND its master key — both must go, or the recreated store is
     *  encrypted under a key the next open still can't use. Deleting the key can't fail meaningfully here. */
    private fun wipeSecrets() {
        context.deleteSharedPreferences(SECRETS_FILE)
        runCatching {
            KeyStore.getInstance("AndroidKeyStore").apply { load(null) }.deleteEntry(MasterKey.DEFAULT_MASTER_KEY_ALIAS)
        }
    }

    actual fun getSecret(key: String): String? {
        val p = prefs
        return if (p != null) p.getString(key, null) else memFallback[key]
    }

    actual fun putSecret(key: String, value: String) {
        val p = prefs
        if (p != null) p.edit().putString(key, value).apply() else memFallback[key] = value
    }

    companion object {
        private const val SECRETS_FILE = "troubastage.secrets.enc"

        /** A54: true (for THIS process) once the encrypted store had to be reset to recover from an
         *  unreadable KeyStore state — Home reads it to show the after-the-fact notice. Process-global
         *  because MainActivity opens two Storage instances and either may be the one that heals; false
         *  again next launch once the recreated store opens cleanly. */
        @Volatile
        var settingsWereReset: Boolean = false
            internal set
    }
}

/** A54 — the outcome of opening the encrypted store, after up to one heal-and-retry. */
internal sealed interface PrefsResult<out P> {
    /** Opened; [healed] is true when the store had to be wiped + recreated to get here. */
    data class Opened<P>(val prefs: P, val healed: Boolean) : PrefsResult<P>

    /** Both the initial open and the post-wipe retry failed — the caller degrades, never throws. */
    data object Failed : PrefsResult<Nothing>
}

/**
 * A54 — open the encrypted prefs, or heal and retry once. [create] is INJECTED (Storage passes the real
 * `EncryptedSharedPreferences.create`) so a test can hand in one that throws — this failure path was
 * unreachable from any test before, which is why the crash shipped. On a `GeneralSecurityException` /
 * `IOException` (the KeyStore-can't-decrypt shapes), [wipe] deletes the corrupt file + master key and we
 * retry ONCE; a second recoverable failure ⇒ [PrefsResult.Failed]. Any OTHER throwable is a real bug and
 * is deliberately NOT swallowed — it propagates.
 */
internal fun <P> openOrHeal(create: () -> P, wipe: () -> Unit): PrefsResult<P> {
    try {
        return PrefsResult.Opened(create(), healed = false)
    } catch (e: GeneralSecurityException) {
        // recoverable — fall through to heal
    } catch (e: IOException) {
        // recoverable — fall through to heal
    }
    wipe()
    return try {
        PrefsResult.Opened(create(), healed = true)
    } catch (e: GeneralSecurityException) {
        PrefsResult.Failed
    } catch (e: IOException) {
        PrefsResult.Failed
    }
}
