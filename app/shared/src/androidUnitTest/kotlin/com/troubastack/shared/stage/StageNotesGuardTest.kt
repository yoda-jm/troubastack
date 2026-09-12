package com.troubastack.shared.stage

import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A70 §4.6 / §6.3 — SOURCE guards for the presenter's FIRST write surface.
 *
 * §4.6 (required by the gate): `stage/` may reach the filesystem/network ONLY through the injected
 * `RehearsalNotes` port. No file/network import may appear in any `stage/` commonMain file — that is the
 * one door, guarded not promised, so a server dependency cannot creep into the presenter (I12's residual
 * worry). §6.3: every drag owner disarmed by `swipeLocked` is ALSO disarmed by `noteMode`, and the note
 * layer registers with the page by reusing the raster image's own modifier + sits after the overlays.
 */
class StageNotesGuardTest {

    private fun stageDir(): File {
        val cwd = System.getProperty("user.dir")
        for (base in listOf(
            "src/commonMain/kotlin/com/troubastack/shared/stage",
            "shared/src/commonMain/kotlin/com/troubastack/shared/stage",
            "app/shared/src/commonMain/kotlin/com/troubastack/shared/stage",
        )) {
            val d = File(cwd, base)
            if (d.isDirectory) return d
        }
        error("could not locate the stage/ source dir under $cwd")
    }

    private fun stageFile(name: String) = File(stageDir(), name).readText()

    // I/O doors that must never appear in stage/: only the RehearsalNotes port may do file/network work.
    private val forbiddenImport = Regex("""^\s*import\s+(java\.io\.|java\.nio\.|okio\.|java\.net\.|io\.ktor\.|android\.(content|net|database|provider)\.|java\.security\.)""")

    @Test
    fun stage_reaches_io_only_through_the_rehearsal_notes_port() {
        val offenders = mutableListOf<String>()
        val scanned = mutableListOf<String>()
        stageDir().walkTopDown().filter { it.isFile && it.extension == "kt" }.forEach { f ->
            scanned += f.name
            f.readText().lineSequence().withIndex().forEach { (i, line) ->
                if (forbiddenImport.containsMatchIn(line)) offenders += "${f.name} L${i + 1}: ${line.trim()}"
            }
        }
        // POSITIVE CONTROL (Fable's gate ruling): an empty offender list is only evidence if the walk actually
        // SAW the stage sources. Without this, a mis-resolved dir yields zero files → zero offenders → green,
        // and the presenter's first write surface goes unguarded under a passing test forever. Assert the walk
        // found a plausible number of stage/ files AND the largest one specifically.
        assertTrue(scanned.size >= 15, "the I/O guard walk saw only ${scanned.size} .kt files — it is not scanning stage/ (${stageDir()})")
        assertTrue("StageScreen.kt" in scanned, "the I/O guard walk did not see StageScreen.kt — wrong dir? scanned=$scanned")
        assertEquals(
            emptyList(), offenders,
            "a stage/ file reaches I/O directly — route it through the RehearsalNotes port (A70 §4.6). Offenders:\n" + offenders.joinToString("\n"),
        )
    }

    @Test
    fun note_pad_reads_the_finger_directly_not_via_the_scroll_detectors() {
        // A71 §5.2 — the pad must not use Compose's slop-based drag/tap detectors (they discard the first
        // ~1.5 mm and report a stab at finger-up). POSITIVE CONTROL: assert the file we read actually contains
        // StrokeReader — an empty "no detector" result from the wrong/empty file is not evidence.
        val src = stageFile("NotePad.kt")
        assertTrue("StrokeReader" in src, "read the wrong NotePad.kt — it does not mention StrokeReader (positive control)")
        assertTrue("detectDragGestures" !in src, "NotePad.kt still uses detectDragGestures — it eats the touch slop (A71 §3)")
        assertTrue("detectTapGestures" !in src, "NotePad.kt still uses detectTapGestures — a stab reports the finger-up point (A71 §3)")
    }

    @Test
    fun every_swipeLocked_drag_owner_is_also_noteMode_gated() {
        val src = stageFile("StageScreen.kt")
        // The turn-swipe condition names both swipeLocked and noteMode.
        assertTrue(
            Regex("""scrollMode\s*\|\|\s*state\.swipeLocked\s*\|\|\s*state\.noteMode""").containsMatchIn(src),
            "the turn-swipe must be disarmed in note mode as well as swipe-lock (§3.7)",
        )
        // stageTaps is dropped in note mode.
        assertTrue(
            Regex("""if\s*\(state\.noteMode\)\s*Modifier\s*else\s*Modifier\.stageTaps""").containsMatchIn(src),
            "stageTaps (chrome toggle) must be dropped in note mode (§3.7)",
        )
        // FIT_WIDTH's vertical scroll is off in note mode.
        assertTrue(
            Regex("""FitMode\.FIT_WIDTH\s*&&\s*notePad\?\.noteMode\s*!=\s*true""").containsMatchIn(src),
            "FIT_WIDTH vertical scroll must be off in note mode (§3.7)",
        )
    }

    @Test
    fun note_layer_reuses_the_raster_modifier_and_sits_after_overlays() {
        val src = stageFile("StageScreen.kt")
        val overlaysAt = src.indexOf("bitmaps.overlays.forEach")
        val noteAt = src.indexOf("NoteLayer(")
        assertTrue(overlaysAt in 0 until noteAt, "the note layer must be composed AFTER the overlays (§3.5 #11)")
        // NoteLayer is handed imageMod (the raster image's own modifier) — the registration mechanism (§2).
        assertTrue(
            Regex("""imageMod\s*=\s*imageMod""").containsMatchIn(src),
            "the note layer must reuse the raster image's modifier so it registers with the page (§2)",
        )
    }
}
