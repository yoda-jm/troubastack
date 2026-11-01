package com.troubastack.app

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.graphics.asImageBitmap
import java.io.OutputStream

/** WIN03a — Android's [NoteBitmapCodec]: the three bitmap calls A70's notes store has always made. */
object AndroidNoteBitmapCodec : NoteBitmapCodec {
    override fun decode(path: String): ImageBitmap? {
        // inMutable: a decoded PNG is otherwise IMMUTABLE, and the loaded note becomes the working bitmap the
        // pad draws/erases into (a Canvas over an immutable bitmap throws). The note always DISPLAYS before it
        // is edited, so the load happens in display mode and the neutral is reused when note mode is entered —
        // it must be mutable from the decode, not copied later. (A70 note-edit crash, reload lifecycle.)
        val opts = BitmapFactory.Options().apply { inMutable = true }
        return runCatching { BitmapFactory.decodeFile(path, opts)?.asImageBitmap() }.getOrNull()
    }

    override fun isAllTransparent(bitmap: ImageBitmap): Boolean? =
        runCatching { bitmap.asAndroidBitmap() }.getOrNull()?.let(::allTransparent)

    override fun writePng(bitmap: ImageBitmap, out: OutputStream) {
        bitmap.asAndroidBitmap().compress(Bitmap.CompressFormat.PNG, 100, out)
    }
}

/** True iff no pixel has any alpha — the "nothing was drawn / all erased" case that deletes the note. */
private fun allTransparent(b: Bitmap): Boolean {
    if (!b.hasAlpha()) return false
    val w = b.width; val h = b.height
    if (w <= 0 || h <= 0) return true
    val row = IntArray(w)
    for (y in 0 until h) {
        b.getPixels(row, 0, w, 0, y, w, 1)
        for (px in row) if ((px ushr 24) != 0) return false
    }
    return true
}
