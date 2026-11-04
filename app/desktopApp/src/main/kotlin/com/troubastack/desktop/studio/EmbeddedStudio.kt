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
)

/**
 * The embedded-Studio provider on the classpath, or null. On Windows the SWT provider is registered through
 * `META-INF/services` (added to the jar only on a Windows host build); elsewhere this is null. Resolved once.
 */
val embeddedStudioProvider: EmbeddedStudioProvider? by lazy {
    runCatching {
        ServiceLoader.load(EmbeddedStudioProvider::class.java, EmbeddedStudioProvider::class.java.classLoader).firstOrNull()
    }.getOrNull()
}
