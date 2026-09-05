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

/** T143 — the setlist id behind a concert id, for the support detail shown in the ⋮ (VLL: "peut etre
 *  dans les ... l'id de la playlist"). A band-wide bake's concert id IS the setlist id; a legacy
 *  per-member variant is "<setlistId>~<userId>" (B07, ParseConcertID), so strip any '~' suffix. */
fun setlistIdOf(concertId: String): String = concertId.substringBefore('~')

/** T143 — one collapsible band section in the on-device library. `bandId` is the stable grouping key;
 *  `bandName` is its display label (already resolved to "Unknown band" for identity-less bundles). */
data class BundleGroup<T>(val bandId: String, val bandName: String, val items: List<T>)

/** The label an identity-less (pre-T143 / old) bundle groups under. */
const val UNKNOWN_BAND_LABEL: String = "Unknown band"

/**
 * T143 — group the on-device library by band, so a performer can answer "je ne sais pas … quel band".
 * Groups are ordered alphabetically by band name (case-insensitive), the identity-less "Unknown band"
 * group always LAST so a real band never hides behind it. Items keep their incoming (already-sorted)
 * order within a band. Bundles with the same `bandId` merge even if an older bake stored a blank name;
 * the group takes the first non-blank name it sees. Pure + deterministic — no bundle is ever dropped.
 */
fun <T> groupByBand(items: List<T>, bandId: (T) -> String, bandName: (T) -> String): List<BundleGroup<T>> {
    val order = ArrayList<String>()           // group keys in first-seen order (stable pre-sort)
    val byKey = LinkedHashMap<String, MutableList<T>>()
    val labels = HashMap<String, String>()
    for (it in items) {
        val key = bandId(it)
        if (key !in byKey) { byKey[key] = ArrayList(); order.add(key) }
        byKey.getValue(key).add(it)
        val nm = bandName(it)
        if (nm.isNotBlank() && labels[key].isNullOrBlank()) labels[key] = nm
    }
    val groups = order.map { key ->
        val label = if (key.isBlank()) UNKNOWN_BAND_LABEL else labels[key]?.takeIf { it.isNotBlank() } ?: UNKNOWN_BAND_LABEL
        BundleGroup(key, label, byKey.getValue(key).toList())
    }
    return groups.sortedWith(
        // Unknown ("" band id) always last; otherwise by display label (case-insensitive), ties by id.
        compareBy<BundleGroup<T>>({ it.bandId.isBlank() }, { it.bandName.lowercase() }, { it.bandId }),
    )
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
