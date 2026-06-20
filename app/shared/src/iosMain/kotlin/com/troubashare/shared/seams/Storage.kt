// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubashare.shared.seams

/**
 * SEAM 3 `actual` (iOS) — storage (I15). ⚠️ iOS-LATER: TODO, not yet implemented.
 * Concrete API: `FileManager` (Documents / Caches) for bundle + temp dirs (atomic swap, I13)
 * and the Keychain for secrets.
 */
actual class Storage {
    actual fun bundlesDir(): String { TODO("iOS-later: FileManager Documents/bundles") }
    actual fun tempDir(): String { TODO("iOS-later: FileManager Caches") }
    actual fun getSecret(key: String): String? { TODO("iOS-later: Keychain read") }
    actual fun putSecret(key: String, value: String) { TODO("iOS-later: Keychain write") }
}
