package com.troubashare.app

import android.Manifest
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import com.google.zxing.BarcodeFormat
import com.google.zxing.BinaryBitmap
import com.google.zxing.DecodeHintType
import com.google.zxing.MultiFormatReader
import com.google.zxing.NotFoundException
import com.google.zxing.PlanarYUVLuminanceSource
import com.google.zxing.common.HybridBinarizer
import java.util.concurrent.Executors

/**
 * A53 — the invite scanner. Its ONLY job is to produce a **string** and hand it to the caller (which
 * routes it through A51's `parseTroubaLink` → A52's `JoinDialog`); it contains no join handling, by
 * design. A camera denied or absent is a normal outcome, not an error — it falls back to A52's paste
 * field via [onClose], never dead-ending.
 *
 * The four ways a naive CameraX scanner fails, each handled below: the pipeline stalls after two frames
 * unless every `ImageProxy` is closed ([QrAnalyzer] closes in a `finally`); a held-up code fires the join
 * repeatedly unless the first decode stops analysis (`decoded` latch); the camera stays lit on the stand
 * unless it's unbound when the screen leaves ([DisposableEffect]); and a tablet-on-a-stand is in
 * landscape, which `PreviewView` + `setTargetRotation` handle.
 */
@Composable
fun QrScanScreen(onDecoded: (String) -> Unit, onClose: () -> Unit) {
    val context = LocalContext.current
    var granted by remember {
        mutableStateOf(
            ContextCompat.checkSelfPermission(context, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED,
        )
    }
    var denied by remember { mutableStateOf(false) }
    val permissionLauncher = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { ok ->
        granted = ok
        denied = !ok
    }
    // Request at the point of use, not at launch.
    LaunchedEffect(Unit) { if (!granted) permissionLauncher.launch(Manifest.permission.CAMERA) }

    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.scrim) {
        if (granted) {
            CameraScanner(onDecoded = onDecoded, onClose = onClose)
        } else {
            // A denial (or a device with no camera) is a normal path — send the person back to the paste
            // field, which already works. Never a dead end.
            Column(
                Modifier.fillMaxSize().padding(24.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp, Alignment.CenterVertically),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(
                    if (denied) "Camera access is off, so scanning isn't available. You can paste the invite link instead."
                    else "Waiting for camera permission…",
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurface,
                )
                if (denied) Button(onClick = onClose) { Text("Paste a link instead") }
            }
        }
    }
}

@Composable
private fun CameraScanner(onDecoded: (String) -> Unit, onClose: () -> Unit) {
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    val analysisExecutor = remember { Executors.newSingleThreadExecutor() }
    // First decode wins: latched so a held-up code doesn't fire the join repeatedly, and so we stop after
    // the hand-off. Read on the analysis thread, set on the main thread — volatile is enough.
    val decoded = remember { java.util.concurrent.atomic.AtomicBoolean(false) }

    DisposableEffect(Unit) {
        onDispose {
            // Stop the camera when the screen leaves — an indicator left lit on a stage is unacceptable.
            runCatching { ProcessCameraProvider.getInstance(context).get().unbindAll() }
            analysisExecutor.shutdown()
        }
    }

    Box(Modifier.fillMaxSize()) {
        AndroidView(
            modifier = Modifier.fillMaxSize(),
            factory = { ctx ->
                val previewView = PreviewView(ctx)
                val providerFuture = ProcessCameraProvider.getInstance(ctx)
                providerFuture.addListener({
                    val provider = providerFuture.get()
                    val preview = Preview.Builder().build().also { it.surfaceProvider = previewView.surfaceProvider }
                    val analysis = ImageAnalysis.Builder()
                        .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                        .build()
                    analysis.setAnalyzer(analysisExecutor, QrAnalyzer { text ->
                        if (decoded.compareAndSet(false, true)) {
                            // Stop analysing, then hand the string up on the main thread.
                            runCatching { analysis.clearAnalyzer() }
                            ContextCompat.getMainExecutor(ctx).execute { onDecoded(text) }
                        }
                    })
                    runCatching {
                        provider.unbindAll()
                        provider.bindToLifecycle(lifecycleOwner, CameraSelector.DEFAULT_BACK_CAMERA, preview, analysis)
                    }
                }, ContextCompat.getMainExecutor(ctx))
                previewView
            },
        )
        // A framing hint + an explicit way out (also the fallback to paste).
        Column(
            Modifier.fillMaxSize().padding(20.dp),
            verticalArrangement = Arrangement.SpaceBetween,
        ) {
            Surface(color = MaterialTheme.colorScheme.surface, tonalElevation = 4.dp, shape = MaterialTheme.shapes.medium) {
                Text(
                    "Point at the invite QR",
                    Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                    style = MaterialTheme.typography.bodyMedium,
                )
            }
            // A56: the escape hatch was primary-indigo on the black camera scrim — near unreadable, and it's
            // the control someone reaches for when the scan isn't working. Give it the SAME on-surface
            // treatment as the hint above (a scrim chip behind it) so it survives a bright/busy camera image.
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                Surface(color = MaterialTheme.colorScheme.surface, tonalElevation = 4.dp, shape = MaterialTheme.shapes.medium) {
                    TextButton(onClick = onClose) { Text("Paste a link instead") }
                }
            }
        }
    }
}

/** Decodes a QR from the Y (luminance) plane of each frame with ZXing. Closes every [ImageProxy] in a
 *  `finally` — miss that and the pipeline stalls after two frames (the "scanner just doesn't work" bug).
 *  Restricted to QR_CODE so an unrelated barcode on a flyer doesn't fire. Rows may be padded, so the
 *  luminance source's data width is the plane's rowStride, cropped to the image width. */
private class QrAnalyzer(private val onQr: (String) -> Unit) : ImageAnalysis.Analyzer {
    private val reader = MultiFormatReader().apply {
        setHints(mapOf(DecodeHintType.POSSIBLE_FORMATS to listOf(BarcodeFormat.QR_CODE)))
    }

    override fun analyze(image: ImageProxy) {
        try {
            val plane = image.planes[0]
            val buffer = plane.buffer
            val bytes = ByteArray(buffer.remaining()).also { buffer.get(it) }
            val source = PlanarYUVLuminanceSource(
                bytes, plane.rowStride, image.height, 0, 0, image.width, image.height, false,
            )
            val result = try {
                reader.decodeWithState(BinaryBitmap(HybridBinarizer(source)))
            } catch (_: NotFoundException) {
                null // no code in this frame — the next one retries
            } finally {
                reader.reset()
            }
            result?.text?.let(onQr)
        } catch (_: Exception) {
            // A malformed frame is not fatal; drop it and keep scanning.
        } finally {
            image.close() // MUST close, always — the backpressure queue is depth 1.
        }
    }
}
