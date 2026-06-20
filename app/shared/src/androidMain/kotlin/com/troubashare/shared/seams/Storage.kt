// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubashare.shared.seams

/**
 * SEAM 3 `actual` (Android) — storage (I15).
 * Concrete API: `Context.filesDir` / `Context.cacheDir` for bundle + temp dirs (atomic swap,
 * I13) and `EncryptedSharedPreferences` (androidx.security.crypto) for secrets.
 */
actual class Storage {
    // TODO(scaffold): construct with an android.content.Context.
    actual fun bundlesDir(): String { TODO("Android: File(context.filesDir, \"bundles\").path") }
    actual fun tempDir(): String { TODO("Android: context.cacheDir.path") }
    actual fun getSecret(key: String): String? { TODO("Android: EncryptedSharedPreferences.getString") }
    actual fun putSecret(key: String, value: String) { TODO("Android: EncryptedSharedPreferences.edit().putString") }
}
