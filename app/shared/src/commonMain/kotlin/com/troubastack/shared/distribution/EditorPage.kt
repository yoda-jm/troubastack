package com.troubastack.shared.distribution

/**
 * A93: where the Studio editor WebView is, folded from the platform WebView's callbacks — the page it is on
 * ([currentUrl], what Reload and Retry reload via [reloadUrl]) and whether the current main-frame load failed.
 *
 * Android WebView still calls onPageFinished after a failed main-frame load (for its own error page). Reported
 * as "loaded", that replaced the error state, so the app's error cover and its Retry never stayed on screen
 * (Loading → Error → Loaded, logged on the emulator). [finished] tells a real finish from that one.
 *
 * Order-proof: a failure is cleared only by the app starting a load ([load]) or by a page start for ANOTHER web
 * URL — not by the start of the failed URL itself, so it holds whether the platform reports the error before or
 * after that start, and not by an error page's own pseudo-URL (`chrome-error://…`, `data:…`, `about:blank`). Those
 * starts are not shown as loading either ([started] is false), so nothing replaces the error cover.
 */
class EditorPage {
    var currentUrl: String? = null
        private set
    private var failedUrl: String? = null

    /** The app loads [url]: the first load, Reload, Retry. */
    fun load(url: String) {
        currentUrl = url
        failedUrl = null
    }

    /**
     * A main-frame page started loading (onPageStarted). True = a real start, to show as loading; false = the
     * start of the failed URL's error page or of a pseudo-URL, which must not replace the error cover.
     */
    fun started(url: String?): Boolean {
        if (url == null || !isWebUrl(url) || url == failedUrl) return false
        currentUrl = url
        failedUrl = null
        return true
    }

    /** The URL changed without a page load: Studio's router (pushState), back/forward (doUpdateVisitedHistory). */
    fun visited(url: String?) {
        if (url != null && isWebUrl(url)) currentUrl = url
    }

    /**
     * The main-frame load of [url] failed with [description]. False — nothing to show — when the load was only
     * interrupted (`net::ERR_ABORTED`: Reload pressed mid-load, or another navigation replaced it).
     */
    fun failed(url: String, description: String): Boolean {
        if (description.contains("ERR_ABORTED")) return false
        currentUrl = url
        failedUrl = url
        return true
    }

    /** A main-frame page finished (onPageFinished): true = really loaded; false = the error page of a failure. */
    fun finished(): Boolean = failedUrl == null
}

private fun isWebUrl(url: String): Boolean =
    url.startsWith("http://", ignoreCase = true) || url.startsWith("https://", ignoreCase = true)
