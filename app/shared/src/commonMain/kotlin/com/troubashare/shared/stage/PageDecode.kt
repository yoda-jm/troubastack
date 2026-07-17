package com.troubashare.shared.stage

/**
 * Result of decoding a page's overlays (B1): the overlays that decoded, and how many FAILED. The
 * failure count is carried so the UI degrades VISIBLY (a badge + retry) — a page must NEVER silently
 * render fewer annotation layers than it has (I12 correctness: a missing decode is not "no layer").
 */
internal data class DecodedOverlays<T>(val overlays: List<T>, val missing: Int)

/**
 * Decode each overlay [refs], keeping the successes and COUNTING the failures (never silently drop a
 * failed one). Pure over an injected [decode] (returns null on failure) so the failure accounting is
 * unit-tested without a real `ImageBitmap`/decoder.
 */
internal fun <T : Any> decodeOverlays(refs: List<String>, decode: (String) -> T?): DecodedOverlays<T> {
    val ok = ArrayList<T>(refs.size)
    var missing = 0
    for (ref in refs) {
        val d = decode(ref)
        if (d != null) ok.add(d) else missing++
    }
    return DecodedOverlays(ok, missing)
}

/** Extra frames a page whose overlay decode failed will retry before it gives up and just badges (B1). */
internal const val OVERLAY_DECODE_RETRIES = 3
