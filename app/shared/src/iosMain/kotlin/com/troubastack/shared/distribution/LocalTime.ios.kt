package com.troubastack.shared.distribution

import platform.Foundation.NSDate
import platform.Foundation.NSTimeZone
import platform.Foundation.localTimeZone

/**
 * T148 (iOS) — the local zone's UTC offset at [epochSec], in seconds. `secondsFromGMTForDate:` is DST-aware
 * for the given instant, mirroring the Android actual. Runtime-unverified (no iOS device in the loop); the
 * offset MATH lives in the pure [concertRowSubtitle] (commonTest), so only this lookup is iOS-specific.
 */
// Seconds between the Unix epoch (1970-01-01) and NSDate's reference date (2001-01-01). NSDate's only
// millis/seconds constructor is `timeIntervalSinceReferenceDate`, so convert our epoch-1970 instant to it.
private const val UNIX_TO_NSDATE_REF_SECONDS = 978_307_200.0

actual fun localUtcOffsetSeconds(epochSec: Long): Int? {
    val date = NSDate(timeIntervalSinceReferenceDate = epochSec.toDouble() - UNIX_TO_NSDATE_REF_SECONDS)
    return NSTimeZone.localTimeZone.secondsFromGMTForDate(date).toInt()
}
