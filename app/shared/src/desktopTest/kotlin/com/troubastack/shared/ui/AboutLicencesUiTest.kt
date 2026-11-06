package com.troubastack.shared.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.runComposeUiTest
import com.troubastack.shared.stage.FitMode
import com.troubastack.shared.stage.StageColorMode
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * WIN08 S2.8 — Parameters → About's licences line appears only where the platform ships licences (SWT on Windows):
 * no line and no Licences button without them; the line, the button, and its press with them. And the Licences
 * screen is a list of entries, then one entry's texts, with Back stepping back to the list before leaving.
 */
@OptIn(ExperimentalTestApi::class)
class AboutLicencesUiTest {
    private val line = "Studio in this app is shown with SWT 3.134.0, open-source software under the Eclipse Public License 2.0."

    private fun ComposeUiTest.about(licencesLine: String?, onShow: (() -> Unit)?) = setContent {
        SettingsScreen(
            ThemePref.SYSTEM, {}, FitMode.FIT_PAGE, {}, StageColorMode.NORMAL, {},
            licencesLine = licencesLine,
            onShowLicences = onShow,
            onBack = {},
        )
    }

    private fun ComposeUiTest.licences(entries: List<LicenceEntry>, opened: () -> LicenceEntry?, onOpen: (LicenceEntry?) -> Unit, onBack: () -> Unit) =
        setContent { LicencesScreen(entries, opened(), onOpen, onBack) }

    @Test
    fun withoutShippedLicences_aboutShowsNoLineAndNoButton() = runComposeUiTest {
        // TEETH: showing the row whatever the platform says (Android, Linux, macOS) reddens both counts.
        about(licencesLine = null, onShow = null)
        onAllNodes(hasText("Licences"), useUnmergedTree = true).assertCountEquals(0)
        onAllNodes(hasText("Eclipse Public License", substring = true), useUnmergedTree = true).assertCountEquals(0)
    }

    @Test
    fun withShippedLicences_aboutShowsTheLine_andLicencesOpensThem() = runComposeUiTest {
        var shown = 0
        about(licencesLine = line, onShow = { shown++ })
        onNodeWithText(line, useUnmergedTree = true).assertExists()
        onNode(hasText("Licences") and hasClickAction()).performScrollTo().performClick()   // About is below the fold
        assertEquals(1, shown)
    }

    @Test
    fun severalEntries_fromTheNoticesIndex_eachOpensWithItsTexts() = runComposeUiTest {
        // A101 — the generated list: several families, each with its components and licence; one opened at a time.
        val entries = com.troubastack.app.parseNotices(
            "entry\tKtor\ncomponent\tio.ktor:ktor-io-jvm:3.0.3\tApache-2.0\tApache-2.0\t\ndoc\tApache License 2.0\ta\n" +
                "entry\tOkHttp and Okio\ncomponent\tcom.squareup.okhttp3:okhttp:4.12.0\tApache-2.0,MPL-2.0\tApache-2.0; MPL-2.0\tn\ndoc\tokhttp: NOTICE\tn\n" +
                "entry\tSkia graphics (in Skiko)\ncarrier\torg.jetbrains.skiko:skiko-awt-runtime-linux-x64:0.9.22.2\ndoc\tSkia (BSD-3-Clause)\ts\n",
        ) { mapOf("a" to "Apache License text", "n" to "publicsuffixes.gz is MPL-2.0", "s" to "Copyright (c) 2011 Google Inc.")[it] }
        var opened by mutableStateOf<LicenceEntry?>(null)
        licences(entries, { opened }, { opened = it }, {})
        for (name in listOf("Ktor", "OkHttp and Okio", "Skia graphics (in Skiko)")) onNode(hasText(name) and hasClickAction()).assertExists()
        onNode(hasText("OkHttp and Okio") and hasClickAction()).performClick()
        onNodeWithText("publicsuffixes.gz is MPL-2.0", useUnmergedTree = true).assertExists()
        onNodeWithText("com.squareup.okhttp3:okhttp 4.12.0 — Apache-2.0; MPL-2.0", useUnmergedTree = true).assertExists()
        onAllNodes(hasText("Apache License text"), useUnmergedTree = true).assertCountEquals(0)   // only the opened entry's texts
        onNode(hasText("‹  Back") and hasClickAction()).performClick()
        onNode(hasText("Skia graphics (in Skiko)") and hasClickAction()).performClick()
        onNodeWithText("Copyright (c) 2011 Google Inc.", useUnmergedTree = true).assertExists()
        onNodeWithText("org.jetbrains.skiko:skiko-awt-runtime-linux-x64 0.9.22.2", useUnmergedTree = true).assertExists()
    }

    @Test
    fun theLicencesScreen_listsEntries_opensOne_andBackStepsToTheListFirst() = runComposeUiTest {
        val swt = LicenceEntry("SWT 3.134.0", listOf(LicenceDocument("SWT in this app", "Version: 3.134.0"), LicenceDocument("Eclipse Public License 2.0", "Eclipse Public License - v 2.0")))
        var opened by mutableStateOf<LicenceEntry?>(null)
        var left = 0
        licences(listOf(swt), { opened }, { opened = it }, { left++ })
        onNodeWithText("Open-source licences", useUnmergedTree = true).assertExists()
        onAllNodes(hasText("Eclipse Public License - v 2.0"), useUnmergedTree = true).assertCountEquals(0)   // texts only once opened
        onNode(hasText("SWT 3.134.0") and hasClickAction()).performClick()
        assertEquals(swt, opened)
        onNodeWithText("Eclipse Public License - v 2.0", useUnmergedTree = true).assertExists()
        onNode(hasText("‹  Back") and hasClickAction()).performClick()
        assertNull(opened, "Back from an entry returns to the list")
        assertEquals(0, left)
        onNode(hasText("‹  Back") and hasClickAction()).performClick()
        assertEquals(1, left, "Back from the list leaves")
    }
}
