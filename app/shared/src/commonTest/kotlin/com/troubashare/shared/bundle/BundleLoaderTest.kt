package com.troubashare.shared.bundle

import kotlinx.serialization.json.Json
import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertTrue

/** In-memory [BundleFiles]: a path→content map. A blob's size is its content length unless overridden. */
private class FakeFiles(
    private val contents: Map<String, String>,
    private val sizes: Map<String, Long> = emptyMap(),
) : BundleFiles {
    override fun exists(path: String): Boolean = contents.containsKey(path)
    override fun readText(path: String): String? = contents[path]
    override fun sizeOf(path: String): Long = sizes[path] ?: contents[path]?.length?.toLong() ?: 0L
}

private const val DIR = "bundle"
private const val MANIFEST = "bundle/bundle.json"

private fun filesWith(manifest: String, blobs: Map<String, String> = emptyMap(), sizes: Map<String, Long> = emptyMap()) =
    FakeFiles(contents = mapOf(MANIFEST to manifest) + blobs.mapKeys { "$DIR/${it.key}" },
        sizes = sizes.mapKeys { "$DIR/${it.key}" })

class BundleLoaderTest {
    private val loader = BundleLoader()

    @Test
    fun validBundle_loads_parses64bitStrings_and_sortsOverlays() {
        val manifest = """
            {
              "concertId":"c1","name":"Spring Gig","concertRev":"7","bakedAt":"1700000000",
              "bakedBy":"maestro","finalLocked":true,
              "songs":[{"songId":"s1","sourceRevision":"3","songRev":"1","pages":[
                {"pageRasterRef":"blobs/p0.webp","rasterHash":"h0","overlays":[
                  {"layerId":"L2","imageRef":"blobs/p0-L2.webp","order":2},
                  {"layerId":"L1","imageRef":"blobs/p0-L1.webp","order":1}
                ]}
              ]}]
            }
        """.trimIndent()
        val files = filesWith(
            manifest,
            blobs = mapOf("blobs/p0.webp" to "raster", "blobs/p0-L1.webp" to "a", "blobs/p0-L2.webp" to "b"),
        )

        val result = loader.load(DIR, files)
        val loaded = assertIs<LoadResult.Loaded>(result)
        assertTrue(loaded.issues.isEmpty(), "no issues expected: ${loaded.issues}")

        val b = loaded.bundle
        assertEquals("c1", b.concertId)
        assertEquals(7uL, b.concertRev)          // uint64 parsed from JSON string
        assertEquals(1700000000L, b.bakedAt)     // int64 parsed from JSON string
        assertTrue(b.finalLocked)
        val overlays = b.songs.single().pages.single().overlays
        assertEquals(listOf("L1", "L2"), overlays.map { it.layerId }, "overlays sorted by order")
    }

    @Test
    fun missingManifest_fails_withReadableReason() {
        val result = loader.load(DIR, FakeFiles(emptyMap()))
        val failed = assertIs<LoadResult.Failed>(result)
        assertEquals("bundle.json is missing", failed.reason)
    }

    @Test
    fun truncatedJson_fails() {
        val result = loader.load(DIR, filesWith("""{"concertId":"c1","songs":[""" /* truncated */))
        val failed = assertIs<LoadResult.Failed>(result)
        assertTrue("json" in failed.reason.lowercase(), "reason should mention the file: ${failed.reason}")
    }

    @Test
    fun unknownKeys_areIgnored() {
        val manifest = """{"concertId":"c1","futureField":42,"songs":[]}"""
        val loaded = assertIs<LoadResult.Loaded>(loader.load(DIR, filesWith(manifest)))
        assertEquals("c1", loaded.bundle.concertId)
    }

    @Test
    fun missingBlob_loadsWithIssue_notFailure() {
        val manifest = """
            {"concertId":"c1","songs":[{"songId":"s1","pages":[
              {"pageRasterRef":"blobs/gone.webp","overlays":[]}
            ]}]}
        """.trimIndent()
        val loaded = assertIs<LoadResult.Loaded>(loader.load(DIR, filesWith(manifest)))  // blob not provided
        val issue = loaded.issues.single()
        assertEquals(BundleIssue.Kind.MISSING_BLOB, issue.kind)
        assertEquals("blobs/gone.webp", issue.ref)
        assertEquals("s1", issue.songId)
        assertEquals(0, issue.page)
    }

