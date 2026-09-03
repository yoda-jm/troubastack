package com.troubastack.shared.stage

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

/** Continuous ("keep running") mode: an effectively unbounded beat count. The count-in length is
 *  metre-aware — [countInUnits] (two bars in metric units), not a fixed 8 (A67 retired the 4/4 constants). */
const val CONTINUOUS_BEATS = Int.MAX_VALUE

// ---------------------------------------------------------------------------------------------------
// A35/T86 — the METRIC GRID. A34 assumed 4/4; T86 makes the metre a property of the
// song and replaces "pulses per bar" with GROUP LENGTHS in metric units (4/4→[1,1,1,1], 6/8→[3,3],
// additive 3+4/8→[3,4]). Every unit then gets a TIER: 0 bar (unit 0) · 1 felt pulse (a group start) ·
// 2 free subdivision (everything else). This is the Kotlin mirror of web/studio/src/beatPhase.ts; both
// runtimes run the same vectors, so they cannot disagree about when a beat is or what tier it is.
// ---------------------------------------------------------------------------------------------------

/** 4/4 — the grid an unset metre uses, so pre-T86 songs (every bundle without `meter`) beat as before. */
val DEFAULT_GROUPS: List<Int> = listOf(1, 1, 1, 1)

/** Below this unit interval, tier-2 (free subdivision) units are muted — they read as a strobe rather
 *  than texture (the same complaint as T85's "pulsating too quickly", one metric level down). */
const val TIER2_MUTE_MS = 130.0

/** Total metric units in a bar. */
fun unitsPerBar(groups: List<Int>): Int = groups.sum()

/** A count-in is two bars, measured in UNITS (4/4→8, 3/4→6, 6/8→12 units = 4 felt pulses). */
fun countInUnits(groups: List<Int>): Int = 2 * unitsPerBar(groups)

/** The tier of unit [u] within a bar: 0 bar (unit 0) · 1 felt pulse (a group start ≠ 0) · 2 else. */
fun tierOf(u: Int, groups: List<Int>): Int {
    if (u == 0) return 0
    var start = 0
    for (g in groups) {
        if (u == start) return 1 // a group start that isn't unit 0 → a felt pulse
        start += g
    }
    return 2 // between group starts → a free subdivision
}

/** ms per UNIT for [bpm] under [groups]. Uniform groups: the pulse IS the group, so a unit is
 *  `pulse / groups[0]` (4/4→quarter, 6/8→eighth). Irregular groups (3+4) have no single pulse length,
 *  so tempo counts UNITS: `60000 / bpm`. See T86 §4 — this is mandatory, not stylistic. */
fun unitIntervalMs(bpm: Int, groups: List<Int>): Double {
    val perPulse = 60_000.0 / bpm
    val uniform = groups.all { it == groups[0] }
    return if (uniform) perPulse / groups[0] else perPulse
}

/** The note the tempo names: [QUARTER] simple · [DOTTED_QUARTER] compound (groups of 3) · [EIGHTH]
 *  irregular-additive — drives the tempo chip glyph (♩ / ♩. / ♪). */
enum class TempoUnit { QUARTER, DOTTED_QUARTER, EIGHTH }

fun tempoUnit(groups: List<Int>): TempoUnit {
    val uniform = groups.all { it == groups[0] }
    if (!uniform) return TempoUnit.EIGHTH // irregular additive → no single pulse length
    return if (groups[0] == 3) TempoUnit.DOTTED_QUARTER else TempoUnit.QUARTER
}

private val METER_DENOMS = setOf(1, 2, 4, 8, 16)

/** A strict ASCII non-negative integer — matches Go `strconv.Atoi` / TS `Number(...)`. Kotlin's own
 *  [toIntOrNull] on the JVM would accept Unicode decimal digits (Character.digit: `٤` → 4) and so
 *  DISAGREE with the other two runtimes on the `٤/٨` vector; requiring `'0'..'9'` keeps all three in
 *  step. A leading `-`, a `.`, or embedded whitespace all fail here (⇒ unset ⇒ 4/4). */
private fun asciiUnit(s: String): Int? =
    if (s.isNotEmpty() && s.all { it in '0'..'9' }) s.toIntOrNull() else null

/**
 * Resolve a metre string to its group lengths — the Kotlin mirror of the TS `meterGroups` and core
 * `app.ParseMeter` (all three pinned to `docs/contracts/meter-groups.vectors.json`; the app, studio
 * and server must never silently disagree). LENIENT: anything malformed returns the 4/4 default rather
 * than failing, so a typo never breaks the beat. Additive literal (`3+4`) · compound
 * (`n % 3 == 0 && n > 3`) → `n/3` threes · simple → `n` ones.
 */
