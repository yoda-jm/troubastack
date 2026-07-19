package com.troubashare.shared.stage

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * P205 view-resolution CONTRACT (cross-lane). Runs the SAME cases the Go printer runs
 * (core/internal/bake/testdata/view-resolution.vectors.json → LayerVisible) against the
 * presenter's [defaultVisible], so "print == screen" is a tested invariant, not a hope
 * (the glyphs.json pattern applied to semantics — see that testdata's README).
 *
 * The vectors file is committed in BOTH lanes; a CI guard diffs the two so they can't drift.
 * JVM-only (real filesystem for the resource), hence androidUnitTest — [defaultVisible] is
 * pure commonMain, so this equally covers the iOS presenter.
 */
class ViewResolutionVectorsTest {

    @Serializable private data class Spec(val cases: List<Case>)
    @Serializable private data class Case(
        val name: String,
        val layer: Layer,
        val viewer: Viewer,
        val expectVisible: Boolean,
    )
    @Serializable private data class Layer(
        val mandatory: Boolean = false,
        val roleTag: String = "",
        val owner: String = "",
        val defaultOn: Boolean? = null,
    )
    @Serializable private data class Viewer(val role: String = "", val memberId: String = "")

    @Test
    fun presenter_matchesTheSharedViewResolutionVectors() {
        val spec = Json { ignoreUnknownKeys = true }.decodeFromString(Spec.serializer(), readVectors())
        assertTrue(spec.cases.isNotEmpty(), "no vector cases — the contract file is empty")
        for (c in spec.cases) {
            val layer = LayerInfo(
                layerId = "v",
                mandatory = c.layer.mandatory,
                roleTag = c.layer.roleTag,
                owner = c.layer.owner,
                defaultOn = c.layer.defaultOn,
            )
            val got = defaultVisible(layer, role = c.viewer.role, identity = c.viewer.memberId)
            assertEquals(c.expectVisible, got, "view-resolution vector mismatch: ${c.name}")
        }
    }

    /** Read the committed contract file (test classpath first, then a CWD-relative fallback). */
    private fun readVectors(): String {
        val name = "view-resolution.vectors.json"
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
