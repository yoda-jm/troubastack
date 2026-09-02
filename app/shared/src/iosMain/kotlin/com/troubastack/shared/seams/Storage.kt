// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.seams

import kotlinx.cinterop.BetaInteropApi
import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.cinterop.addressOf
import kotlinx.cinterop.alloc
import kotlinx.cinterop.convert
import kotlinx.cinterop.memScoped
import kotlinx.cinterop.ptr
import kotlinx.cinterop.reinterpret
import kotlinx.cinterop.sizeOf
import kotlinx.cinterop.usePinned
import kotlinx.cinterop.value
import platform.CoreFoundation.CFDictionaryAddValue
import platform.CoreFoundation.CFDictionaryCreateMutable
import platform.CoreFoundation.CFMutableDictionaryRef
import platform.CoreFoundation.CFRelease
import platform.CoreFoundation.CFTypeRefVar
import platform.CoreFoundation.kCFTypeDictionaryKeyCallBacks
import platform.CoreFoundation.kCFTypeDictionaryValueCallBacks
import platform.Foundation.CFBridgingRelease
import platform.Foundation.CFBridgingRetain
import platform.Foundation.NSCachesDirectory
import platform.Foundation.NSData
import platform.Foundation.NSDocumentDirectory
import platform.Foundation.NSFileManager
import platform.Foundation.NSSearchPathForDirectoriesInDomains
import platform.Foundation.NSString
import platform.Foundation.NSURL
import platform.Foundation.NSUserDomainMask
import platform.Foundation.NSUTF8StringEncoding
import platform.Foundation.create
import platform.Foundation.dataUsingEncoding
import platform.Foundation.dataWithContentsOfFile
import platform.Foundation.writeToFile
import platform.Security.SecItemAdd
import platform.Security.SecItemCopyMatching
import platform.Security.SecItemDelete
import platform.Security.errSecSuccess
import platform.Security.kSecAttrAccount
import platform.Security.kSecAttrService
import platform.Security.kSecClass
import platform.Security.kSecClassGenericPassword
import platform.Security.kSecMatchLimit
import platform.Security.kSecMatchLimitOne
import platform.Security.kSecReturnData
import platform.Security.kSecValueData
import platform.posix.memcpy
import platform.zlib.Z_FINISH
import platform.zlib.Z_OK
import platform.zlib.Z_STREAM_END
import platform.zlib.ZLIB_VERSION
import platform.zlib.inflate
import platform.zlib.inflateEnd
import platform.zlib.inflateInit2_
import platform.zlib.z_stream

/**
 * SEAM 3 `actual` (iOS) — storage (I15). `NSFileManager` Documents/Caches for bundle + temp dirs
 * (atomic swap, I13) and the Keychain (Security framework) for secrets. No third-party dependency.
 */