fun meterGroups(meter: String?): List<Int> {
    val s = (meter ?: "").trim()
    if (s.isEmpty()) return DEFAULT_GROUPS
    val parts = s.split("/")
    if (parts.size != 2) return DEFAULT_GROUPS
    val d = asciiUnit(parts[1].trim()) ?: return DEFAULT_GROUPS
    if (d !in METER_DENOMS) return DEFAULT_GROUPS
    val num = parts[0].trim()
    val groups: List<Int> = if (num.contains("+")) {
        num.split("+").map { asciiUnit(it.trim()) ?: return DEFAULT_GROUPS }
    } else {
        val n = asciiUnit(num) ?: return DEFAULT_GROUPS
        if (n < 1 || n > 32) return DEFAULT_GROUPS
        if (n % 3 == 0 && n > 3) List(n / 3) { 3 } else List(n) { 1 }
    }
    if (groups.isEmpty() || groups.size > 16) return DEFAULT_GROUPS
    var sum = 0
    for (g in groups) {
        if (g < 1 || g > 32) return DEFAULT_GROUPS
        sum += g
    }
    if (sum > 64) return DEFAULT_GROUPS
    return groups
}

/** The phase of one unit: which unit, whether its discrete on-window is lit, its [tier] (0 bar / 1
 *  felt pulse / 2 free subdivision), and [emphasis] (`tier == 0`, held across the unit's fade so a
 *  fading frame keeps the downbeat colour). */
data class BeatPhase(val beatIndex: Int, val lit: Boolean, val tier: Int, val emphasis: Boolean)

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
 * The phase at [elapsedMs] for [beats] UNITS at [intervalMs] per unit, under the grid [groups]. Pure;
 * reads the clock only through its argument, so the caller (a withFrameNanos loop, or a stubbed clock
 * in a test) owns time. [beats] = [CONTINUOUS_BEATS] is continuous mode; [groups] defaults to 4/4 so
 * pre-T86 callers and vectors are unchanged. Tier-2 units go dark below [TIER2_MUTE_MS]; the bar and
 * felt pulses always light.
 */
fun beatPhase(elapsedMs: Double, intervalMs: Double, beats: Int, groups: List<Int> = DEFAULT_GROUPS): BeatPhase {
    val beatIndex = floor(elapsedMs / intervalMs).toInt()
    val active = elapsedMs >= 0 && beatIndex < beats
    val perBar = unitsPerBar(groups)
    val u = ((beatIndex % perBar) + perBar) % perBar
    val tier = tierOf(u, groups)
    val msSinceBeat = elapsedMs - beatIndex * intervalMs
    val baseLit = active && msSinceBeat < litWindowMs(intervalMs)
    val lit = baseLit && !(tier == 2 && intervalMs < TIER2_MUTE_MS)
    val emphasis = active && tier == 0
    return BeatPhase(beatIndex, lit, tier, emphasis)
}

/**
 * The visual envelope for the edge frame at [elapsedMs], or null when the frame is dark (before start,
 * between pulses, or after the count finishes). [env] is `(1 - t)²` over a decay clamped to clear
 * before the next beat — a square on/off reads as a strobe, so the pulse is full at the beat and eased
 * away. The RENDERER applies width/colour/glow (see StageScreen) so those stay re-tunable for the
 * stage; this keeps only the pure timing so it's unit-testable. Port of T85's `beatFrameStyle`.
 */
data class BeatFrame(val env: Float, val tier: Int)

fun beatFrame(elapsedMs: Double, intervalMs: Double, beats: Int, groups: List<Int> = DEFAULT_GROUPS): BeatFrame? {
    val beatIndex = floor(elapsedMs / intervalMs).toInt()
    if (!(elapsedMs >= 0 && beatIndex < beats)) return null
    val perBar = unitsPerBar(groups)
    val u = ((beatIndex % perBar) + perBar) % perBar
    val tier = tierOf(u, groups)
    if (tier == 2 && intervalMs < TIER2_MUTE_MS) return null // free subdivision below the strobe floor
    val decay = decayMs(intervalMs)
    val msSinceBeat = elapsedMs - beatIndex * intervalMs
    if (msSinceBeat >= decay) return null // between pulses — dark
    val env = (1.0 - msSinceBeat / decay).pow(2).toFloat()
    return BeatFrame(env, tier)
}
