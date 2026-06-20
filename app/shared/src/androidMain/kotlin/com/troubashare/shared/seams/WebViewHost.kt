// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubashare.shared.seams

/**
 * SEAM 1 `actual` (Android) — WebView host (I15, I10).
 * Concrete API: `android.webkit.WebView` with a JS bridge via `WebMessagePort`
 * (preferred) or `@JavascriptInterface`. Loads the built Studio SPA; never reimplements it.
 */
actual class WebViewHost {
    // TODO(scaffold): hold an android.webkit.WebView; settings.javaScriptEnabled = true.
    actual fun load(url: String) { TODO("Android: WebView.loadUrl(url) + bind WebMessagePort") }
    actual fun postToStudio(message: String) { TODO("Android: post over WebMessagePort") }
    actual fun onStudioMessage(handler: (String) -> Unit) { TODO("Android: WebMessagePort.setWebMessageCallback") }
}
