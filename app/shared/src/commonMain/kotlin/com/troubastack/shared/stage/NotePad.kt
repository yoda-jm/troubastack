// A70 §3.5/§3.6 — the rehearsal-note DRAWING surface: one transparent bitmap per page, pencil + eraser,
// opaque. Draws into a Compose ImageBitmap in commonMain (Canvas over the bitmap); the host port only
// persists/loads the PNG (no pixels cross a seam except through the port). Display goes through A64's
// transformOverlayBitmap so a red note stays red and a black one inverts on dark paper.
package com.troubastack.shared.stage

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Canvas as GraphicsCanvas
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.Paint
import androidx.compose.ui.graphics.PaintingStyle
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.painter.BitmapPainter
import androidx.compose.ui.input.pointer.changedToDown
import androidx.compose.ui.input.pointer.changedToUp
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.input.pointer.positionChanged
import androidx.compose.ui.layout.ContentScale
import com.troubastack.shared.stage.notes.NoteEntry
import com.troubastack.shared.stage.notes.NoteGeometry
import com.troubastack.shared.stage.notes.NoteKey
import com.troubastack.shared.stage.notes.NoteTool
import com.troubastack.shared.stage.notes.NoteTools
import com.troubastack.shared.stage.notes.RehearsalNotes
import com.troubastack.shared.stage.notes.StrokePoint
import com.troubastack.shared.stage.notes.StrokeReader
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext

/** Metadata the note bitmap does not carry, snapshotted at save time for labels + Part B placement (§3.3). */
data class NoteMeta(
    val songTitle: String,
    val bandName: String,
    val concertRev: Long,
    val takenAs: String,
    val pageInSong: Int,
)

/**
 * Everything a page needs to render + (in note mode) edit its note, threaded from the host through
 * [StageScreen]. Null when the notes feature is inactive (iOS host / tests), so the reading surface is
 * byte-for-byte unchanged where notes are off. [shownFor] gates the display-only case (a page with a live,
 * un-hidden note outside note mode).
 */
class NotePad(
    val notes: RehearsalNotes,
    val concertId: String,
    val scheme: StageColorMode,
    val noteMode: Boolean,
    val tool: NoteTool,
    val penWidth: Int,
    val penColour: Long,
    val noteRevision: Int,
    val now: () -> Long,
    val meta: (StagePage) -> NoteMeta,
    val shownFor: (StagePage) -> Boolean,
    val onIndexChanged: (List<NoteEntry>) -> Unit,
    val onBumpRevision: () -> Unit,
)

/** The inert port for hosts/tests without a notes backend — the surface renders and edits nothing. */
val NoOpRehearsalNotes: RehearsalNotes = object : RehearsalNotes {
    override fun index(concertId: String): List<NoteEntry> = emptyList()
    override fun load(concertId: String, key: NoteKey): ImageBitmap? = null
    override fun save(concertId: String, entry: NoteEntry, bitmap: ImageBitmap) {}
    override fun delete(concertId: String, key: NoteKey) {}
    override fun deleteAll(concertId: String) {}
    override fun markSent(concertId: String, key: NoteKey, at: Long) {}
}

/**
 * §3.5 — the note layer over one page, ABOVE every baked overlay (#11). Displays the stored note
 * (scheme-transformed) always; when [editable] (the current page in note mode) it also captures pencil /
 * eraser input into an in-memory NEUTRAL bitmap, commits each stroke at pen-up, and persists it (a private
 * copy) through the [notes] port. [imageMod] MUST be the raster Image's modifier so the note registers
 * with the page (§2 — the sibling-Image registration mechanism), and [fillWidth] its ContentScale.
 */
