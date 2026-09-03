package com.troubastack.shared.stage

import androidx.compose.ui.graphics.Color
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A64 — the annotation-colour rule, asserted over the REAL palette (COLOR_SWATCHES + CUE_PALETTE, the
 * eleven distinct user-accessible colours) and the REAL scheme matrices (paper/ink are white/black
 * through StageColorMode). Thresholds are asserted, not eyeballed hex, so the test survives a solver
 * tweak while still guarding the guarantees: a code keeps its hue and clears legibility; achromatic ink
 * inverts and stays legible; Warm never muddies black. Includes the discriminating vector the spec
 * requires — today's straight inversion of red must FAIL the identity the rule's output passes.
 */
class AnnotationColorTest {

    // The eleven distinct colours: COLOR_SWATCHES (web/studio/src/editor.ts) ∪ CUE_PALETTE
    // (MyCuesEditor.tsx). Ten chromatic codes + one achromatic near-black.
    private val red = Color(0xFFE11D48)
    private val blue = Color(0xFF2563EB)
    private val emerald = Color(0xFF059669)
    private val amber = Color(0xFFF59E0B)
    private val violet = Color(0xFF7C3AED)
    private val orange = Color(0xFFEA580C)
    private val amberDark = Color(0xFFD97706)
    private val green = Color(0xFF16A34A)
    private val teal = Color(0xFF0D9488)
    private val pink = Color(0xFFDB2777)
    private val nearBlack = Color(0xFF111827) // Tailwind gray-900 — a desaturated navy, but INK

    private val chromatic = listOf(red, blue, emerald, amber, violet, orange, amberDark, green, teal, pink)
    private val darkSchemes = listOf(StageColorMode.NIGHT, StageColorMode.AMBER)

    // --- classification (clause 1/2 gate) ---------------------------------------------------------

    @Test
    fun nearBlack_classifiesAsInk_theCaseTheFirstDraftGotWrong() {
        // The spec's warning: HLS saturation would call #111827 a "colour" (S≈0.39) and refuse to invert
        // it. Lab chroma correctly calls it ink (≈11.5 < 20).
        assertTrue(chroma(nearBlack) < CHROMA_INK_THRESHOLD, "near-black must be INK, chroma=${chroma(nearBlack)}")
    }

    @Test
    fun everyCodeIsAboveTheChromaGate() {
        // The gate sits in an empty band (11.5 .. 35.3): every real code is comfortably a code.
        for (c in chromatic) {
            assertTrue(chroma(c) >= CHROMA_INK_THRESHOLD, "chromatic code below gate: $c chroma=${chroma(c)}")
        }
    }

    // --- clause 2: codes keep their identity AND clear legibility on dark grounds ------------------

    @Test
    fun onDarkGrounds_everyCode_keepsHue_clearsContrast_andClearsDeltaE() {
        for (scheme in darkSchemes) {
            val paper = schemePaper(scheme)
            val ink = schemePrintedInk(scheme)
            for (c in chromatic) {
                val out = annotationInk(c, scheme)
                assertTrue(
                    contrastRatio(out, paper) >= MIN_MARK_CONTRAST,
                    "$scheme code $c → $out contrast ${contrastRatio(out, paper)} < $MIN_MARK_CONTRAST",
                )
                assertTrue(
                    deltaE76(out, ink) >= MIN_INK_DELTA_E,
                    "$scheme code $c → $out ΔE ${deltaE76(out, ink)} < $MIN_INK_DELTA_E vs ink",
                )
                assertTrue(
                    hueDistance(out, c) <= 25f,
                    "$scheme code $c → $out shifted hue by ${hueDistance(out, c)}° (identity lost)",
                )
            }
        }
    }

    // --- clause 1: achromatic ink inverts and stays legible on dark grounds ------------------------

    @Test
    fun onDarkGrounds_nearBlackInk_invertsLight_andClearsContrast() {
        for (scheme in darkSchemes) {
            val paper = schemePaper(scheme)
            val out = annotationInk(nearBlack, scheme)
            // It inverted: near-black ink is now LIGHTER than the dark paper.
            assertTrue(relativeLuminance(out) > relativeLuminance(paper), "$scheme near-black did not invert: $out")
            assertTrue(contrastRatio(out, paper) >= MIN_MARK_CONTRAST, "$scheme near-black contrast ${contrastRatio(out, paper)}")
        }
    }

    // --- the discriminating vector (spec: today's inversion of red MUST fail what the rule passes) --

