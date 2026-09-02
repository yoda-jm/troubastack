package com.troubastack.shared.seams

/**
 * A38 — the persisted-session keys and the storage half of signing out, in shared commonMain so the
 * I12 promise ("Disconnect keeps the server address and never removes concerts") is a TEST, not a
 * claim. [HttpTransport] (androidApp) can't be exercised from commonTest — it touches
 * `android.webkit.CookieManager` — so the storage-clearing logic lives here, pure.
 *
 * The server address key ("coreUrl") is deliberately NOT here: it must survive sign-out so that
 * **Sign in resumes with only a password**. [clearSession] names only the two session keys, so it
 * cannot clear it; and being a put-only function it cannot reach the concerts on disk either.
 */
const val SESSION_COOKIE_KEY = "sessionCookie"
const val SESSION_ORIGIN_KEY = "sessionOrigin"

/**
 * Clear ONLY the two session keys. Pure over a `put` function so a test can prove it leaves the server
 * address (and everything else) untouched and can't touch `bundlesDir` (I12). The Android caller
 * ([HttpTransport.signOut]) calls this and then wipes the shared WebView cookie jar.
 */
fun clearSession(put: (String, String) -> Unit) {
    put(SESSION_COOKIE_KEY, "")
    put(SESSION_ORIGIN_KEY, "")
}
