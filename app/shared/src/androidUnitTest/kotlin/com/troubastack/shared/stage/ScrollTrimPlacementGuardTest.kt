package com.troubastack.shared.stage

import java.io.File
import kotlin.test.Test
import kotlin.test.assertTrue

/**
 * ⟨R1⟩ (Fable, on e2f5f0a0) — a SOURCE guard on the scroll trim COMPOSITION, not just the arithmetic.
 *
 * The clipped-title regression was a Compose composition, not a formula:
 * `Box(Modifier.height(trimmed).clipToBounds()) { Box(Modifier.requiredHeight(full)) { page } }` centres
 * the oversized child regardless of alignment, so the clip cut the song title off the TOP. A pure-function
 * test on [scrollTrimPlacement] cannot catch a revert to that form — every such test stays green while the
 * title clips again (the same blind spot [scrollTrimFraction]'s tests had one level along).
 *
 * So this guards the surface: the scroll trim page must (a) route its placement through
 * [scrollTrimPlacement], and (b) contain no `requiredHeight(` — its only use here was the centring
 * regression; a top-anchored clip uses a Layout that places the raster at y=0. Reverting the composition to
 * the nested-Box form removes (a) and reintroduces (b), so the suite reddens.
 *
 * Mirrors [NoRawChromeSurfaceTest]: JVM sourceset so it can read the file; fails on the pattern, not a pixel.
 */
class ScrollTrimPlacementGuardTest {
    /** StageScreen source with `//` line comments stripped, so the comment that *names* the regression
     *  (`requiredHeight(full)`) doesn't trip the guard — only real code counts. */
    private fun stageScreenCode(): String =
        readStageScreen().lineSequence().joinToString("\n") { it.substringBefore("//") }

    @Test
    fun scroll_trim_branch_routes_through_scrollTrimPlacement() {
        assertTrue(
            "scrollTrimPlacement(" in stageScreenCode(),
            "the SCROLL trim page must place via scrollTrimPlacement(); a revert to the nested " +
                "requiredHeight/clipToBounds Box centres the oversized raster and clips the song title off " +
                "the top (the clipped-title regression, GO 10354b48).",
        )
    }

    @Test
    fun stage_screen_has_no_requiredHeight_composition() {
        val offenders = stageScreenCode().lineSequence().withIndex()
            .filter { (_, l) -> "requiredHeight(" in l }
            .map { (i, l) -> "L${i + 1}: ${l.trim()}" }
            .toList()
        assertTrue(
            offenders.isEmpty(),
            "requiredHeight( in StageScreen.kt reintroduces the oversized-child-in-a-clipped-box shape that " +
                "Compose centres (clipping the title). Place the raster at y=0 via a Layout instead. Offenders: $offenders",
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
