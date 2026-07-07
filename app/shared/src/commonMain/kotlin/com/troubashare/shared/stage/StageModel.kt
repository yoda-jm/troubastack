// TroubaStage presenter — SHARED Kotlin (commonMain). It performs pre-baked bundles (A02 model)
// and does NOTHING else (I12): a pure image compositor + pager. No annotation model, no
// access-control, NO network, NO writes. Every input/environment problem is a STATE the UI renders
// (never an exception) — a crash or dead-end mid-performance is this product's worst failure.
package com.troubashare.shared.stage

import com.troubashare.shared.bundle.BundleIssue
import com.troubashare.shared.bundle.ConcertBundle
import com.troubashare.shared.bundle.LayerImage
import com.troubashare.shared.bundle.LoadResult

/**
 * The reading mode, cycled on the single Stage toggle (A14). FIT_PAGE shows the whole page (and is
 * the only mode that goes two-up in landscape, A12); FIT_WIDTH fills width and scrolls one page
 * vertically; SCROLL is a continuous vertical column of every page at fit-width.
 */
enum class FitMode {
    FIT_PAGE, FIT_WIDTH, SCROLL;

    companion object {
        /** Parse a persisted mode (A14 persistence, A10 pattern); null/unknown → FIT_PAGE (the default). */
        fun parse(raw: String?): FitMode = entries.firstOrNull { it.name == raw } ?: FIT_PAGE
    }
}

/** Cycle the reading mode on the single toggle (A14): page → width → scroll → page. */
fun nextFitMode(mode: FitMode): FitMode = when (mode) {
    FitMode.FIT_PAGE -> FitMode.FIT_WIDTH
    FitMode.FIT_WIDTH -> FitMode.SCROLL
    FitMode.SCROLL -> FitMode.FIT_PAGE
}

/** Scroll-mode page turn (A14): move the topmost page forward one, clamped to the concert. */
fun scrollNextPage(top: Int, pageCount: Int): Int = (top + 1).coerceIn(0, (pageCount - 1).coerceAtLeast(0))

/** Scroll-mode page turn (A14): move the topmost page back one, clamped at the first page. */
fun scrollPrevPage(top: Int): Int = (top - 1).coerceAtLeast(0)

/** A page either performs or shows a neutral placeholder — never a crash. */
enum class PageStatus { READY, UNAVAILABLE }

/** One performable page, flattened across songs. Blob refs are resolved by the decoder at draw time. */
data class StagePage(
    val songId: String,
    val songName: String,
    val pageInSong: Int,            // 0-based index within its song
    val rasterRef: String,
    val overlays: List<LayerImage>, // already sorted by z-order (A02 loader)
    val status: PageStatus,         // UNAVAILABLE when the raster blob was flagged missing/empty
    // Setlist metadata (A08) — carried from BakedSong; shown only on the song's first page (pageInSong 0).
    val displayNotes: String = "",
    val key: String = "",
    val tempo: Int = 0,
)

/** A song entry for the picker: jumps to the song's first global page. */
data class SongInfo(val songId: String, val name: String, val firstPage: Int)

/** A distinct layer aggregated across the bundle, for the Layers panel. */
data class LayerInfo(val layerId: String, val mandatory: Boolean, val roleTag: String)

/**
 * The whole presenter state. Pure data, no Android deps — all transitions are total and clamped, so
 * this is where the resilience contract is unit-tested. When [failure] is non-null the loader could
 * not read the bundle at all and the UI shows a friendly failure screen with a way back.
 */
data class StageState(
    val failure: String? = null,
    val pages: List<StagePage> = emptyList(),
    val songs: List<SongInfo> = emptyList(),
    val layers: List<LayerInfo> = emptyList(),
    val current: Int = 0,
    val fitMode: FitMode = FitMode.FIT_PAGE,
    val visibleLayers: Set<String> = emptySet(),
    val role: String = "",
) {
    val pageCount: Int get() = pages.size
    val currentPage: StagePage? get() = pages.getOrNull(current)

    /** The song index the current page belongs to (for highlighting the picker), or -1 if none. */
    val currentSong: Int
        get() = songs.indexOfLast { it.firstPage <= current }
}

/**
 * Default layer visibility (I12): empty roleTag ⇒ visible; non-empty ⇒ visible only when it equals
 * the local reading role; a mandatory layer is ALWAYS visible (the viewer cannot hide it).
 */
