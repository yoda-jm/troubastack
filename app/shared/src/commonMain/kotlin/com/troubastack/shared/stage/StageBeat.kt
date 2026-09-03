package com.troubastack.shared.stage

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
 * A35/T86 — the beat follows the song's METRE (see CountIn.kt's grid). Three tiers, each its own colour:
 * amber bar · aqua felt-pulse · grey free-subdivision. Amber-vs-aqua is warm/cool (survives red-green
 * deficiency); grey reads as texture, not "off". The base width is re-tuned LARGER than the studio's
 * 9 px desk value, for a dark stage at arm's length. Read-only (I12).
 */
private val TIER0 = Color(0xFFFFB02E) // amber — the bar (downbeat, unit 0)
private val TIER1 = Color(0xFF3EE0D4) // aqua — a felt pulse (group start)
private val TIER2 = Color(0xFF6B7A90) // grey — a free subdivision (muted, no glow)

/** The colour for a metric [tier] (0 bar · 1 felt pulse · 2 free subdivision). Package-visible so the
 *  metronome-icon tint (StageScreen) echoes the same three colours as the frame. */
internal fun tierColor(tier: Int): Color = when (tier) {
    0 -> TIER0
    1 -> TIER1
    else -> TIER2
}

/** Peak frame weight for the stage. Wider than the desk's 9 px — read from across a room, hands full. */
private val BEAT_BASE = 16.dp

/** The centre count for one unit: its [number] (1-based within the bar) and its metric [tier]. */
data class BeatMark(val number: Int, val tier: Int)

/**
 * A silent, read-only beat controller. [toggle] starts/stops the metronome; the [continuous] (∞)
 * flag chooses keep-running vs a two-bar metric count-in that self-stops ([countInUnits] — 8 units in
 * 4/4, 6 in 3/4, 12 in 6/8; NOT a fixed 8 beats). Idle when [beats] is 0. Each
 * start/stop bumps [runToken] so the driver effect (re)starts. The current visual is pushed out
 * through [frame]/[beatLabel] (snapshot-state the Stage observes), so the border + centre count
 * recompose but the controller stays render-agnostic.
 */
class StageBeat {
    var runToken by mutableStateOf(0)
        private set

    /** The current border pulse, observed by [StageBeatFrame]; null = dark (transient, ~200 ms). */
    var frame by mutableStateOf<BeatFrame?>(null)
        internal set

    /** The unit-in-bar mark to show, held for the WHOLE unit while running (null = idle): the number
     *  (1..unitsPerBar) so the player keeps their place, plus its [BeatMark.tier] so the centre count
     *  is tinted like the frame (amber 1 / aqua felt pulse / grey subdivision). Distinct from [frame]'s
     *  brief pulse. Cycles per bar (4/4: 1 2 3 4; 6/8: 1 2 3 4 5 6). */
    var beatLabel by mutableStateOf<BeatMark?>(null)
        internal set

    var tempo = 0
        private set

    /** The metre grid this run beats to (group lengths in units); 4/4 default. Set on [start]. */
    var groups by mutableStateOf(DEFAULT_GROUPS)
        private set

    /** ∞ mode (studio parity — the ∞ loop toggle in the editor). **Defaults OFF (A40):** tapping the
     *  metronome runs a **two-bar count-in** that self-stops; ∞ is the opt-in keep-running mode, turned
     *  on for the session from the ∞ chip beside the metronome. Not persisted — reopening the Stage
     *  starts from the count-in again (matches the studio's per-editor `useBeat`, and keeps I12). Affects
     *  the NEXT start; a run already going finishes as it started. */
    var continuous by mutableStateOf(false)

    /** Observable so the chrome-auto-hide (and anything else) can react to start/stop. */
    var beats by mutableStateOf(0)
        private set

    val running: Boolean get() = beats > 0

    /** Tap the metronome: stop if running, else start — forever ([continuous]) or a two-bar count-in
     *  under [meter]'s grid (4/4→8 units, 3/4→6, 6/8→12). A metronome you switch on and off. No-op for
     *  an out-of-range tempo. [meter] defaults to 4/4, so a call site without a metre beats as before. */
    fun toggle(tempoBpm: Int, meter: List<Int> = DEFAULT_GROUPS) {
        if (running) stop() else start(tempoBpm, meter, if (continuous) CONTINUOUS_BEATS else countInUnits(meter))
    }

