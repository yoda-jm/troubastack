package com.troubastack.shared.stage

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * T158 — the running-order NUMBERING contract, actually READ from the canonical vectors. ONE rule,
 * THREE runtimes that must never silently disagree: Go `runningorder.Numbers` (the export), TS
 * `runningOrderNumbers` (Studio), Kotlin [runningOrderNumbers] (Stage). Go and TS read
 * `docs/contracts/running-order-numbering.vectors.json` directly; this mirrors it into commonTest
 * resources (KMP resource loading needs the copy) and a CI job diffs the two, so the copy can't drift —
 * the same view-resolution / beat-phase / meter-groups pattern.
 *
 * THE RULE: a number belongs only to a main-order song — an on-call (bench) song or an intermission
 * carries none and never shifts the count. Each `expected` is the exact per-entry numbering the rule
 * yields (null = no number). [RunningOrderNumberingTest] keeps the same cases hand-transcribed as
 * documentation; THIS test is the one bound to the shared contract, so a divergence between Stage and the
 * printed sheet reddens here.
 *
 * JVM-only (real filesystem for the resource), hence androidUnitTest — [runningOrderNumbers] is pure
 * commonMain, so this equally covers the iOS runtime.
 */
class RunningOrderNumberingVectorsTest {

    @Serializable private data class Spec(val cases: List<Case>)
    @Serializable private data class Case(val name: String, val entries: List<Entry>, val expected: List<Int?>)
    @Serializable private data class Entry(val kind: String, val onCall: Boolean = false)

    @Test
    fun runningOrderNumbers_matchesTheSharedVectors() {
        val spec = Json { ignoreUnknownKeys = true }.decodeFromString(Spec.serializer(), readVectors())
        assertTrue(spec.cases.size >= 7, "too few numbering vectors (${spec.cases.size}) — the contract shrank")
        for (c in spec.cases) {
            val entries = c.entries.map { e ->
                val kind = when (e.kind) {
                    "song" -> RunningOrderKind.SONG
                    "intermission" -> RunningOrderKind.INTERMISSION
                    else -> error("unknown kind \"${e.kind}\" in case \"${c.name}\"")
                }
                RunningOrderEntry(kind, e.onCall)
            }
            assertEquals(c.expected, runningOrderNumbers(entries), c.name)
        }
    }

    @Test
    fun the_bundle_constant_is_the_contract_intermission_literal() {
        // T153 — [BAKED_KIND_INTERMISSION] is the string the baker writes and stageStateFrom reads. Pin it to
        // the shared contract so a rename can't silently make every break read as a song: the vectors (Go and
        // Kotlin both run them) must contain an entry whose `kind` IS this constant. IntermissionMapperTest
        // builds its fixtures FROM this constant, so it can't detect a wrong value — this assertion can. (The
        // Go baker's matching literal, baker.go, gets the same pin on the core side.)
        val spec = Json { ignoreUnknownKeys = true }.decodeFromString(Spec.serializer(), readVectors())
        val kinds = spec.cases.flatMap { it.entries }.map { it.kind }.toSet()
        assertTrue(
            BAKED_KIND_INTERMISSION in kinds,
            "the bundle constant \"$BAKED_KIND_INTERMISSION\" must equal the contract's intermission literal (contract kinds: $kinds)",
        )
    }

    private fun readVectors(): String {
        val name = "running-order-numbering.vectors.json"
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
