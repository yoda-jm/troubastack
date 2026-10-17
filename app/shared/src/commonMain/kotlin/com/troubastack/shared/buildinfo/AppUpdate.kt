// A88 — "is there a newer TroubaStage?", answered from the public GitHub `latest` release against this build's
// identity (A84's BuildInfo). PURE: the host fetches the release and persists the two remembered values; these
// functions decide. Not on the Play Store, so this is how a tablet hears about a new build — and a development
// build never asks on its own (VLL: "so I am not bothered with it while developing").
package com.troubastack.shared.buildinfo

/** The repository whose `latest` release carries the APK — the ONE place it is named (A88 §1). */
const val UPDATE_REPOSITORY = "yoda-jm/troubastack"

/** The API endpoint for that release (public, unauthenticated, 60 requests/hour). */
const val UPDATE_RELEASE_URL = "https://api.github.com/repos/$UPDATE_REPOSITORY/releases/tags/latest"

/** The release asset that is the app. */
const val UPDATE_ASSET_NAME = "troubastage-debug.apk"

/** What the `latest` release offers: its commit, when it was published (ISO-8601 UTC), and the APK asset. */
data class LatestRelease(val commit: String, val publishedAt: String, val apkUrl: String, val apkSize: Long)

/** What the app should say about [LatestRelease] against the installed build. */
sealed interface UpdateVerdict {
    /** The installed build IS the release. */
    data object UpToDate : UpdateVerdict

    /** A newer build is published — offer it. */
    data class Available(val release: LatestRelease) : UpdateVerdict

    /** Newer, but "Not now" was said to exactly this release: stay quiet until a newer one appears. */
    data class Dismissed(val release: LatestRelease) : UpdateVerdict

    /** The installed build is newer than the release (a development build ahead of main): nothing to offer. */
    data object InstalledNewer : UpdateVerdict
}

/**
 * The verdict for [release] against the installed build ([installedGit] = `git describe --always --dirty`, e.g.
 * `cad7478` or `558d3fbe-dirty`, or `unknown`; [installedBuiltAt] = its ISO build time). [dismissedCommit] is the
 * release last answered "Not now"; a [manual] check ("Check for updates") ignores it.
 *
 * Same commit → up to date. A different commit published AFTER the installed build → available. Otherwise the
 * installed build is the newer one → nothing. An installed build that cannot say what it is offers the release.
 */
fun updateVerdict(installedGit: String, installedBuiltAt: String?, release: LatestRelease, dismissedCommit: String?, manual: Boolean): UpdateVerdict {
    val hash = installedGit.removeSuffix("-dirty").lowercase()
    val known = hash != "unknown" && hash.length >= 7 && hash.all { it in '0'..'9' || it in 'a'..'f' }
    if (known && release.commit.lowercase().startsWith(hash)) return UpdateVerdict.UpToDate
    val built = installedBuiltAt?.let(::isoEpochSeconds)
    val published = isoEpochSeconds(release.publishedAt)
    val newer = !known || built == null || published == null || published > built
    if (!newer) return UpdateVerdict.InstalledNewer
    return if (!manual && dismissedCommit != null && dismissedCommit.equals(release.commit, ignoreCase = true)) UpdateVerdict.Dismissed(release)
    else UpdateVerdict.Available(release)
}

/** At most one automatic check a day. */
const val AUTO_CHECK_INTERVAL_MS: Long = 24L * 60 * 60 * 1000

/**
 * Whether to check automatically now. ⟨D2⟩ a `"dev"` build never does unless the person switched automatic checks on
 * ([autoChecks] null = the channel's default: on for `"ci"`, off for `"dev"`). Never during a concert or while a note
 * is being drawn or sent (§1). At most once per [AUTO_CHECK_INTERVAL_MS] — a failed check counts too, so an offline
 * tablet or a rate limit just tries again the next day.
 */
fun shouldAutoCheck(nowMs: Long, lastCheckMs: Long?, channel: String, autoChecks: Boolean?, inConcert: Boolean, noteBusy: Boolean): Boolean {
    val enabled = autoChecks ?: (channel == "ci")
    if (!enabled || inConcert || noteBusy) return false
    return lastCheckMs == null || nowMs - lastCheckMs >= AUTO_CHECK_INTERVAL_MS || nowMs < lastCheckMs
}

/** The About line for the latest release: `Latest: 195ebe9a · built 2026-10-17 16:52 UTC`, `Up to date`, `Couldn't check`. */
fun latestLine(verdict: UpdateVerdict?, failed: Boolean): String = when {
    failed -> "Couldn't check"
    verdict == null -> ""
    verdict is UpdateVerdict.UpToDate -> "Up to date"
    verdict is UpdateVerdict.InstalledNewer -> "Up to date (this build is newer than the latest release)"
    verdict is UpdateVerdict.Available -> "Latest: ${verdict.release.commit.take(8)} · built ${utcMinute(verdict.release.publishedAt) ?: verdict.release.publishedAt}"
    verdict is UpdateVerdict.Dismissed -> "Latest: ${verdict.release.commit.take(8)} · built ${utcMinute(verdict.release.publishedAt) ?: verdict.release.publishedAt}"
    else -> ""
}

/** `2026-10-17T16:52:33Z` (fraction optional) → seconds since the epoch, UTC; anything else → null. */
internal fun isoEpochSeconds(iso: String): Long? {
    val m = Regex("""^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2})(?:\.\d+)?)?Z$""").find(iso.trim()) ?: return null
    val (y, mo, d, h, mi) = m.destructured
    val s = m.groupValues[6].ifEmpty { "0" }.toLong()
    return daysFromCivil(y.toLong(), mo.toLong(), d.toLong()) * 86_400 + h.toLong() * 3600 + mi.toLong() * 60 + s
}

/** Days since 1970-01-01 for a proleptic Gregorian date (Howard Hinnant's days_from_civil). */
private fun daysFromCivil(y0: Long, m: Long, d: Long): Long {
    val y = if (m <= 2) y0 - 1 else y0
    val era = (if (y >= 0) y else y - 399) / 400
    val yoe = y - era * 400
    val doy = (153 * (if (m > 2) m - 3 else m + 9) + 2) / 5 + d - 1
    val doe = yoe * 365 + yoe / 4 - yoe / 100 + doy
    return era * 146097 + doe - 719468
}