@OptIn(ExperimentalForeignApi::class, BetaInteropApi::class)
actual class Storage {

    actual fun bundlesDir(): String {
        val dir = "${documentsDir()}/bundles"
        NSFileManager.defaultManager.createDirectoryAtPath(dir, true, null, null)
        return dir
    }

    actual fun tempDir(): String {
        val dir = cachesDir()
        NSFileManager.defaultManager.createDirectoryAtPath(dir, true, null, null)
        return dir
    }

    actual fun getSecret(key: String): String? = memScoped {
        val result = alloc<CFTypeRefVar>()
        val query = keychainQuery(key)
        CFDictionaryAddValue(query, kSecReturnData, kCFBooleanTrue())   // constants — no ownership
        CFDictionaryAddValue(query, kSecMatchLimit, kSecMatchLimitOne)
        val status = SecItemCopyMatching(query, result.ptr)
        CFRelease(query)
        if (status != errSecSuccess) return@memScoped null
        // CFBridgingRelease transfers the +1 returned by SecItem* into ARC.
        val data = CFBridgingRelease(result.value) as? NSData ?: return@memScoped null
        NSString.create(data, NSUTF8StringEncoding) as String?
    }

    actual fun putSecret(key: String, value: String) {
        // Idempotent write: delete any existing item, then add. (Simpler + total, per A05 intent.)
        val deleteQuery = keychainQuery(key)
        SecItemDelete(deleteQuery)
        CFRelease(deleteQuery)

        val data = (value as NSString).dataUsingEncoding(NSUTF8StringEncoding) ?: return
        val addQuery = keychainQuery(key)
        val dataRef = CFBridgingRetain(data)
        CFDictionaryAddValue(addQuery, kSecValueData, dataRef)
        CFRelease(dataRef)   // the dict retained it; drop our creating +1
        SecItemAdd(addQuery, null)
        CFRelease(addQuery)
    }

    /**
     * Build a mutable Keychain query dict with our class/service/account set. Caller owns the
     * returned dict — release it with `CFRelease`. The bridged string values are retained by the
     * dict, so the creating +1 from `CFBridgingRetain` is dropped here to avoid a leak.
     */
    private fun keychainQuery(account: String): CFMutableDictionaryRef? {
        val q = CFDictionaryCreateMutable(
            null, 0,
            kCFTypeDictionaryKeyCallBacks.ptr, kCFTypeDictionaryValueCallBacks.ptr,
        )
        CFDictionaryAddValue(q, kSecClass, kSecClassGenericPassword)   // constant — no ownership
        val service = CFBridgingRetain(SERVICE as NSString)
        CFDictionaryAddValue(q, kSecAttrService, service)
        CFRelease(service)
        val acct = CFBridgingRetain(account as NSString)
        CFDictionaryAddValue(q, kSecAttrAccount, acct)
        CFRelease(acct)
        return q
    }

    private fun documentsDir(): String = searchPath(NSDocumentDirectory)
    private fun cachesDir(): String = searchPath(NSCachesDirectory)

    private fun searchPath(directory: platform.Foundation.NSSearchPathDirectory): String =
        NSSearchPathForDirectoriesInDomains(directory, NSUserDomainMask, true)
            .firstOrNull() as? String
            ?: throw IllegalStateException("no path for directory $directory")

    private companion object {
        const val SERVICE = "troubastage.secrets"
    }
}

private const val MAX_ARCHIVE_BYTES = 512L * 1024 * 1024 // 512 MB cap (zip-bomb guard) — matches Android

@OptIn(ExperimentalForeignApi::class, BetaInteropApi::class)
actual fun unpackBundle(zipPath: String, destDir: String): UnpackResult {
    val fm = NSFileManager.defaultManager
    // Size-gate BEFORE reading the bytes: attributesOfItemAtPath avoids pulling a multi-GB pick
    // fully into memory only to reject it (jetsam risk on device).
    val preSize = (fm.attributesOfItemAtPath(zipPath, null)
        ?.get(platform.Foundation.NSFileSize) as? platform.Foundation.NSNumber)?.longLongValue
    if (preSize != null && preSize > MAX_ARCHIVE_BYTES) {
        return UnpackResult.Failed("that file is too large to be a concert")
    }
    val data = NSData.dataWithContentsOfFile(zipPath)
        ?: return UnpackResult.Failed("the selected file could not be read")
    if (data.length.toLong() > MAX_ARCHIVE_BYTES) {
        return UnpackResult.Failed("that file is too large to be a concert")
    }

    val archive = try {
        ZipArchive.parse(data.toByteArray())
    } catch (e: Exception) {
        return UnpackResult.Failed("that file isn't a valid concert archive")
    }

    fm.createDirectoryAtPath(destDir, true, null, null)
    var written = 0L
    return try {
        for (entry in archive.entries) {
            // Zip-slip guard (portable): the entry name must be a safe relative path.
            if (!isSafeZipEntryName(entry.name)) {
                return UnpackResult.Failed("that file isn't a valid concert (unsafe entry)")
            }
            val target = "$destDir/${entry.name}"
            if (entry.isDirectory) {
                fm.createDirectoryAtPath(target, true, null, null)
                continue
            }
            // Size-cap guard before inflating (bomb protection) — portable + JVM-tested.
            if (exceedsSizeCap(written, entry.uncompressedSize, MAX_ARCHIVE_BYTES)) {
                return UnpackResult.Failed("that file is too large to be a concert")
            }
            val parent = (NSString.create(string = target).URLByDeletingLastPathComponentPath())
                ?: destDir
            fm.createDirectoryAtPath(parent, true, null, null)
            val content = archive.readEntry(entry)
            written += content.size
            if (!content.toNSData().writeToFile(target, atomically = false)) {
                return UnpackResult.Failed("that file isn't a valid concert archive")
            }
        }
        UnpackResult.Ok(destDir)
    } catch (e: Exception) {
        UnpackResult.Failed("that file isn't a valid concert archive")
    }
}

