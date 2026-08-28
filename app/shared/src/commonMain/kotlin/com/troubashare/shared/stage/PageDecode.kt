package com.troubashare.shared.stage

/**
 * Result of decoding a page's overlays (B1): the overlays that decoded, and how many FAILED. The
 * failure count is carried so the UI degrades VISIBLY (a badge + retry) — a page must NEVER silently
 * render fewer annotation layers than it has (I12 correctness: a missing decode is not "no layer").
 */
internal data class DecodedOverlays<T>(val overlays: List<T>, val missing: Int)

/**
 * Decode each overlay [items], keeping the successes and COUNTING the failures (never silently drop a
 * failed one). Pure over an injected [decode] (returns null on failure) so the failure accounting is
 * unit-tested without a real `ImageBitmap`/decoder. Generic over the item type [E] so callers can pass
 * either bare refs (tests) or the overlay model (which carries the R10 content hash — task #23).
 */
internal suspend fun <E, T : Any> decodeOverlays(items: List<E>, decode: suspend (E) -> T?): DecodedOverlays<T> {
    val ok = ArrayList<T>(items.size)
    var missing = 0
    for (item in items) {
        val d = decode(item)
        if (d != null) ok.add(d) else missing++
    }
    return DecodedOverlays(ok, missing)
}

/** Extra frames a page whose overlay decode failed will retry before it gives up and just badges (B1). */
internal const val OVERLAY_DECODE_RETRIES = 3
