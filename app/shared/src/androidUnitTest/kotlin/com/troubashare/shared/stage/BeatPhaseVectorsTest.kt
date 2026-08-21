package com.troubashare.shared.stage

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A34 — the visual-beat CONTRACT (cross-lane). Runs the SAME cases the Studio runs
 * (`docs/contracts/beat-phase.vectors.json`, authored by T85, mirrored into commonTest resources)
 * against the Kotlin [beatPhase], so "when is a beat" is a tested invariant across the two runtimes,
 * not a hope (the view-resolution / glyphs.json pattern). Five cases are the 90 bpm (666.67)
 * truncation guard — exactly the app bug that started this. A CI job diffs the two file copies.
 *
 * JVM-only (real filesystem for the resource), hence androidUnitTest — [beatPhase] is pure commonMain,
 * so this equally covers the iOS runtime.
 */
class BeatPhaseVectorsTest {

    @Serializable private data class Spec(val cases: List<Case>)
    @Serializable private data class Case(
        val elapsedMs: Double,
        val intervalMs: Double,
        val beats: Int,
        val beatIndex: Int,
        val lit: Boolean,
        val emphasis: Boolean,
    )

    @Test
    fun beatPhase_matchesTheSharedVectors() {
        val spec = Json { ignoreUnknownKeys = true }.decodeFromString(Spec.serializer(), readVectors())
        assertTrue(spec.cases.isNotEmpty(), "no vector cases — the contract file is empty")
        for (c in spec.cases) {
            val got = beatPhase(c.elapsedMs, c.intervalMs, c.beats)
            val where = "@ elapsed=${c.elapsedMs} interval=${c.intervalMs}"
            assertEquals(c.beatIndex, got.beatIndex, "beatIndex $where")
            assertEquals(c.lit, got.lit, "lit $where")
            assertEquals(c.emphasis, got.emphasis, "emphasis $where")
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
