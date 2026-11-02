package com.troubastack.shared.stage

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.runComposeUiTest
import androidx.compose.ui.unit.dp
import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import com.troubastack.app.FileRehearsalNotes
import com.troubastack.app.SkiaNoteBitmapCodec
import java.io.File
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * WIN03 follow-up (Fable's GO 440f877d) — every glyph-only Stage control is named for a screen reader with its tooltip
 * label, ON the clickable (a name without the click action would announce a control nobody can press), and its glyph
 * is not read after the name. Run the way Android runs Stage: no tooltip provider.
 */
@OptIn(ExperimentalTestApi::class)
class StageAccessibleNamesUiTest {
    private val decoder = ImageDecoder { _, w, h -> Result.success(ImageBitmap(w.coerceAtLeast(1), h.coerceAtLeast(1))) }

    private fun ComposeUiTest.stage(root: File): StageViewModel {
        val vm = StageViewModel(
            LoadResult.Loaded(
                ConcertBundle(
                    concertId = "c1",
                    songs = listOf(BakedSong(songId = "a", tempo = 120, meter = "4/4", pages = (1..2).map { PageImages(pageRasterRef = "a-p$it.png", rasterHash = "h$it") })),
                ),
                emptyList(),
            ),
        )
        val notes = FileRehearsalNotes(File(root, "notes").path, SkiaNoteBitmapCodec)
        mainClock.autoAdvance = false // the chrome stays up: its auto-hide never runs
        setContent {
            Box(Modifier.size(600.dp, 800.dp)) {
                StageScreen(vm, decoder, onExit = {}, notes = notes, concertId = "c1", noteNow = { 1L }, fullScreen = false)
            }
        }
        mainClock.advanceTimeBy(500)
        return vm
    }

    /**
     * One node carries [label] as its name AND can be clicked — in the UNMERGED tree, so the name sits on the clickable
     * itself — and that node has no semantics children: with any (even a Text whose semantics are cleared), Android's
     * accessibility tree moves the name and the role onto separate "fake" child nodes (Compose's emitFakeNodes), and the
     * node a screen reader focuses then carries the action but not the name. Measured on the emulator.
     */
    private fun ComposeUiTest.assertNamedClickable(label: String) {
        val nodes = onAllNodes(hasContentDescription(label) and hasClickAction(), useUnmergedTree = true).fetchSemanticsNodes()
        assertEquals(1, nodes.size, "one clickable named '$label'")
        assertEquals(emptyList(), nodes[0].children.map { it.config.toString() }, "'$label' has no semantics children (Android would split its name off)")
    }

    private fun ComposeUiTest.assertGlyphsNotRead(vararg glyphs: String) {
        for (g in glyphs) onAllNodes(hasText(g), useUnmergedTree = true).assertCountEquals(0)
    }

    @Test
    fun theStageButtonsAndTheMetronome_areNamedOnTheirClickables_andTheirGlyphsAreNotRead() = runComposeUiTest {
        val root = Files.createTempDirectory("stage-a11y").toFile()
        try {
            stage(root)
            // TEETH: a name on a wrapper (or no name) leaves "☰"/"‹"/"∞" as what a screen reader says.
            for (label in listOf("Songs", "Note mode", "Full screen", "Parameters", "Leave the concert", "Previous page", "Next page", "Metronome", "Keep running")) {
                assertNamedClickable(label)
            }
            assertGlyphsNotRead("☰", "✎", "⚙", "✕", "‹", "›", "∞")
            onAllNodes(hasContentDescription("Metronome") and SemanticsMatcher.expectValue(SemanticsProperties.Role, Role.Button)).assertCountEquals(1)
        } finally {
            root.deleteRecursively()
        }
    }

    @Test
    fun keepRunning_isASwitchThatSaysItsState() = runComposeUiTest {
        val root = Files.createTempDirectory("stage-a11y").toFile()
        try {
            stage(root)
            val keepRunning = hasContentDescription("Keep running") and SemanticsMatcher.expectValue(SemanticsProperties.Role, Role.Switch)
            onAllNodes(keepRunning and SemanticsMatcher.expectValue(SemanticsProperties.ToggleableState, ToggleableState.Off)).assertCountEquals(1)
            onNode(keepRunning).performClick()
            mainClock.advanceTimeBy(100)
            onAllNodes(keepRunning and SemanticsMatcher.expectValue(SemanticsProperties.ToggleableState, ToggleableState.On)).assertCountEquals(1)
        } finally {
            root.deleteRecursively()
        }
    }

    @Test
    fun noteModesZoomChipAndItsMenu_areNamedOnTheirClickables_andTheChipsNameFollowsTheFactor() = runComposeUiTest {
        val root = Files.createTempDirectory("stage-a11y").toFile()
        try {
            val vm = stage(root)
            vm.requestNoteMode()
            vm.confirmNoteMode()
            mainClock.advanceTimeBy(500)
            assertNamedClickable("Zoom, 1×")
            // Through its click action: in this 600 dp test window a touch click at the chip's centre did not reach it
            // (the note bar scrolls horizontally); the menu it opens is what is tested here.
            onNode(hasContentDescription("Zoom, 1×") and hasClickAction(), useUnmergedTree = true).performSemanticsAction(SemanticsActions.OnClick)
            mainClock.advanceTimeBy(500)
            // The menu: − / + / 1:1 (at 1× "Zoom out" and "Zoom to actual size" are disabled, still named and clickable).
            for (label in listOf("Zoom out", "Zoom in", "Zoom to actual size")) assertNamedClickable(label)
            assertGlyphsNotRead("−", "+", "1:1", "⌕ 1×")
            onNode(hasContentDescription("Zoom in") and hasClickAction(), useUnmergedTree = true).performClick()
            mainClock.advanceTimeBy(500)
            // TEETH: the chip's NAME carries the factor (it is the only place a screen reader hears it now).
            assertNamedClickable("Zoom, 1.5×")
            onAllNodes(hasContentDescription("Zoom, 1×"), useUnmergedTree = true).assertCountEquals(0)
            assertGlyphsNotRead("⌕ 1.5×")
            // Zoomed, the overview opens with its own "–" to hide it (A86): the same rule.
            assertNamedClickable("Hide zoom overview")
            assertGlyphsNotRead("–")
        } finally {
            root.deleteRecursively()
        }
    }

    @Test
    fun noteModesPencilAndEraser_areNamedOnTheirClickables() = runComposeUiTest {
        val root = Files.createTempDirectory("stage-a11y").toFile()
        try {
            val vm = stage(root)
            vm.requestNoteMode()
            vm.confirmNoteMode()
            mainClock.advanceTimeBy(500)
            assertNamedClickable("Pencil")
            assertNamedClickable("Eraser")
            assertNamedClickable("Finish notes") // named before this change; now through the same call
            assertGlyphsNotRead("✎", "⌫")
        } finally {
            root.deleteRecursively()
        }
    }
}
