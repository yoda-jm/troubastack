package com.troubastack.shared.stage

import android.graphics.Bitmap
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.graphics.asImageBitmap

/**
 * A64 part 2 (Android) — walk the overlay's pixels through [transformOverlayPixel]. `getPixels` returns
 * straight-alpha packed `0xAARRGGBB` ints, which is exactly what [transformOverlayPixel] expects.
 */
actual fun transformOverlayBitmap(src: ImageBitmap, scheme: StageColorMode): ImageBitmap {
    val ab = src.asAndroidBitmap()
    val w = ab.width
    val h = ab.height
    if (w == 0 || h == 0) return src
    val px = IntArray(w * h)
    ab.getPixels(px, 0, w, 0, 0, w, h)
    var i = 0
    while (i < px.size) {
        px[i] = transformOverlayPixel(px[i], scheme)
        i++
    }
    val out = Bitmap.createBitmap(w, h, Bitmap.Config.ARGB_8888)
    out.setPixels(px, 0, w, 0, 0, w, h)
    return out.asImageBitmap()
}
