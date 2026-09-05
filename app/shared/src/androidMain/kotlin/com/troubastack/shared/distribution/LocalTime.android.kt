package com.troubastack.shared.distribution

import java.util.TimeZone

/**
 * T148 (Android) — the default time zone's UTC offset at [epochSec], in seconds. `TimeZone.getOffset`
 * takes epoch millis and returns the offset in millis WITH DST resolved for that instant, so a bake made
 * in summer time renders correctly in winter and vice-versa.
 */
actual fun localUtcOffsetSeconds(epochSec: Long): Int? =
    TimeZone.getDefault().getOffset(epochSec * 1000L) / 1000
