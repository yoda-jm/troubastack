// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.seams

/**
 * SEAM 3 of 3 — storage. The last of the ONLY three places native code is allowed (I15).
 *
 * Provides platform paths and secure key/value access so shared code (the downloader's atomic
 * swap, I13; downloaded-revision bookkeeping; cached bundles) can persist without knowing the
 * platform. The *policy* (what to store, when to swap) is shared (distribution/Updates.kt);
 * this seam is only the where/how of bytes on disk.
 *
 *  - Android `actual` → `Context.filesDir` / `cacheDir` + `EncryptedSharedPreferences`.
 *  - iOS `actual`     → `FileManager` (Documents/Caches) + Keychain.  // iOS-later
 */
expect class Storage {

    /** Root for self-contained baked bundles (flattened images, I12). */
    fun bundlesDir(): String

    /** Scratch dir for downloads pending verification, enabling atomic swap (I13). */
    fun tempDir(): String

    /** Read a small secure value (e.g. auth token, device id). */
    fun getSecret(key: String): String?

    /** Write a small secure value. */
    fun putSecret(key: String, value: String)
}

/** Result of unpacking a `.tstage` archive (docs/design/08-bundle-container.md). Never thrown — a value. */
sealed interface UnpackResult {
    /** Archive extracted into [dir] (the destination passed in). */
    data class Ok(val dir: String) : UnpackResult
    /** Extraction refused/failed; [reason] is human-readable (bad zip, too big, unsafe entry). */
    data class Failed(val reason: String) : UnpackResult
}

/**
 * Extract a `.tstage` zip at [zipPath] into [destDir] (part of seam 3 — the where/how of bytes on
 * disk). Total: returns [UnpackResult.Failed] rather than throwing. Implementations MUST guard
 * against zip-slip (entries escaping [destDir]) and zip bombs (oversized archive/entries).
 *
 *  - Android `actual` → `java.util.zip.ZipInputStream`.
 *  - iOS `actual`     → `FileManager`/`NSFileManager` unzip.  // iOS-later
 */
expect fun unpackBundle(zipPath: String, destDir: String): UnpackResult
