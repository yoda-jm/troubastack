// A70 §3.5/§3.6 — the rehearsal-note DRAWING surface: one transparent bitmap per page, pencil + eraser,
// opaque. Draws into a Compose ImageBitmap in commonMain (Canvas over the bitmap); the host port only
// persists/loads the PNG (no pixels cross a seam except through the port). Display goes through A64's
// transformOverlayBitmap so a red note stays red and a black one inverts on dark paper.
package com.troubastack.shared.stage

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
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
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import com.troubastack.shared.stage.notes.NoteEntry
import com.troubastack.shared.stage.notes.NoteGeometry
import com.troubastack.shared.stage.notes.NoteKey
import com.troubastack.shared.stage.notes.NoteTool
import com.troubastack.shared.stage.notes.NoteTools
import com.troubastack.shared.stage.notes.RehearsalNotes
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
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
 * §3.5 — the note layer over one page, ABOVE every baked overlay (#11). Displays the stored note
 * (scheme-transformed) always; when [editable] (the current page in note mode) it also captures pencil /
 * eraser input into an in-memory NEUTRAL bitmap, commits each stroke at pen-up, and persists on idle /
 * dispose through the [notes] port. [imageMod] MUST be the raster Image's modifier so the note registers
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
        val bmp = loaded ?: if (editable) ImageBitmap(noteDim.width.coerceAtLeast(1), noteDim.height.coerceAtLeast(1)) else null
        neutral = bmp
        display = bmp?.let { withContext(Dispatchers.Default) { transformOverlayBitmap(it, scheme) } }
    }
    // Re-transform the display when the scheme changes (the stored bytes are always neutral).
    LaunchedEffect(scheme) {
        val n = neutral ?: return@LaunchedEffect
        display = withContext(Dispatchers.Default) { transformOverlayBitmap(n, scheme) }
    }

    // Persistence flush: pen-up marks dirty; save after IDLE_FLUSH_MS of quiet or on leave (§4.5). Saving an
    // all-transparent bitmap deletes the note (the port's job). After a save, refresh the index so counts and
    // the ✎ badge update.
    val flush = remember(key) { com.troubastack.shared.stage.notes.NoteFlushPolicy() }
    var dirtyTick by remember(key) { mutableStateOf(0) }
    val latestNeutral = rememberUpdatedState(neutral)
    suspend fun persist() {
        val n = latestNeutral.value ?: return
        val entry = NoteEntry(
            songId = page.songId, rasterHash = page.rasterHash,
            file = "", // the port names the file from the key
            pageInSong = meta.pageInSong, songTitle = meta.songTitle, bandName = meta.bandName,
            concertRev = meta.concertRev, takenAs = meta.takenAs,
            width = n.width, height = n.height, updatedAt = monotonicNow(),
        )
        withContext(Dispatchers.Default) { notes.save(concertId, entry, n) }
        onIndexChanged(withContext(Dispatchers.Default) { notes.index(concertId) })
    }
    LaunchedEffect(dirtyTick, key) {
        if (dirtyTick == 0) return@LaunchedEffect
        delay(NoteTools.IDLE_FLUSH_MS)
        if (flush.due(monotonicNow())) { persist(); flush.cleared() }
    }
    DisposableEffect(key) {
        onDispose { /* force flush handled by the host onStop / exit; a compose dispose cannot suspend */ }
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
            Modifier.pointerInput(key, tool, penWidth, penColour) {
                detectDragGestures(
                    onDragStart = { off -> wet = listOf(off) },
                    onDrag = { change, _ ->
                        change.consume()
                        val n = latestNeutral.value ?: return@detectDragGestures
                        if (tool == NoteTool.ERASER) {
                            // Erase applies immediately to the bitmap (no wet preview can be painted over).
                            val p = NoteGeometry.touchToNote(change.position.x, change.position.y, size.width, size.height, n.width, n.height, fillWidth) ?: return@detectDragGestures
                            eraseInto(n, p, NoteTools.eraserWidth(penWidth).toFloat() * n.width / NoteTools.NOTE_W)
                            display = transformOverlayBitmap(n, scheme)
                            flush.dirty(monotonicNow()); dirtyTick++
                        } else {
                            wet = wet + change.position
                        }
                    },
                    onDragEnd = {
                        val n = latestNeutral.value
                        if (tool == NoteTool.PENCIL && n != null && wet.size >= 1) {
                            // Commit the wet stroke into the NEUTRAL bitmap in note space, in one draw (§3.6).
                            val pts = wet.mapNotNull { NoteGeometry.touchToNote(it.x, it.y, size.width, size.height, n.width, n.height, fillWidth) }
                            if (pts.isNotEmpty()) {
                                strokeInto(n, pts, penColour, penWidth.toFloat() * n.width / NoteTools.NOTE_W)
                                display = transformOverlayBitmap(n, scheme)
                                flush.dirty(monotonicNow()); dirtyTick++
                            }
                        }
                        wet = emptyList()
                    },
                )
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
