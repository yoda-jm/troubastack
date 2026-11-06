package com.troubastack.app

import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
import com.google.zxing.BarcodeFormat
import com.google.zxing.qrcode.QRCodeWriter
import kotlin.test.Test
import kotlin.test.assertEquals

/** WIN09 — the QR is drawn with Compose's Canvas now; its pixels must be exactly ZXing's matrix, as A65's were. */
class QrImageTest {
    @Test
    fun everyPixelIsTheMatrix_blackOrWhite_noAntiAliasing() {
        val text = "https://band.example/join/0123456789abcdef0123456789abcdef"
        val size = 720
        val matrix = QRCodeWriter().encode(text, BarcodeFormat.QR_CODE, size, size)
        val pixels = qrImage(text, size).toPixelMap()
        var wrong = 0
        for (y in 0 until size) for (x in 0 until size) {
            // TEETH: an off-by-one run end, an anti-aliased edge, or a blank image (the qrblank sabotage) counts here.
            val want = if (matrix.get(x, y)) 0xFF000000.toInt() else 0xFFFFFFFF.toInt()
            if (pixels[x, y].toArgb() != want) wrong++
        }
        assertEquals(0, wrong, "pixels differing from the matrix")
    }
}
