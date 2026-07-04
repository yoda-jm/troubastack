// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubashare.shared.seams

/**
 * Portable, minimal ZIP reader used by the iOS Storage seam's [unpackBundle] (there is no zip API in
 * Kotlin/Native's stdlib or the Apple SDK). Android keeps `java.util.zip` (see the androidMain
 * actual); this parser exists so the format handling can be **unit-tested off-device on the JVM**
 * even though it only ships on iOS (I15 keeps platform code in the seam actuals — this is portable
 * common code, not platform code).
 *
 * Scope: exactly what our own `.tstage` archives use (docs/design/08-bundle-container.md) — STORE (0)
 * and DEFLATE (8), no zip64, no encryption, no data descriptors on the central-directory path. It
 * reads the End Of Central Directory record, walks the central directory, then locates each file's
 * data via its local header. DEFLATE is inflated through the tiny [rawInflate] seam so the rest is
 * pure Kotlin.
 */

/**
 * Raw DEFLATE inflate — no zlib/gzip wrapper ("nowrap", windowBits = -15). [expectedSize] is the
 * uncompressed length from the central directory; the result is exactly that many bytes.
 *
 *  - Android/JVM `actual` → `java.util.zip.Inflater(nowrap = true)`.
 *  - iOS `actual`         → `platform.zlib` `inflateInit2(-15)` + `inflate`.
 */
internal expect fun rawInflate(deflated: ByteArray, expectedSize: Int): ByteArray

/** Malformed/unsupported archive. Never leaks past [unpackBundle], which maps it to a friendly reason. */
internal class ZipFormatException(message: String) : Exception(message)

/** One file/dir parsed from the central directory. */
internal data class ZipEntry(
    val name: String,
    val isDirectory: Boolean,
    val method: Int,             // 0 = stored, 8 = deflate
    val compressedSize: Int,
    val uncompressedSize: Int,
    val localHeaderOffset: Int,
)

/**
 * A parsed archive. [parse] reads the directory; [readEntry] materialises one entry's bytes on
 * demand (inflating if needed). Nothing is inflated until asked for, so the size-cap guard can be
 * enforced entry-by-entry by the caller before allocating.
 */
internal class ZipArchive private constructor(
    private val bytes: ByteArray,
    val entries: List<ZipEntry>,
) {
    /** Inflate (or copy, if STORE) one entry's uncompressed bytes. */
    fun readEntry(entry: ZipEntry): ByteArray {
        val loc = entry.localHeaderOffset
        if (loc < 0 || loc + LOCAL_HEADER_MIN > bytes.size) throw ZipFormatException("bad local header offset")
        if (u32(loc) != LOCAL_SIG) throw ZipFormatException("bad local header signature")
        val nameLen = u16(loc + 26)
        val extraLen = u16(loc + 28)
        val dataStart = loc + LOCAL_HEADER_MIN + nameLen + extraLen
        if (dataStart + entry.compressedSize > bytes.size) throw ZipFormatException("truncated entry data")
        return when (entry.method) {
            METHOD_STORE -> bytes.copyOfRange(dataStart, dataStart + entry.compressedSize)
            METHOD_DEFLATE -> {
                val deflated = bytes.copyOfRange(dataStart, dataStart + entry.compressedSize)
                rawInflate(deflated, entry.uncompressedSize)
            }
            else -> throw ZipFormatException("unsupported compression method ${entry.method}")
        }
    }

    // ---- little-endian readers ----
    private fun u16(off: Int): Int =
        (bytes[off].toInt() and 0xFF) or ((bytes[off + 1].toInt() and 0xFF) shl 8)

    private fun u32(off: Int): Int =
        (bytes[off].toInt() and 0xFF) or
            ((bytes[off + 1].toInt() and 0xFF) shl 8) or
            ((bytes[off + 2].toInt() and 0xFF) shl 16) or
            ((bytes[off + 3].toInt() and 0xFF) shl 24)

    companion object {
        private const val EOCD_SIG = 0x06054b50
        private const val CEN_SIG = 0x02014b50
        private const val LOCAL_SIG = 0x04034b50
        private const val EOCD_MIN = 22
        private const val CEN_MIN = 46
        private const val LOCAL_HEADER_MIN = 30
        private const val METHOD_STORE = 0
        private const val METHOD_DEFLATE = 8

        fun parse(bytes: ByteArray): ZipArchive {
            if (bytes.size < EOCD_MIN) throw ZipFormatException("too small to be a zip")
            val eocd = findEocd(bytes)
            val count = u16(bytes, eocd + 10)
            val cdOffset = u32(bytes, eocd + 16)
            if (cdOffset < 0 || cdOffset > bytes.size) throw ZipFormatException("bad central directory offset")

            val entries = ArrayList<ZipEntry>(count)
            var p = cdOffset
            repeat(count) {
                if (p + CEN_MIN > bytes.size) throw ZipFormatException("truncated central directory")
                if (u32(bytes, p) != CEN_SIG) throw ZipFormatException("bad central directory signature")
                val method = u16(bytes, p + 10)
                val compressedSize = u32(bytes, p + 20)
                val uncompressedSize = u32(bytes, p + 24)
                val nameLen = u16(bytes, p + 28)
                val extraLen = u16(bytes, p + 30)
                val commentLen = u16(bytes, p + 32)
                val localOffset = u32(bytes, p + 42)
                if (p + CEN_MIN + nameLen > bytes.size) throw ZipFormatException("truncated entry name")
                val name = bytes.decodeToString(p + CEN_MIN, p + CEN_MIN + nameLen)
                val isDir = name.endsWith("/")
                entries += ZipEntry(name, isDir, method, compressedSize, uncompressedSize, localOffset)
                p += CEN_MIN + nameLen + extraLen + commentLen
            }
            return ZipArchive(bytes, entries)
        }

        /** Scan backwards for the EOCD signature (the trailing comment is 0..65535 bytes). */
        private fun findEocd(bytes: ByteArray): Int {
            val minStart = maxOf(0, bytes.size - EOCD_MIN - 0xFFFF)
            var i = bytes.size - EOCD_MIN
            while (i >= minStart) {
                if (u32(bytes, i) == EOCD_SIG) return i
                i--
            }
            throw ZipFormatException("no end-of-central-directory record")
        }

        private fun u16(b: ByteArray, off: Int): Int =
            (b[off].toInt() and 0xFF) or ((b[off + 1].toInt() and 0xFF) shl 8)

        private fun u32(b: ByteArray, off: Int): Int =
            (b[off].toInt() and 0xFF) or
                ((b[off + 1].toInt() and 0xFF) shl 8) or
                ((b[off + 2].toInt() and 0xFF) shl 16) or
                ((b[off + 3].toInt() and 0xFF) shl 24)
    }
}

/**
 * Zip-slip guard, portable (no `File`/canonicalisation). True iff [name] is a safe *relative* path
 * that cannot escape the destination when joined to it. Mirrors the intent of the Android actual's
 * canonical-path check, expressed on the entry name so it works identically on Kotlin/Native.
 *
 * Rejects: empty names, absolute paths (`/…`), Windows drive letters (`C:…`), backslashes, and any
 * `..` path segment. Allows `.` and empty segments (trailing slash on directories).
 */
internal fun isSafeZipEntryName(name: String): Boolean {
    if (name.isEmpty()) return false
    if (name.startsWith("/")) return false
    if (name.contains('\\')) return false                 // backslash: Windows separator / escape
    if (name.length >= 2 && name[1] == ':') return false  // drive letter, e.g. C:
    return name.split('/').none { it == ".." }
}
