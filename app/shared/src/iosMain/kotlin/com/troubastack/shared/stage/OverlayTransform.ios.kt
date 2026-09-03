package com.troubastack.shared.stage

import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asComposeImageBitmap
import androidx.compose.ui.graphics.asSkiaBitmap
import org.jetbrains.skia.Bitmap
import org.jetbrains.skia.ColorAlphaType
import org.jetbrains.skia.ColorType
import org.jetbrains.skia.ImageInfo

/**
 * A64 part 2 (iOS/Skia) — same rule as Android, via Skia read/installPixels. We read into an explicit
 * RGBA_8888 / UNPREMUL layout so the byte order is unambiguous (R,G,B,A straight), transform each pixel
 * through the shared [transformOverlayPixel], and install the bytes back.
 *
 * NOTE: iOS is not built in CI and there is no iOS device in the loop; this actual is written to the
 * documented Skia contract and compiles against it, but the Android actual is the runtime-verified path.
 * The colour rule itself is commonMain and unit-tested, so only the pixel plumbing is iOS-specific.
 */
actual fun transformOverlayBitmap(src: ImageBitmap, scheme: StageColorMode): ImageBitmap {
    val w = src.width
    val h = src.height
    if (w == 0 || h == 0) return src
    val info = ImageInfo(w, h, ColorType.RGBA_8888, ColorAlphaType.UNPREMUL)
    val rowBytes = (w * 4).toLong()
    val bytes = src.asSkiaBitmap().readPixels(info, rowBytes, 0, 0) ?: return src
    var p = 0
    val n = w * h
    while (p < n) {
        val o = p * 4
        val r = bytes[o].toInt() and 0xFF
        val g = bytes[o + 1].toInt() and 0xFF
        val b = bytes[o + 2].toInt() and 0xFF
        val a = bytes[o + 3].toInt() and 0xFF
        val t = transformOverlayPixel((a shl 24) or (r shl 16) or (g shl 8) or b, scheme)
        bytes[o] = ((t ushr 16) and 0xFF).toByte()
        bytes[o + 1] = ((t ushr 8) and 0xFF).toByte()
        bytes[o + 2] = (t and 0xFF).toByte()
        bytes[o + 3] = ((t ushr 24) and 0xFF).toByte()
        p++
    }
    val out = Bitmap()
    out.setImageInfo(info)
    out.installPixels(bytes)
    return out.asComposeImageBitmap()
}
