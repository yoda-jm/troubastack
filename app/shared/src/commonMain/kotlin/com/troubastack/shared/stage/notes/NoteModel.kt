// A70 — Rehearsal notes on Stage (Part A). PURE model + geometry + tools for a single per-page bitmap note.
//
// A note is deliberately second-class (docs/tasks/A70 §1.4): keyed by the page's RASTER HASH so it orphans
// the moment the lyrics change, it lives on one tablet under one Stage identity and never enters a bundle,
// and it nags after a few bakes. Every limit here is a reason to recopy the note into a real annotation in
// Studio while it still means something. Nothing in this package does I/O — the host implements the
// RehearsalNotes port (§4.1); `stage/` only draws into memory and hands bitmaps over (I12, amended §3.1).
package com.troubastack.shared.stage.notes

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.ImageBitmap
import kotlinx.serialization.Serializable
import kotlin.math.roundToInt

/** §3.3 — a note's identity: the OWNING song plus the page's raster content hash. Page INDEX is never part
 *  of the key (T145's lesson: an index is silently re-pointed by a reflow); it is recorded for labels and
 *  Part B placement only. `songId` is in the key because two songs can share a byte-identical raster. */
data class NoteKey(val songId: String, val rasterHash: String)

/** The two tools (VLL #1/#12): pencil and eraser, opaque, no undo/clear/opacity. */
enum class NoteTool { PENCIL, ERASER }

/**
 * §3.3 — one persisted note's metadata (the bitmap is a sibling PNG). `takenAs` is the Stage identity at
 * save time (#8). `sentAt` is set by the tablet's send action, not the server's state (#9). `bakesSinceTouched`
 * drives the `Notes ⚠` nag (§3.4). Page/rev/titles are snapshots for labels and orphan display, never keys.
 */
@Serializable
data class NoteEntry(
    val songId: String,
    val rasterHash: String,
    val file: String,
    val pageInSong: Int,
    val songTitle: String,
    val bandName: String,
    val concertRev: Long,
    val takenAs: String,
    val width: Int,
    val height: Int,
    val updatedAt: Long,
    val sentAt: Long? = null,
    val bakesSinceTouched: Int = 0,
) {
    val key: NoteKey get() = NoteKey(songId, rasterHash)
}

/** §3.6 / §3.5 — the fixed constants. Widths in NOTE px; the executor tunes on the tablet and reports. */
object NoteTools {
    /** Canonical note-bitmap width; height derives from the decoded raster aspect (§3.5). */
    const val NOTE_W = 1600

    // §3.6 — three widths: a line to write with, a medium to circle with, a wide to redact with.
    const val FINE = 3
    const val MEDIUM = 9
    const val WIDE = 40
    val WIDTHS = listOf(FINE, MEDIUM, WIDE)

    /** The eraser is the selected width ×3, never finer than MEDIUM (its preview cannot be painted over). */
    fun eraserWidth(penWidth: Int): Int = maxOf(penWidth * 3, MEDIUM)

    // §3.6 — four opaque colours as neutral ARGB (the STORED colours). All either achromatic (black inverts
    // with the paper) or chromatic (hue kept, lightness remapped) under A64, so each reads the same in every
    // scheme. Paper-white was rejected (VLL): a redaction must stay visible, so it is a WIDE black stroke.
    const val BLACK = 0xFF111111L
    const val RED = 0xFFE53935L
    const val BLUE = 0xFF1E63D6L
    const val GREEN = 0xFF2E8B3AL
    val COLOURS = listOf(BLACK, RED, BLUE, GREEN)

    /** §3.4 — a note is "old" (nags) once this many bakes have passed since it was last touched/sent. */
    const val NAG_BAKES = 3

    /** §3.8 — the reserved per-song layer id for the note row. No bake layer id starts with `~` (guarded). */
    const val RESERVED_LAYER_ID = "~notes"
}

/** §3.4 — the `Notes ⚠` tab-label state: no warning, orange (a note is old), or red (a note is very old). */
enum class NotesWarning { NONE, ORANGE, RED }

