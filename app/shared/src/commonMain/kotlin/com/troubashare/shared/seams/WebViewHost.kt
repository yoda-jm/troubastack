// Generated proto types (ConcertBundle, Mutation, …) come from gen/ — single source of
// truth is proto/ (I1). Never hand-write a wire type here.
package com.troubashare.shared.seams

/**
 * SEAM 1 of 3 — the WebView host. One of the ONLY three places native code is allowed (I15).
 *
 * Hosts the **canonical** TroubaStudio editor SPA inside a platform webview. The editor is
 * NEVER reimplemented natively (I10); the app embeds the *built* Studio assets and talks to
 * them over a thin bridge. `app` never imports Studio *source* — it depends only on the
 * contract in proto/ (I14).
 *
 * Responsibilities (kept deliberately minimal):
 *  - load the Studio SPA (from bundled assets or a core-served URL),
 *  - expose a tiny JS bridge so shared Kotlin can feed it (e.g. song id, auth) and react to
 *    editor events (commit, ready),
 *  - on platforms with the native ink overlay (seam 2), report stroke-start/commit so the
 *    wet stroke can migrate native → web (I9).
 *
 * Everything *about* editing — tools, selection, optimistic objects, zoom — lives in Studio,
 * not here (I10). This seam is glue.
 *
 *  - Android `actual` → `android.webkit.WebView` (+ `WebMessagePort` / `@JavascriptInterface`).
 *  - iOS `actual`     → `WKWebView` (+ `WKScriptMessageHandler`).  // iOS-later
 */
expect class WebViewHost {

    /** Load the built Studio SPA and bind the JS bridge. `url` is bundled-asset or core-served. */
    fun load(url: String)

    /** Push a message into Studio (JSON over the bridge). Wire shapes come from proto/ (I1). */
    fun postToStudio(message: String)

    /** Register a sink for events coming out of Studio (ready, commit, …). */
    fun onStudioMessage(handler: (String) -> Unit)

    // TODO(scaffold): formalise the bridge contract once Studio exposes it; signatures only here.
}