    @Test
    fun discriminating_todaysNightInversionOfRed_failsTheIdentity_ruleOutputPasses() {
        // TODAY: the naive path applied the page matrix to the ink too — red → teal in Night.
        val today = applySchemeMatrix(red, StageColorMode.NIGHT)
        assertTrue(hueDistance(today, red) >= 90f, "today's inversion should scramble red's hue: was ${hueDistance(today, red)}°")

        // THE RULE: red stays red.
        val ruled = annotationInk(red, StageColorMode.NIGHT)
        assertTrue(hueDistance(ruled, red) <= 25f, "rule must keep red's hue: was ${hueDistance(ruled, red)}°")

        // The exact test the rule passes and today fails: hue preserved within tolerance.
        val identity = { c: Color -> hueDistance(c, red) <= 25f }
        assertTrue(identity(ruled), "rule output must pass the identity test")
        assertTrue(!identity(today), "today's inversion must FAIL the identity test (else the test guards nothing)")
    }

    // --- Warm's load-bearing property: black stays black ------------------------------------------

    @Test
    fun warm_blackStaysBlack() {
        val out = annotationInk(Color.Black, StageColorMode.WARM)
        assertEquals(0f, out.red, "Warm must not muddy black (R)")
        assertEquals(0f, out.green, "Warm must not muddy black (G)")
        assertEquals(0f, out.blue, "Warm must not muddy black (B)")
    }

    // --- clause 2 leaves light grounds alone (codes were authored for white paper) -----------------

    @Test
    fun onLightGrounds_codesAreUntouched() {
        for (scheme in listOf(StageColorMode.NORMAL, StageColorMode.WARM)) {
            for (c in chromatic) {
                val out = annotationInk(c, scheme)
                assertEquals(c.red, out.red, 1e-4f, "$scheme must leave code $c red untouched")
                assertEquals(c.green, out.green, 1e-4f, "$scheme must leave code $c green untouched")
                assertEquals(c.blue, out.blue, 1e-4f, "$scheme must leave code $c blue untouched")
            }
        }
    }

    // --- clause 3: highlight alpha drops on dark grounds, unchanged on light -----------------------

    @Test
    fun highlightAlpha_dropsOnDarkGrounds_only() {
        assertEquals(HIGHLIGHT_ALPHA_LIGHT, highlightFillAlpha(StageColorMode.NORMAL))
        assertEquals(HIGHLIGHT_ALPHA_LIGHT, highlightFillAlpha(StageColorMode.WARM))
        assertEquals(HIGHLIGHT_ALPHA_DARK, highlightFillAlpha(StageColorMode.NIGHT))
        assertEquals(HIGHLIGHT_ALPHA_DARK, highlightFillAlpha(StageColorMode.AMBER))
    }

    // --- part 2 core: the per-pixel overlay transform (pure, mechanism-agnostic) ------------------

    @Test
    fun transformOverlayPixel_transparentPassesThrough() {
        // A fully-transparent pixel must stay exactly transparent — never tint the invisible margins.
        assertEquals(0x00000000, transformOverlayPixel(0x00000000, StageColorMode.NIGHT))
        assertEquals(0x00123456, transformOverlayPixel(0x00123456, StageColorMode.AMBER))
    }

    @Test
    fun transformOverlayPixel_preservesAlpha_andKeepsRedRed() {
        // Opaque red in Night: alpha byte preserved, hue still red (not teal).
        val out = transformOverlayPixel(0xFFE11D48.toInt(), StageColorMode.NIGHT)
        assertEquals(0xFF, (out ushr 24) and 0xFF, "alpha must survive")
        val outColor = Color(
            red = ((out ushr 16) and 0xFF) / 255f,
            green = ((out ushr 8) and 0xFF) / 255f,
            blue = (out and 0xFF) / 255f,
        )
        assertTrue(hueDistance(outColor, red) <= 25f, "overlay red must stay red: ${hueDistance(outColor, red)}°")
    }

    @Test
    fun transformOverlayPixel_partialAlphaPreservedVerbatim() {
        // A semi-transparent (highlight) pixel keeps its alpha — clause 3 is NOT applied per-pixel.
        val out = transformOverlayPixel(0x80E11D48.toInt(), StageColorMode.AMBER)
        assertEquals(0x80, (out ushr 24) and 0xFF, "partial alpha must be preserved verbatim")
    }

    // --- the paper/ink a scheme settles to ARE the real matrices applied to white/black ------------

    @Test
    fun schemePaperAndInk_matchTheRealMatrices() {
        assertEquals(Color.White, schemePaper(StageColorMode.NORMAL))
        assertEquals(Color.Black, schemePrintedInk(StageColorMode.NORMAL))
        // Night: paper black, ink white.
        assertTrue(relativeLuminance(schemePaper(StageColorMode.NIGHT)) < 0.02f)
        assertTrue(relativeLuminance(schemePrintedInk(StageColorMode.NIGHT)) > 0.9f)
        // Amber: paper black, ink amber (#FFBF73-ish) — warm, not white.
        assertTrue(relativeLuminance(schemePaper(StageColorMode.AMBER)) < 0.02f)
        val amberInk = schemePrintedInk(StageColorMode.AMBER)
        assertTrue(amberInk.red > amberInk.green && amberInk.green > amberInk.blue, "amber ink should be warm: $amberInk")
    }
}