@Composable
fun NoteLayer(
    page: StagePage,
    noteDim: androidx.compose.ui.unit.IntSize,
    concertId: String,
    notes: RehearsalNotes,
    scheme: StageColorMode,
    editable: Boolean,
    tool: NoteTool,
    penWidth: Int,
    penColour: Long,
    imageMod: Modifier,
    fillWidth: Boolean,
    noteRevision: Int,
    meta: NoteMeta,
    monotonicNow: () -> Long,
    onIndexChanged: (List<NoteEntry>) -> Unit,
    onBumpRevision: () -> Unit,
) {
    val key = NoteKey(page.songId, page.rasterHash)
    // The NEUTRAL working bitmap for this page (the stored colours). Loaded once per (page, revision); a
    // fresh transparent bitmap when there is nothing on disk yet. Editing mutates it in place.
    var neutral by remember(key, noteRevision) { mutableStateOf<ImageBitmap?>(null) }
    var display by remember(key, noteRevision) { mutableStateOf<ImageBitmap?>(null) }
    LaunchedEffect(key, noteRevision) {
        val loaded = withContext(Dispatchers.Default) { notes.load(concertId, key) }
        // A note decoded from disk is an IMMUTABLE bitmap; editing it (strokeInto / eraseInto open a Canvas
        // over it) throws "Immutable bitmap passed to Canvas". A freshly-created ImageBitmap is mutable, so
        // this only bites when EDITING a note that already exists on disk — which the in-session pixel pass
        // never did. When editable, take a mutable working copy; display-only keeps the cheap immutable decode.
        val bmp = when {
            loaded != null -> if (editable) withContext(Dispatchers.Default) { copyBitmap(loaded) } else loaded
            editable -> ImageBitmap(noteDim.width.coerceAtLeast(1), noteDim.height.coerceAtLeast(1))
            else -> null
        }
        neutral = bmp
        display = bmp?.let { withContext(Dispatchers.Default) { transformOverlayBitmap(it, scheme) } }
    }
    // Re-transform the display when the scheme changes (the stored bytes are always neutral).
    LaunchedEffect(scheme) {
        val n = neutral ?: return@LaunchedEffect
        display = withContext(Dispatchers.Default) { transformOverlayBitmap(n, scheme) }
    }

    // Persistence: each pen-up commits the stroke to the bitmap then SAVES through the port (§4.5). Saving
    // eagerly (at pen-up, not on an idle timer) means leaving Stage right after a stroke never loses it. An
    // all-transparent bitmap deletes the note (the port's job). After a save, refresh the index so the
    // counts + ✎ badge update. Runs on the composition scope, which survives note-mode exit (page still shows).
    val scope = rememberCoroutineScope()
    val latestNeutral = rememberUpdatedState(neutral)
    // Saves are serialized per page so two overlapping saves can't finish out of order and leave an older
    // snapshot as the file on disk.
    val saveMutex = remember(key) { Mutex() }
    fun persist() {
        val n = latestNeutral.value ?: return
        // Snapshot the bitmap on THIS (main) thread before handing it to the off-main encoder. The next
        // stroke mutates `neutral` in place on the main thread, and Android's compress() would otherwise
        // read that same bitmap concurrently → a torn PNG or a native crash. The copy is cheap next to a
        // stroke and happens once per pen-up, so the encoder always sees a stable, private image.
        val snapshot = copyBitmap(n)
        val entry = NoteEntry(
            songId = page.songId, rasterHash = page.rasterHash,
            file = "", // the port names the file from the key
            pageInSong = meta.pageInSong, songTitle = meta.songTitle, bandName = meta.bandName,
            concertRev = meta.concertRev, takenAs = meta.takenAs,
            width = snapshot.width, height = snapshot.height, updatedAt = monotonicNow(),
        )
        scope.launch(Dispatchers.Default) {
            saveMutex.withLock {
                notes.save(concertId, entry, snapshot)
                val idx = notes.index(concertId)
                // Refresh the index (counts + ✎ badge). NOT a revision bump — the in-memory bitmap is already
                // current, so re-keying it would blank+reload the note mid-stroke.
                withContext(Dispatchers.Main) { onIndexChanged(idx) }
            }
        }
    }

    // Draw the committed note (scheme-transformed), and — while editing — the wet stroke on top.
    display?.let { Image(BitmapPainter(it), contentDescription = null, modifier = imageMod, contentScale = if (fillWidth) ContentScale.FillWidth else ContentScale.Fit, colorFilter = null) }

    if (!editable) return

    // The wet pencil stroke, in SCREEN space, drawn in the scheme-transformed colour so it matches the
    // committed ink. The eraser has no wet preview (it applies to the bitmap on every move — §3.6).
    var wet by remember(key) { mutableStateOf<List<Offset>>(emptyList()) }
    val drawColour = Color(transformOverlayPixel(penColour.toInt(), scheme)) // Color(Int) reads 0xAARRGGBB
    Canvas(
        imageMod.then(
            // A71 §3/§4 — read the finger DIRECTLY: no slop, the touchdown is the first point. Note mode owns
            // the touch (§2: chrome tap, turn-swipe, fit-width scroll and jump taps are all off in note mode),
            // so there is nothing to disambiguate and no reason for the scroll-oriented detectors that discard
            // the first ~1.5 mm and report a stab at finger-up. One StrokeReader, both tools, first pointer wins.
            Modifier.pointerInput(key, tool, penWidth, penColour) {
                val reader = StrokeReader()
                // Eraser: apply a dab at each point immediately (no wet preview can be painted over — §3.6).
                // Pencil: grow the wet preview from the reader's own point list (which includes the touchdown).
                fun onPoint(x: Float, y: Float) {
                    if (tool == NoteTool.ERASER) {
                        val n = latestNeutral.value ?: return
                        val p = NoteGeometry.touchToNote(x, y, size.width, size.height, n.width, n.height, fillWidth) ?: return
                        eraseInto(n, p, NoteTools.eraserWidth(penWidth).toFloat() * n.width / NoteTools.NOTE_W)
                        display = transformOverlayBitmap(n, scheme)
                    } else {
                        wet = reader.current.map { Offset(it.x, it.y) }
                    }
                }
                fun onCommit(points: List<StrokePoint>) {
                    val n = latestNeutral.value
                    if (n != null) {
                        if (tool == NoteTool.PENCIL) {
                            // Commit the whole stroke — touchdown included — into the NEUTRAL bitmap, note space.
                            val pts = points.mapNotNull { NoteGeometry.touchToNote(it.x, it.y, size.width, size.height, n.width, n.height, fillWidth) }
                            if (pts.isNotEmpty()) {
                                strokeInto(n, pts, penColour, penWidth.toFloat() * n.width / NoteTools.NOTE_W)
                                display = transformOverlayBitmap(n, scheme)
                            }
                        }
                        persist() // pencil committed OR eraser dabs applied live → save either way
                    }
                    wet = emptyList()
                }
                awaitEachGesture {
                    val first = awaitFirstDown(requireUnconsumed = false)
                    reader.down(first.id.value, first.position.x, first.position.y)?.let { onPoint(it.x, it.y) }
                    first.consume() // D4: consume what we read
                    while (reader.drawing) {
                        val event = awaitPointerEvent()
                        for (c in event.changes) {
                            when {
                                c.changedToDown() -> { reader.down(c.id.value, c.position.x, c.position.y)?.let { onPoint(it.x, it.y) }; c.consume() }
                                c.changedToUp() -> { reader.up(c.id.value)?.let { onCommit(it) }; c.consume() }
                                c.pressed && c.positionChanged() -> { reader.move(c.id.value, c.position.x, c.position.y)?.let { onPoint(it.x, it.y) }; c.consume() }
                            }
                        }
                    }
                }
            },
        ),
    ) {
        if (tool == NoteTool.PENCIL && wet.size >= 2) {
            val path = Path().apply {
                moveTo(wet[0].x, wet[0].y)
                for (i in 1 until wet.size) lineTo(wet[i].x, wet[i].y)
            }
            // The wet stroke is in SCREEN px; scale the pen width from note px (NOTE_W wide) to screen px by
            // the box width (approximate on a letterboxed FIT_PAGE — the COMMITTED stroke is exact).
            val screenW = penWidth.toFloat() * size.width / NoteTools.NOTE_W
            drawPath(path, drawColour, style = Stroke(width = maxOf(screenW, 1.5f), cap = StrokeCap.Round, join = StrokeJoin.Round))
        }
    }
}

