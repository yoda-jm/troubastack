// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.seams

/**
 * SEAM 1 `actual` (desktop) — the embedded Studio WebView host. There is NO embedded browser on desktop
 * in this track: which engine (Chromium/JCEF, WebView2, or the system browser) is WIN05's measured
 * decision, and WIN03 shows "Coming soon" where Studio will go. So this actual is never constructed.
 *
 * It fails LOUDLY if anything does construct it — the message names WIN05 so the next lane knows exactly
 * what is missing rather than meeting a bare `NotImplementedError`. The skeleton `:desktopApp` window
 * (WIN02) builds no editor screen, so nothing reaches here today.
 */
private const val NO_WEBVIEW =
    "No embedded Studio WebView on desktop yet — the engine is WIN05's decision (Chromium/JCEF vs " +
        "WebView2 vs the system browser), and WIN03 shows \"Coming soon\" in its place. " +
        "If you need to host Studio on desktop, implement this seam under WIN05."

actual class WebViewHost {
    init {
        error(NO_WEBVIEW)
    }

    actual fun load(url: String): Unit = error(NO_WEBVIEW)
    actual fun postToStudio(message: String): Unit = error(NO_WEBVIEW)
    actual fun onStudioMessage(handler: (String) -> Unit): Unit = error(NO_WEBVIEW)
}