    @Test
    fun emptyBlob_isFlaggedAsEmpty() {
        val manifest = """{"concertId":"c1","songs":[{"songId":"s1","pages":[{"pageRasterRef":"blobs/p0.webp","overlays":[]}]}]}"""
        val files = filesWith(manifest, blobs = mapOf("blobs/p0.webp" to ""), sizes = mapOf("blobs/p0.webp" to 0L))
        val loaded = assertIs<LoadResult.Loaded>(loader.load(DIR, files))
        assertEquals(BundleIssue.Kind.EMPTY_BLOB, loaded.issues.single().kind)
    }

    @Test
    fun zeroSongsAndZeroPages_loadEmpty_noCrash() {
        assertTrue(assertIs<LoadResult.Loaded>(loader.load(DIR, filesWith("""{"concertId":"c1","songs":[]}"""))).bundle.songs.isEmpty())

        val onePageless = """{"concertId":"c1","songs":[{"songId":"s1","pages":[]}]}"""
        val loaded = assertIs<LoadResult.Loaded>(loader.load(DIR, filesWith(onePageless)))
        assertTrue(loaded.issues.isEmpty())
        assertTrue(loaded.bundle.songs.single().pages.isEmpty())
    }

    @Test
    fun duplicateLayerId_onSamePage_isDroppedAndFlagged() {
        val manifest = """
            {"concertId":"c1","songs":[{"songId":"s1","pages":[
              {"pageRasterRef":"blobs/p0.webp","overlays":[
                {"layerId":"L1","imageRef":"blobs/a.webp","order":1},
                {"layerId":"L1","imageRef":"blobs/dup.webp","order":2}
              ]}
            ]}]}
        """.trimIndent()
        val files = filesWith(manifest, blobs = mapOf("blobs/p0.webp" to "r", "blobs/a.webp" to "a", "blobs/dup.webp" to "d"))
        val loaded = assertIs<LoadResult.Loaded>(loader.load(DIR, files))

        assertEquals(1, loaded.bundle.songs.single().pages.single().overlays.size, "duplicate dropped")
        val dup = loaded.issues.single { it.kind == BundleIssue.Kind.DUPLICATE_LAYER }
        assertEquals("L1", dup.ref)
    }

    @Test
    fun canonicalJson_encodes64bitFieldsAsStrings() {
        // A future buf/protobuf encoder must produce the same bytes: int64/uint64 are JSON strings.
        val bundle = ConcertBundle(concertId = "c1", concertRev = 7uL, bakedAt = 1700000000L)
        val text = Json.encodeToString(ConcertBundle.serializer(), bundle)
        assertContains(text, "\"concertRev\":\"7\"")
        assertContains(text, "\"bakedAt\":\"1700000000\"")
    }

    @Test
    fun maps_title_and_onCall_fields() {
        // T26 (title=9) + T23 (on_call=8): the loader now MAPS both bundle fields (previously the
        // app ignored on_call; it's consumed since the T26 app half). Empty/absent stay defaulted.
        val manifest = """
            {"concertId":"c1","songs":[
              {"songId":"s1","title":"Wonderwall","pages":[{"pageRasterRef":"blobs/p0.webp","overlays":[]}]},
              {"songId":"s2","title":"Encore","onCall":true,"pages":[{"pageRasterRef":"blobs/p1.webp","overlays":[]}]}
            ]}
        """.trimIndent()
        val files = filesWith(manifest, blobs = mapOf("blobs/p0.webp" to "r", "blobs/p1.webp" to "r"))
        val loaded = assertIs<LoadResult.Loaded>(loader.load(DIR, files))
        val (s1, s2) = loaded.bundle.songs
        assertEquals("Wonderwall", s1.title)
        assertEquals(false, s1.onCall)
        assertEquals("Encore", s2.title)
        assertEquals(true, s2.onCall)
    }

