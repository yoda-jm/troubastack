// A51 — the join-link grammar. Pure commonMain (no platform APIs, no network, no UI), because the input
// is HOSTILE by construction: a QR/deep-link string is handed to you by whoever printed it, and the
// person scanning it is by definition not reading it. Turning that string into a DECISION — and refusing
// the dangerous shapes here rather than in the UI — is the whole point. A52 (deep link) and A53 (camera)
// only supply the string and act on the decision.
package com.troubastack.shared.join

/** What a scanned/pasted string turned out to be. */
sealed interface TroubaLink {
    /** A band invite: redeem [token] against [origin] (canonical, see [normalizeOrigin]). */
    data class Join(val origin: String, val token: String) : TroubaLink

    /** A password-reset link — the app has no reset UI; A52 tells the user to open it in a browser. */
    data class PasswordReset(val origin: String, val token: String) : TroubaLink

    /** Not something the app will act on; [reason] is human-readable. */
    data class Unsupported(val reason: String) : TroubaLink
}

/** What the app should DO with a parsed link, given where it currently points. Security posture lives here. */
sealed interface JoinAction {
    /** Same server, signed in ⇒ redeem straight away. */
    data class Redeem(val origin: String, val token: String) : JoinAction

    /** Same server, not signed in ⇒ sign in first, then redeem. */
    data class SignIn(val origin: String, val token: String) : JoinAction

    /** A DIFFERENT (or first-ever) server ⇒ the person must confirm the host before typing a password
     *  into it. Never skippable, including first-run ([current] == null). */
    data class ConfirmServer(val current: String?, val target: String, val token: String) : JoinAction

    /** Nothing to do — a reset link, or an unsupported/hostile string. [reason] is shown. */
    data class Blocked(val reason: String) : JoinAction
}

/**
 * The canonical `scheme://host[:port]` origin of [url], or null if it isn't a usable http(s) origin.
 * A52/A53 compare and DISPLAY this, so it must be canonical: lowercase scheme+host, keep an explicit
 * NON-default port, drop a default port (`http://h:80` ≡ `http://h`, `https://h:443` ≡ `https://h`),
 * drop path/query/fragment, keep an IPv6 literal (`[::1]`) intact. **Userinfo is rejected (null)** — not
 * stripped — because `http://trusted-host@192.0.2.9/…` has host `192.0.2.9`, and any UI that displayed a
 * stripped host would show the wrong server.
 */
internal fun normalizeOrigin(url: String): String? {
    val t = url.trim()
    val sep = t.indexOf("://")
    if (sep <= 0) return null
    val scheme = t.substring(0, sep).lowercase()
    if (scheme != "http" && scheme != "https") return null
    val authority = t.substring(sep + 3).substringBefore('/').substringBefore('?').substringBefore('#')
    if (authority.isEmpty() || authority.contains('@')) return null // userinfo rejected outright
    val (host, port) = splitHostPort(authority) ?: return null
    if (host.isEmpty()) return null
    val hostLc = host.lowercase()
    val defaultPort = if (scheme == "http") 80 else 443
    return if (port != null && port != defaultPort) "$scheme://$hostLc:$port" else "$scheme://$hostLc"
}

/** Split an authority into host + optional numeric port, keeping an IPv6 `[..]` literal whole. Null if
 *  malformed (unterminated bracket, non-numeric port). */
private fun splitHostPort(authority: String): Pair<String, Int?>? {
    if (authority.startsWith("[")) {
        val close = authority.indexOf(']')
        if (close < 0) return null
        val host = authority.substring(0, close + 1) // keep the brackets
        val rest = authority.substring(close + 1)
        val port = when {
            rest.isEmpty() -> null
            rest.startsWith(":") -> rest.substring(1).toIntOrNull() ?: return null
            else -> return null
        }
        return host to port
    }
    val colon = authority.indexOf(':')
    if (colon < 0) return authority to null
    val port = authority.substring(colon + 1).toIntOrNull() ?: return null
    return authority.substring(0, colon) to port
}

private fun isTokenChar(c: Char): Boolean =
    c in 'A'..'Z' || c in 'a'..'z' || c in '0'..'9' || c == '_' || c == '-'

/**
 * A51 — turn an arbitrary scanned/pasted string into a [TroubaLink]. Refuses, in order: any scheme other
 * than http/https; a URL with userinfo; a URL with no host; a path that isn't exactly `/join/<tok>` or
 * `/reset-password/<tok>` (one trailing slash, and a query/fragment, are ignored); a token that is empty,
 * over 512 chars, or contains anything outside `[A-Za-z0-9_-]`. The token length is NOT pinned to today's
 * 32 chars — that is the server's business (widening it must not break the app); charset + a sane cap are.
 */
fun parseTroubaLink(raw: String): TroubaLink {
    val t = raw.trim()
    val sep = t.indexOf("://")
    if (sep <= 0) return TroubaLink.Unsupported("Not a link.")
    val scheme = t.substring(0, sep).lowercase()
    if (scheme != "http" && scheme != "https") return TroubaLink.Unsupported("Only http and https links are supported.")
    val afterScheme = t.substring(sep + 3)
    val authority = afterScheme.substringBefore('/').substringBefore('?').substringBefore('#')
    // Userinfo rejection lives in normalizeOrigin (the single origin-trust gate), so it can't be
    // masked here: a `user@host` URL yields a null origin ⇒ Unsupported. Don't add a second '@' check.
    val origin = normalizeOrigin(t) ?: return TroubaLink.Unsupported("No trustworthy server in the link.")

    val path = afterScheme.substring(authority.length).substringBefore('?').substringBefore('#')
    val parts = path.trim('/').split('/')
    if (parts.size != 2) return TroubaLink.Unsupported("Not a join or reset link.")
    val (kind, token) = parts
    if (token.isEmpty() || token.length > 512 || !token.all(::isTokenChar)) {
        return TroubaLink.Unsupported("The invite code is malformed.")
    }
    return when (kind) {
        "join" -> TroubaLink.Join(origin, token)
        "reset-password" -> TroubaLink.PasswordReset(origin, token)
        else -> TroubaLink.Unsupported("Not a join or reset link.")
    }
}

/**
 * A51 — decide what to do with a parsed [link], given the app's [currentOrigin] (its stored server, or
 * null if never connected) and whether it [hasSession]. The security posture: a Join whose origin differs
 * from the current one — OR when there is no current one — ALWAYS routes to [JoinAction.ConfirmServer], so
 * the person sees the host before typing a password into it. Comparison is on the CANONICAL origin (both
 * sides normalised), so `http://Host:80` and `http://host` are the same server and don't spuriously nag.
 */
fun joinDecision(link: TroubaLink, currentOrigin: String?, hasSession: Boolean): JoinAction = when (link) {
    is TroubaLink.PasswordReset -> JoinAction.Blocked("Open this password-reset link in a web browser.")
    is TroubaLink.Unsupported -> JoinAction.Blocked(link.reason)
    is TroubaLink.Join -> {
        val current = currentOrigin?.let { normalizeOrigin(it) }
        if (current != null && current == link.origin) {
            if (hasSession) JoinAction.Redeem(link.origin, link.token) else JoinAction.SignIn(link.origin, link.token)
        } else {
            JoinAction.ConfirmServer(current = current, target = link.origin, token = link.token)
        }
    }
}