/** Copy [src] on the calling (main) thread so an off-main PNG encode can't race the next stroke's in-place
 *  mutation of the working bitmap (§4.5). */
private fun copyBitmap(src: ImageBitmap): ImageBitmap {
    val copy = ImageBitmap(src.width, src.height)
    GraphicsCanvas(copy).drawImage(src, Offset.Zero, Paint())
    return copy
}

/** Draw a polyline stroke into [bmp] at [noteColour] (opaque), round cap/join (§3.6). Note-space coords. */
private fun strokeInto(bmp: ImageBitmap, pts: List<Offset>, noteColour: Long, width: Float) {
    val canvas = GraphicsCanvas(bmp)
    val paint = Paint().apply {
        color = Color(noteColour or 0xFF000000L)
        style = PaintingStyle.Stroke
        strokeWidth = maxOf(width, 1f)
        strokeCap = StrokeCap.Round
        strokeJoin = StrokeJoin.Round
        isAntiAlias = true
    }
    if (pts.size == 1) {
        // A dot: a zero-length stroke with a round cap renders nothing on some backends — draw a fill blob.
        val dot = Paint().apply { color = paint.color; style = PaintingStyle.Fill; isAntiAlias = true }
        canvas.drawCircle(pts[0], maxOf(width, 1f) / 2f, dot)
        return
    }
    val path = Path().apply { moveTo(pts[0].x, pts[0].y); for (i in 1 until pts.size) lineTo(pts[i].x, pts[i].y) }
    canvas.drawPath(path, paint)
}

/** Erase a round dab into [bmp] at [p] (note space), clearing to transparent (§3.6 — undo IS the eraser). */
private fun eraseInto(bmp: ImageBitmap, p: Offset, width: Float) {
    val canvas = GraphicsCanvas(bmp)
    val paint = Paint().apply {
        blendMode = BlendMode.Clear
        style = PaintingStyle.Fill
        color = Color.Transparent
        isAntiAlias = true
    }
    canvas.drawCircle(p, maxOf(width, 1f) / 2f, paint)
}
