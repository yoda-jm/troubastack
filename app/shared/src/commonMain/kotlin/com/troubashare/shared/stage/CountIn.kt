// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubashare.shared.stage

/**
 * Visual count-in (A11) — pure timing. Silent, always (it's a stage; no audio). The performer taps
 * the tempo chip and gets a few beats to feel the tempo, then it self-stops. Read-only (I12): nothing
 * persisted, no writes.
 */

/** Fixed length: two bars of 4/4. We don't know the real meter, and 8 beats counts in ~anything. */
const val COUNT_IN_BEATS = 8

/**
 * Milliseconds per beat for [tempo] BPM, or null if the tempo is out of a sane range (20..300) — in
 * which case the tap is ignored rather than producing an absurd flash rate.
 */
fun countInIntervalMs(tempo: Int): Long? = if (tempo in 20..300) 60_000L / tempo else null

/** Beat 1 of each 4/4 bar (emphasized in the pulse). Beats are 0-indexed. */
fun isDownbeat(beatIndex: Int): Boolean = beatIndex % 4 == 0
