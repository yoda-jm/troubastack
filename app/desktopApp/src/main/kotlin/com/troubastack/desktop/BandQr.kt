package com.troubastack.desktop

import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.unit.dp
import com.troubastack.app.HttpTransport.StudioBand

/**
 * WIN09 — the embedded Studio's ⋮ "Show band QR…": what it opens, and what Back walks. Several admin bands open the
 * [Picker]; a [Qr] opened from it goes back to it ([Qr.picker]), one opened directly goes back to Studio (null).
 */
internal sealed interface BandQrView {
    data class Picker(val bands: List<StudioBand>) : BandQrView
    data class Qr(val band: StudioBand, val picker: Picker?) : BandQrView
}

/** WIN09 — the bands the item offers: the ones this person administers, in the server's order. */
internal fun bandQrChoices(bands: List<StudioBand>): List<StudioBand> {
    val admin = bands.filter { it.isAdmin }
    // Sabotage itemfornonadmin (the --win09-qr smoke only): someone who administers no band still gets the item
    // (offering every band) → the non-admin check reds, and the admin phases are untouched.
    return if (admin.isEmpty() && System.getProperty(WIN09_SABOTAGE_PROPERTY) == "itemfornonadmin") bands else admin
}

/** WIN09 — what the item opens: nothing when no band is administered (the item is then absent), the one band's QR,
 *  or the picker. */
internal fun bandQrEntry(choices: List<StudioBand>): BandQrView? = when (choices.size) {
    0 -> null
    1 -> BandQrView.Qr(choices.single(), picker = null)
    else -> BandQrView.Picker(choices)
}

/**
 * WIN09 — the host's screen over the embedded Studio, which is collapsed to 0.dp underneath (hostOverlay). It takes the
 * keyboard focus as it opens and on each change, so a Browser that held it can't take Alt+← and walk Studio's history
 * behind the QR; its Back is the newest BackHandler, so it wins over Studio's. [qr] is the platform's PresentQr.
 * [takeWindowFocus] first brings the native focus back to this window (measured on the runner: with the Studio WebView
 * focused, Compose's request alone left the keys going to the WebView).
 */
@Composable
internal fun BandQrOverlay(
    view: BandQrView,
    onView: (BandQrView?) -> Unit,
    qr: @Composable (band: StudioBand, onBack: () -> Unit) -> Unit,
    backHandler: @Composable (onBack: () -> Unit) -> Unit,
    takeWindowFocus: () -> Unit = {},
) {
    val focus = remember { FocusRequester() }
    Box(Modifier.fillMaxSize().focusRequester(focus).focusable()) {
        when (view) {
            is BandQrView.Picker -> {
                BandPicker(view.bands, onPick = { onView(BandQrView.Qr(it, view)) }, onBack = { onView(null) })
                backHandler { onView(null) }
            }
            is BandQrView.Qr -> qr(view.band) { onView(view.picker) }
        }
    }
    // Sabotage qrnofocus (the --win09-qr smoke only): leave the focus where it was → the overlay-focus check reds.
    LaunchedEffect(view) {
        if (System.getProperty(WIN09_SABOTAGE_PROPERTY) == "qrnofocus") return@LaunchedEffect
        takeWindowFocus()
        runCatching { focus.requestFocus() }
    }
}

/** WIN09 — the bands this person administers, and nothing else: choosing one opens its QR. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun BandPicker(bands: List<StudioBand>, onPick: (StudioBand) -> Unit, onBack: () -> Unit) {
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize()) {
            TopAppBar(
                title = { Text("Show band QR") },
                navigationIcon = { TextButton(onClick = onBack) { Text("‹  Back") } },
            )
            Column(
                Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).padding(24.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                for (band in bands) {
                    TextButton(onClick = { onPick(band) }) { Text(band.name.ifBlank { band.id }, style = MaterialTheme.typography.titleMedium) }
                }
            }
        }
    }
}

/** WIN09 — the `--win09-qr` smoke's sabotage (set from WIN09_SABOTAGE on that path only); the shared screen reads it too. */
internal const val WIN09_SABOTAGE_PROPERTY = "troubastage.win09.sabotage"
