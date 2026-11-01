package com.troubastack.shared.stage

import androidx.compose.foundation.layout.Box
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.foundation.layout.size
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.click
import androidx.compose.ui.test.isRoot
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performKeyInput
import androidx.compose.ui.test.performMouseInput
import androidx.compose.ui.test.pressKey
import androidx.compose.ui.test.rightClick
import androidx.compose.ui.test.runComposeUiTest
import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * WIN03 acceptance 4 — Stage driven with a MOUSE, through the real StageScreen on the desktop. The clock is advanced
 * by hand, so the chrome's auto-hide never races a click.
 */
@OptIn(ExperimentalTestApi::class)
class StageMouseUiTest {
    private fun vm() = StageViewModel(
        LoadResult.Loaded(
            ConcertBundle(
                concertId = "c1",
                songs = listOf(
                    BakedSong(songId = "a", pages = (1..3).map { PageImages(pageRasterRef = "a-p$it.png") }),
                    BakedSong(songId = "b", pages = listOf(PageImages(pageRasterRef = "b-p1.png"))),
                ),
            ),
            emptyList(),
        ),
    )

    private val decoder = ImageDecoder { _, w, h -> Result.success(ImageBitmap(w.coerceAtLeast(1), h.coerceAtLeast(1))) }

    /**
     * Stage in a PORTRAIT box, so page mode shows one page (landscape is two-up, A12) and a turn is one page. It is
     * entered the way a performer enters it, by clicking Open: a fresh test scene has no focus at all until it is
     * clicked (measured), where the app's window has it.
     */
    private fun ComposeUiTest.stage(vm: StageViewModel, onExit: () -> Unit = {}) {
        mainClock.autoAdvance = false
        var open by mutableStateOf(false)
        setContent {
            Box(Modifier.size(600.dp, 800.dp)) {
                if (open) StageScreen(vm, decoder, onExit = onExit) else Button(onClick = { open = true }) { Text("Open") }
            }
        }
        onNodeWithText("Open").performMouseInput { click() }
        mainClock.advanceTimeBy(500)
    }

    private fun ComposeUiTest.pageDown() {
        onAllNodes(isRoot()).onFirst().performKeyInput { pressKey(Key.PageDown) }
        mainClock.advanceTimeBy(500)
    }

    @Test
    fun onEntry_pageDownTurns() = runComposeUiTest {
        val vm = vm()
        stage(vm)
        pageDown()
        assertEquals(1, vm.state.value.current, "PageDown turns as soon as Stage opens")
    }

    @Test
    fun afterClickingAButtonThatOpensNothing_pageDownStillTurns() = runComposeUiTest {
        // A mouse click focuses the button clicked (measured on ⚙); › opens no overlay, so A50's refocus never runs.
        val vm = vm()
        stage(vm)
        onNodeWithText("›").performMouseInput { click() }
        mainClock.advanceTimeBy(500)
        assertEquals(1, vm.state.value.current, "› turned")
        pageDown()
        assertEquals(2, vm.state.value.current, "PageDown turned after a click on ›")
    }

    @Test
    fun afterAClickOnThePage_pageDownStillTurns() = runComposeUiTest {
        val vm = vm()
        stage(vm)
        clickPage()
        pageDown()
        assertEquals(1, vm.state.value.current, "PageDown turned after a click on the page")
    }

    private fun ComposeUiTest.chromeShown(): Boolean =
        onAllNodes(androidx.compose.ui.test.hasText("⚙")).fetchSemanticsNodes().isNotEmpty()

    private fun ComposeUiTest.clickPage() {
        onAllNodes(isRoot()).onFirst().performMouseInput { click(Offset(width / 2f, height / 2f)) }
        mainClock.advanceTimeBy(500)
    }

    @Test
    fun theCloseButton_leavesTheConcert() = runComposeUiTest {
        var exited = false
        stage(vm(), onExit = { exited = true })
        onNodeWithText("✕").performMouseInput { click() }
        mainClock.advanceTimeBy(500)
        assertTrue(exited, "✕ clicked with the mouse calls onExit")
    }

    @Test
    fun aClickOnThePage_togglesTheChrome() = runComposeUiTest {
        stage(vm())
        assertTrue(chromeShown(), "the chrome is shown on entry")
        clickPage()
        assertEquals(false, chromeShown(), "a click hides it")
        clickPage()
        assertTrue(chromeShown(), "a second click shows it again")
    }

    @Test
    fun afterClickingAChromeButton_pageDownStillTurns() = runComposeUiTest {
        val vm = vm()
        stage(vm)
        onNodeWithText("⚙").performMouseInput { click() }
        mainClock.advanceTimeBy(800)
        onAllNodes(isRoot()).onLast().performKeyInput { pressKey(Key.Escape) } // the sheet (its own root) closes on Esc
        mainClock.advanceTimeBy(800)
        pageDown()
        assertEquals(1, vm.state.value.current, "PageDown turned after a click on ⚙")
    }

    /** One wheel gesture: [events] wheel events [everyMs] apart, each [delta] notches (positive = down). */
    private fun ComposeUiTest.wheelBurst(events: Int, delta: Float = 1f, everyMs: Long = 16) {
        onAllNodes(isRoot()).onFirst().performMouseInput {
            moveTo(Offset(width / 2f, height / 2f))
            repeat(events) { scroll(delta); advanceEventTime(everyMs) }
        }
        mainClock.advanceTimeBy(600)
    }

    @Test
    fun oneLongWheelBurst_turnsExactlyOnePage_aSecondBurstTurnsAnother_andUpGoesBack() = runComposeUiTest {
        // TEETH: a wheel that turns per event moves 40 pages (clamped at the end), not one.
        val vm = vm()
        stage(vm)
        wheelBurst(40)
        assertEquals(1, vm.state.value.current, "one burst of 40 events turned one page")
        wheelBurst(40)
        assertEquals(2, vm.state.value.current, "a second burst after the gap turned one more")
        wheelBurst(5, delta = -1f)
        assertEquals(1, vm.state.value.current, "a burst upward turned back one")
    }

    @Test
    fun lockSwipe_locksTheWheel_too() = runComposeUiTest {
        val vm = vm()
        vm.toggleSwipeLock()
        stage(vm)
        wheelBurst(10)
        assertEquals(0, vm.state.value.current, "a locked Stage does not turn on the wheel")
        pageDown()
        assertEquals(1, vm.state.value.current, "keys still turn while locked")
    }

    @Test
    fun aRightClick_doesNothing() = runComposeUiTest {
        val vm = vm()
        stage(vm)
        onAllNodes(isRoot()).onFirst().performMouseInput { rightClick(Offset(width / 2f, height / 2f)) }
        mainClock.advanceTimeBy(500)
        assertTrue(chromeShown(), "a right-click does not toggle the chrome")
        assertEquals(0, vm.state.value.current)
    }
}
