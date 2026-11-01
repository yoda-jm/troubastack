package com.troubastack.app

import androidx.compose.ui.graphics.ImageBitmap
import com.troubastack.shared.stage.notes.NoteEntry
import com.troubastack.shared.stage.notes.NoteKey
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import java.io.FileOutputStream
import java.io.OutputStream
import java.nio.file.AtomicMoveNotSupportedException
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.security.MessageDigest

/** The on-disk shape of one concert's note index (§3.3 — the index is the truth about what exists). */
@Serializable
private data class NoteIndexFile(val entries: List<NoteEntry> = emptyList())

/**
 * WIN03a — the bitmap half of the rehearsal-notes store: the only platform calls it makes. Android: BitmapFactory /
 * `Bitmap.compress`; desktop: Skia.
 */
interface NoteBitmapCodec {
    /** Decode the PNG at [path] as a MUTABLE bitmap (the pad draws and erases into it), or null. */
    fun decode(path: String): ImageBitmap?

    /** True iff no pixel has any alpha (nothing drawn, or all erased). Null when [bitmap] cannot be read here. */
    fun isAllTransparent(bitmap: ImageBitmap): Boolean?

    /** Encode [bitmap] as a PNG into [out]. */
    fun writePng(bitmap: ImageBitmap, out: OutputStream)
}

/**
 * A70 §4.4 — the rehearsal-notes host: PNG-encode/decode a per-page bitmap under
 * `<notesRoot>/<concertId>/`, keyed by `sha256(songId "\n" rasterHash)[..16]`. Write order is PNG-first
 * then index-last, both via tmp+rename, so a crash can only leave a file without an index entry — which
 * [index] reconciles away (§3.3). A save of an all-transparent bitmap DELETES the note (#10). Never throws
 * (a value at every edge, like the rest of the stage host); on any I/O failure it degrades to "no note".
 * WIN03a: the on-disk format, shared by the JVM platforms; [codec] does the bitmap calls.
 */
class FileRehearsalNotes(private val notesRoot: String, private val codec: NoteBitmapCodec) : RehearsalNotesStore {
    private val json = Json { ignoreUnknownKeys = true; encodeDefaults = true }

    private fun dir(concertId: String) = File(notesRoot, concertId)
    private fun indexFile(concertId: String) = File(dir(concertId), "index.json")
    private fun fileNameFor(key: NoteKey) = sha256("${key.songId}\n${key.rasterHash}").substring(0, 16) + ".png"

    private fun readEntries(concertId: String): List<NoteEntry> {
        val f = indexFile(concertId)
        if (!f.exists()) return emptyList()
        return runCatching { json.decodeFromString<NoteIndexFile>(f.readText()).entries }.getOrDefault(emptyList())
    }

    private fun writeIndex(concertId: String, entries: List<NoteEntry>) {
        val d = dir(concertId).apply { mkdirs() }
        val tmp = File(d, "index.json.tmp")
        runCatching {
            tmp.writeText(json.encodeToString(NoteIndexFile(entries)))
            replace(tmp, indexFile(concertId))
        }
    }

    override fun index(concertId: String): List<NoteEntry> {
        val d = dir(concertId)
        if (!d.exists()) return emptyList()
        val entries = readEntries(concertId)
        // Reconcile (§3.3): an entry whose PNG is gone is dropped; a PNG with no entry is unaddressable
        // garbage (its name is an irreversible hash) and is deleted.
        val present = entries.filter { File(d, it.file).exists() }
        val known = present.mapTo(HashSet()) { it.file }.apply { add("index.json") }
        d.listFiles()?.forEach { if (it.name.endsWith(".png") && it.name !in known) it.delete() }
        if (present.size != entries.size) writeIndex(concertId, present)
        return present
    }

    override fun load(concertId: String, key: NoteKey): ImageBitmap? {
        val f = File(dir(concertId), fileNameFor(key))
        if (!f.exists()) return null
        return codec.decode(f.path)
    }

    override fun save(concertId: String, entry: NoteEntry, bitmap: ImageBitmap) {
        val blank = codec.isAllTransparent(bitmap) ?: return
        if (blank) { delete(concertId, entry.key); return }
        val d = dir(concertId).apply { mkdirs() }
        val fname = fileNameFor(entry.key)
        val tmp = File(d, "$fname.tmp")
        val ok = runCatching {
            FileOutputStream(tmp).use { codec.writePng(bitmap, it) }
            replace(tmp, File(d, fname))
        }.getOrDefault(false)
        if (ok != true) { tmp.delete(); return }
        // Index last: upsert this key, reset the nag (a fresh save is freshly untouched + unsent — §3.4).
        val others = readEntries(concertId).filterNot { it.key == entry.key }
        val saved = entry.copy(file = fname, bakesSinceTouched = 0, sentAt = null)
        writeIndex(concertId, others + saved)
    }

    override fun delete(concertId: String, key: NoteKey) {
        File(dir(concertId), fileNameFor(key)).delete()
        writeIndex(concertId, readEntries(concertId).filterNot { it.key == key })
    }

    override fun deleteAll(concertId: String) { dir(concertId).deleteRecursively() }

    override fun markSent(concertId: String, key: NoteKey, at: Long) {
        writeIndex(concertId, readEntries(concertId).map { if (it.key == key) it.copy(sentAt = at) else it })
    }

    /** T170 §6 — the raw stored PNG bytes for one note, for the send-to-Studio underlay. Returns the file
     *  VERBATIM, never a re-encode: the note must travel neutral, exactly the pixels that were authored
     *  (T170 §1 — Studio renders it as-is, no scheme transform). Null if the PNG is gone. */
    override fun pngBytes(concertId: String, key: NoteKey): ByteArray? {
        val f = File(dir(concertId), fileNameFor(key))
        return if (f.exists()) runCatching { f.readBytes() }.getOrNull() else null
    }

    /** Put [tmp] in place of [target] in one step, replacing it. Not `File.renameTo`: on Windows it refuses an
     *  existing target (returns false), so every index rewrite after the first and every re-save was lost there
     *  (WIN03a, seen on the windows CI job). Same-directory move: rename(2) on Linux/Android, MoveFileEx on Windows. */
    private fun replace(tmp: File, target: File): Boolean = runCatching {
        try {
            Files.move(tmp.toPath(), target.toPath(), StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE)
        } catch (_: AtomicMoveNotSupportedException) {
            Files.move(tmp.toPath(), target.toPath(), StandardCopyOption.REPLACE_EXISTING) // as WIN02's desktop Storage
        }
        true
    }.getOrDefault(false)

    private fun sha256(s: String): String {
        val d = MessageDigest.getInstance("SHA-256").digest(s.encodeToByteArray())
        return d.joinToString("") { ((it.toInt() and 0xFF) + 0x100).toString(16).substring(1) }
    }
}