    @Test
    fun p205_bandWideFields_decode_and_areAdditive() {
        // P205: a band-wide bundle carries roster, per-layer owner + optional default_on,
        // and per-member cues. All additive: this reader decodes them; a pre-P205 bundle
        // (next test / the older cases above) loads them as defaults.
        val manifest = """
            {
              "concertId":"c1",
              "roster":[
                {"memberId":"m-marie","displayName":"Marie","role":"admin"},
                {"memberId":"m-leo","displayName":"Leo","role":"member"}
              ],
              "songs":[{"songId":"s1","pages":[
                {"pageRasterRef":"blobs/p0.webp","overlays":[
                  {"layerId":"L1","imageRef":"blobs/p0-L1.webp","order":1,"name":"Conductor cues","mandatory":true,"defaultOn":true},
                  {"layerId":"L2","imageRef":"blobs/p0-L2.webp","order":2,"name":"My notes","owner":"m-marie","defaultOn":false},
                  {"layerId":"L3","imageRef":"blobs/p0-L3.webp","order":3,"name":"Form"}
                ]}
              ],"memberCues":[
                {"memberId":"m-marie","cues":[{"icon":"mic"},{"icon":"guitar-electric","color":"#e11d48"}]},
                {"memberId":"m-leo","cues":[{"icon":"tambourine"}]}
              ]}]
            }
        """.trimIndent()
        val files = filesWith(
            manifest,
            blobs = mapOf(
                "blobs/p0.webp" to "r", "blobs/p0-L1.webp" to "a",
                "blobs/p0-L2.webp" to "b", "blobs/p0-L3.webp" to "c",
            ),
        )
        val b = assertIs<LoadResult.Loaded>(loader.load(DIR, files)).bundle

        assertEquals(listOf("m-marie", "m-leo"), b.roster.map { it.memberId })
        assertEquals("Marie", b.roster[0].displayName)
        assertEquals("member", b.roster[1].role)

        val ovs = b.songs.single().pages.single().overlays  // sorted by order → L1,L2,L3
        assertEquals("", ovs[0].owner, "shared layer owner is empty")
        assertEquals("m-marie", ovs[1].owner, "personal layer carries its owner")
        assertEquals(true, ovs[0].defaultOn)   // present-true
        assertEquals(false, ovs[1].defaultOn)  // present-false (distinct from absent)
        assertEquals(null, ovs[2].defaultOn, "absent default_on decodes as null (compute-as-today)")

        val mc = b.songs.single().memberCues
        assertEquals(listOf("m-marie", "m-leo"), mc.map { it.memberId })
        assertEquals(listOf("mic", "guitar-electric"), mc[0].cues.map { it.icon })
        assertEquals("#e11d48", mc[0].cues[1].color)
    }

    @Test
    fun p205_oldBundle_hasEmptyBandWideFields() {
        // A pre-P205 bundle omits all of it — roster empty, owner "", default_on null, memberCues empty.
        val manifest = """
            {"concertId":"c1","songs":[{"songId":"s1","pages":[
              {"pageRasterRef":"blobs/p0.webp","overlays":[{"layerId":"L1","imageRef":"blobs/p0-L1.webp"}]}
            ]}]}
        """.trimIndent()
        val files = filesWith(manifest, blobs = mapOf("blobs/p0.webp" to "r", "blobs/p0-L1.webp" to "a"))
        val b = assertIs<LoadResult.Loaded>(loader.load(DIR, files)).bundle
        assertTrue(b.roster.isEmpty())
        val ov = b.songs.single().pages.single().overlays.single()
        assertEquals("", ov.owner)
        assertEquals(null, ov.defaultOn)
        assertTrue(b.songs.single().memberCues.isEmpty())
    }

    @Test
    fun tolerates_genuinely_unknown_field() {
        // The additive-field backward-compat guarantee still holds for keys this reader doesn't know
        // (a future proto field): the loader ignores them and loads normally (ignoreUnknownKeys).
        val manifest = """
            {"concertId":"c1","someFutureField":42,"songs":[{"songId":"s1","artistSubtitle":"x","pages":[
              {"pageRasterRef":"blobs/p0.webp","overlays":[]}
            ]}]}
        """.trimIndent()
        val files = filesWith(manifest, blobs = mapOf("blobs/p0.webp" to "r"))
        val loaded = assertIs<LoadResult.Loaded>(loader.load(DIR, files))
        assertEquals("s1", loaded.bundle.songs.single().songId, "unknown keys must not break the load")
    }
}
