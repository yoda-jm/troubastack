// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubashare.shared.seams

/**
 * SEAM 1 `actual` (iOS) — WebView host (I15, I10). ⚠️ iOS-LATER: TODO, not yet implemented.
 * Concrete API: `WKWebView` with a `WKScriptMessageHandler` bridge. Loads the built Studio SPA;
 * never reimplements the editor. Filling in this `actual` is essentially all iOS needs (I15).
 */
actual class WebViewHost {
    actual fun load(url: String) { TODO("iOS-later: WKWebView.load(URLRequest)") }
    actual fun postToStudio(message: String) { TODO("iOS-later: evaluateJavaScript / postMessage") }
    actual fun onStudioMessage(handler: (String) -> Unit) { TODO("iOS-later: WKScriptMessageHandler") }
}
