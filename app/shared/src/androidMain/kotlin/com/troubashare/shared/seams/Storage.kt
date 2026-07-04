// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubashare.shared.seams

import android.content.Context
import java.io.File
import java.util.zip.Inflater
import java.util.zip.ZipInputStream

/**
 * SEAM 3 `actual` (Android) — storage (I15). `Context.filesDir` / `cacheDir` for bundle + temp dirs
 * (atomic swap, I13). Constructed with the application Context by androidApp.
 */
actual class Storage(private val context: Context) {
    actual fun bundlesDir(): String = File(context.filesDir, "bundles").apply { mkdirs() }.path

    actual fun tempDir(): String = context.cacheDir.path

    // Secrets: holds NOTHING sensitive yet, so this is plain SharedPreferences. HARDEN to
    // EncryptedSharedPreferences (androidx.security.crypto) BEFORE any auth token is stored here.
    // No crypto dependency is pulled in for an unused path (per A05).
    private val prefs by lazy { context.getSharedPreferences("troubashare.secrets", Context.MODE_PRIVATE) }

    actual fun getSecret(key: String): String? = prefs.getString(key, null)

    actual fun putSecret(key: String, value: String) {
        prefs.edit().putString(key, value).apply()
    }
}

private const val MAX_ARCHIVE_BYTES = 512L * 1024 * 1024 // 512 MB cap (zip-bomb guard)

actual fun unpackBundle(zipPath: String, destDir: String): UnpackResult {
    val zip = File(zipPath)
    if (!zip.isFile) return UnpackResult.Failed("the selected file could not be read")
    if (zip.length() > MAX_ARCHIVE_BYTES) return UnpackResult.Failed("that file is too large to be a concert")

    val dest = File(destDir)
    val destCanonical = dest.canonicalFile
    dest.mkdirs()
    var written = 0L
    return try {
        ZipInputStream(zip.inputStream().buffered()).use { zis ->
            while (true) {
                val entry = zis.nextEntry ?: break
                // Zip-slip guard: the resolved target must stay inside destDir.
                val target = File(dest, entry.name).canonicalFile
                if (target != destCanonical && !target.path.startsWith(destCanonical.path + File.separator)) {
                    return UnpackResult.Failed("that file isn't a valid concert (unsafe entry)")
                }
                if (entry.isDirectory) {
                    target.mkdirs()
                } else {
                    target.parentFile?.mkdirs()
                    target.outputStream().buffered().use { out ->
                        val buf = ByteArray(64 * 1024)
                        while (true) {
                            val n = zis.read(buf)
                            if (n < 0) break
                            written += n
                            if (written > MAX_ARCHIVE_BYTES) return UnpackResult.Failed("that file is too large to be a concert")
                            out.write(buf, 0, n)
                        }
                    }
                }
                zis.closeEntry()
            }
        }
        UnpackResult.Ok(destDir)
    } catch (e: Exception) {
        UnpackResult.Failed("that file isn't a valid concert archive")
    }
}

/**
 * Android/JVM `actual` for the [rawInflate] seam — raw DEFLATE (nowrap) via `java.util.zip.Inflater`.
 * Android's own [unpackBundle] above uses `ZipInputStream` directly; this actual exists so the common
 * [ZipArchive] parser (which ships on iOS) is exercisable by the JVM unit tests.
 */
internal actual fun rawInflate(deflated: ByteArray, expectedSize: Int): ByteArray {
    if (expectedSize == 0) return ByteArray(0)
    val inflater = Inflater(/* nowrap = */ true)
    try {
        inflater.setInput(deflated)
        val out = ByteArray(expectedSize)
        var off = 0
        while (off < expectedSize && !inflater.finished()) {
            val n = inflater.inflate(out, off, expectedSize - off)
            if (n == 0 && (inflater.finished() || inflater.needsInput() || inflater.needsDictionary())) break
            off += n
        }
        if (off != expectedSize) throw ZipFormatException("deflate stream shorter than declared size")
        return out
    } finally {
        inflater.end()
    }
}
