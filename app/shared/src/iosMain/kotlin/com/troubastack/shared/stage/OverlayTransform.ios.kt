package com.troubastack.shared.stage

import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.toComposeImageBitmap
import androidx.compose.ui.graphics.toPixelMap
import org.jetbrains.skia.ColorAlphaType
import org.jetbrains.skia.ColorType
import org.jetbrains.skia.Image
import org.jetbrains.skia.ImageInfo

/**
 * A64 part 2 (iOS/Skia) — same rule as Android, using only PUBLIC APIs: read the pixels via Compose's
 * common [toPixelMap] (ARGB ints), run each through the shared [transformOverlayPixel], and rebuild via
 * Skia's [Image.makeRaster] (an explicit RGBA_8888 / UNPREMUL raster) → [toComposeImageBitmap].
 *
 * (The earlier draft called `org.jetbrains.skia.Bitmap.readPixels`, which is INTERNAL and broke the iOS
 * compile — CI compiles iOS at ci.yml:250. This route avoids it entirely.)
 *
 * The colour rule itself is commonMain and unit-tested; only this pixel plumbing is iOS-specific, and it
 * mirrors the Android actual byte-for-byte. Runtime-unverified (no iOS device in the loop); Android is
 * the verified path.
 */
actual fun transformOverlayBitmap(src: ImageBitmap, scheme: StageColorMode): ImageBitmap {
    val w = src.width
    val h = src.height
    if (w == 0 || h == 0) return src
    val pm = src.toPixelMap()
    val bytes = ByteArray(w * h * 4)
    var y = 0
    while (y < h) {
        var x = 0
        while (x < w) {
            val argb = pm.buffer[pm.bufferOffset + y * pm.stride + x]
            val t = transformOverlayPixel(argb, scheme)
            val o = (y * w + x) * 4
            bytes[o] = ((t ushr 16) and 0xFF).toByte()   // R
            bytes[o + 1] = ((t ushr 8) and 0xFF).toByte() // G
            bytes[o + 2] = (t and 0xFF).toByte()          // B
            bytes[o + 3] = ((t ushr 24) and 0xFF).toByte() // A
            x++
        }
        y++
    }
    val info = ImageInfo(w, h, ColorType.RGBA_8888, ColorAlphaType.UNPREMUL)
    return Image.makeRaster(info, bytes, w * 4).toComposeImageBitmap()
}
