package com.troubastack.shared.distribution

import java.time.Instant
import java.time.ZoneId

/**
 * T148 (desktop/JVM) — the default zone's UTC offset at [epochSec], in seconds. `java.time` (the spec's
 * choice for this target): resolve the system zone's rules AT that instant, so a bake made under a
 * different DST offset than "now" still renders with the offset that was in force when it was baked.
 * `ZoneId.systemDefault()` always resolves on a desktop JVM, so this never returns null in practice.
 */
actual fun localUtcOffsetSeconds(epochSec: Long): Int? =
    runCatching {
        ZoneId.systemDefault().rules.getOffset(Instant.ofEpochSecond(epochSec)).totalSeconds
    }.getOrNull()
