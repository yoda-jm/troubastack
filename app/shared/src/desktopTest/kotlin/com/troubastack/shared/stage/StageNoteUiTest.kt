package com.troubastack.shared.stage

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.isRoot
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.performMouseInput
import androidx.compose.ui.test.runComposeUiTest
import androidx.compose.ui.unit.dp
import com.troubastack.app.FileRehearsalNotes
import com.troubastack.app.SkiaNoteBitmapCodec
import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.PageImages
import java.io.File
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * WIN03 — a note drawn with the MOUSE on the desktop is saved, and the index comes back to Stage (the ✎ badge, the
 * counts, A77's send). The save hops back with Dispatchers.Main: before kotlinx-coroutines-swing was on the desktop,
 * every pen-up threw "Module with the Main dispatcher is missing" and the index never came back.
 */
@OptIn(ExperimentalTestApi::class)
class StageNoteUiTest {
    @Test
    fun aMouseStroke_inNoteMode_isSaved_andTheIndexComesBackToStage() = runComposeUiTest {
        val root = Files.createTempDirectory("stage-note").toFile()
        try {
            val notes = FileRehearsalNotes(File(root, "notes").path, SkiaNoteBitmapCodec)
            val vm = StageViewModel(
                LoadResult.Loaded(
                    ConcertBundle(concertId = "c1", songs = listOf(BakedSong(songId = "a", pages = listOf(PageImages(pageRasterRef = "a-p1.png", rasterHash = "h1"))))),
                    emptyList(),
                ),
            )
            val decoder = ImageDecoder { _, w, h -> Result.success(ImageBitmap(w.coerceAtLeast(1), h.coerceAtLeast(1))) }
            setContent {
                Box(Modifier.size(600.dp, 800.dp)) { StageScreen(vm, decoder, onExit = {}, notes = notes, concertId = "c1", noteNow = { 1L }) }
            }
            waitForIdle()
            vm.requestNoteMode()
            vm.confirmNoteMode()
            waitForIdle()
            onAllNodes(isRoot()).onFirst().performMouseInput {
                press(); moveTo(Offset(150f, 300f)); repeat(10) { moveBy(Offset(20f, 4f)) }; release()
            }
            waitUntil(timeoutMillis = 5_000) { vm.state.value.notes.isNotEmpty() }
            assertEquals(1, notes.index("c1").size, "the stroke was saved")
            assertEquals(1, vm.state.value.notes.size, "and the index came back to Stage")
        } finally {
            root.deleteRecursively()
        }
    }
}
