package com.troubastack.desktop

import com.troubastack.app.OPEN_SOURCE_LINE
import com.troubastack.app.ShippedLicences
import com.troubastack.app.parseNotices
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
internal fun licencesDir(): File? = resourcesDir()?.let { File(it, "licences") }

/** The image's app resources (Compose's `compose.application.resources.dir`); null when run without them. */
internal fun resourcesDir(): File? = System.getProperty("compose.application.resources.dir")?.let(::File)

/**
 * A101 — everything About → Licences lists in this image: SWT first where it ships (S2.8, Windows), then the entries
 * the build generated from what it resolved (`resources/notices/`), then the Java runtime the image bundles (its own
 * `legal/`, read in place). About's line: S2.8's SWT line on Windows, else [OPEN_SOURCE_LINE]. null when the image
 * ships neither (a dev `run`), so About shows no line.
 */
internal fun shippedLicences(resources: File?, javaHome: File?): ShippedLicences? {
    val swt = shippedSwtLicences(resources?.let { File(it, "licences") })
    val index = resources?.let { File(it, "notices/index.txt") }?.takeIf { it.isFile }
    if (swt == null && index == null) return null
    return ShippedLicences(line = swt?.line ?: OPEN_SOURCE_LINE) {
        val generated = index?.let { f ->
            parseNotices(f.readText()) { id -> File(f.parentFile, "texts/$id.txt").takeIf { it.isFile }?.readText() }
        }.orEmpty()
        swt?.entries?.invoke().orEmpty() + generated + listOfNotNull(javaRuntimeEntry(javaHome))
    }
}

/**
 * A101 — the Java runtime the image bundles, from its own `legal/` folder, read in place (nothing is copied at build
 * time). Linux and macOS images make most of `legal/` symlinks to java.base's files (java.desktop/LICENSE →
 * ../java.base/LICENSE); on Windows jlink writes a one-line pointer instead (`Please see ..\java.base\LICENSE`,
 * DefaultImageBuilder.writeSymLinkEntry), which counts as the file it points at ([pointerTarget]). Either way each
 * distinct text shows once, keyed by its content, titled by the first file that carries it and how many others do.
 * null when there is no `legal/`.
 */
internal fun javaRuntimeEntry(javaHome: File?): LicenceEntry? {
    val legal = javaHome?.let { File(it, "legal") }?.takeIf { it.isDirectory } ?: return null
    val byContent = LinkedHashMap<String, Pair<MutableList<String>, String>>()
    java.nio.file.Files.walk(legal.toPath()).use { paths ->
        paths.filter { java.nio.file.Files.isRegularFile(it) }   // follows a symlink to its file
            .sorted()
            .forEach { p ->
                val own = java.nio.file.Files.readAllBytes(p)   // reads through a symlink
                val bytes = pointerTarget(legal, p, own)?.let { java.nio.file.Files.readAllBytes(it) } ?: own
                val key = java.security.MessageDigest.getInstance("SHA-256").digest(bytes).joinToString("") { "%02x".format(it) }
                val rel = legal.toPath().relativize(p).toString().replace(File.separatorChar, '/')
                byContent.getOrPut(key) { mutableListOf<String>() to String(bytes, Charsets.UTF_8) }.first += rel
            }
    }
    val docs = byContent.values.map { (paths, text) ->
        LicenceDocument(if (paths.size == 1) paths[0] else "${paths[0]} (and ${paths.size - 1} more)", text)
    }
    val v = Runtime.version()
    return LicenceEntry("Java runtime (OpenJDK ${v.feature()}.${v.interim()}.${v.update()})", docs)
}

/**
 * A101 — the file a jlink pointer [bytes] (at [file]) stands for: a single line `Please see <path>` whose path, relative
 * to the pointer's folder (backslashes on Windows), is a regular file inside [legal]. null for anything else: a real
 * text, a pointer that leaves `legal/`, or one whose target is missing (then it shows as it is).
 */
internal fun pointerTarget(legal: File, file: java.nio.file.Path, bytes: ByteArray): java.nio.file.Path? {
    if (bytes.size > 512) return null
    val text = String(bytes, Charsets.UTF_8).trimEnd('\r', '\n')
    if ('\n' in text) return null
    val target = Regex("^Please see (.+)$").find(text)?.groupValues?.get(1)?.trim() ?: return null
    val root = legal.toPath().toAbsolutePath().normalize()
    val resolved = file.toAbsolutePath().parent.resolve(target.replace('\\', '/')).normalize()
    return resolved.takeIf { it.startsWith(root) && it != file.toAbsolutePath().normalize() && java.nio.file.Files.isRegularFile(it) }
}
