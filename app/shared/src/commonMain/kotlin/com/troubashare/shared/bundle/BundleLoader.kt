// Resilient loader for baked concert bundles (docs/design/08-bundle-container.md). This is the
// presenter's resilience foundation (I12: one bad page must never take down a performance), so it
// is a TOTAL function — `load` never throws for bad INPUT; every failure comes back as a value.
package com.troubashare.shared.bundle

import kotlinx.serialization.json.Json

/**
 * Read-only filesystem access, injected by the caller so commonMain stays free of platform IO and
 * the loader is unit-testable with an in-memory fake. The real implementation lands in A05 (Storage
 * seam). Implementations must be total too — return null / 0 rather than throwing on a bad path.
 */
interface BundleFiles {
    /** True if a file exists at [path]. */
    fun exists(path: String): Boolean

    /** File contents as text, or null if missing/unreadable. */
    fun readText(path: String): String?

    /** File size in bytes; 0 (or less) if missing/empty. */
    fun sizeOf(path: String): Long
}

/** One non-fatal problem found while loading — lets the presenter show a placeholder for one page. */
data class BundleIssue(
    val songId: String,   // which song the page belongs to
    val page: Int,        // page index within that song
    val ref: String,      // the blob ref involved (or the layerId, for DUPLICATE_LAYER)
    val kind: Kind,
) {
    enum class Kind {
        MISSING_BLOB,     // referenced blob file (or the ref itself) is absent
        EMPTY_BLOB,       // blob file exists but is 0 bytes
        DUPLICATE_LAYER,  // a second overlay with an already-seen layerId on the same page (dropped)
    }
}

/** The outcome of [BundleLoader.load]. Failure is a value, never an exception. */
sealed interface LoadResult {
    /** The bundle parsed. [issues] flags per-page problems; the bundle is still performable. */
    data class Loaded(val bundle: ConcertBundle, val issues: List<BundleIssue>) : LoadResult

    /** The bundle could not be loaded at all. [reason] is human-readable — no stack trace, no JSON path. */
    data class Failed(val reason: String) : LoadResult
}

/**
 * Loads a bundle directory (`bundle.json` + `blobs/…`) via [BundleFiles].
 *
 * - Missing/unreadable/malformed `bundle.json` ⇒ [LoadResult.Failed] with a musician-readable reason.
 * - A missing or empty blob referenced by a page ⇒ NOT a failure: the bundle loads and the page is
 *   flagged via a [BundleIssue] (I12 resilience).
 * - Overlays are sorted by `order`; duplicate `layerId`s on a page are dropped and flagged.
 */
class BundleLoader(private val json: Json = defaultJson) {

    fun load(bundleDir: String, files: BundleFiles): LoadResult {
        val manifestPath = join(bundleDir, MANIFEST)

        if (!files.exists(manifestPath)) return LoadResult.Failed("bundle.json is missing")
        val text = files.readText(manifestPath) ?: return LoadResult.Failed("bundle.json could not be read")

        val parsed = try {
            json.decodeFromString(ConcertBundle.serializer(), text)
        } catch (e: Exception) {
            // Any decoding problem (malformed JSON, bad 64-bit scalar, wrong shape) is an INPUT
            // failure — turn it into a value. Deliberately generic: no JSON path, no stack trace.
            return LoadResult.Failed("bundle.json is damaged or not a valid bundle")
        }

        val issues = ArrayList<BundleIssue>()
        val songs = parsed.songs.map { song -> normalizeSong(bundleDir, song, files, issues) }
        return LoadResult.Loaded(parsed.copy(songs = songs), issues)
    }

    /** Clean a song's pages: sort/dedup overlays and record blob issues. Pure w.r.t. [issues] append. */
    private fun normalizeSong(
        bundleDir: String,
        song: BakedSong,
        files: BundleFiles,
        issues: MutableList<BundleIssue>,
    ): BakedSong {
        val pages = song.pages.mapIndexed { pageIndex, page ->
            checkBlob(bundleDir, song.songId, pageIndex, page.pageRasterRef, files, issues)

            val kept = ArrayList<LayerImage>(page.overlays.size)
            val seen = HashSet<String>()
            for (overlay in page.overlays) {
                if (!seen.add(overlay.layerId)) {
                    issues.add(BundleIssue(song.songId, pageIndex, overlay.layerId, BundleIssue.Kind.DUPLICATE_LAYER))
                    continue
                }
                checkBlob(bundleDir, song.songId, pageIndex, overlay.imageRef, files, issues)
                kept.add(overlay)
            }
            page.copy(overlays = kept.sortedBy { it.order })
        }
        return song.copy(pages = pages)
    }

    /** Flag a page-referenced blob that is absent (or empty). An empty ref counts as MISSING. */
    private fun checkBlob(
        bundleDir: String,
        songId: String,
        page: Int,
        ref: String,
        files: BundleFiles,
        issues: MutableList<BundleIssue>,
    ) {
        if (ref.isEmpty()) {
            issues.add(BundleIssue(songId, page, ref, BundleIssue.Kind.MISSING_BLOB))
            return
        }
        val path = join(bundleDir, ref)
        when {
            !files.exists(path) -> issues.add(BundleIssue(songId, page, ref, BundleIssue.Kind.MISSING_BLOB))
            files.sizeOf(path) <= 0L -> issues.add(BundleIssue(songId, page, ref, BundleIssue.Kind.EMPTY_BLOB))
        }
    }

    companion object {
        private const val MANIFEST = "bundle.json"

        /** Forward-compatible: tolerate unknown keys so newer bundles still load on older clients. */
        val defaultJson: Json = Json { ignoreUnknownKeys = true }

        private fun join(dir: String, rel: String): String = when {
            dir.isEmpty() -> rel
            dir.endsWith("/") -> dir + rel
            else -> "$dir/$rel"
        }
    }
}
