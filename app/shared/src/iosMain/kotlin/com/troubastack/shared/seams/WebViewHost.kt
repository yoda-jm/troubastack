// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.seams

import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.cinterop.ObjCSignatureOverride
import platform.CoreGraphics.CGRectMake
import platform.Foundation.NSError
import platform.Foundation.NSURL
import platform.Foundation.NSURLRequest
import platform.WebKit.WKNavigation
import platform.WebKit.WKNavigationDelegateProtocol
import platform.WebKit.WKScriptMessage
import platform.WebKit.WKScriptMessageHandlerProtocol
import platform.WebKit.WKUserContentController
import platform.WebKit.WKUserScript
import platform.WebKit.WKUserScriptInjectionTime
import platform.WebKit.WKWebView
import platform.WebKit.WKWebViewConfiguration
import platform.darwin.NSObject

/**
 * SEAM 1 `actual` (iOS) — WebView host (I15, I10). Hosts the CANONICAL TroubaStudio SPA in a
 * `WKWebView`; never reimplements it. Transport mirrors the Android actual: web→shell via a
 * `WKScriptMessageHandler` (bridged so `window.TroubaStageShell.receive(json)` from bridge.ts keeps
 * working), shell→web via `evaluateJavaScript` calling `window.__troubastageShell.deliver`.
 *
 * [view] is exposed so `iosApp` can embed it in Compose via `UIKitView` (that UIKit glue lives in
 * IOS02's iosApp, not in this seam).
 */
@OptIn(ExperimentalForeignApi::class)
actual class WebViewHost {

    /** WebView load lifecycle surfaced to the app so it can show loading/error instead of a blank view. */
    sealed interface State {
        data object Loading : State
        data object Loaded : State
        data class Error(val message: String) : State
    }

    var onState: ((State) -> Unit)? = null
    private var handler: ((String) -> Unit)? = null

    private val messageHandler = object : NSObject(), WKScriptMessageHandlerProtocol {
        override fun userContentController(
            userContentController: WKUserContentController,
            didReceiveScriptMessage: WKScriptMessage,
        ) {
            (didReceiveScriptMessage.body as? String)?.let { handler?.invoke(it) }
        }
    }

    private val navigationDelegate = object : NSObject(), WKNavigationDelegateProtocol {
        @ObjCSignatureOverride
        override fun webView(webView: WKWebView, didStartProvisionalNavigation: WKNavigation?) {
            onState?.invoke(State.Loading)
        }

        @ObjCSignatureOverride
        override fun webView(webView: WKWebView, didFinishNavigation: WKNavigation?) {
            onState?.invoke(State.Loaded)
            // Handshake kickoff: greet Studio; its bridge.ts replies {"type":"ready"} (I10 feature-detected).
            postToStudio("""{"type":"hello","shell":"troubastage-ios"}""")
        }

        @ObjCSignatureOverride
        override fun webView(webView: WKWebView, didFailNavigation: WKNavigation?, withError: NSError) {
            onState?.invoke(State.Error(withError.localizedDescription))
        }

        @ObjCSignatureOverride
        override fun webView(webView: WKWebView, didFailProvisionalNavigation: WKNavigation?, withError: NSError) {
            onState?.invoke(State.Error(withError.localizedDescription))
        }
    }

    val view: WKWebView

    init {
        val controller = WKUserContentController()
        controller.addScriptMessageHandler(messageHandler, BRIDGE_NAME)
        // Route bridge.ts's `window.TroubaStageShell.receive(json)` into the script-message handler,
        // matching the Android @JavascriptInterface surface so one bridge.ts serves both shells.
        val shim = "window.TroubaStageShell = { receive: function(j) { " +
            "window.webkit.messageHandlers.$BRIDGE_NAME.postMessage(j); } };"
        controller.addUserScript(
            WKUserScript(
                shim,
                WKUserScriptInjectionTime.WKUserScriptInjectionTimeAtDocumentStart,
                forMainFrameOnly = true,
            ),
        )
        val config = WKWebViewConfiguration().apply { userContentController = controller }
        view = WKWebView(frame = CGRectMake(0.0, 0.0, 0.0, 0.0), configuration = config).apply {
            navigationDelegate = this@WebViewHost.navigationDelegate
        }
    }

    actual fun load(url: String) {
        val nsUrl = NSURL.URLWithString(url) ?: return
        view.loadRequest(NSURLRequest.requestWithURL(nsUrl))
    }

    actual fun postToStudio(message: String) {
        view.evaluateJavaScript(
            "window.__troubastageShell && window.__troubastageShell.deliver(${jsQuote(message)})",
            null,
        )
    }

    actual fun onStudioMessage(handler: (String) -> Unit) {
        this.handler = handler
    }

    private companion object {
        const val BRIDGE_NAME = "TroubaStageShell"
    }
}
