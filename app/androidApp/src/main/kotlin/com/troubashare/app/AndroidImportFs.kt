package com.troubashare.app

import com.troubashare.shared.bundle.ImportFs
import com.troubashare.shared.seams.Storage
import com.troubashare.shared.seams.UnpackResult
import com.troubashare.shared.seams.unpackBundle
import java.io.File
import java.util.UUID

/**
 * Android [ImportFs]: real filesystem ops over java.io.File, dirs from the Storage seam (seam 3),
 * unpack via the seam's unpackBundle. The loader is given absolute paths, so reads resolve as-is.
 */
class AndroidImportFs(private val storage: Storage) : ImportFs {
    override fun exists(path: String): Boolean = File(path).exists()
    override fun readText(path: String): String? = File(path).let { if (it.isFile) it.readText() else null }
    override fun sizeOf(path: String): Long = File(path).let { if (it.isFile) it.length() else 0L }

    override fun makeImportTempDir(): String =
        File(storage.tempDir(), "import-${UUID.randomUUID()}").apply { mkdirs() }.path

    override fun unpack(zipPath: String, destDir: String): UnpackResult = unpackBundle(zipPath, destDir)

    // Concert ids come from untrusted bundle.json — sanitize to a single safe path segment so a
    // hostile id (e.g. "../../x") can never escape bundlesDir.
    override fun bundleTargetDir(concertId: String): String =
        File(storage.bundlesDir(), safeSegment(concertId)).path

    override fun move(fromDir: String, toDir: String): Boolean {
        val to = File(toDir)
        to.parentFile?.mkdirs()
        return File(fromDir).renameTo(to)
    }

    override fun deleteRecursively(path: String) {
        File(path).deleteRecursively()
    }
}

/** Reduce an arbitrary id to one safe filename segment (no separators, no dot-only names). */
private fun safeSegment(id: String): String {
    val cleaned = id.map { if (it.isLetterOrDigit() || it == '-' || it == '_') it else '_' }.joinToString("")
    return cleaned.ifBlank { "bundle" }
}
