package com.troubastack.shared.distribution

/**
 * T148 — the device's UTC offset (in seconds, e.g. +7200 for UTC+2) AT the given instant, so a bake time
 * can be shown in the musician's own clock. Instant-aware on purpose: a bake made under a different DST
 * offset than "now" still renders with the offset that was in force when it was baked.
 *
 * A platform seam rather than a datetime library (a dependency is a decision, not a detail — T148): Android
 * reads `java.util.TimeZone`, iOS reads `NSTimeZone`. Returns null only if the zone genuinely cannot be
 * resolved; [concertRowSubtitle] then falls back to a LABELLED UTC time rather than a silent non-local one.
 */
expect fun localUtcOffsetSeconds(epochSec: Long): Int?
