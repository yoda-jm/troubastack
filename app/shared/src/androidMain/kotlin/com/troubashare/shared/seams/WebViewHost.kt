// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubashare.shared.seams

import android.annotation.SuppressLint
import android.content.Context
import android.webkit.JavascriptInterface
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import org.json.JSONObject

/**
 * SEAM 1 `actual` (Android) — WebView host (I15, I10). Hosts the CANONICAL TroubaStudio SPA in an
 * `android.webkit.WebView`; never reimplements it. The bridge transport is `@JavascriptInterface`
 * (web→shell) + `evaluateJavascript` (shell→web) — simpler and more robust than WebMessagePort, and
 * enough for the A06 handshake (the ink overlay that needs a richer channel is A07, blocked).
 *
 * Constructed with a Context by androidApp, which drops [view] into a Compose AndroidView.
 */
@SuppressLint("SetJavaScriptEnabled") // hosting our own web app (I10) — JS is the whole point
actual class WebViewHost(context: Context) {

    /** WebView load lifecycle surfaced to androidApp so it can show loading/error instead of a blank view. */
    sealed interface State {
        data object Loading : State
        data object Loaded : State
        data class Error(val message: String) : State
    }

    var onState: ((State) -> Unit)? = null
    private var handler: ((String) -> Unit)? = null

    val view: WebView = WebView(context).apply {
        settings.javaScriptEnabled = true
        settings.domStorageEnabled = true      // Studio keeps its session/login here
        settings.allowFileAccess = false       // safe default — no local file access
        addJavascriptInterface(Bridge(), BRIDGE_NAME)
        webViewClient = object : WebViewClient() {
            override fun onPageStarted(v: WebView?, url: String?, favicon: android.graphics.Bitmap?) {
                onState?.invoke(State.Loading)
            }

            override fun onPageFinished(v: WebView?, url: String?) {
                onState?.invoke(State.Loaded)
                // Handshake kickoff: greet Studio; its bridge.ts replies {"type":"ready"} (I10 feature-detected).
                postToStudio("""{"type":"hello","shell":"troubashare-android"}""")
            }

            override fun onReceivedError(v: WebView?, req: WebResourceRequest?, err: WebResourceError?) {
                if (req?.isForMainFrame == true) {
                    onState?.invoke(State.Error(err?.description?.toString() ?: "Couldn't reach the server"))
                }
            }
        }
    }

    actual fun load(url: String) {
        view.loadUrl(url)
    }

    actual fun postToStudio(message: String) {
        // Deliver on the UI thread; JSONObject.quote makes `message` a safe JS string literal.
        view.post {
            view.evaluateJavascript(
                "window.__troubashareShell && window.__troubashareShell.deliver(${JSONObject.quote(message)})",
                null,
            )
        }
    }

    actual fun onStudioMessage(handler: (String) -> Unit) {
        this.handler = handler
    }

    fun canGoBack(): Boolean = view.canGoBack()

    fun goBack() {
        view.goBack()
    }

    private inner class Bridge {
        @JavascriptInterface
        fun receive(json: String) {
            handler?.invoke(json)
        }
    }

    private companion object {
        const val BRIDGE_NAME = "TroubaShareShell"
    }
}
