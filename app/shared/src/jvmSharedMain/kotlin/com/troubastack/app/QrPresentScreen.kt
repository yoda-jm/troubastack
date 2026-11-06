package com.troubastack.app

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Button
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Canvas
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.Paint
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.google.zxing.BarcodeFormat
import com.google.zxing.qrcode.QRCodeWriter

/**
 * A65 §4 — the room-facing invite QR, held up for everyone to scan. The invite LINK is server-issued
 * (Studio owns the logic); this screen only DRAWS it (ZXing) and keeps the screen awake. Per Fable:
 *  - LIST first and REUSE a suitable standing link; create ONLY on the explicit button, never on open.
 *  - state the terms (role, multi-use, no expiry) on the same screen — a projected QR is photographed by
 *    everyone in the room, so the admin must know what they are granting.
 *  - REVOKE is not here: it is destructive and outlives the moment → [onManageInStudio] deep-links to
 *    Studio's InviteLinks, where every link shows with its terms.
 * WIN09 — shared by Android and Windows. Each platform's PresentQr adds its own Back handling and passes
 * [keepScreenOn], the one native part: Android's FLAG_KEEP_SCREEN_ON, Windows' SetThreadExecutionState.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun QrPresentScreen(
    transport: HttpTransport,
    bandId: String,
    bandName: String,
    onManageInStudio: () -> Unit,
    onBack: () -> Unit,
    keepScreenOn: (on: Boolean) -> Unit,
) {
    var link by remember { mutableStateOf<HttpTransport.InviteLink?>(null) }
    var loading by remember { mutableStateOf(true) }
    var creating by remember { mutableStateOf(false) }
    var failed by remember { mutableStateOf(false) }

    // LIST-FIRST/REUSE (never create on open): find an existing room-suitable link.
    LaunchedEffect(bandId) {
        loading = true
        // WIN09 sabotage createonopen (the --win09-qr smoke only): mint a link as the screen opens → check 3 reds.
        if (qrSabotage == "createonopen") transport.createStandingInviteLink(bandId)
        link = transport.fetchInviteLinks(bandId).firstOrNull { it.roomSuitable }
        loading = false
    }

    // Keep the screen awake + bright while the QR is up (a phone that dims mid-room defeats the whole thing),
    // released when the person leaves the screen.
    val hold by rememberUpdatedState(keepScreenOn)
    DisposableEffect(Unit) {
        hold(true)
        onDispose { hold(false) }
    }

    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize()) {
            TopAppBar(
                title = { Text("Show band QR") },
                navigationIcon = { TextButton(onClick = onBack) { Text("‹  Back") } },
            )
            Column(Modifier.fillMaxSize().padding(24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                // The band name headlines every state (loading / no-link / QR) — VLL: "no name of the band"
                // on the create screen. You must always know which band you're about to open the door to.
                Text(
                    bandName.ifBlank { "Join the band" },
                    style = MaterialTheme.typography.headlineSmall,
                    textAlign = TextAlign.Center,
                    modifier = Modifier.fillMaxWidth().padding(bottom = 20.dp),
                )
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    when {
                        loading -> CircularProgressIndicator()
                        link != null -> QrView(link!!, onManageInStudio)
                        else -> NoLinkYet(
                            creating = creating,
                            failed = failed,
                            onCreate = {
                                creating = true; failed = false
                                // create runs in a coroutine scope tied to this composition
                            },
                            onCreateConfirmed = { l -> link = l; creating = false },
                            onCreateFailed = { creating = false; failed = true },
                            transport = transport,
                            bandId = bandId,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun QrView(link: HttpTransport.InviteLink, onManageInStudio: () -> Unit) {
    Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(20.dp)) {
        // A high-contrast QR on white, big, so it scans from across a room.
        val qr = remember(link.url) { qrImage(if (qrSabotage == "qrwrongtext") link.url + "-wrong" else link.url, 720) }
        Box(Modifier.background(Color.White).padding(16.dp)) {
            Image(qr, contentDescription = "Band invite QR", modifier = Modifier.size(320.dp))
        }
        // The terms — stated, not discovered (T122): a room QR is multi-use, no-expiry, and anyone who
        // photographs it can join.
        Text(
            "Anyone who scans this can join as ${link.role} — multi-use, no expiry.",
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center,
        )
        OutlinedButton(onClick = onManageInStudio) { Text("Manage / revoke in Studio") }
    }
}

@Composable
private fun NoLinkYet(
    creating: Boolean,
    failed: Boolean,
    onCreate: () -> Unit,
    onCreateConfirmed: (HttpTransport.InviteLink) -> Unit,
    onCreateFailed: () -> Unit,
    transport: HttpTransport,
    bandId: String,
) {
    // The create is an EXPLICIT action; kick it off when `creating` flips true (button press).
    LaunchedEffect(creating) {
        if (creating) {
            val l = transport.createStandingInviteLink(bandId)
            if (l != null) onCreateConfirmed(l) else onCreateFailed()
        }
    }
    Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(16.dp)) {
        Text(
            "No standing invite yet. Creating one makes a QR anyone can scan to join as member — " +
                "multi-use, with no expiry — until you revoke it in Studio.",
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center,
        )
        if (failed) Text("Couldn't create the link — check the connection.", color = MaterialTheme.colorScheme.error, textAlign = TextAlign.Center)
        if (creating) CircularProgressIndicator() else Button(onClick = onCreate) { Text("Create standing QR") }
    }
}

/**
 * ZXing QR encode → a white/black [ImageBitmap] drawn with Compose's Canvas (WIN09: no Android Bitmap, so the
 * desktop draws it too). Each row's black runs are whole-pixel rects without anti-aliasing, so the pixels are
 * exactly the matrix's — the same image the A65 setPixel loop made.
 */
internal fun qrImage(text: String, size: Int): ImageBitmap {
    val matrix = QRCodeWriter().encode(text, BarcodeFormat.QR_CODE, size, size)
    val image = ImageBitmap(size, size)
    val canvas = Canvas(image)
    val white = Paint().apply { color = Color.White; isAntiAlias = false }
    val black = Paint().apply { color = Color.Black; isAntiAlias = false }
    canvas.drawRect(0f, 0f, size.toFloat(), size.toFloat(), white)
    if (qrSabotage == "qrblank") return image   // WIN09 sabotage qrblank (the smoke only): a white square → check 2 reds
    for (y in 0 until size) {
        var x = 0
        while (x < size) {
            if (!matrix.get(x, y)) { x++; continue }
            val start = x
            while (x < size && matrix.get(x, y)) x++
            canvas.drawRect(start.toFloat(), y.toFloat(), x.toFloat(), (y + 1).toFloat(), black)
        }
    }
    return image
}

/** WIN09 — the `--win09-qr` smoke's sabotage, set as a system property on that path only (never on Android). */
private val qrSabotage: String? get() = System.getProperty("troubastage.win09.sabotage")
