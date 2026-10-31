package com.troubastack.shared.distribution

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

/**
 * A93: the URL that ⋮ → Reload and the error cover's Retry load — the page the WebView is on, or the URL
 * that just failed ([current]), NOT [entry], the URL the screen was opened with. Reloading [entry] threw
 * the user back to the band after any band → song navigation inside Studio.
 *
 * [current] is reloaded exactly as it is: rebuilding it (e.g. re-appending [EMBEDDED_PARAM]) would make it a
 * different URL, so the WebView would push a new history entry and Back would land on the same page.
 * Embedded mode survives anyway — Studio keeps it in sessionStorage, which a reload of the same WebView keeps.
 *
 * Falls back to [entry] when there is no current URL yet (the first load failed before anything committed),
 * and when [current] is not on [entry]'s server: an error/blank page (`about:blank`, `chrome-error://…`), or
 * the old server's page right after the server URL changed.
 */
fun reloadUrl(current: String?, entry: String): String {
    val c = current?.trim().orEmpty()
    val origin = originOf(entry)
    return if (c.isNotEmpty() && origin.isNotEmpty() && originOf(c) == origin) c else entry
}