/** Parent directory path of a "/"-joined path, or null if there is no separator. */
private fun NSString.URLByDeletingLastPathComponentPath(): String? {
    val s = this as String
    val idx = s.lastIndexOf('/')
    return if (idx <= 0) null else s.substring(0, idx)
}

// ---- ByteArray <-> NSData ----

@OptIn(ExperimentalForeignApi::class)
private fun NSData.toByteArray(): ByteArray {
    val size = length.toInt()
    if (size == 0) return ByteArray(0)
    val out = ByteArray(size)
    out.usePinned { pinned -> memcpy(pinned.addressOf(0), this.bytes, length) }
    return out
}

@OptIn(ExperimentalForeignApi::class, BetaInteropApi::class)
private fun ByteArray.toNSData(): NSData {
    if (isEmpty()) return NSData()
    return usePinned { pinned ->
        NSData.create(bytes = pinned.addressOf(0), length = size.convert())
    }
}

/** kCFBooleanTrue as a CFTypeRef for query values. */
@OptIn(ExperimentalForeignApi::class)
private fun kCFBooleanTrue(): platform.CoreFoundation.CFBooleanRef? = platform.CoreFoundation.kCFBooleanTrue

/**
 * iOS `actual` for the [rawInflate] seam. Raw DEFLATE (nowrap) via `platform.zlib`
 * `inflateInit2(windowBits = -15)` — the Apple SDK ships zlib but no zip API, so [ZipArchive] parses
 * the container and this inflates each entry. The common parser is JVM-tested (ZipArchiveTest); this
 * is the only iOS-specific half. Lives with seam 3 (Storage) since [unpackBundle] is its only caller.
 */
@OptIn(ExperimentalForeignApi::class)
internal actual fun rawInflate(deflated: ByteArray, expectedSize: Int): ByteArray {
    if (expectedSize == 0) return ByteArray(0)
    if (deflated.isEmpty()) throw ZipFormatException("empty deflate stream")
    val out = ByteArray(expectedSize)
    memScoped {
        val stream = alloc<z_stream>()
        // Raw DEFLATE: negative windowBits tells zlib there is no zlib/gzip wrapper.
        if (inflateInit2_(stream.ptr, -15, ZLIB_VERSION, sizeOf<z_stream>().convert()) != Z_OK) {
            throw ZipFormatException("zlib inflateInit2 failed")
        }
        try {
            deflated.usePinned { inPin ->
                out.usePinned { outPin ->
                    stream.next_in = inPin.addressOf(0).reinterpret()
                    stream.avail_in = deflated.size.convert()
                    stream.next_out = outPin.addressOf(0).reinterpret()
                    stream.avail_out = expectedSize.convert()
                    var rc = inflate(stream.ptr, Z_FINISH)
                    // Fail-closed on a stream longer than declared, WITHOUT rejecting one that
                    // exactly fills the buffer: when avail_out hits 0 zlib may return Z_OK and only
                    // report Z_STREAM_END on a follow-up call (avail_out == 0). So if we got Z_OK,
                    // call once more; a valid exactly-filled stream then reports Z_STREAM_END, while
                    // a longer stream can't make progress and stays non-END. (Mirrors the Android
                    // actual's finished() probe so both agree.)
                    if (rc == Z_OK) rc = inflate(stream.ptr, Z_FINISH)
                    if (rc != Z_STREAM_END) {
                        throw ZipFormatException("deflate stream did not end at declared size ($rc)")
                    }
                    if (stream.total_out.convert<Int>() != expectedSize) {
                        throw ZipFormatException("deflate stream shorter than declared size")
                    }
                }
            }
        } finally {
            inflateEnd(stream.ptr)
        }
    }
    return out
}
