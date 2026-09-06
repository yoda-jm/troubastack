package com.troubastack.shared.stage

/**
 * T147 — the rehearsal chronometer as a pure, suspend-safe state machine.
 *
 * It stores a start INSTANT and accumulated elapsed, **not a tick counter**, so the elapsed value is
 * DERIVED from the clock at read time. That is what keeps it correct across screen-off, process death and
 * a configuration change: the tablet sleeping for ten minutes cannot make the chrono lose ten minutes,
 * because nothing was counting — the answer is recomputed from `now`. A chrono that quietly loses time is
 * worse than none (T147), so this is the whole design.
 *
 * Time is passed IN (a monotonic `now`, e.g. `SystemClock.elapsedRealtime()` on Android — it advances
 * through deep sleep), never read here, so the machine is fully testable without ever sleeping.
 */
data class Chrono(
    /** Elapsed folded in from previous run segments, in ms. */
    val accumulatedMs: Long = 0L,
    /** The monotonic instant the current run segment started, or null when paused/stopped. */
    val runningSince: Long? = null,
) {
    val running: Boolean get() = runningSince != null

    /**
     * start / resume — begin (or continue) counting from [now], keeping whatever is already accumulated.
     * A second start while ALREADY running is a NO-OP: it must not restart the segment or drop elapsed.
     */
    fun started(now: Long): Chrono = if (running) this else copy(runningSince = now)

    /**
     * pause — stop counting, folding the live segment into [accumulatedMs]. A second pause while ALREADY
     * paused is a NO-OP: it must not double-count the already-folded segment.
     */
    fun paused(now: Long): Chrono =
        if (!running) this else Chrono(accumulatedMs = elapsedMs(now), runningSince = null)

    /** reset — back to 00:00, paused. */
    fun reset(): Chrono = Chrono()

    /**
     * Total elapsed at [now]: accumulated plus the live segment when running. Clamped at 0 so a backward
     * `now` (a persisted instant carried across a reboot, where a monotonic clock resets) can never render
     * a negative time — it degrades to the accumulated total, never garbage.
     */
    fun elapsedMs(now: Long): Long {
        val live = runningSince?.let { (now - it).coerceAtLeast(0L) } ?: 0L
        return accumulatedMs + live
    }
}

/**
 * T147 — elapsed ms → "M:SS", or "H:MM:SS" once past an hour. Minutes are not zero-padded under an hour
 * (a stopwatch reads "7:04", not "07:04"); seconds and the sub-hour minutes always are. Negative input is
 * clamped to zero.
 */
fun formatChrono(ms: Long): String {
    val totalSec = ms.coerceAtLeast(0L) / 1000L
    val h = totalSec / 3600L
    val m = (totalSec % 3600L) / 60L
    val s = totalSec % 60L
    fun p2(n: Long): String = if (n < 10) "0$n" else "$n"
    return if (h > 0) "$h:${p2(m)}:${p2(s)}" else "$m:${p2(s)}"
}

/**
 * T147 — persist a chrono across process death AND reboot. A running segment is stored by its start in
 * WALL-CLOCK time ("accumulated:R:startWallMillis"); a paused chrono as "accumulated:" (no live segment).
 *
 * Why wall, not monotonic: the live [Chrono.runningSince] is a MONOTONIC instant (elapsedRealtime — right
 * for LIVE counting: immune to clock changes, advances through sleep), but that clock RESETS on reboot, so
 * a monotonic instant is meaningless once carried across one. The earlier format stored the raw monotonic
 * instant and leaned on [Chrono.elapsedMs]'s clamp — but the clamp only catches `now < since` (a reboot
 * that left `now` SMALLER); a reboot that left `now` LARGER than the stale instant produced a garbage
 * elapsed (the T147 restore bug — a chrono reading ~16h). So at the persistence boundary we convert to
 * wall: startWall = nowWall − liveMs. Real elapsed on restore is nowWall − startWall regardless of reboots
 * (VLL 2026-09-06: the chrono counts real elapsed since Start).
 *
 * [nowMono]/[nowWall] are the two clocks read together at persist/restore (Android: SystemClock
 * .elapsedRealtime() and System.currentTimeMillis()); passed in so this stays pure and testable.
 */
fun encodeChrono(c: Chrono, nowMono: Long, nowWall: Long): String {
    val since = c.runningSince ?: return "${c.accumulatedMs}:"
    val liveMs = (nowMono - since).coerceAtLeast(0L)
    return "${c.accumulatedMs}:R:${nowWall - liveMs}"
}

/**
 * Inverse of [encodeChrono]. A running value ("acc:R:startWall") is re-anchored into THIS process's
 * monotonic timebase — runningSince = nowMono − (nowWall − startWall) — so [Chrono.elapsedMs] keeps
 * counting live from the correct total (accumulated + the real wall gap since Start). A null/blank/malformed
 * value → a fresh (00:00, paused) chrono, so a corrupt store can never crash Stage. A value in the pre-fix
 * "acc:mono" shape (a bare number after the colon) degrades to PAUSED at its accumulated: its monotonic
 * instant can't be trusted across the upgrade, and paused-at-accumulated is safe — never garbage.
 */
fun decodeChrono(s: String?, nowMono: Long, nowWall: Long): Chrono {
    if (s.isNullOrBlank()) return Chrono()
    val i = s.indexOf(':')
    if (i < 0) return Chrono()
    val acc = (s.substring(0, i).toLongOrNull() ?: return Chrono()).coerceAtLeast(0L)
    val rest = s.substring(i + 1)
    return when {
        rest.isEmpty() -> Chrono(accumulatedMs = acc, runningSince = null) // paused
        rest.startsWith("R:") -> {
            val startWall = rest.substring(2).toLongOrNull()
                ?: return Chrono(accumulatedMs = acc, runningSince = null)
            val liveMs = (nowWall - startWall).coerceAtLeast(0L)
            Chrono(accumulatedMs = acc, runningSince = nowMono - liveMs) // re-anchored to this boot's clock
        }
        else -> Chrono(accumulatedMs = acc, runningSince = null) // legacy "acc:mono" → paused (safe, never garbage)
    }
}
