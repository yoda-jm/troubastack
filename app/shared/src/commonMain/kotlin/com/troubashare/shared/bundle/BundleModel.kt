// Hand-written Kotlin mirror of proto/troubastack/v1/bundle.proto (I1 source of truth). There is
// no Kotlin proto codegen yet; per T09/T12 clients carry a hand-written mirror until codegen is
// adopted. Every type below names its proto message; NEVER add a field that isn't in bundle.proto.
//
// Serialization is proto3 CANONICAL JSON (docs/design/08-bundle-container.md): lowerCamelCase field
// names (so Kotlin property names == JSON names, no @SerialName needed), and 64-bit integers
// (int64/uint64) encoded as JSON STRINGS via the serializers below. int32 (`order`) stays a number.
// All fields carry proto defaults so a bundle.json that omits default-valued fields still parses.
package com.troubashare.shared.bundle

import kotlinx.serialization.KSerializer
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import kotlinx.serialization.json.JsonDecoder
import kotlinx.serialization.json.JsonPrimitive

/** proto `troubastack.v1.PageImages` — one performable page: a raster + transparent overlays. */
@Serializable
data class PageImages(
    val pageRasterRef: String = "",           // proto page_raster_ref — PDF page raster blob (WebP)
    val rasterHash: String = "",              // proto raster_hash — content hash (R10)
    val overlays: List<LayerImage> = emptyList(), // proto overlays — one per layer, composited in z-order
)

/** proto `troubastack.v1.LayerImage` — one layer's transparent overlay on one page. */
@Serializable
data class LayerImage(
    val layerId: String = "",                 // proto layer_id
    val imageRef: String = "",                // proto image_ref — transparent WebP blob for this layer
    val contentHash: String = "",             // proto content_hash — change detection (R10)
    val order: Int = 0,                       // proto order — z-order (int32; JSON number)
    val mandatory: Boolean = false,           // proto mandatory — viewer cannot hide
    val roleTag: String = "",                 // proto role_tag — default-visibility targeting
)

/** proto `troubastack.v1.BakedSong` — one song's baked pages. */
@Serializable
data class BakedSong(
    val songId: String = "",                                                    // proto song_id
    @Serializable(with = ProtoUInt64Serializer::class) val sourceRevision: ULong = 0uL, // proto source_revision
    @Serializable(with = ProtoUInt64Serializer::class) val songRev: ULong = 0uL,        // proto song_rev
    val pages: List<PageImages> = emptyList(),                                  // proto pages
    // Setlist overrides carried as metadata (B02); absent → proto default. The
    // presenter may display these; the loader tolerates their absence.
    val displayNotes: String = "",                                              // proto display_notes
    val key: String = "",                                                       // proto key
    val tempo: Int = 0,                                                         // proto tempo
    // T23: baked into the concert but sits on the "bench" (on call) — outside the running
    // order, still jumpable. Additive/default-false; old bundles omit it (proto on_call = 8).
    val onCall: Boolean = false,                                                // proto on_call
    // T26: the song's Title at bake time (a bundle is a snapshot; no rename propagation).
    // Additive/default-empty; empty/absent falls back to "Song N" client-side (proto title = 9).
    val title: String = "",                                                     // proto title
    // T50/A20: the baked-for member's PERSONAL song cues (icon + tint). The per-member bake injects
    // THAT member's cues; the shared bake carries none. Additive/default-empty (proto cues = 10) — old
    // bundles omit it, an unknown icon id renders as the `note` fallback client-side.
    val cues: List<SongCue> = emptyList(),                                      // proto cues
)

/** proto `troubastack.v1.SongCue` (AUTHORITY: bundle.proto) — one personal cue: a stable icon id + an
 *  optional "#rrggbb" tint ("" = neutral). T50/A20. Unknown [icon] → the `note` fallback (never an error). */
@Serializable
data class SongCue(
    val icon: String = "",   // proto icon (stable id from the curated set; unknown → `note`)
    val color: String = "",  // proto color (optional "#rrggbb"; "" = neutral/untinted)
)

/** proto `troubastack.v1.ConcertBundle` — the self-contained, performable baked concert (I11/I12). */
@Serializable
data class ConcertBundle(
    val concertId: String = "",                                             // proto concert_id
    val name: String = "",                                                  // proto name
    @Serializable(with = ProtoUInt64Serializer::class) val concertRev: ULong = 0uL, // proto concert_rev
    @Serializable(with = ProtoInt64Serializer::class) val bakedAt: Long = 0L,       // proto baked_at (epoch)
    val bakedBy: String = "",                                               // proto baked_by
    val finalLocked: Boolean = false,                                       // proto final_locked (I13)
    val songs: List<BakedSong> = emptyList(),                               // proto songs
)

/** proto `troubastack.v1.AvailableConcert` — cheap "what's available to me" metadata (I13). */
@Serializable
data class AvailableConcert(
    val concertId: String = "",                                              // proto concert_id
    val name: String = "",                                                   // proto name
    @Serializable(with = ProtoUInt64Serializer::class) val currentRev: ULong = 0uL, // proto current_rev
    @Serializable(with = ProtoInt64Serializer::class) val updatedAt: Long = 0L,     // proto updated_at
    val finalLocked: Boolean = false,                                        // proto final_locked
    val songs: List<SongRev> = emptyList(),                                  // proto songs (per-song revs)
) {
    /** proto `troubastack.v1.AvailableConcert.SongRev`. */
    @Serializable
    data class SongRev(
        val songId: String = "",                                                 // proto song_id
        @Serializable(with = ProtoUInt64Serializer::class) val rev: ULong = 0uL,  // proto rev
    )
}

/** proto `troubastack.v1.AvailableConcerts` — the manifest envelope. */
@Serializable
data class AvailableConcerts(
    val concerts: List<AvailableConcert> = emptyList(),  // proto concerts
)

// --- proto3 canonical-JSON scalar serializers -------------------------------------------------
// int64/uint64 travel as JSON strings in canonical proto3 JSON. These read the canonical string
// form and also tolerate a bare JSON number (lenient input). A non-numeric value throws inside
// kotlinx decoding, which BundleLoader.load catches and turns into a Failed value — it never
// escapes the loader (see BundleLoader).

/** uint64 <-> JSON string. */
internal object ProtoUInt64Serializer : KSerializer<ULong> {
    override val descriptor: SerialDescriptor =
        PrimitiveSerialDescriptor("troubastack.uint64", PrimitiveKind.STRING)

    override fun serialize(encoder: Encoder, value: ULong) = encoder.encodeString(value.toString())
    override fun deserialize(decoder: Decoder): ULong = decoder.scalarString().toULong()
}

/** int64 <-> JSON string. */
internal object ProtoInt64Serializer : KSerializer<Long> {
    override val descriptor: SerialDescriptor =
        PrimitiveSerialDescriptor("troubastack.int64", PrimitiveKind.STRING)

    override fun serialize(encoder: Encoder, value: Long) = encoder.encodeString(value.toString())
    override fun deserialize(decoder: Decoder): Long = decoder.scalarString().toLong()
}

/** Read a 64-bit scalar as a string, accepting both `"123"` (canonical) and `123` (lenient). */
private fun Decoder.scalarString(): String =
    if (this is JsonDecoder) {
        (decodeJsonElement() as? JsonPrimitive
            ?: throw SerializationException("expected a scalar for a 64-bit integer")).content
    } else {
        decodeString()
    }
