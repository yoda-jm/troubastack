package com.troubastack.app

import android.graphics.BitmapFactory
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import com.troubastack.shared.bundle.BundleFiles
import com.troubastack.shared.stage.ImageDecoder
import java.io.File

/**
 * The injected image decoder for Stage (plain DI — NOT a new expect/actual seam, I15). Resolves a
 * blob ref against the bundle dir and decodes DOWNSAMPLED to the target size (inSampleSize) to avoid
 * OOM. Total: any failure comes back as a failed Result, never an exception.
 */
class AndroidImageDecoder(private val root: File) : ImageDecoder {
    override fun decode(ref: String, targetW: Int, targetH: Int): Result<ImageBitmap> = runCatching {
        val path = File(root, ref).path
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeFile(path, bounds)
        val opts = BitmapFactory.Options().apply {
            inSampleSize = sampleSize(bounds.outWidth, bounds.outHeight, targetW, targetH)
        }
        val bitmap = BitmapFactory.decodeFile(path, opts) ?: error("could not decode $ref")
        bitmap.asImageBitmap()
    }
}

/** Largest power-of-two that keeps the decoded image at least the target size. */
private fun sampleSize(w: Int, h: Int, targetW: Int, targetH: Int): Int {
    if (w <= 0 || h <= 0 || targetW <= 0 || targetH <= 0) return 1
    var sample = 1
    while (w / (sample * 2) >= targetW && h / (sample * 2) >= targetH) sample *= 2
    return sample
}

/** File-backed [BundleFiles] — the loader is given absolute paths (bundleDir prefix), so read as-is. */
class FileBundleFiles : BundleFiles {
    override fun exists(path: String): Boolean = File(path).exists()
    override fun readText(path: String): String? = File(path).let { if (it.isFile) it.readText() else null }
    override fun sizeOf(path: String): Long = File(path).let { if (it.isFile) it.length() else 0L }
}
