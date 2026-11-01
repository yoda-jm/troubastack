package com.troubastack.app

import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asComposeImageBitmap
import androidx.compose.ui.graphics.asSkiaBitmap
import androidx.compose.ui.graphics.toComposeImageBitmap
import com.troubastack.shared.stage.ImageDecoder
import org.jetbrains.skia.Bitmap
import org.jetbrains.skia.Canvas
import org.jetbrains.skia.EncodedImageFormat
import org.jetbrains.skia.FilterMipmap
import org.jetbrains.skia.FilterMode
import org.jetbrains.skia.Image
import org.jetbrains.skia.MipmapMode
import org.jetbrains.skia.Rect
import java.io.File
import java.io.OutputStream

/**
 * WIN03 D1 — the desktop's Stage page decoder (Skia), the contract of Android's AndroidImageDecoder: resolve a blob
 * ref against the bundle dir and decode DOWNSAMPLED by the largest power of two that keeps the image at least the
 * target size. Total: any failure is a failed Result, never an exception.
 */
class SkiaImageDecoder(private val root: File) : ImageDecoder {
    override fun decode(ref: String, targetW: Int, targetH: Int): Result<ImageBitmap> = runCatching {
        val image = Image.makeFromEncoded(File(root, ref).readBytes())
        val sample = decodeSampleSize(image.width, image.height, targetW, targetH)
        if (sample == 1) return@runCatching image.toComposeImageBitmap()
        val w = image.width / sample
        val h = image.height / sample
        val out = Bitmap().apply { allocN32Pixels(w, h) }
        Canvas(out).drawImageRect(
            image, Rect.makeWH(image.width.toFloat(), image.height.toFloat()), Rect.makeWH(w.toFloat(), h.toFloat()),
            FilterMipmap(FilterMode.LINEAR, MipmapMode.NONE), null, true,
        )
        out.setImmutable()
        out.asComposeImageBitmap()
    }
}

/** Largest power-of-two that keeps the decoded image at least the target size (as Android's inSampleSize). */
internal fun decodeSampleSize(w: Int, h: Int, targetW: Int, targetH: Int): Int {
    if (w <= 0 || h <= 0 || targetW <= 0 || targetH <= 0) return 1
    var sample = 1
    while (w / (sample * 2) >= targetW && h / (sample * 2) >= targetH) sample *= 2
    return sample
}

/** WIN03 D1 — the desktop's [NoteBitmapCodec] (Skia), the contract of AndroidNoteBitmapCodec. */
object SkiaNoteBitmapCodec : NoteBitmapCodec {
    /** A MUTABLE bitmap: the note pad draws and erases into the loaded note. */
    override fun decode(path: String): ImageBitmap? = runCatching {
        Bitmap.makeFromImage(Image.makeFromEncoded(File(path).readBytes())).asComposeImageBitmap()
    }.getOrNull()

    override fun isAllTransparent(bitmap: ImageBitmap): Boolean? = runCatching {
        if (!bitmap.hasAlpha) return@runCatching false
        val w = bitmap.width
        if (w <= 0 || bitmap.height <= 0) return@runCatching true
        val row = IntArray(w)
        for (y in 0 until bitmap.height) {
            bitmap.readPixels(row, startX = 0, startY = y, width = w, height = 1)
            for (px in row) if ((px ushr 24) != 0) return@runCatching false
        }
        true
    }.getOrNull()

    override fun writePng(bitmap: ImageBitmap, out: OutputStream) {
        val png = Image.makeFromBitmap(bitmap.asSkiaBitmap()).encodeToData(EncodedImageFormat.PNG)
            ?: error("Skia produced no PNG data")
        out.write(png.bytes)
    }
}
