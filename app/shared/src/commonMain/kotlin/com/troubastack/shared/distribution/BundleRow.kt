package com.troubastack.shared.distribution

/**
 * T143 — a concert row must distinguish two bakes with the SAME name (VLL's rehearsal: "2 bakes with the
 * same name, I don't know which version"). The bundle already carries `concertRev` (incrementing) and a
 * distinct `bakedAt` per bake; the row just wasn't showing them. This is the pure, testable subtitle:
 * "rev N · YYYY-MM-DD HH:MM" (UTC), the time omitted when absent (rev 0 / pre-timestamp bundles).
 */
fun concertRowSubtitle(rev: ULong, bakedAtEpochSec: Long): String =
    if (bakedAtEpochSec > 0L) "rev $rev · ${formatUtcMinute(bakedAtEpochSec)}" else "rev $rev"

/** epoch seconds → "YYYY-MM-DD HH:MM" in UTC. Pure (no kotlinx-datetime dep): the civil-from-days
 *  algorithm; inputs are positive bake timestamps, so plain division suffices. */
internal fun formatUtcMinute(epochSec: Long): String {
    val days = epochSec / 86400L
    val secOfDay = epochSec % 86400L
    val hh = (secOfDay / 3600L).toInt()
    val mm = ((secOfDay % 3600L) / 60L).toInt()
    // Howard Hinnant's days-from-civil, inverted.
    val z = days + 719468L
    val era = (if (z >= 0) z else z - 146096L) / 146097L
    val doe = z - era * 146097L
    val yoe = (doe - doe / 1460L + doe / 36524L - doe / 146096L) / 365L
    val doy = doe - (365L * yoe + yoe / 4L - yoe / 100L)
    val mp = (5L * doy + 2L) / 153L
    val d = (doy - (153L * mp + 2L) / 5L + 1L).toInt()
    val m = (if (mp < 10L) mp + 3L else mp - 9L).toInt()
    val y = (yoe + era * 400L + if (m <= 2) 1L else 0L)
    fun p2(n: Int) = if (n < 10) "0$n" else "$n"
    return "$y-${p2(m)}-${p2(d)} ${p2(hh)}:${p2(mm)}"
}

/** T143 — the actions a concert row's ⋮ offers, by intent. */
enum class BundleAction { Freeze, Unfreeze, Pin, Unpin, Delete }

/**
 * T143 — which ⋮ actions a bundle row offers. **Perform (lean) offers NONE** — managing bundles is not a
 * performance affordance (VLL: keep the performing surface lean). **Manage** adds Delete: a HEALTHY
 * bundle gets the freeze/pin controls PLUS Delete (VLL couldn't remove a healthy duplicate — Delete was
 * only reachable when damaged); a damaged one offers only Delete.
 */
fun bundleMenuActions(lean: Boolean, damaged: Boolean): List<BundleAction> = when {
    lean -> emptyList()
    damaged -> listOf(BundleAction.Delete)
    else -> listOf(
        BundleAction.Freeze,
        BundleAction.Unfreeze,
        BundleAction.Pin,
        BundleAction.Unpin,
        BundleAction.Delete,
    )
}