    fun stop() {
        beats = 0
        runToken++
    }

    private fun start(tempoBpm: Int, meter: List<Int>, count: Int) {
        if (tempoIntervalMs(tempoBpm) == null) return
        tempo = tempoBpm
        groups = meter
        beats = count
        runToken++
    }
}

/**
 * A [StageBeat] plus its frame-clock driver, scoped to [resetKey] — the current SONG, so the beat
 * keeps ticking across page turns within a song and only a song change makes a fresh instance (its
 * tempo may differ). Render [StageBeatFrame] with it.
 */
@Composable
fun rememberStageBeat(resetKey: Any): StageBeat {
    val beat = remember(resetKey) { StageBeat() }
    // Key on the instance too, not just its token: a fresh (song-change) instance starts at token 0,
    // which must re-run this effect into the idle branch rather than reuse a running predecessor's.
    LaunchedEffect(beat, beat.runToken) {
        beat.frame = null
        beat.beatLabel = null
        if (!beat.running) return@LaunchedEffect
        val groups = beat.groups
        val perBar = unitsPerBar(groups)
        val interval = unitIntervalMs(beat.tempo, groups)   // schedule on the UNIT, not the pulse (A35)
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
                    beat.frame = beatFrame(elapsedMs, interval, beats, groups) // brief border pulse
                    val u = beatIndex % perBar
                    beat.beatLabel = BeatMark(u + 1, tierOf(u, groups))        // held the whole unit
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
 * The running beat: a pulsing frame on the page border PLUS a big, semi-transparent unit number
 * (1 2 3 … per bar) in the middle, both in the unit's TIER colour (amber bar / aqua felt pulse / grey
 * subdivision). The border pulse is a brief transient; the number is held for the whole unit so the
 * player keeps their place.
 * Purely visual — it holds NO pointer input, so a tap on the page still turns the page / toggles the
 * chrome (you stop the beat from its FAB, not by touching the score). Nothing renders when idle.
 */
@Composable
fun StageBeatFrame(beat: StageBeat, colorMode: StageColorMode = StageColorMode.NORMAL, modifier: Modifier = Modifier) {
    val frame = beat.frame
    val label = beat.beatLabel
    val shape = RoundedCornerShape(10.dp)
    Box(modifier.fillMaxSize()) {
        if (frame != null) {
            val env = frame.env.coerceIn(0f, 1f)
            val color = tierColor(frame.tier)
            val width = BEAT_BASE * (0.45f + 0.55f * env)
            if (frame.tier == 2) {
                // free subdivision: a single flat frame at ~45% opacity, NO halo/glow — texture, not a
                // pulse (spec A35 §4). Same width as the bar/pulse so the grid reads as one thing.
                Box(Modifier.fillMaxSize().padding(3.dp).border(width, color.copy(alpha = env * 0.45f), shape))
            } else {
                // bar + felt pulse: soft outer halo + crisp inner frame — they "breathe" rather than snap.
                Box(Modifier.fillMaxSize().border(width * 1.7f, color.copy(alpha = env * 0.28f), shape))
                Box(Modifier.fillMaxSize().padding(3.dp).border(width, color.copy(alpha = env * 0.92f), shape))
            }
        }
        if (label != null) {
            // A37 Ruling 2: on AMBER the page ground is amber ink — an amber tier-0/tier-2 numeral is
            // illegible, and A35's grey tier-2 vanishes (Interaction 1). Tint the centre count with the
            // aqua (felt-pulse) colour for EVERY tier there; cool-on-warm reads cleanly and the number
            // still keeps your place. The border PULSE and the shared amber/aqua contract are untouched.
            val countTier = if (colorMode == StageColorMode.AMBER) 1 else label.tier
            val tint = tierColor(countTier).copy(alpha = 0.34f)
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text("${label.number}", color = tint, fontSize = 168.sp, fontWeight = FontWeight.Bold)
            }
        }
    }
}
