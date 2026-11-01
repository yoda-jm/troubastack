// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.seams

import java.io.File
import java.util.zip.Inflater
import java.util.zip.ZipInputStream

/**
 * SEAM 3 `actual` (shared JVM) — `unpackBundle` + `rawInflate`, on plain `java.util.zip`. ONE copy shared by
 * Android and the Windows desktop target (both compile jvmSharedMain); iOS keeps its own Kotlin/Native
 * `ZipReader`-based actual (no zip API in the K/N stdlib). F1 (2026-11-01): this collapses the two byte-identical
 * JVM copies that WIN02 had to duplicate while jvmSharedMain did not yet exist.
 */

private const val MAX_ARCHIVE_BYTES = 512L * 1024 * 1024 // 512 MB cap (zip-bomb guard)

actual fun unpackBundle(zipPath: String, destDir: String): UnpackResult =
    unpackBundle(zipPath, destDir, MAX_ARCHIVE_BYTES)

/** The cap is a parameter so a test can drive the size-cap refusal with a tiny [maxArchiveBytes] instead of a
 *  512 MB fixture; the `actual` above pins the real 512 MB. */
internal fun unpackBundle(zipPath: String, destDir: String, maxArchiveBytes: Long): UnpackResult {
    val zip = File(zipPath)
    if (!zip.isFile) return UnpackResult.Failed("the selected file could not be read")
    if (zip.length() > maxArchiveBytes) return UnpackResult.Failed("that file is too large to be a concert")

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
                            if (written > maxArchiveBytes) return UnpackResult.Failed("that file is too large to be a concert")
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
 * Shared-JVM `actual` for the [rawInflate] seam — raw DEFLATE (nowrap) via `java.util.zip.Inflater`, so the
 * portable common [ZipArchive] parser is exercisable on a JVM (Android and desktop). Fail-closed on a stream
 * shorter OR longer than its declared size (mirrors the iOS actual).
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
        if (!inflater.finished() && inflater.inflate(ByteArray(1)) != 0) {
            throw ZipFormatException("deflate stream longer than declared size")
        }
        return out
    } finally {
        inflater.end()
    }
}
