// A42 ② — one-tap re-bake from Home, on T103's kick-and-poll contract. The app mints a bake id, POSTs
// …/setlists/{id}/bake (which now returns 202 promptly — the bake runs on the SERVER's context, so a
// dropped client can't cancel it), and polls …/bakes/{id}/progress to a terminal state. The POLL is the
// single source of truth for the outcome (T103). This file holds the SHARED, testable pieces: the
// progress snapshot type, the pure Home line, and the pure per-poll decision.
package com.troubastack.shared.distribution

import kotlinx.serialization.Serializable

/**
 * A live bake's progress snapshot from `GET …/setlists/{id}/bakes/{bakeId}/progress` (T96/T99/T103).
 * `done` advances 1..`total` as each song is baked; `song` names the current (or last) song; `error` is
 * set only on a failed bake (user-safe since T102 — never a raw stack trace). Mirrors core
 * `bake.BakeProgress`; unknown fields (e.g. terminal `warnings`) are ignored by the transport's lenient
 * Json.
 */
@Serializable
data class BakeProgress(
    val state: String = "running", // "running" | "succeeded" | "failed"
    val done: Int = 0,
    val total: Int = 0,
    val song: String = "",
    val error: String = "",
)

/**
 * A42 ② — the PURE Home line for a bake in flight, from the latest [p] (null = the poll hasn't answered,
 * a 404, or an old server). Honours T99's rule that made the feature worth building:
 *  - `done == total` with NO song is the flatten/zip tail → **"Finishing…"**, never a frozen "N of N"
 *    (the exact thing T99 exists to avoid),
 *  - no snapshot yet ⇒ a plain **"Baking…"** (degrade, never block; the bake still completes),
 *  - otherwise **"Baking <song> — <done> of <total>"**.
 */
fun bakeLabel(p: BakeProgress?): String = when {
    p == null -> "Baking…"
    p.total > 0 && p.done >= p.total && p.song.isEmpty() -> "Finishing…"
    p.song.isNotEmpty() && p.total > 0 -> "Baking ${p.song} — ${p.done} of ${p.total}"
    p.total > 0 -> "Baking ${p.done} of ${p.total}"
    else -> "Baking…"
}

/** The Home re-bake row (A42 ②). [Hidden] when idle / done; [Baking] carries the live line; [Failed]
 *  carries the server's user-safe error (T102) with a retry. Pure — the host renders it, decides nothing. */
sealed interface BakeStatus {
    data object Hidden : BakeStatus
    data class Baking(val label: String) : BakeStatus
    data class Failed(val message: String) : BakeStatus
}

/** One step of the poll loop: the [status] to show now, and whether the bake has reached a terminal
 *  state so the loop can stop. */
data class BakeStep(val status: BakeStatus, val done: Boolean)

/**
 * A42 ② — the PURE per-poll decision, driven by [BakeProgress.state] ALONE (T103: the poll is the single
 * source of truth for the outcome). This is the load-bearing choice:
 *  - `state == "succeeded"` ⇒ terminal, row clears ([BakeStatus.Hidden]);
 *  - `state == "failed"` ⇒ terminal, show the server's user-safe [error] (T102), NEVER a transport guess
 *    like "couldn't reach the server";
 *  - anything else (running / a null poll / an unknown state) ⇒ NOT terminal, keep the [BakeStatus.Baking]
 *    line and keep polling.
 *
 * Terminal is decided by `state`, never by `done == total`: a `done == total` snapshot whose state is
 * still "running" is T99's "Finishing…" tail — treating it as done would stop the poll one flatten/zip
 * step early and could miss a late failure.
 */
fun bakePollStep(p: BakeProgress?): BakeStep = when (p?.state) {
    "succeeded" -> BakeStep(BakeStatus.Hidden, done = true)
    "failed" -> BakeStep(BakeStatus.Failed(p.error.ifBlank { "The bake failed — try again" }), done = true)
    else -> BakeStep(BakeStatus.Baking(bakeLabel(p)), done = false)
}
