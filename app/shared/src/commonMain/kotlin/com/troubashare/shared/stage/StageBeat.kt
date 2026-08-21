package com.troubashare.shared.stage

import androidx.compose.foundation.border
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
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

    /** The current visual, observed by [StageBeatFrame]; null = dark. Written by the driver only. */
    var frame by mutableStateOf<BeatFrame?>(null)
        internal set

    var tempo = 0
        private set
    var beats = 0
        private set

    val running: Boolean get() = beats > 0

    /** Tap: a fixed 8-beat count-in. No-op for an out-of-range tempo. */
    fun tap(tempoBpm: Int) = start(tempoBpm, COUNT_IN_BEATS)

    /** Long-press: keep running (metronome) until tapped off or a page turn. */
    fun toggleContinuous(tempoBpm: Int) {
        if (running) stop() else start(tempoBpm, CONTINUOUS_BEATS)
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
        if (!beat.running) return@LaunchedEffect
        val interval = intervalMs(beat.tempo)
        val beats = beat.beats
        var startNanos = -1L
        while (true) {
            var done = false
            withFrameNanos { now ->
                if (startNanos < 0) startNanos = now
                val elapsedMs = (now - startNanos) / 1_000_000.0
                if (floor(elapsedMs / interval).toInt() >= beats) {
                    beat.frame = null
                    done = true
                } else {
                    beat.frame = beatFrame(elapsedMs, interval, beats)
                }
            }
            if (done) break
        }
        beat.stop() // reached the end → back to idle (bumps runToken into the idle branch)
    }
    return beat
}

/** The pulsing edge frame. Border-only (transparent centre) so it never covers the music, and it holds
 *  no pointer input so taps still reach the page/chrome underneath. Nothing when [frame] is null. */
@Composable
fun StageBeatFrame(frame: BeatFrame?, modifier: Modifier = Modifier) {
    if (frame == null) return
    val env = frame.env.coerceIn(0f, 1f)
    val color = if (frame.downbeat) DOWNBEAT else OFFBEAT
    val width = BEAT_BASE * (0.45f + 0.55f * env)
    val shape = RoundedCornerShape(10.dp)
    // A soft outer halo (the studio's box-shadow, approximated as a wider, fainter frame) + the crisp
    // frame on top — together they "breathe" rather than snap.
    androidx.compose.foundation.layout.Box(modifier.fillMaxSize()) {
        androidx.compose.foundation.layout.Box(
            Modifier.fillMaxSize().border(width * 1.7f, color.copy(alpha = env * 0.28f), shape),
        )
        androidx.compose.foundation.layout.Box(
            Modifier.fillMaxSize().padding(3.dp).border(width, color.copy(alpha = env * 0.92f), shape),
        )
    }
}
