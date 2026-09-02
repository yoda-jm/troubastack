package com.troubastack.shared.stage

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A35/T92 — the metre PARSER contract, the primitive that feeds [beatPhase]'s groups. ONE parser,
 * THREE runtimes that must never silently disagree: Go `app.ParseMeter`, TS `meterGroups`, Kotlin
 * [meterGroups]. Go and TS read `docs/contracts/meter-groups.vectors.json` directly; this mirrors it
 * into commonTest resources (KMP resource loading needs the copy) and a CI job diffs the two, so the
 * copy can't drift — the same view-resolution / beat-phase pattern.
 *
 * `groups: null` means "the parser must treat this metre as UNSET"; each runtime expresses unset in its
 * own idiom (Go returns (nil,false); TS returns the 4/4 default; Kotlin returns [DEFAULT_GROUPS], since
 * a lenient beat reads unset as 4/4). A non-null `groups` is the exact grouping every runtime returns.
 *
 * JVM-only (real filesystem for the resource), hence androidUnitTest — [meterGroups] is pure commonMain,
 * so this equally covers the iOS runtime. The `٤/٨` case is the reason for the strict-ASCII guard: the
 * JVM's own integer parse would accept Unicode digits and disagree with Go/TS.
 */
class MeterGroupsVectorsTest {

    @Serializable private data class Spec(val cases: List<Case>)
    @Serializable private data class Case(val meter: String, val groups: List<Int>? = null)

    @Test
    fun meterGroups_matchesTheSharedVectors() {
        val spec = Json { ignoreUnknownKeys = true }.decodeFromString(Spec.serializer(), readVectors())
        assertTrue(spec.cases.size >= 30, "too few parser vectors (${spec.cases.size}) — the contract shrank")
        for (c in spec.cases) {
            val got = meterGroups(c.meter)
            // Kotlin's unset shape IS the 4/4 default — a lenient beat treats an unparseable metre as 4/4.
            val expected = c.groups ?: DEFAULT_GROUPS
            assertEquals(expected, got, "meter=\"${c.meter}\" (groups=${c.groups ?: "unset⇒4/4"})")
        }
    }

    private fun readVectors(): String {
        val name = "meter-groups.vectors.json"
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
