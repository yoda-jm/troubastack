package com.troubastack.desktop.studio

import androidx.compose.runtime.Composable
import java.io.File
import java.util.ServiceLoader

/**
 * WIN08 — hosting Studio (WebView2 via SWT's `Browser(SWT.EDGE)`) inside the TroubaStage window. The
 * implementation imports SWT, so it lives in `src/windows/kotlin` and is compiled ONLY on a Windows host build;
 * the common desktop code reaches it through this interface, resolved at RUNTIME via [embeddedStudioProvider].
 * On Linux and macOS no provider is registered, so `StudioHost` keeps rendering "Coming soon." and the app image
 * carries no SWT jar — and nothing here references an SWT type, so the module compiles on every OS.
 */
interface EmbeddedStudioProvider {
    /** Render the embedded Studio screen: the app bar (‹ Back, [title], ⋮ Reload) above Studio in a SwingPanel. */
    @Composable
    fun Host(args: EmbeddedStudioArgs)

    /** S2.3: drop the WebView2 cookies for [oldOrigin]'s session ([cookieName]). Applied at once on a live Browser,
     *  else persisted under [edgeDataDir] as a pending clear the next Studio open runs before seeding + the first load. */
    fun clearCookies(edgeDataDir: java.io.File, oldOrigin: String?, cookieName: String?)

    /** S2.3b: like [clearCookies] but ALWAYS deferred to the next open, even with a live Browser — for an in-Studio
     *  re-point, where clearing on the old controller could wipe the new controller's freshly seeded session. */
    fun clearCookiesOnNextOpen(edgeDataDir: java.io.File, oldOrigin: String?, cookieName: String?)
}

/**
 * Everything the Windows host needs, gathered by the common `StudioHost` so the provider imports no app internals.
 * The provider builds the load URL from [serverUrl] + [initialPath] (shared `embeddedUrl`) and derives the origin
 * it locks navigation + the bridge to.
 */
class EmbeddedStudioArgs(
    val serverUrl: String?,
    val initialPath: String?,
    val title: String,
    /** The session cookie `name=value`, from `sessionCookieFor` (this origin only); null when signed out. */
    val sessionCookie: String?,
    /** `org.eclipse.swt.browser.EdgeDataDir` — under the app data root (WIN03 seam), never temp or install dir. */
    val edgeDataDir: File,
    val onBack: () -> Unit,
    /** F11 from inside Studio toggles the app's full screen (S1.7); the host forwards it on the EDT. */
    val toggleFullScreen: () -> Unit,
    val isFullScreen: () -> Boolean,
    /** The TroubaStage frame, for GA_ROOT checks and an owned menu window; null until the window exists. */
    val frameWindow: () -> java.awt.Window?,
    /**
     * S2.1 (R1): pre-load reachability. true if the server answered (Online or Unauthorized — Studio can load and
     * handle its own sign-in); false if it couldn't be reached. The host shows the cover at once on false, instead of
     * waiting out the 10s no-ready timer. null when there's no server configured. Runs the app's own `probePresence`.
     */
    val probeReachable: (suspend () -> Boolean)?,
    /**
     * S2.2: the provider hands back the embedded screen's Back action (controller.requestBack) through this, so the
     * host's registered BackHandler — which fires on WindowAction.BACK (Alt+← / Cmd+[ / Cmd+← with the Compose chrome
     * focused) — walks Studio's history instead of leaving. null on "Coming soon." (keeps onBack).
     */
    val provideRequestBack: ((() -> Unit) -> Unit)? = null,
    /**
     * S2.3b: the ⋮ "Server URL…" dialog's Save hands the new URL here. The host saves it, clears the old origin's
     * cookie through the NEXT-OPEN path (never the live controller), and bumps the serverNonce so a fresh controller
     * loads the new server. null when no server is set yet (the menu item is hidden).
     */
    val onServerUrlChange: ((newUrl: String) -> Unit)? = null,
    /**
     * S2.4: open an off-origin http(s) link in the person's default browser (a separate browser, so WITHOUT the
     * WebView2 session). Returns true if the OS accepted it; false (unsupported, or it threw) makes the embedded
     * screen show one line instead of failing silently. The host wires java.awt.Desktop.browse; the CI smoke passes a
     * recording stub. Only ever called with an http/https URL — the scheme filter is applied before this.
     */
    val openInDefaultBrowser: (url: String) -> Boolean = { false },
    /**
     * WIN09: the ⋮ "Show band QR…" item. The host sets it when the person administers a band; that list arrives after
     * the first composition, so the content re-reads this on every composition. null hides the item.
     */
    val onShowBandQr: (() -> Unit)? = null,
    /**
     * WIN09: the host is drawing its own screen (the band picker, the QR) over the embedded one. The SwingPanel then
     * collapses to 0.dp (S2.1's mechanism, never isVisible=false), so the Browser lives on underneath and Back returns
     * to Studio where it was. Focus leaves the Browser while it shows (the host's overlay takes it) and comes back after.
     */
    val hostOverlay: Boolean = false,
    /** WIN09: the provider hands back a same-origin load through this ("Manage / revoke in Studio" opens
     *  /bands/<id>/settings in the same Browser). Only an absolute path is loaded, through the origin lock. */
    val provideOpenPath: (((path: String) -> Unit) -> Unit)? = null,
    /** WIN09: a ⋮ press first awaits this (the host bounds it, ≤ 800 ms), which refreshes the admin list and answers
     *  with the "Show band QR…" action, fresh or cached (null: no item); then the menu opens. null: open at once. */
    val refreshMenu: (suspend () -> (() -> Unit)?)? = null,
    /** S2.7: the app's RESOLVED theme (themePref.resolveDark(), not the OS setting), read when the ⋮ menu opens so the
     *  ComposePanel menu matches a Parameters theme that differs from Windows'. Default light. */
    val resolveDark: () -> Boolean = { false },
)

/** WIN09 — the URL [path] loads in place: the server's [origin] plus an absolute path ("/…", never "//…" or a URL);
 *  null refuses it. The Browser's origin lock still vets the navigation. */
internal fun studioPathUrl(origin: String, path: String): String? =
    if (path.startsWith("/") && !path.startsWith("//") && '\\' !in path) origin + path else null

/**
 * The embedded-Studio provider on the classpath, or null. On Windows the SWT provider is registered through
 * `META-INF/services` (added to the jar only on a Windows host build); elsewhere this is null. Resolved once.
 */
val embeddedStudioProvider: EmbeddedStudioProvider? by lazy {
    runCatching {
        ServiceLoader.load(EmbeddedStudioProvider::class.java, EmbeddedStudioProvider::class.java.classLoader).firstOrNull()
    }.getOrNull()
}
