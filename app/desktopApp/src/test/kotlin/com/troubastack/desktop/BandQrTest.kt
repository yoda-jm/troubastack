package com.troubastack.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.runComposeUiTest
import androidx.compose.ui.unit.dp
import com.troubastack.app.HttpTransport.StudioBand
import com.troubastack.desktop.studio.studioPathUrl
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** WIN09 — the ⋮ "Show band QR…" item: which bands it offers, what it opens, and where Back goes. */
@OptIn(ExperimentalTestApi::class)
class BandQrTest {
    private val alpha = StudioBand("a", "Alpha", isAdmin = true)
    private val beta = StudioBand("b", "Beta", isAdmin = false)
    private val gamma = StudioBand("c", "Gamma", isAdmin = true)

    @Test
    fun theItemOffersTheAdminBands_inTheServersOrder() {
        // TEETH: offering every band (the itemfornonadmin sabotage's shape) keeps Beta and reddens this.
        assertEquals(listOf(alpha, gamma), bandQrChoices(listOf(alpha, beta, gamma)))
        assertEquals(emptyList(), bandQrChoices(listOf(beta)))
    }

    @Test
    fun noAdminBand_noItem_oneOpensItsQr_severalOpenThePicker() {
        assertNull(bandQrEntry(emptyList()))
        assertEquals(BandQrView.Qr(alpha, picker = null), bandQrEntry(listOf(alpha)))
        assertEquals(BandQrView.Picker(listOf(alpha, gamma)), bandQrEntry(listOf(alpha, gamma)))
    }

    @Test
    fun openPath_loadsOnlyAnAbsolutePathOfTheServersOrigin() {
        assertEquals("http://127.0.0.1:8080/bands/a/settings", studioPathUrl("http://127.0.0.1:8080", "/bands/a/settings"))
        for (bad in listOf("bands/a", "//evil.example/x", "https://evil.example/", "/\\evil.example", "")) {
            assertNull(studioPathUrl("http://127.0.0.1:8080", bad), "refused: [$bad]")
        }
    }

    @Test
    fun thePicker_listsTheBandsAndNothingElse_choosingOneOpensItsQr_andBackWalksBack() = runComposeUiTest {
        var view by mutableStateOf<BandQrView?>(bandQrEntry(listOf(alpha, gamma)))
        setContent {
            MaterialTheme {
                Box(Modifier.size(800.dp, 600.dp)) {
                    view?.let { v ->
                        BandQrOverlay(
                            view = v,
                            onView = { view = it },
                            qr = { band, back -> Text("QR of ${band.name}"); TextButton(onClick = back) { Text("QR back") } },
                            backHandler = {},
                        )
                    }
                }
            }
        }
        // ‹ Back and one button per band: nothing else to press.
        onAllNodes(hasClickAction()).assertCountEquals(3)
        onAllNodes(hasText("Beta")).assertCountEquals(0)
        onNodeWithText("Gamma").performClick()
        onNodeWithText("QR of Gamma").assertExists()
        // Back from a QR chosen in the picker returns to the picker; Back from the picker returns to Studio.
        onNodeWithText("QR back").performClick()
        onNodeWithText("Alpha").assertExists()
        onNodeWithText("‹  Back").performClick()
        waitForIdle()
        assertNull(view)
    }

    @Test
    fun aQrOpenedDirectly_backReturnsToStudio() = runComposeUiTest {
        var view by mutableStateOf<BandQrView?>(bandQrEntry(listOf(alpha)))
        setContent {
            MaterialTheme {
                view?.let { v -> BandQrOverlay(v, onView = { view = it }, qr = { band, back -> TextButton(onClick = back) { Text("QR of ${band.name}") } }, backHandler = {}) }
            }
        }
        onNodeWithText("QR of Alpha").performClick()
        waitForIdle()
        assertNull(view)
    }
}