/** A note bitmap's pixel dimensions (a pure return type; the UI maps it to IntSize). */
data class NoteDim(val width: Int, val height: Int)

/**
 * §3.5 / §4.1 — pure geometry. No Compose UI state, so it is unit-tested off-device (A58).
 */
object NoteGeometry {
    /** §3.5 — the canonical note size for a page: fixed width, height from the DECODED raster's aspect. */
    fun noteSize(rasterW: Int, rasterH: Int): NoteDim {
        if (rasterW <= 0 || rasterH <= 0) return NoteDim(0, 0)
        return NoteDim(NoteTools.NOTE_W, (NoteTools.NOTE_W.toDouble() * rasterH / rasterW).roundToInt())
    }

    /**
     * §4.1 — map a touch (px, in the page composable's own box) to note-bitmap pixel coords, or null if the
     * touch fell OUTSIDE the drawable page. [fillWidth] = false is FIT_PAGE (ContentScale.Fit centres +
     * contains the raster, so a letterbox touch is null); true is FIT_WIDTH (the box IS the raster,
     * width-filled and vertically scrolled by [scrollOffsetY], so only the visible band is drawable).
     * A stroke leaving the page is the caller's clamp; this returns null for a touch clean off the page.
     */
    fun touchToNote(
        tapX: Float, tapY: Float, boxW: Int, boxH: Int, noteW: Int, noteH: Int,
        fillWidth: Boolean, scrollOffsetY: Int = 0,
    ): Offset? {
        if (boxW <= 0 || boxH <= 0 || noteW <= 0 || noteH <= 0) return null
        if (fillWidth) {
            // The raster fills the box width; full content height = boxW * noteH/noteW. The visible window is
            // [scrollOffsetY, scrollOffsetY+boxH]. Both axes scale by noteW/boxW (x directly; y because the
            // content is rendered at box width). null only if the touch is off the box horizontally.
            if (tapX < 0f || tapX > boxW) return null
            val s = noteW.toDouble() / boxW
            val nx = tapX * s
            val ny = (tapY + scrollOffsetY) * s
            if (ny < 0.0 || ny > noteH) return null
            return Offset(nx.toFloat(), ny.toFloat())
        }
        // FIT_PAGE: the raster is scaled to the LARGER letterbox that still fits, then centred (like
        // tapToPagePermille). A touch in a bar maps to null.
        val aspect = noteW.toDouble() / noteH
        val boxAspect = boxW.toDouble() / boxH
        val rw: Double; val rh: Double; val ox: Double; val oy: Double
        if (aspect > boxAspect) { // width-bound, bars top/bottom
            rw = boxW.toDouble(); rh = boxW / aspect; ox = 0.0; oy = (boxH - rh) / 2.0
        } else { // height-bound, bars left/right
            rh = boxH.toDouble(); rw = boxH * aspect; oy = 0.0; ox = (boxW - rw) / 2.0
        }
        if (rw <= 0.0 || rh <= 0.0) return null
        val u = (tapX - ox) / rw
        val v = (tapY - oy) / rh
        if (u < 0.0 || u > 1.0 || v < 0.0 || v > 1.0) return null
        return Offset((u * noteW).toFloat(), (v * noteH).toFloat())
    }
}

/**
 * §4.1 — the host port `stage/` draws into (DI like ImageDecoder, NOT a new seam). The only I/O `stage/`
 * may reference (guarded by §4.6). Implementations write PNG-first then index-last, both tmp+rename, and a
 * save of an all-transparent bitmap DELETES the note (#10). `PlatformBitmap` is the host's opaque handle.
 */
interface RehearsalNotes {
    fun index(concertId: String): List<NoteEntry>
    fun load(concertId: String, key: NoteKey): ImageBitmap?
    fun save(concertId: String, entry: NoteEntry, bitmap: ImageBitmap)
    fun delete(concertId: String, key: NoteKey)
    fun deleteAll(concertId: String)
    fun markSent(concertId: String, key: NoteKey, at: Long)
}
