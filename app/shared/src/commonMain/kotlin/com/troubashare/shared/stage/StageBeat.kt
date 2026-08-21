package com.troubashare.shared.stage

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.math.floor

/**
 * A34 — the Stage visual beat: a pulsing frame on the page BORDER (settled with VLL: *"very visual and
 * does not interfere with looking at the page content"* — never over the music, never full-screen).
 *
 * The timeline is [beatPhase]/[beatFrame] (the shared, vector-tested contract in CountIn.kt); this file
 * is the STAGE renderer + a tiny controller. Driven off [withFrameNanos] so the phase is derived from
 * the frame clock — not accumulated `delay()` calls, which drift under load (the A11 bug).
 *
 * Colours are amber-vs-aqua (warm/cool, survives red-green deficiency); the base width is re-tuned
 * LARGER than the studio's 9 px desk value, for a dark stage at arm's length. Read-only (I12).
 */
private val DOWNBEAT = Color(0xFFFFB02E) // amber — every 4th beat
private val OFFBEAT = Color(0xFF3EE0D4) // aqua — grey would read as "off"

/** Peak frame weight for the stage. Wider than the desk's 9 px — read from across a room, hands full. */
private val BEAT_BASE = 16.dp

/**
 * A silent, read-only beat controller. Tap → an 8-beat count-in; long-press → continuous until a
 * second tap or a page turn. Idle when [beats] is 0. Each start/stop bumps [runToken] so the driver
 * effect (re)starts. The current visual is pushed out through [onFrame] (a snapshot-state setter the
 * Stage owns), so the border recomposes but the controller stays render-agnostic.
 */
class StageBeat {
    var runToken by mutableStateOf(0)
        private set

    /** The current border pulse, observed by [StageBeatFrame]; null = dark (transient, ~200 ms). */
    var frame by mutableStateOf<BeatFrame?>(null)
        internal set

    /** The beat-in-bar number to show, 1..4, held for the WHOLE beat while running (null = idle). Cycles
     *  1 2 3 4 1 2 3 4 so the player can keep their place; distinct from [frame]'s brief pulse. */
    var beatLabel by mutableStateOf<Int?>(null)
        internal set

    var tempo = 0
        private set

    /** ∞ mode (studio parity — the ∞ loop toggle in the editor): ON → [toggle] keeps the beat running;
     *  OFF → it's an 8-beat count-in that self-stops. Defaults ON (VLL's preferred "keep it running").
     *  Affects the NEXT start; a run already going finishes as it started. Set it via the ∞ FAB. */
    var continuous by mutableStateOf(true)

    /** Observable so the chrome-auto-hide (and anything else) can react to start/stop. */
    var beats by mutableStateOf(0)
        private set

    val running: Boolean get() = beats > 0

    /** Tap the metronome: stop if running, else start — forever ([continuous]) or an 8-beat count-in.
     *  A metronome you switch on and off. No-op for an out-of-range tempo. */
    fun toggle(tempoBpm: Int) {
        if (running) stop() else start(tempoBpm, if (continuous) CONTINUOUS_BEATS else COUNT_IN_BEATS)
    }

    fun stop() {
        beats = 0
        runToken++
    }

    private fun start(tempoBpm: Int, count: Int) {
        if (tempoIntervalMs(tempoBpm) == null) return
        tempo = tempoBpm
        beats = count
        runToken++
    }
}

/**
 * A [StageBeat] plus its frame-clock driver, scoped to [resetKey] (the current page) so a page turn
 * makes a fresh instance and cancels any in-progress count-in. Render [StageBeatFrame] with its frame.
 */
@Composable
fun rememberStageBeat(resetKey: Any): StageBeat {
    val beat = remember(resetKey) { StageBeat() }
    LaunchedEffect(beat.runToken) {
        beat.frame = null
        beat.beatLabel = null
        if (!beat.running) return@LaunchedEffect
        val interval = intervalMs(beat.tempo)
        val beats = beat.beats
        var startNanos = -1L
        while (true) {
            var done = false
            withFrameNanos { now ->
                if (startNanos < 0) startNanos = now
                val elapsedMs = (now - startNanos) / 1_000_000.0
                val beatIndex = floor(elapsedMs / interval).toInt()
                if (beatIndex >= beats) {
                    beat.frame = null
                    beat.beatLabel = null
                    done = true
                } else {
                    beat.frame = beatFrame(elapsedMs, interval, beats)         // brief border pulse
                    beat.beatLabel = beatIndex % BEATS_PER_BAR + 1             // 1..4, held the whole beat
                }
            }
            if (done) break
        }
        beat.stop() // reached the end → back to idle (bumps runToken into the idle branch)
    }
    return beat
}

/** A little metronome (trapezoid body + leaning pendulum) drawn to [modifier]'s size in [tint]. Reads
 *  as "beat" where a bare dot didn't; tinted amber/aqua on the lit beat so it also echoes the pulse. */
@Composable
fun MetronomeIcon(tint: Color, modifier: Modifier) {
    Canvas(modifier) {
        val w = size.width
        val h = size.height
        val sw = h * 0.085f
        val body = Path().apply {
            moveTo(w * 0.24f, h * 0.90f)
            lineTo(w * 0.76f, h * 0.90f)
            lineTo(w * 0.60f, h * 0.15f)
            lineTo(w * 0.40f, h * 0.15f)
            close()
        }
        drawPath(body, tint, style = Stroke(width = sw, cap = StrokeCap.Round, join = StrokeJoin.Round))
        drawLine(tint, Offset(w * 0.50f, h * 0.84f), Offset(w * 0.635f, h * 0.30f), strokeWidth = sw, cap = StrokeCap.Round)
        drawCircle(tint, radius = h * 0.075f, center = Offset(w * 0.635f, h * 0.30f))
    }
}

/**
 * The running beat: a pulsing frame on the page border PLUS a big, semi-transparent beat number
 * (1 2 3 4 …) in the middle, both in the beat's colour (amber downbeat / aqua off-beat). The border
 * pulse is a brief transient; the number is held for the whole beat so the player keeps their place.
 * Purely visual — it holds NO pointer input, so a tap on the page still turns the page / toggles the
 * chrome (you stop the beat from its FAB, not by touching the score). Nothing renders when idle.
 */
@Composable
fun StageBeatFrame(beat: StageBeat, modifier: Modifier = Modifier) {
    val frame = beat.frame
    val label = beat.beatLabel
    val shape = RoundedCornerShape(10.dp)
    Box(modifier.fillMaxSize()) {
        if (frame != null) {
            val env = frame.env.coerceIn(0f, 1f)
            val color = if (frame.downbeat) DOWNBEAT else OFFBEAT
            val width = BEAT_BASE * (0.45f + 0.55f * env)
            // soft outer halo + crisp inner frame — they "breathe" rather than snap.
            Box(Modifier.fillMaxSize().border(width * 1.7f, color.copy(alpha = env * 0.28f), shape))
            Box(Modifier.fillMaxSize().padding(3.dp).border(width, color.copy(alpha = env * 0.92f), shape))
        }
        if (label != null) {
            val tint = (if (label == 1) DOWNBEAT else OFFBEAT).copy(alpha = 0.34f)
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text("$label", color = tint, fontSize = 168.sp, fontWeight = FontWeight.Bold)
            }
        }
    }
}
