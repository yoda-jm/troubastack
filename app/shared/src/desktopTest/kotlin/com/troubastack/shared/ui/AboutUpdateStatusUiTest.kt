package com.troubastack.shared.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.ProgressBarRangeInfo
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.runComposeUiTest
import com.troubastack.shared.buildinfo.UpdatePhase
import com.troubastack.shared.stage.FitMode
import com.troubastack.shared.stage.StageColorMode
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * A96 — Parameters → About during an update shows where it is, as Home's banner does: the phase's line, the bar, and
 * Cancel while the app owns the step — and no Update to press again (VLL pressed it in About and saw nothing).
 */
@OptIn(ExperimentalTestApi::class)
class AboutUpdateStatusUiTest {
    private val bar = SemanticsMatcher.keyIsDefined(SemanticsProperties.ProgressBarRangeInfo)

    private fun ComposeUiTest.about(phase: () -> UpdatePhase?, onCancel: () -> Unit = {}) = setContent {
        SettingsScreen(
            ThemePref.SYSTEM, {}, FitMode.FIT_PAGE, {}, StageColorMode.NORMAL, {},
            latestLine = "Latest: 38a131a1 · built 2026-11-03 18:54 UTC",
            onCheckForUpdates = {},
            onUpdateApp = {},
            updatePhase = phase(),
            onCancelUpdate = onCancel,
            onBack = {},
        )
    }

    private fun ComposeUiTest.count(text: String) = onAllNodes(hasText(text), useUnmergedTree = true)

    @Test
    fun whileDownloading_aboutShowsTheStatusTheBarAndCancel_notUpdate() = runComposeUiTest {
        // TEETH: remove About's phase branch and the latest-release row (with Update) shows instead: red.
        var cancelled = 0
        about({ UpdatePhase.Downloading(0.4f) }, onCancel = { cancelled++ })
        onNodeWithText("Downloading the app update… 40 %", useUnmergedTree = true).assertExists()
        onAllNodes(SemanticsMatcher.expectValue(SemanticsProperties.ProgressBarRangeInfo, ProgressBarRangeInfo(0.4f, 0f..1f))).assertCountEquals(1)
        count("Update").assertCountEquals(0)
        count("Check for updates").assertCountEquals(0)
        count("Latest: 38a131a1 · built 2026-11-03 18:54 UTC").assertCountEquals(0)
        onNode(hasText("Cancel") and hasClickAction()).performScrollTo().performClick() // About is below the fold
        assertEquals(1, cancelled)
    }

    @Test
    fun eachPhase_inAbout() = runComposeUiTest {
        mainClock.autoAdvance = false // Checking's bar is indeterminate: an animation that never ends
        var phase by mutableStateOf<UpdatePhase?>(UpdatePhase.Checking)
        about({ phase })
        mainClock.advanceTimeBy(100)
        onNodeWithText("Checking the download…", useUnmergedTree = true).assertExists()
        onAllNodes(SemanticsMatcher.expectValue(SemanticsProperties.ProgressBarRangeInfo, ProgressBarRangeInfo.Indeterminate)).assertCountEquals(1)
        count("Cancel").assertCountEquals(1)

        phase = UpdatePhase.Confirming(reopenYourself = false)
        mainClock.advanceTimeBy(100)
        onNodeWithText("Android will ask you to confirm the update.", useUnmergedTree = true).assertExists()
        onAllNodes(bar).assertCountEquals(0) // Android owns this step: no bar
        count("Cancel").assertCountEquals(0) // and nothing of ours to cancel
        count("Update").assertCountEquals(0)

        phase = UpdatePhase.Confirming(reopenYourself = true)
        mainClock.advanceTimeBy(100)
        onNodeWithText("Android will ask you to confirm the update. Then open TroubaStage again.", useUnmergedTree = true).assertExists()

        phase = null // not installed (or cancelled): back to Update
        mainClock.advanceTimeBy(100)
        count("Update").assertCountEquals(1)
        onNodeWithText("Latest: 38a131a1 · built 2026-11-03 18:54 UTC", useUnmergedTree = true).assertExists()
        onAllNodes(bar).assertCountEquals(0)
    }
}
