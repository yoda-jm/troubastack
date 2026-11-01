package com.troubastack.app

import com.troubastack.shared.bundle.BundleFiles
import java.io.File

/** File-backed [BundleFiles] — the loader is given absolute paths (bundleDir prefix), so read as-is. */
class FileBundleFiles : BundleFiles {
    override fun exists(path: String): Boolean = File(path).exists()
    override fun readText(path: String): String? = File(path).let { if (it.isFile) it.readText() else null }
    override fun sizeOf(path: String): Long = File(path).let { if (it.isFile) it.length() else 0L }
}
