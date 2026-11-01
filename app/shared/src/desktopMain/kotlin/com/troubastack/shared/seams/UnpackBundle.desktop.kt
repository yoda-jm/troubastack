// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.seams

import java.io.File
import java.util.zip.Inflater
import java.util.zip.ZipInputStream

/**
 * SEAM 3 `actual` (desktop) — `unpackBundle` + `rawInflate`. Both are PLAIN JVM (`java.util.zip`), the same
 * code as the Android actual, COPIED with its zip-slip + size-cap guards and its tests (desktopTest). This
 * duplication is deliberate for WIN02: WIN01's `jvmSharedMain` (the JVM source set Android and desktop will
 * share) does not exist yet, so there is nowhere shared to put it. Whoever lands second collapses the copy
 * into `jvmSharedMain` (D2 / the WIN01 coordination note).
 */

private const val MAX_ARCHIVE_BYTES = 512L * 1024 * 1024 // 512 MB cap (zip-bomb guard)

actual fun unpackBundle(zipPath: String, destDir: String): UnpackResult =
    unpackBundle(zipPath, destDir, MAX_ARCHIVE_BYTES)

/** The cap is a parameter so desktopTest can drive the size-cap refusal with a tiny [maxArchiveBytes]
 *  instead of a 512 MB fixture; the `actual` above pins the real 512 MB. */
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
 * Desktop/JVM `actual` for the [rawInflate] seam — raw DEFLATE (nowrap) via `java.util.zip.Inflater`, so the
 * portable common [ZipArchive] parser is exercisable on the desktop JVM too. Byte-identical to the Android
 * actual, including the fail-closed checks for a stream shorter OR longer than its declared size.
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
