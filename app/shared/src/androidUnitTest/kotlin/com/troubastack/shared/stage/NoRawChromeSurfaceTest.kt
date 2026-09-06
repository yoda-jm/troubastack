package com.troubastack.shared.stage

import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * A69 Ruling 3 — a SOURCE-level guard: no Stage chrome may take a raw M3 `colorScheme.surface` /
 * `.secondaryContainer` / `.surfaceVariant` for a container that can cover the page. Every such surface must
 * route through [stageChrome]/[stageChromePalette] so it follows the reading scheme; otherwise "we handled
 * the surfaces" is true only for the moment, and the next surface added is a white slab on a black stage
 * again (the drawer, the two sheets, the two dialogs and the update-notice were exactly that before A69).
 *
 * This mirrors the studio's no-raw-hex guard: cheap, and it fails on the pattern rather than on a pixel.
 * The one sanctioned exception — the baseline reads inside `stageChrome`/`PlaceholderCard` — uses an aliased
 * `cs.` receiver, which this grep intentionally does not see. JVM sourceset so it can read the file.
 */
class NoRawChromeSurfaceTest {
    private val forbidden = Regex("""colorScheme\.(surface|secondaryContainer|surfaceVariant)\b""")

    @Test
    fun stage_screen_has_no_raw_page_covering_chrome_token() {
        val src = readStageScreen()
        val offenders = src.lineSequence().withIndex()
            .filter { (_, line) -> forbidden.containsMatchIn(line) }
            .map { (i, line) -> "L${i + 1}: ${line.trim()}" }
            .toList()
        assertEquals(
            emptyList(), offenders,
            "raw M3 chrome tokens on Stage surface(s) — route them through stageChrome()/stageChromePalette() so they follow the reading scheme (A69 Ruling 3)",
        )
    }

    private fun readStageScreen(): String {
        val name = "StageScreen.kt"
        val cwd = System.getProperty("user.dir")
        for (base in listOf(
            "src/commonMain/kotlin/com/troubastack/shared/stage",
            "shared/src/commonMain/kotlin/com/troubastack/shared/stage",
            "app/shared/src/commonMain/kotlin/com/troubastack/shared/stage",
        )) {
            val f = File(cwd, "$base/$name")
            if (f.isFile) return f.readText()
        }
        error("could not locate $name under $cwd")
    }
}
