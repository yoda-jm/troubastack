package com.troubastack.shared.stage

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A34 — the visual-beat CONTRACT (cross-lane). Runs the SAME cases the Studio runs
 * (`docs/contracts/beat-phase.vectors.json`, authored by T85, mirrored into commonTest resources)
 * against the Kotlin [beatPhase], so "when is a beat, and what TIER" is a tested invariant across the
 * two runtimes, not a hope (the view-resolution / glyphs.json pattern). Five cases are the 90 bpm
 * (666.67) truncation guard — exactly the app bug that started this. T86 adds metre cases carrying
 * `groups` + `tier` (3/4, 6/8, 12/8, additive); the 4/4 cases have neither and assert no tier — the
 * backward-compat proof that pre-T86 bundles beat unchanged. A CI job diffs the two file copies.
 *
 * JVM-only (real filesystem for the resource), hence androidUnitTest — [beatPhase] is pure commonMain,
 * so this equally covers the iOS runtime.
 */
class BeatPhaseVectorsTest {

    // The JSON carries a mix: 4/4 cases (no `groups`, no `tier`) and T86 metre cases (both present).
    // A null `groups` decodes as the 4/4 default; a null `tier` means "this vector doesn't pin a tier".
    // The comment/marker entries carry no `elapsedMs`, so those fields are nullable and such rows skipped.
    @Serializable private data class Spec(val cases: List<Case>)
    @Serializable private data class Case(
        val elapsedMs: Double? = null,
        val intervalMs: Double? = null,
        val beats: Int? = null,
        val groups: List<Int>? = null,
        val beatIndex: Int? = null,
        val lit: Boolean? = null,
        val tier: Int? = null,
        val emphasis: Boolean? = null,
    )

    @Test
    fun beatPhase_matchesTheSharedVectors() {
        val spec = Json { ignoreUnknownKeys = true }.decodeFromString(Spec.serializer(), readVectors())
        val cases = spec.cases.filter { it.elapsedMs != null } // drop the `_comment` / marker rows
        assertTrue(cases.size >= 24, "too few vector cases (${cases.size}) — the metre cases went missing")
        for (c in cases) {
            val groups = c.groups ?: DEFAULT_GROUPS
            val got = beatPhase(c.elapsedMs!!, c.intervalMs!!, c.beats!!, groups)
            val where = "@ elapsed=${c.elapsedMs} interval=${c.intervalMs} groups=$groups"
            assertEquals(c.beatIndex, got.beatIndex, "beatIndex $where")
            assertEquals(c.lit, got.lit, "lit $where")
            assertEquals(c.emphasis, got.emphasis, "emphasis $where")
            if (c.tier != null) assertEquals(c.tier, got.tier, "tier $where") // pinned only on metre cases
        }
    }

    private fun readVectors(): String {
        val name = "beat-phase.vectors.json"
        javaClass.classLoader?.getResource(name)?.let { res ->
            if (res.protocol == "file") return File(res.toURI()).readText()
            res.openStream().use { return it.readBytes().decodeToString() }
        }
        val cwd = System.getProperty("user.dir")
        for (base in listOf("src/commonTest/resources", "shared/src/commonTest/resources", "app/shared/src/commonTest/resources")) {
            val f = File(cwd, "$base/$name")
            if (f.isFile) return f.readText()
        }
        error("could not locate $name on the test classpath or under $cwd")
    }
}
