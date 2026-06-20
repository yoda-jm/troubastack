// Generated proto types (AvailableConcerts, AvailableConcert, ConcertBundle, …) come from
// gen/ — single source of truth is proto/ (I1). Referenced here, never redefined.
package com.troubashare.shared.distribution

/**
 * Distribution — SHARED Kotlin (commonMain). The downloader + revision/availability logic.
 * NOT a native seam (I15): the only platform touch is via Storage (seam 3) for bytes on disk.
 *
 * Responsibilities (docs/design/05-distribution.md, I13):
 *  - track per-song DOWNLOADED revisions for each concert,
 *  - run a cheap metadata-only manifest diff → the "what's available to me" set,
 *  - apply updates EXPLICITLY (user action) — never automatically, never mid-performance,
 *  - ATOMIC SWAP on re-download: fetch to temp, verify, then replace; keep the old revision
 *    until the new one is verified (a half-download must never corrupt a working concert),
 *  - honor freeze/lock at the device tier (local pin + admin band-wide lock).
 */

/** Per-concert update policy (I13). Default is Prompt. */
enum class UpdatePolicy { PROMPT, FROZEN, AUTO }

/** Two kinds of freeze (I13). */
sealed interface Freeze {
    /** Performer freezes their own copy; no chips; explicit unfreeze. */
    data class LocalPin(val atRev: ULong) : Freeze
    /** Leader marks a revision `final` band-wide; no new baked revisions emitted. */
    data class AdminLock(val atRev: ULong) : Freeze
}

/** Result of diffing local downloaded revisions against the server manifest. */
sealed interface Availability {
    /** Downloaded and server rev > local → "New version of Concert A available". */
    data class UpdateOffered(val concertId: String, val localRev: ULong, val serverRev: ULong) : Availability
    /** Per-song content change → "Song A changed — apply?" (granular re-bake + swap). */
    data class SongChanged(val concertId: String, val songId: String, val serverRev: ULong) : Availability
    /** Not downloaded but shared to the band → "Concert B is available for your band". */
    data class NewlyAvailable(val concertId: String) : Availability
}

/** The downloader / update manager. All shared; persistence via Storage (seam 3). */
interface Updates {

    /** Cheap metadata-only manifest call (a few KB); maps to proto `AvailableConcerts` (I1). */
    suspend fun fetchManifest(): /* AvailableConcerts */ Any = TODO("scaffold: wire gen proto + transport")

    /** Diff the manifest vs per-song downloaded revisions → chips to surface (I13). */
    fun diff(manifest: /* AvailableConcerts */ Any): List<Availability> = TODO("scaffold")

    /**
     * Apply an offered update — EXPLICIT only, refused while the concert is in performance (I13).
     * Downloads to temp, verifies, then atomically swaps; old revision retained until verified.
     * Applying songs individually yields a mixed-revision bundle.
     */
    suspend fun apply(offer: Availability) { TODO("scaffold: fetch→verify→atomic swap") }

    /** Set per-concert policy / freeze; FROZEN and AdminLock suppress chips (I13). */
    fun setPolicy(concertId: String, policy: UpdatePolicy) { TODO("scaffold") }
    fun setFreeze(concertId: String, freeze: Freeze?) { TODO("scaffold") }
}
