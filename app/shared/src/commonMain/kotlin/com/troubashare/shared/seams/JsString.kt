// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubashare.shared.seams

/**
 * Quote [s] as a safe JavaScript string literal for the shell→web bridge (double-quoted, with
 * control chars escaped). Shared so it's unit-testable off-device; the iOS WebViewHost uses it to
 * build the `evaluateJavaScript` call (Android uses `JSONObject.quote`, the same semantics). Not a
 * seam — pure string logic.
 */
internal fun jsQuote(s: String): String {
    val sb = StringBuilder(s.length + 2)
    sb.append('"')
    for (c in s) {
        when (c) {
            '\\' -> sb.append("\\\\")
            '"' -> sb.append("\\\"")
            '\n' -> sb.append("\\n")
            '\r' -> sb.append("\\r")
            '\t' -> sb.append("\\t")
            else -> if (c < ' ') sb.append("\\u").append(c.code.toString(16).padStart(4, '0')) else sb.append(c)
        }
    }
    sb.append('"')
    return sb.toString()
}