internal fun defaultVisible(layer: LayerInfo, role: String): Boolean =
    layer.mandatory || layer.roleTag.isEmpty() || layer.roleTag == role

/** Build the initial [StageState] from a loader result. Total: a Failed load becomes a failure state. */
internal fun stageStateFrom(result: LoadResult, role: String): StageState = when (result) {
    is LoadResult.Failed -> StageState(failure = result.reason, role = role)
    is LoadResult.Loaded -> buildLoaded(result.bundle, result.issues, role)
}

private fun buildLoaded(bundle: ConcertBundle, issues: List<BundleIssue>, role: String): StageState {
    // Refs flagged missing/empty for a specific (song,page) — used to mark a page's RASTER unavailable.
    val badRefs: Set<String> = issues
        .filter { it.kind == BundleIssue.Kind.MISSING_BLOB || it.kind == BundleIssue.Kind.EMPTY_BLOB }
        .map { blobKey(it.songId, it.page, it.ref) }
        .toSet()

    val pages = ArrayList<StagePage>()
    val songs = ArrayList<SongInfo>()
    bundle.songs.forEachIndexed { songIdx, song ->
        val songName = "Song ${songIdx + 1}"
        songs.add(SongInfo(song.songId, songName, pages.size))
        song.pages.forEachIndexed { pageIdx, page ->
            val rasterBad = blobKey(song.songId, pageIdx, page.pageRasterRef) in badRefs
            pages.add(
                StagePage(
                    songId = song.songId,
                    songName = songName,
                    pageInSong = pageIdx,
                    rasterRef = page.pageRasterRef,
                    overlays = page.overlays,
                    status = if (rasterBad) PageStatus.UNAVAILABLE else PageStatus.READY,
                    displayNotes = song.displayNotes,
                    key = song.key,
                    tempo = song.tempo,
                ),
            )
        }
    }

    val layers = aggregateLayers(bundle)
    val visible = layers.filter { defaultVisible(it, role) }.map { it.layerId }.toSet()
    return StageState(pages = pages, songs = songs, layers = layers, visibleLayers = visible, role = role)
}

/** Distinct layers across the bundle, insertion-ordered; mandatory if any occurrence is mandatory. */
private fun aggregateLayers(bundle: ConcertBundle): List<LayerInfo> {
    val byId = LinkedHashMap<String, LayerInfo>()
    for (song in bundle.songs) {
        for (page in song.pages) {
            for (o: LayerImage in page.overlays) {
                val prev = byId[o.layerId]
                byId[o.layerId] = LayerInfo(
                    layerId = o.layerId,
                    mandatory = (prev?.mandatory ?: false) || o.mandatory,
                    roleTag = prev?.roleTag?.ifEmpty { o.roleTag } ?: o.roleTag,
                )
            }
        }
    }
    return byId.values.toList()
}

private fun blobKey(songId: String, page: Int, ref: String): String = "$songId#$page#$ref"

/**
 * The one-line setlist-metadata strip for a song's first page (A08): notes · key · ♩=tempo, omitting
 * empty fields; tempo 0 is omitted. Returns null when nothing to show (⇒ no strip renders, layout
 * unchanged). Visual one-line truncation is the caller's job (Text maxLines=1 + ellipsis).
 */
internal fun metaStripText(displayNotes: String, key: String, tempo: Int): String? {
    val parts = buildList {
        if (displayNotes.isNotBlank()) add(displayNotes.trim())
        if (key.isNotBlank()) add(key.trim())
        if (tempo > 0) add("♩=$tempo") // ♩=N (quarter-note); a display, not a click track
    }
    return if (parts.isEmpty()) null else parts.joinToString("  ·  ")
}

/**
 * The A15 song-drawer meta line for the song at [songIndex]: the same notes · key · ♩=tempo as the
 * A08 strip, read from the song's FIRST page (where the setlist metadata is carried). Null when the
 * song or its page is missing, or when the song has no metadata to show. Pure so it's unit-tested.
 */
internal fun songMetaLine(state: StageState, songIndex: Int): String? {
    val song = state.songs.getOrNull(songIndex) ?: return null
    val page = state.pages.getOrNull(song.firstPage) ?: return null
    return metaStripText(page.displayNotes, page.key, page.tempo)
}
