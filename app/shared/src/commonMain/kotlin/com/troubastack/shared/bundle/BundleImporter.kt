// Atomic bundle import (I13): unpack → validate → swap into place, so a half-imported bundle is
// NEVER visible to the presenter. Shared (commonMain); all IO goes through the injected ImportFs so
// this is unit-testable with a fake and free of platform APIs (the real impl wraps the Storage seam).
package com.troubastack.shared.bundle

import com.troubastack.shared.seams.UnpackResult

/** Filesystem operations the importer needs. Read ops come from [BundleFiles]; the rest mutate. */
interface ImportFs : BundleFiles {
    /** Create and return a fresh, unique empty directory under the temp area (for staging an import). */
    fun makeImportTempDir(): String

    /** Extract a `.tstage` zip into [destDir] (delegates to the Storage seam's unpackBundle). */
    fun unpack(zipPath: String, destDir: String): UnpackResult

    /** The install location for a given concert id (e.g. bundlesDir/<concertId>). */
    fun bundleTargetDir(concertId: String): String

    /** Rename [fromDir] to [toDir] on the same volume. Returns false if it failed. */
    fun move(fromDir: String, toDir: String): Boolean

    /** Recursively delete [path]; a no-op if it doesn't exist. */
    fun deleteRecursively(path: String)
}

/** Outcome of an import. Failure is a value, never an exception (mirrors A02's loader). */
sealed interface ImportResult {
    data class Imported(val concertId: String) : ImportResult
    data class Failed(val reason: String) : ImportResult
}

class BundleImporter(
    private val fs: ImportFs,
    private val loader: BundleLoader = BundleLoader(),
) {
    /**
     * Import a `.tstage` at [zipPath]: unpack to temp, validate with the loader, then atomically swap
     * into bundlesDir/<concertId>. On any failure the temp is cleaned and the existing bundle (if any)
     * is left untouched — the bundles dir always holds either the old state or the complete new one.
     */
    fun import(zipPath: String): ImportResult {
        val temp = fs.makeImportTempDir()

        when (val u = fs.unpack(zipPath, temp)) {
            is UnpackResult.Failed -> return fail(temp, u.reason)
            is UnpackResult.Ok -> Unit
        }

        val bundle = when (val load = loader.load(temp, fs)) {
            is LoadResult.Failed -> return fail(temp, load.reason)
            is LoadResult.Loaded -> load.bundle
        }
        val id = bundle.concertId
        if (id.isEmpty()) return fail(temp, "this concert is missing an id")

        val target = fs.bundleTargetDir(id)
        val aside = "$target.replacing" // fixed suffix: only one import at a time; clear any stale leftover
        fs.deleteRecursively(aside)

        val hadExisting = fs.exists(target)
        if (hadExisting && !fs.move(target, aside)) return fail(temp, "couldn't replace the existing concert")

        if (!fs.move(temp, target)) {
            if (hadExisting) fs.move(aside, target) // roll back to the old bundle
            return fail(temp, "couldn't install the concert")
        }

        if (hadExisting) fs.deleteRecursively(aside) // new bundle is in place — drop the old one
        return ImportResult.Imported(id)
    }

    private fun fail(temp: String, reason: String): ImportResult.Failed {
        fs.deleteRecursively(temp)
        return ImportResult.Failed(reason)
    }
}
