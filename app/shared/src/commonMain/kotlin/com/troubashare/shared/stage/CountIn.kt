package com.troubashare.shared.stage

import kotlin.math.floor
import kotlin.math.pow

/**
 * A34 — the visual-beat CONTRACT in Kotlin, the port of T85's `web/studio/src/beatPhase.ts`.
 *
 * [beatPhase] answers one question — *when is a beat, and what kind* — as a pure function of elapsed
 * time, with zero rendering. The Stage (here) and the Studio (T85, TS) each draw the beat their own
 * way but must agree on the timeline, so BOTH runtimes execute the SAME shared table of vectors
 * (`docs/contracts/beat-phase.vectors.json`, mirrored into commonTest resources with a CI drift
 * guard — the glyphs.json / view-resolution pattern). If the two ever drift, a vector fails.
 *
 * This replaces A11's count-in maths, whose bug started all this: `60_000L / tempo` TRUNCATED
 * (90 bpm → 666 ms, not 666.67), drifting a bar every ~40 s. Interval maths is [Double] now.
 *
 * Read-only (I12): silent, nothing persisted, no writes. The visual envelope ([beatFrame]) is tuning,
 * kept next to the phase but re-tuned for the stage (a dark room at arm's length, not a lit desk).
 */

/** Beats per bar — downbeats fall on beat 0, 4, 8, … */
const val BEATS_PER_BAR = 4

/** A count-in is two bars of 4/4. Tap = count-in; long-press = continuous (passes a huge count). */
const val COUNT_IN_BEATS = BEATS_PER_BAR * 2

/** Continuous ("keep running") mode: an effectively unbounded beat count. */
const val CONTINUOUS_BEATS = Int.MAX_VALUE

/** The phase of one beat: which beat, whether its discrete on-window is lit, and whether it's a
 *  downbeat (the emphasis channel, held across the whole downbeat so a fade keeps its colour). */
data class BeatPhase(val beatIndex: Int, val lit: Boolean, val emphasis: Boolean)

/** ms between beats for [bpm]. Kept a Double — truncating this was the original bug. */
fun intervalMs(bpm: Int): Double = 60_000.0 / bpm

/** Tempo range guard: [tempo] outside 20..300 → null (the tap is a no-op, no absurd flash rate). */
fun tempoIntervalMs(tempo: Int): Double? = if (tempo in 20..300) intervalMs(tempo) else null

/** How long the frame takes to fade after a beat; clamped so it always clears before the next one. */
fun decayMs(interval: Double): Double = minOf(220.0, interval * 0.75)

/** The `lit` window — the on-portion of a beat, capped at 30% of the interval so the discrete signal
 *  is a transient at every tempo (the sequence test asserts ≤ 35%). */
fun litWindowMs(interval: Double): Double = minOf(decayMs(interval), interval * 0.3)

/**
 * The phase at [elapsedMs] for [beats] beats at [intervalMs] spacing. Pure; reads the clock only
 * through its argument, so the caller (a withFrameNanos loop, or a stubbed clock in a test) owns time.
 * [beats] = [CONTINUOUS_BEATS] is continuous mode.
 */
fun beatPhase(elapsedMs: Double, intervalMs: Double, beats: Int): BeatPhase {
    val beatIndex = floor(elapsedMs / intervalMs).toInt()
    val active = elapsedMs >= 0 && beatIndex < beats
    val msSinceBeat = elapsedMs - beatIndex * intervalMs
    val lit = active && msSinceBeat < litWindowMs(intervalMs)
    val emphasis = active && beatIndex % BEATS_PER_BAR == 0
    return BeatPhase(beatIndex, lit, emphasis)
}

/** Beat 1 of each 4/4 bar. */
fun isDownbeat(beatIndex: Int): Boolean = beatIndex % BEATS_PER_BAR == 0

/**
 * The visual envelope for the edge frame at [elapsedMs], or null when the frame is dark (before start,
 * between pulses, or after the count finishes). [env] is `(1 - t)²` over a decay clamped to clear
 * before the next beat — a square on/off reads as a strobe, so the pulse is full at the beat and eased
 * away. The RENDERER applies width/colour/glow (see StageScreen) so those stay re-tunable for the
 * stage; this keeps only the pure timing so it's unit-testable. Port of T85's `beatFrameStyle`.
 */
data class BeatFrame(val env: Float, val downbeat: Boolean)

fun beatFrame(elapsedMs: Double, intervalMs: Double, beats: Int): BeatFrame? {
    val beatIndex = floor(elapsedMs / intervalMs).toInt()
    if (!(elapsedMs >= 0 && beatIndex < beats)) return null
    val decay = decayMs(intervalMs)
    val msSinceBeat = elapsedMs - beatIndex * intervalMs
    if (msSinceBeat >= decay) return null // between pulses — dark
    val env = (1.0 - msSinceBeat / decay).pow(2).toFloat()
    return BeatFrame(env, beatIndex % BEATS_PER_BAR == 0)
}
