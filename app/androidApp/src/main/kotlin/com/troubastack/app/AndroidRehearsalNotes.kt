package com.troubastack.app

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.graphics.asImageBitmap
import com.troubastack.shared.stage.notes.NoteEntry
import com.troubastack.shared.stage.notes.NoteKey
import com.troubastack.shared.stage.notes.RehearsalNotes
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import java.io.FileOutputStream
import java.security.MessageDigest

/** The on-disk shape of one concert's note index (§3.3 — the index is the truth about what exists). */
@Serializable
private data class NoteIndexFile(val entries: List<NoteEntry> = emptyList())

/**
 * A70 §4.4 — the Android rehearsal-notes host: PNG-encode/decode a per-page bitmap under
 * `<notesRoot>/<concertId>/`, keyed by `sha256(songId "\n" rasterHash)[..16]`. Write order is PNG-first
 * then index-last, both via tmp+rename, so a crash can only leave a file without an index entry — which
 * [index] reconciles away (§3.3). A save of an all-transparent bitmap DELETES the note (#10). Never throws
 * (a value at every edge, like the rest of the stage host); on any I/O failure it degrades to "no note".
 */
class AndroidRehearsalNotes(private val notesRoot: String) : RehearsalNotes {
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
            tmp.renameTo(indexFile(concertId))
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
        // inMutable: a decoded PNG is otherwise IMMUTABLE, and the loaded note becomes the working bitmap the
        // pad draws/erases into (a Canvas over an immutable bitmap throws). The note always DISPLAYS before it
        // is edited, so the load happens in display mode and the neutral is reused when note mode is entered —
        // it must be mutable from the decode, not copied later. (A70 note-edit crash, reload lifecycle.)
        val opts = BitmapFactory.Options().apply { inMutable = true }
        return runCatching { BitmapFactory.decodeFile(f.path, opts)?.asImageBitmap() }.getOrNull()
    }

    override fun save(concertId: String, entry: NoteEntry, bitmap: ImageBitmap) {
        val android = runCatching { bitmap.asAndroidBitmap() }.getOrNull() ?: return
        if (isAllTransparent(android)) { delete(concertId, entry.key); return }
        val d = dir(concertId).apply { mkdirs() }
        val fname = fileNameFor(entry.key)
        val tmp = File(d, "$fname.tmp")
        val ok = runCatching {
            FileOutputStream(tmp).use { android.compress(Bitmap.CompressFormat.PNG, 100, it) }
            tmp.renameTo(File(d, fname))
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

    /** True iff no pixel has any alpha — the "nothing was drawn / all erased" case that deletes the note. */
    private fun isAllTransparent(b: Bitmap): Boolean {
        if (!b.hasAlpha()) return false
        val w = b.width; val h = b.height
        if (w <= 0 || h <= 0) return true
        val row = IntArray(w)
        for (y in 0 until h) {
            b.getPixels(row, 0, w, 0, y, w, 1)
            for (px in row) if ((px ushr 24) != 0) return false
        }
        return true
    }

    private fun sha256(s: String): String {
        val d = MessageDigest.getInstance("SHA-256").digest(s.encodeToByteArray())
        return d.joinToString("") { ((it.toInt() and 0xFF) + 0x100).toString(16).substring(1) }
    }
}
