package com.troubastack.shared.distribution

/**
 * The `scheme://host[:port]` origin of [url], lowercased; `""` when there is no scheme+host.
 *
 * Used to **bind a stored session to the server that issued it**: the app persists only a bare
 * `name=value` session cookie, and the server URL is independently user-editable, so without this a
 * session from server A could be replayed to a different user-typed server B (a cross-origin session
 * disclosure — both on the ktor transport and, once seeded, in the Edit WebView). Callers compare the
 * origin the session was issued by against the origin they're about to talk to, and only use the
 * session on a match. Pure + shared so the match logic is unit-tested off-device.
 *
 * Path/query/fragment and any `user:pw@` userinfo are dropped; a missing scheme (`"host:8080"`) or a
 * missing host yields `""`, which callers treat as "no usable origin" (⇒ signed out).
 */
fun originOf(url: String): String {
    val t = url.trim()
    if (!t.contains("://")) return ""
    val scheme = t.substringBefore("://")
    if (scheme.isEmpty()) return ""
    val authority = t.substringAfter("://")
        .substringBefore('/').substringBefore('?').substringBefore('#')
    val hostPort = authority.substringAfter('@') // strip userinfo if present
    if (hostPort.isEmpty()) return ""
    return "${scheme.lowercase()}://${hostPort.lowercase()}"
}
