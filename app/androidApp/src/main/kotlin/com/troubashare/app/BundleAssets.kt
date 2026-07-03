package com.troubashare.app

import android.content.Context
import java.io.File

/**
 * A selectable bundle shipped in assets. A04 STOPGAP so the presenter has real input to perform;
 * real import (unzip a `.tstage` via the Storage seam) lands in A05. Includes the A03 demo bundle
 * plus the torture variants so the never-crash contract can be exercised on-device.
 */
data class DemoBundle(val label: String, val assetPath: String)

val DEMO_BUNDLES: List<DemoBundle> = listOf(
    DemoBundle("Demo Concert", "fixtures/demo"),
    DemoBundle("Torture — missing page", "fixtures/torture/missing-blob"),
    DemoBundle("Torture — damaged manifest", "fixtures/torture/bad-json"),
    DemoBundle("Torture — empty concert", "fixtures/torture/empty"),
    DemoBundle("Torture — no manifest", "fixtures/torture/no-manifest"),
)

/**
 * Copy an asset bundle directory into cacheDir so the loader's file interface (and the decoder) can
 * read plain files, and return that directory. Fixtures are tiny, so a fresh copy per open is fine.
 */
fun copyBundleToCache(context: Context, assetPath: String): File {
    val dest = File(context.cacheDir, "bundles/${assetPath.substringAfterLast('/')}")
    dest.deleteRecursively()
    dest.mkdirs()
    copyAsset(context, assetPath, dest)
    return dest
}

/** Recursively copy an asset path (dir or file) to [dest]. A leaf (list() empty) is a file. */
private fun copyAsset(context: Context, assetPath: String, dest: File) {
    val children = context.assets.list(assetPath) ?: emptyArray()
    if (children.isEmpty()) {
        dest.parentFile?.mkdirs()
        context.assets.open(assetPath).use { input -> dest.outputStream().use { input.copyTo(it) } }
        return
    }
    dest.mkdirs()
    for (child in children) copyAsset(context, "$assetPath/$child", File(dest, child))
}
