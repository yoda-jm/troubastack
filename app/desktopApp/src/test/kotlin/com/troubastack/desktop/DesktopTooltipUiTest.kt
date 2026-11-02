package com.troubastack.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.performMouseInput
import androidx.compose.ui.test.runComposeUiTest
import androidx.compose.ui.unit.dp
import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import com.troubastack.shared.stage.ImageDecoder
import com.troubastack.shared.stage.LocalGlyphTooltip
import com.troubastack.shared.stage.StageScreen
import com.troubastack.shared.stage.StageViewModel
import kotlin.test.Test

/**
 * The accessible-names follow-up must leave WIN03's desktop tooltips as they were: hovering a glyph control still shows
 * its label, through the REAL desktop tooltip (desktopGlyphTooltip), now that the label also names the control.
 */
@OptIn(ExperimentalTestApi::class)
class DesktopTooltipUiTest {
    private val decoder = ImageDecoder { _, w, h -> Result.success(ImageBitmap(w.coerceAtLeast(1), h.coerceAtLeast(1))) }

    private fun ComposeUiTest.stage() {
        val vm = StageViewModel(
            LoadResult.Loaded(
                ConcertBundle(concertId = "c1", songs = listOf(BakedSong(songId = "a", tempo = 120, meter = "4/4", pages = listOf(PageImages(pageRasterRef = "a-p1.png"))))),
                emptyList(),
            ),
        )
        mainClock.autoAdvance = false
        setContent {
            CompositionLocalProvider(LocalGlyphTooltip provides desktopGlyphTooltip) {
                Box(Modifier.size(600.dp, 800.dp)) { StageScreen(vm, decoder, onExit = {}) }
            }
        }
        mainClock.advanceTimeBy(500)
    }

    private fun ComposeUiTest.hoverShowsTooltip(label: String) {
        onAllNodes(hasText(label)).assertCountEquals(0) // no tooltip before the hover
        onNode(hasContentDescription(label)).performMouseInput { moveTo(center) }
        mainClock.advanceTimeBy(800) // past the tooltip's 500 ms delay
        onAllNodes(hasText(label)).assertCountEquals(1)
        onNode(hasContentDescription(label)).performMouseInput { moveTo(androidx.compose.ui.geometry.Offset(-200f, -200f)) }
        mainClock.advanceTimeBy(300)
    }

    @Test
    fun hoveringAStageButton_stillShowsItsTooltip() = runComposeUiTest {
        stage()
        hoverShowsTooltip("Parameters")
    }

    @Test
    fun hoveringTheKeepRunningSwitch_stillShowsItsTooltip() = runComposeUiTest {
        stage()
        hoverShowsTooltip("Keep running")
    }
}
