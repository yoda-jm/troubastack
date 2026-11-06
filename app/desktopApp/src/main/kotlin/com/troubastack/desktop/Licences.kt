package com.troubastack.desktop

import com.troubastack.app.ShippedLicences
import com.troubastack.shared.ui.LicenceDocument
import com.troubastack.shared.ui.LicenceEntry
import java.io.File

/**
 * WIN08 S2.8 — the licences the app image ships under `resources/licences/` (Compose's
 * `compose.application.resources.dir`). Only the Windows image has them, because only it ships SWT (the build's
 * `swtLicences` task writes them from the SWT jar it resolves), so About shows its line exactly where SWT ships.
 * null when there is no notice, or it names no version.
 */
internal fun shippedSwtLicences(dir: File?): ShippedLicences? {
    val notice = dir?.let { File(it, SWT_NOTICE) }?.takeIf { it.isFile } ?: return null
    val version = swtNoticeVersion(notice.readText()) ?: return null
    return ShippedLicences(
        line = "Studio in this app is shown with SWT $version, open-source software under the Eclipse Public License 2.0.",
        entries = { listOf(LicenceEntry("SWT $version", swtDocuments(dir))) },
    )
}

/** The version the notice states on its `Version:` line (the build writes the one it resolved). */
internal fun swtNoticeVersion(notice: String): String? =
    Regex("""(?m)^\s*Version:\s*(\S+)\s*$""").find(notice)?.groupValues?.get(1)

/** SWT's texts in reading order; a file the image lacks is left out rather than shown empty. */
private fun swtDocuments(dir: File): List<LicenceDocument> {
    val known = listOf(
        SWT_NOTICE to "SWT in this app",
        "EPL-2.0.txt" to "Eclipse Public License 2.0",
        "SWT-about.txt" to "Included in SWT",
        "about_files/WebView2_LICENSE.txt" to "Microsoft WebView2 loader licence",
        "about_files/IJG_README" to "Independent JPEG Group's JPEG software",
        "about_files/mpl-v11.txt" to "Mozilla Public License 1.1",
    )
    val docs = known.mapNotNull { (path, title) -> File(dir, path).takeIf { it.isFile }?.let { LicenceDocument(title, it.readText()) } }
    // Any other text SWT ships in about_files (a later version may add one) is shown under its own file name.
    val others = File(dir, "about_files").listFiles().orEmpty().sortedBy { it.name }
        .filter { f -> f.isFile && known.none { it.first == "about_files/${f.name}" } }
        .map { LicenceDocument(it.name, it.readText()) }
    return docs + others
}

internal const val SWT_NOTICE = "SWT-NOTICE.txt"

/** The image's licences folder, where Compose puts the app resources; null when run without them (tests, `run`). */
internal fun licencesDir(): File? = System.getProperty("compose.application.resources.dir")?.let { File(it, "licences") }
