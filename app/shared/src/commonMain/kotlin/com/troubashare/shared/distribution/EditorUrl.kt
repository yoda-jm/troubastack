package com.troubashare.shared.distribution

/**
 * The query flag the app appends to the Studio editor URL to say "you're hosted inside the app"
 * (A16 ↔ web-core T46). Studio, seeing it at first load, hides its own topbar nav + logout and
 * persists the mode in sessionStorage so SPA navigation stays embedded. This is the SHARED contract
 * string — the app appends it and Studio reads it; keep both sides on this exact token.
 */
const val EMBEDDED_PARAM = "embedded=1"

/**
 * The Studio URL to load in the embedded editor: [base] origin + optional [path] (a deep-link like
 * `/bands/{id}/songs/{id}`, normalized to a leading `/`; null ⇒ `/`, the band list) + [EMBEDDED_PARAM]
 * appended with the correct separator. Pure + shared so the contract is unit-tested off-device.
 */
fun embeddedUrl(base: String, path: String?): String {
    val b = base.trim().trimEnd('/')
    val p = path?.trim()?.takeIf { it.isNotEmpty() }?.let { if (it.startsWith("/")) it else "/$it" } ?: "/"
    val sep = if (p.contains('?')) "&" else "?"
    return "$b$p$sep$EMBEDDED_PARAM"
}
