package com.troubastack.desktop.studio

import com.troubastack.shared.distribution.reloadUrl
import com.sun.jna.platform.win32.WinDef.HWND
import org.eclipse.swt.SWT
import org.eclipse.swt.browser.Browser
import org.eclipse.swt.browser.BrowserFunction
import org.eclipse.swt.browser.LocationAdapter
import org.eclipse.swt.browser.LocationEvent
import org.eclipse.swt.browser.ProgressAdapter
import org.eclipse.swt.browser.ProgressEvent
import org.eclipse.swt.events.KeyAdapter
import org.eclipse.swt.events.KeyEvent
import org.eclipse.swt.layout.FillLayout
import org.eclipse.swt.widgets.Display
import org.eclipse.swt.widgets.Shell
import java.awt.Canvas
import java.io.File
import javax.swing.SwingUtilities

/**
 * WIN08 — the SWT/WebView2 side of the embedded Studio host, isolated from Compose on its own thread.
 *
 * Threading (design GO, binding B1):
 *  - A dedicated **daemon** thread owns the SWT [Display] and runs its `readAndDispatch` pump (WebView2 is COM-STA
 *    and needs OS-message pumping; `readAndDispatch` is that pump). Nothing touches SWT off this thread — callers
 *    use [exec] (`display.asyncExec`).
 *  - Callbacks that change Compose state hop to the EDT first (`SwingUtilities.invokeLater`).
 *  - [dispose] runs an ordered teardown and a BOUNDED join (~2 s), so a stuck SWT thread can never keep the JVM
 *    alive (WIN04's updater waits for the process to end). The EDT never blocks on this thread but that join.
 *
 * The Browser's native DLLs unpack to `swt.library.path` (set under the data root in Main), never `~/.swt`; the
 * WebView2 user-data folder is [edgeDataDir] under the data root, set before the Browser is created.
 */
internal class SwtBrowserController(
    private val canvas: Canvas,
    private val loadUrl: String,
    private val origin: String,
    private val edgeDataDir: File,
    private val sessionCookie: String?,
    private val forceNoWebview: Boolean,
    private val onReady: () -> Unit,
    private val onEngineMissing: () -> Unit,
    private val onStartupTimeout: () -> Unit,
    // S2.1 (R1): a navigation completed but no `ready` arrived within 10s — a load that failed (server up, Studio silent).
    private val onNavNoReady: () -> Unit = {},
    private val onFullScreenKey: () -> Unit,
    // S2.2: leave the Studio screen (the app's Back) only when Studio's own history is exhausted.
    private val onExitBack: () -> Unit = {},
    // S2.4: open an off-origin http(s) link in the default browser (returns false if the OS refused); and show the
    // one-line "couldn't open" notice when it does. Defaults keep the smoke/tests that don't route links simple.
    private val openInDefaultBrowser: (url: String) -> Boolean = { false },
    private val onLinkError: () -> Unit = {},
    private val log: (String) -> Unit,
) {
    @Volatile var browserType: String? = null; private set
    @Volatile var bridgeReady: Boolean = false; private set
    @Volatile var helloAnswered: Boolean = false; private set
    @Volatile var navVetoed: Boolean = false; private set
    @Volatile var started: Boolean = false; private set
    @Volatile var engineMissingReported: Boolean = false; private set
    @Volatile var startupTimedOut: Boolean = false; private set
    @Volatile var shellEverShown: Boolean = false; private set   // true once the embedded Shell was made visible
    @Volatile var readyCount: Int = 0; private set   // S2.1: each page handshake bumps this; a reload with the Shell HIDDEN must still bump it (R1)
    @Volatile var navNoReadyFired: Boolean = false; private set   // S2.1 (R1): a host load went 10s with no ready → cover (smoke checks this)
    @Volatile var newWindowRequested: Boolean = false; private set // S1.4 report: a target=_blank/window.open fired
    @Volatile var docCookie: String? = null; private set   // what document.cookie sees (item 5 — must NOT hold the session)
    @Volatile var openWindowBlocked: Boolean? = null; private set  // item 7 — the page's window.open() must return null (popup blocked)
    @Volatile var lastOpenInPlace: String? = null; private set   // S2.4: last same-origin link routed in place
    @Volatile var lastOpenExternal: String? = null; private set  // S2.4: last other-origin http(s) link sent to the default browser
    @Volatile var lastOpenDropped: String? = null; private set   // S2.4: last non-http(s) link dropped by the scheme filter
    @Volatile var droppedUrls: String = ""; private set          // S2.4: all dropped links (newline-joined), so a fast pair isn't lost to a race
    @Volatile var linksInjected: Boolean = false; private set    // S2.4: the page-level link capture is attached (smoke waits for it)
    @Volatile var downloadDone: Boolean = false; private set     // S2.5: saveBlob ran (the file save is then async in WebView2)
    @Volatile var downloadName: String? = null; private set      // S2.5: the a.download name (from Content-Disposition)
    @Volatile var downloadErr: String? = null; private set       // S2.5: the fetch/download threw (e.g. a 401)
    @Volatile private var disposed: Boolean = false
    // B2 DPI measurement (filled by measureDpi, read by the smoke).
    @Volatile var swtThreadDpiAwareness: Int = -1; private set
    @Volatile var canvasClient: String = "?"; private set
    @Volatile var shellClient: String = "?"; private set
    @Volatile var rectsMatch: Boolean = false; private set

    @Volatile private var display: Display? = null
    @Volatile private var shell: Shell? = null
    @Volatile private var browser: Browser? = null
    @Volatile private var canvasHwnd: HWND? = null
    // Daemon so a stuck SWT thread can never keep the JVM alive (B1) — the reliable close path is the window close.
    private val thread = Thread(::run, "win08-swt-display").apply { isDaemon = true }

    private fun reportEngineMissing() { engineMissingReported = true; onEdt { onEngineMissing() } }
    private fun reportStartupTimeout() { startupTimedOut = true; onEdt { onStartupTimeout() } }

    /** WIN08 sabotage knob (dormant unless WIN08_SABOTAGE is set — only the CI sabotage runs set it, never
     *  production). Each value breaks exactly one guarded behaviour so its smoke check can be shown to red. */
    private val sabotage: String = System.getenv("WIN08_SABOTAGE").orEmpty()

    fun start() = thread.start()

    /** Re-entry (item 3): the run() thread is alive until its Shell is disposed and the loop exits. */
    fun threadAlive(): Boolean = thread.isAlive

    /** Run [block] on the SWT display thread (no-op once disposed). */
    private fun exec(block: () -> Unit) {
        val d = display ?: return
        if (!d.isDisposed) runCatching { d.asyncExec { if (shell?.isDisposed == false) block() } }
    }

    private fun onEdt(block: () -> Unit) = SwingUtilities.invokeLater(block)

    /** ⋮ Reload — A93: stay on the page you are on (or the entry if it is blank / a different origin). */
    fun reload() = exec {
        val b = browser ?: return@exec
        b.url = reloadUrl(b.url, loadUrl)
        armNoReadyTimer()   // S2.1 (R1): ⋮ Reload / Retry is host-initiated → cover if it doesn't ready in 10s
        onEdt { onReloadStarted?.invoke() }   // host resets to "Loading…" (so a failed reload can show the cover again)
    }

    /**
     * S2.1 (R1, refinement 2): arm the 10s no-ready → cover timer for a HOST-initiated load (first load / ⋮ Reload /
     * Retry) only. Captures readyCount; if no fresh `ready` arrives within 10s, the load failed → onNavNoReady. In-page
     * navigations (S2.4's in-place same-origin links, e.g. opening a PDF) never call this, so they can't trip the cover.
     */
    private fun armNoReadyTimer() {
        if (sabotage == "noreadytimer") { log("win08: SABOTAGE noreadytimer — not arming the no-ready timer"); return }  // → never-ready check reds
        val before = readyCount
        val d = browser?.display ?: return
        d.timerExec(10_000) {
            if (!disposed && readyCount == before) {
                navNoReadyFired = true
                log("win08: no ready 10s after a host-initiated load — cover")
                onEdt(onNavNoReady)
            }
        }
    }

    /**
     * SMOKE ONLY (S2.1): hide/show the embedded Shell, to prove a not-visible Browser still loads and sends `ready`.
     * It is NOT how the host shows "Loading…"/the cover — that collapses the SwingPanel to 0.dp (see the provider).
     * Hiding the Shell (or the canvas) does not reveal Compose: the SwingPanel's heavyweight interop reserves the hole.
     */
    fun setShellVisible(visible: Boolean) = exec { shell?.takeIf { !it.isDisposed }?.isVisible = visible }

    /** Smoke only: try to navigate off-origin; the LocationListener must veto it ([navVetoed] is the proof). */
    fun probeOffOriginNav(foreign: String) = exec { browser?.setUrl(foreign) }

    /** Smoke only (S2.1 R2): a same-origin IN-PAGE navigation (like a link click) — NOT host-initiated, so it does NOT
     *  arm the no-ready timer. Proof that S2.4's in-place links can't trip the cover (navNoReadyFired must stay false). */
    fun smokeInPageNav(sameOrigin: String) = exec { browser?.setUrl(sameOrigin) }

    /** Smoke only (S2.2): dispatch a synthetic mouseup with button===3 in the page, to exercise the injected mouse-back
     *  listener → bridge {type:"back"} → requestBack chain (not the real XBUTTON1; that's the --win08-back recon). */
    fun smokeDispatchMouseBack() = exec { browser?.execute("window.dispatchEvent(new MouseEvent('mouseup',{button:3,bubbles:true}))") }

    /** S2.4 smoke: create an <a href target> and click it, so the injected capture routes it (like a real link click). */
    fun smokeClickLink(href: String, target: String) = exec { browser?.execute(
        "var a=document.createElement('a');a.setAttribute('href','$href');a.setAttribute('target','$target');a.textContent='x';document.body.appendChild(a);a.click();") }

    /** S2.5 smoke: run Studio's REAL download path verbatim — fetch(credentials) → Blob → filenameFromDisposition →
     *  saveBlob (createObjectURL → temp <a download> → click → immediate revokeObjectURL). Copied from
     *  web/studio/src/pages/song-editor/FilePdfButton.tsx (saveBlob) and api.ts (filenameFromDisposition, the fetch). */
    fun smokeSaveBlob(url: String) = exec { browser?.execute(
        "(function(){" +
        "function fn(h,fb){var d=h||'';var e=/filename\\*\\s*=\\s*UTF-8''([^;]+)/i.exec(d);" +
        "if(e){try{var n=decodeURIComponent(e[1].trim());if(n)return n;}catch(x){}}" +
        "var p=/filename\\s*=\\s*\"([^\"]*)\"/i.exec(d)||/filename\\s*=\\s*([^;]+)/i.exec(d);var m=p&&p[1].trim();return m?m:fb;}" +
        "function sb(blob,filename){var u=URL.createObjectURL(blob);var a=document.createElement('a');a.href=u;a.download=filename;document.body.appendChild(a);a.click();a.remove();URL.revokeObjectURL(u);}" +
        "fetch('$url',{credentials:'include'}).then(function(res){if(!res.ok)throw new Error('status '+res.status);" +
        "return res.blob().then(function(b){var f=fn(res.headers.get('Content-Disposition'),'file.pdf');sb(b,f);" +
        "var r=window.TroubaStageShellReceive;if(r)r(JSON.stringify({type:'downloaddone',name:f}));});})" +
        ".catch(function(err){var r=window.TroubaStageShellReceive;if(r)r(JSON.stringify({type:'downloaderr',msg:String(err)}));});" +
        "})()"
    ) }

    // ── S2.4: route a captured new-window / target link by origin ────────────────────────────────────────────────
    // The page injection (below) forwards target/window.open/ctrl+middle-click URLs here (resolved absolute). Only
    // http(s) ever reaches the default browser; the server's origin loads in place; everything else (search-ms:,
    // ms-*:, file:, custom schemes) is DROPPED + logged — browse() on those launches Windows protocol handlers, an
    // attack path. No in-app window ever opens (the OpenWindowListener backstop + this preventDefault the request).
    // Sabotages: nolinkfilter skips the scheme filter; linksalwaysinplace / linksalwaysexternal ignore the origin.
    private fun routeOpenUrl(url: String?) {
        if (url.isNullOrBlank()) return
        val sab = System.getenv("WIN08_SABOTAGE")
        val scheme = url.substringBefore(':', "").lowercase()
        if (sab != "nolinkfilter" && scheme != "http" && scheme != "https") {
            lastOpenDropped = url; droppedUrls += "$url\n"; log("win08: S2.4 dropped non-http(s) link: $url"); return
        }
        val inPlace = when (sab) {
            "linksalwaysinplace" -> true
            "linksalwaysexternal" -> false
            else -> sameOrigin(url, origin)
        }
        if (inPlace) {
            lastOpenInPlace = url; log("win08: S2.4 same-origin link → in place: $url")
            // Sabotage linksnoplaceload: make the in-place DECISION but never load it → the page doesn't change, so the
            // same-origin check (which asserts the main browser ended on the link) reds. "In place" must actually navigate.
            if (sab != "linksnoplaceload") exec { browser?.setUrl(url) }
        } else {
            lastOpenExternal = url; log("win08: S2.4 other-origin link → default browser: $url")
            Thread { if (!openInDefaultBrowser(url)) { log("win08: S2.4 default browser refused $url"); onEdt(onLinkError) } }
                .apply { isDaemon = true; start() }
        }
    }

    // ── S2.3: WebView2 cookie clearing (server change / sign-out) ────────────────────────────────────────────────
    private val pendingClearFile get() = java.io.File(edgeDataDir, "pending-cookie-clear")

    /** Clear the old session's WebView2 cookie. Option C — expire the exact cookie by (name, host-only origin, Path=/):
     *  a cookie is keyed by (name, domain, path), so Max-Age=0 deletes it whether it was a session or a persistent
     *  (+30d) cookie. Then option A — clearSessions() as a backup. Runs on the SWT thread. */
    private fun applyCookieClear(oldOrigin: String?, cookieName: String?) {
        // Sabotage clearonlya: skip option C (do only clearSessions). The smoke's persistent cookie then survives, which
        // proves C — not clearSessions — is what deletes the persistent half (the cleared-cookie check reds).
        val skipC = System.getenv("WIN08_SABOTAGE") == "clearonlya"
        if (!skipC && !cookieName.isNullOrEmpty() && !oldOrigin.isNullOrEmpty()) {
            runCatching { Browser.setCookie("$cookieName=; Path=/; Max-Age=0", oldOrigin) }
                .onSuccess { ok -> log("win08: cookie clear C setCookie($cookieName Max-Age=0 @ $oldOrigin)=$ok") }
                .onFailure { log("win08: cookie clear C failed: ${it.message}") }
        }
        runCatching { Browser.clearSessions() }.onSuccess { log("win08: cookie clear A clearSessions()") }
    }

    /** Applied in run() before seeding (S2.3): a clear requested while no Browser was live was persisted here. */
    private fun applyPendingCookieClear() {
        // Sabotage nopending: never apply the pending clear. The reopen then carries the old persistent cookie → the
        // pending-clear-applied-on-reopen check reds, proving the pre-seed clear is what keeps the reopen clean.
        if (System.getenv("WIN08_SABOTAGE") == "nopending") { log("win08: WIN08_SABOTAGE=nopending — skipping pending clear"); return }
        val f = pendingClearFile
        if (!f.exists()) return
        val records = runCatching { f.readLines() }.getOrDefault(emptyList()).filter { it.isNotBlank() }
        log("win08: applying ${records.size} pending cookie clear record(s)")
        for (rec in records) {
            val parts = rec.split('\t')
            applyCookieClear(parts.getOrNull(0)?.takeIf { it.isNotEmpty() }, parts.getOrNull(1)?.takeIf { it.isNotEmpty() })
        }
        appliedPendingRecords += records.size   // S2.3b: the serverurl smoke asserts the re-point's record was applied
        runCatching { f.delete() }
    }

    /** S2.3b smoke: how many pending-clear records this controller applied in run() before seeding — a re-point writes
     *  a record and the new controller must apply it, so this is >0 after a re-point (0 if the write or apply is skipped). */
    @Volatile var appliedPendingRecords = 0; private set

    /** S2.3: clear the WebView2 cookies NOW on this live Browser (server change from inside Studio). */
    fun clearCookiesNow(oldOrigin: String?, cookieName: String?) = exec { applyCookieClear(oldOrigin, cookieName) }

    /**
     * S2.2 Back: ‹ Back / Alt+← / the mouse back button first walk Studio's own history (`browser.back()` while
     * `isBackEnabled`), as Android's BackHandler does; they leave the Studio screen ([onExitBack]) only when there is
     * nothing left to go back to. [wentBack] records the last outcome for the smoke.
     */
    @Volatile var wentBack: Boolean? = null; private set
    @Volatile var mouseBackListenerFired: Boolean = false; private set   // S2.2 recon: the injected page listener saw the mouse back
    @Volatile var swtSawMouseBack: Boolean = false; private set          // S2.2 recon: SWT's own MouseListener saw XBUTTON1
    @Volatile var navCount: Int = 0; private set                         // S2.2 recon: completed navigations (to count one-vs-two on a back)
    @Volatile var lastUrl: String = ""; private set                      // S2.2 recon: the URL after the last completed navigation
    fun requestBack() = exec {
        val b = browser
        // Sabotage backalwaysleaves: ignore Studio's history and always leave → the walk-first check reds.
        if (sabotage == "backalwaysleaves") { wentBack = false; log("win08: SABOTAGE backalwaysleaves — leaving regardless of history"); onEdt { onExitBack() }; return@exec }
        if (b != null && !b.isDisposed && runCatching { b.isBackEnabled }.getOrDefault(false)) {
            wentBack = true; log("win08: Back — walking Studio history (browser.back())"); b.back()
        } else {
            wentBack = false; log("win08: Back — no Studio history, leaving the screen"); onEdt { onExitBack() }
        }
    }

    /**
     * Item 8 — the Studio ⋮ menu, over the heavyweight WebView. A HEAVYWEIGHT `JPopupMenu` (lightweight disabled) is a
     * transient popup: it floats over the browser's child HWND, is NEVER activating (its window is non-focusable) and
     * is NEVER topmost — so the frame keeps activation (title bar stays active). This replaces the Compose DropdownMenu,
     * which needed process-wide compose.layers.type=WINDOW (that stole activation from the frame on every popup).
     * Smoke proof: with it open, GetForegroundWindow()==the frame AND no process window is WS_EX_TOPMOST.
     * Sabotage 'menufocus': a FOCUSABLE menu window takes activation (GetForegroundWindow != frame) → the check reds.
     */
    @Volatile var lastMenuWindow: java.awt.Window? = null; private set  // the ⋮ menu window, for the smoke's topmost check
    /** Set by EmbeddedStudioContent — the L2 sabotage opens the REAL bad path (a Compose DropdownMenu, under
     *  compose.layers.type=WINDOW) through this, since that is what actually steals the frame's activation. */
    @Volatile var requestComposeMenu: (() -> Unit)? = null
    /** S2.3b: set by EmbeddedStudioContent when a server is configured — the ⋮ "Server URL…" item calls it to open the
     *  in-canvas re-point dialog. null hides the item (no server set). */
    @Volatile var requestServerDialog: (() -> Unit)? = null
    // S2.1 (R1): a host-initiated reload started (⋮ Reload or the cover's Retry) — the host resets to "Loading…" so a
    // reload that fails can bring the cover back (even after a prior success, since bridgeReady stays latched).
    @Volatile var onReloadStarted: (() -> Unit)? = null

    fun openStudioMenu() = onEdt {
        if (!canvas.isShowing) return@onEdt
        // L2 sabotage 'menufocus': take the REAL bad path — a Compose DropdownMenu with compose.layers.type=WINDOW
        // (set in Main when the env is present). That popup is a separate window that DOES take activation on this
        // runner, so the foreground half of menu-no-activation-steal goes red (SetForegroundWindow on a plain JWindow
        // is restricted on CI, so the owned-window path couldn't prove the foreground red).
        if (sabotage == "menufocus") { log("win08: SABOTAGE menufocus — opening a Compose DropdownMenu (WINDOW layer)"); requestComposeMenu?.invoke(); return@onEdt }
        val owner = javax.swing.SwingUtilities.getWindowAncestor(canvas) ?: return@onEdt
        val w = javax.swing.JWindow(owner)
        w.isAlwaysOnTop = false                     // NEVER topmost (Fable's criterion)
        w.focusableWindowState = false              // never activating → the frame keeps activation (title bar active)
        val panel = javax.swing.JPanel().apply {
            layout = javax.swing.BoxLayout(this, javax.swing.BoxLayout.Y_AXIS)
            border = javax.swing.BorderFactory.createLineBorder(java.awt.Color(0x60, 0x60, 0x60))
        }
        panel.add(javax.swing.JMenuItem("Reload").apply { addActionListener { reload(); w.dispose() } })
        // S2.3b: "Server URL…" opens the in-canvas re-point dialog (only when a server is set → requestServerDialog != null).
        requestServerDialog?.let { req -> panel.add(javax.swing.JMenuItem("Server URL…").apply { addActionListener { w.dispose(); req() } }) }
        w.contentPane.add(panel); w.pack()
        val loc = canvas.locationOnScreen
        w.setLocation(loc.x + canvas.width - w.width - 8, loc.y + 2)
        w.isVisible = true
        lastMenuWindow = w
        // --- L1 dismissal. A press inside the WebView is NOT an AWT event (it is another process's window under SWT's
        // Shell), so the AWTEventListener never sees it. Dismiss on: a press on a Swing component outside the menu, the
        // Browser gaining focus (a click in Studio → Edge handleGotFocus), and the frame moving/resizing/deactivating. ---
        val toolkit = java.awt.Toolkit.getDefaultToolkit()
        val dismiss = java.awt.event.AWTEventListener { ev ->
            val me = ev as? java.awt.event.MouseEvent ?: return@AWTEventListener
            if (me.id == java.awt.event.MouseEvent.MOUSE_PRESSED &&
                javax.swing.SwingUtilities.getWindowAncestor(me.component) !== w) w.dispose()
        }
        toolkit.addAWTEventListener(dismiss, java.awt.AWTEvent.MOUSE_EVENT_MASK)
        val browserFocus = object : org.eclipse.swt.events.FocusAdapter() {
            override fun focusGained(e: org.eclipse.swt.events.FocusEvent) { onEdt { if (w.isDisplayable) w.dispose() } }
        }
        // Sabotage 'menusticky': skip the Browser focus listener → a Studio click leaves the menu open → L1 vector reds.
        if (sabotage != "menusticky") exec { browser?.addFocusListener(browserFocus) }
        val frameListener = object : java.awt.event.ComponentAdapter(), java.awt.event.WindowFocusListener {
            override fun componentMoved(e: java.awt.event.ComponentEvent?) { w.dispose() }
            override fun componentResized(e: java.awt.event.ComponentEvent?) { w.dispose() }
            override fun windowLostFocus(e: java.awt.event.WindowEvent?) { w.dispose() }
            override fun windowGainedFocus(e: java.awt.event.WindowEvent?) {}
        }
        owner.addComponentListener(frameListener); owner.addWindowFocusListener(frameListener)
        w.addWindowListener(object : java.awt.event.WindowAdapter() {
            override fun windowClosed(e: java.awt.event.WindowEvent?) {
                toolkit.removeAWTEventListener(dismiss)
                exec { runCatching { browser?.removeFocusListener(browserFocus) } }
                owner.removeComponentListener(frameListener); owner.removeWindowFocusListener(frameListener)
                if (lastMenuWindow === w) lastMenuWindow = null
            }
        })
        log("win08: studio ⋮ menu shown (owned JWindow, focusable=false, alwaysOnTop=false)")
    }

    /** L1 smoke vector: model a click in Studio by giving the Browser focus → the menu's Focuslistener must dismiss it. */
    fun focusBrowserForSmoke() = exec { runCatching { browser?.forceFocus() } }

    /** Keep the reparented child Shell filling the AWT canvas, in PHYSICAL pixels (item 4): size from the canvas
     *  HWND's own client rect via SetWindowPos, so it is right even when AWT's scale differs from SWT's (e.g.
     *  -Dsun.java2d.uiScale=1.5). SWT's setBounds (SWT points) is only correct while both toolkits share a scale. */
    private fun fitShellToCanvas(shell: Shell) {
        if (shell.isDisposed) return
        val ch = canvasHwnd
        // "logicalfit" sabotage (item 4): use SWT's logical setBounds instead of the physical GetClientRect, so at
        // -Dsun.java2d.uiScale=1.5 the Shell comes out ~2/3 of the canvas and the DPI check reds.
        if (ch != null && sabotage != "logicalfit") Win08Native.fitChildToParent(Win08Native.hwnd(shell.handle), ch)
        else shell.setBounds(0, 0, maxOf(1, canvas.width), maxOf(1, canvas.height))
    }

    /** B2 — on the SWT thread, read this thread's DPI awareness and compare the embedded Shell's client rect to the
     *  canvas's (equal at 100 %). The process and EDT awareness are read by the smoke on their own threads. */
    fun measureDpi() {
        val d = display ?: return
        if (d.isDisposed) return
        runCatching {
            d.syncExec {
                swtThreadDpiAwareness = Win08Native.threadDpiAwareness()
                val ch = canvasHwnd
                val sh = shell?.takeIf { !it.isDisposed }?.let { Win08Native.hwnd(it.handle) }
                if (ch != null && sh != null) {
                    val (cw, chh) = Win08Native.clientSize(ch)
                    val (sw, shh) = Win08Native.clientSize(sh)
                    canvasClient = "${cw}x${chh}"
                    shellClient = "${sw}x${shh}"
                    rectsMatch = cw == sw && chh == shh && cw > 0 && chh > 0
                }
            }
        }
    }

    /** Ordered teardown + bounded join, from the screen's onDispose (every exitApplication path runs it). Sets
     *  [disposed] FIRST and wakes the display, so a dispose() that arrives before the Shell exists is honoured by
     *  the run() thread (which checks [disposed] at each step) rather than silently dropped. */
    fun dispose() {
        disposed = true
        val d = display
        runCatching { d?.takeIf { !it.isDisposed }?.asyncExec { shell?.takeIf { !it.isDisposed }?.dispose() } }
        runCatching { d?.wake() }
        runCatching { thread.join(2000) }
    }

    private fun run() {
        var display: Display? = null
        try {
            log("ctrl: run start (thread=${Thread.currentThread().name})")
            // EdgeDataDir MUST be set before the first Browser(SWT.EDGE) creates the WebView2 environment.
            System.setProperty("org.eclipse.swt.browser.EdgeDataDir", edgeDataDir.also { it.mkdirs() }.absolutePath)
            display = Display()
            this.display = display
            started = true
            log("ctrl: Display created")
            if (forceNoWebview) {
                // S1.6 CI path: behave as if WebView2 is absent, without creating a Shell/Browser.
                log("ctrl: forceNoWebview — reporting engine missing")
                reportEngineMissing()
                return
            }
            if (disposed) { log("ctrl: disposed before the Shell existed — bailing, no window"); return }
            // Wait for the AWT canvas to be realized (SwingPanel adds + shows it) before creating the Shell.
            val deadline = System.currentTimeMillis() + 10_000
            while (!canvas.isDisplayable && !disposed && System.currentTimeMillis() < deadline) Thread.sleep(20)
            if (disposed) { log("ctrl: disposed during the canvas wait — bailing, no window"); return }
            if (!canvas.isDisplayable) { log("ctrl: canvas never became displayable"); reportEngineMissing(); return }
            log("ctrl: canvas displayable; creating TOP-LEVEL Shell + Browser(SWT.EDGE)")

            // A top-level, SWT-thread-owned Shell (NOT SWT_AWT.new_Shell): WebView2's controller-ready callback then
            // pumps on the SWT thread (WIN05: a bare Shell readies in ~80ms). Under SWT_AWT the embedded Shell was
            // AWT-affine and the first Edge op timed out at 5s. We reparent this Shell into the AWT canvas below.
            val shell = Shell(display, SWT.NO_TRIM)
            this.shell = shell
            shell.layout = FillLayout()

            val tBrowser = System.currentTimeMillis()
            val browser = try {
                Browser(shell, SWT.EDGE)
            } catch (e: Throwable) {
                log("win08: Browser(SWT.EDGE) threw: ${e.message}")
                reportEngineMissing()
                runLoopUntilDisposed(display, shell)
                return
            }
            val type = if (sabotage == "engine") "ie" else runCatching { browser.browserType }.getOrNull()
            browserType = type
            log("win08: browserType=$type (sabotage=$sabotage)")
            if (type != "edge") {
                // Never fall back to IE: a non-edge backend is "no WebView2".
                browser.dispose()
                reportEngineMissing()
                runLoopUntilDisposed(display, shell)
                return
            }
            this.browser = browser

            // Child-HWND embedding FIRST (before any synchronous Edge op): reparent the top-level Shell's HWND into
            // the heavyweight AWT canvas and show it. WebView2 initialises its controller for a shown window; doing
            // this first is what the reliable runs did — reordering it after createFunction left the controller
            // unready. Track the canvas size so the Shell keeps filling it.
            val canvasHwnd = Win08Native.hwndOf(canvas)
            this.canvasHwnd = canvasHwnd
            if (canvasHwnd != null && sabotage != "reparent") {
                Win08Native.embed(Win08Native.hwnd(shell.handle), canvasHwnd)
                if (sabotage == "nofit") shell.setBounds(0, 0, 120, 80) else fitShellToCanvas(shell)
                if (sabotage != "nofit") canvas.addComponentListener(object : java.awt.event.ComponentAdapter() {
                    override fun componentResized(e: java.awt.event.ComponentEvent?) = exec { fitShellToCanvas(shell) }
                })
                shell.isVisible = true; shellEverShown = true
                log("ctrl: reparented shell into canvas hwnd=$canvasHwnd (sabotage=$sabotage)")
            } else if (sabotage == "reparent") {
                // SABOTAGE only: leave the Shell top-level so no WebView2 window is a frame descendant (GA_ROOT reds).
                shell.setBounds(0, 0, 800, 600); shell.isVisible = true
                log("ctrl: shell NOT embedded (sabotage=reparent)")
            } else {
                // Production with no canvas HWND: never show a stray top-level window — report engine missing instead.
                log("ctrl: no canvas HWND; reporting engine missing (no window shown)")
                browser.dispose(); reportEngineMissing(); runLoopUntilDisposed(display, shell); return
            }
            if (disposed) { log("ctrl: disposed during setup — tearing down"); shell.dispose(); runLoopUntilDisposed(display, shell); return }

            // Bridge: a BrowserFunction whose bare-identifier name Studio's bridge.ts picks up (TroubaStageShellReceive).
            // SWT 3.134.0 injects it via AddScriptToExecuteOnDocumentCreated, so it exists before the page's scripts.
            // The handler acts ONLY on calls from the server's origin (S1.4, item 6). SWT's Edge.timeout was raised to
            // 30s in main(), so this SINGLE createFunction waits long enough for a cold controller — no warm-up, no
            // retry (the first timeout would poison the Browser). The "bridge-late" sabotage registers it after load.
            fun registerBridge() {
                object : BrowserFunction(browser, "TroubaStageShellReceive") {
                    override fun function(arguments: Array<out Any?>?): Any? {
                        if (!sameOrigin(browser.url ?: "", origin)) return null
                        val json = arguments?.getOrNull(0) as? String ?: return null
                        when {
                            // S2.2: the host-injected mouseup/button===3 listener forwards the mouse back button here;
                            // we act on it like Alt+← — walk Studio's history, or leave the screen at the root.
                            json.contains("\"back\"") -> { log("win08: mouse back via bridge — Back"); requestBack() }
                            json.contains("\"mousebackseen\"") -> { mouseBackListenerFired = true; log("win08: mouse back listener fired in the page") }
                            json.contains("\"cookie\"") -> { docCookie = json; log("win08: document.cookie reported") }
                            json.contains("\"openwindow\"") -> { openWindowBlocked = json.contains("\"blocked\":true"); log("win08: openwindow reported blocked=$openWindowBlocked") }
                            // S2.4: a target/window.open/ctrl+middle-click link the page injection captured (absolute URL).
                            json.contains("\"openurl\"") -> routeOpenUrl(json.substringAfter("\"url\":\"", "").substringBefore("\""))
                            json.contains("\"linksready\"") -> { linksInjected = true; log("win08: S2.4 link capture attached") }
                            // S2.5: the injected saveBlob (verbatim from Studio) ran, or the fetch/download threw.
                            json.contains("\"downloaddone\"") -> { downloadName = json.substringAfter("\"name\":\"", "").substringBefore("\""); downloadDone = true; log("win08: S2.5 saveBlob ran, name=$downloadName") }
                            json.contains("\"downloaderr\"") -> { downloadErr = json.substringAfter("\"msg\":\"", "").substringBefore("\""); log("win08: S2.5 download error: $downloadErr") }
                            json.contains("\"ready\"") -> {
                                readyCount++   // S2.1: proves the page loaded & handshook, even while the SwingPanel is collapsed/hidden
                                if (!bridgeReady) bridgeReady = true else helloAnswered = true // first ready latches bridgeReady; a later one = our hello answered
                                log("win08: ready (readyCount=$readyCount)")
                                onEdt(onReady)   // S2.1 (R1): EVERY ready reveals the studio — the first load AND a host reload/Retry (bridgeReady stays latched)
                            }
                        }
                        return null
                    }
                }
            }
            if (sabotage != "bridge" && sabotage != "bridge-late") {
                try {
                    registerBridge()
                    log("ctrl: TIMING createFunction returned ${System.currentTimeMillis() - tBrowser} ms after Browser()")
                } catch (e: Throwable) {
                    // The controller did not become ready within SWT's (raised) timeout — a real startup timeout, NOT
                    // "no WebView2". Its own words, Back offered, no IE fallback.
                    log("win08: createFunction failed — controller not ready in time: ${e.message}")
                    reportStartupTimeout(); runLoopUntilDisposed(display, shell); return
                }
            }

            // Origin lock: veto any navigation off the server's origin.
            if (sabotage != "nav") browser.addLocationListener(object : LocationAdapter() {
                override fun changing(event: LocationEvent) {
                    val to = event.location ?: return
                    if (to != "about:blank" && !sameOrigin(to, origin)) {
                        event.doit = false
                        navVetoed = true
                        log("win08: vetoed navigation to $to")
                    }
                }
            })

            // F11 forwarded to the app's full screen (S1.7); nothing else consumed, so Esc still reaches Studio.
            // S2.2: Alt+← walks Studio's own history (requestBack), like Android — consumed so WebView2 doesn't ALSO
            // navigate (a double-back); when there's nothing left it leaves the screen. Sabotage 'noaltback' skips it.
            browser.addKeyListener(object : KeyAdapter() {
                override fun keyPressed(e: KeyEvent) {
                    if (e.keyCode == SWT.F11) { e.doit = false; onEdt(onFullScreenKey) }
                    else if (sabotage != "noaltback" && e.keyCode == SWT.ARROW_LEFT && (e.stateMask and SWT.ALT) != 0) {
                        e.doit = false; log("win08: Alt+Left — Back"); requestBack()
                    }
                }
            })

            // S2.2 recon: does SWT's MouseListener on the Browser see the mouse back button (XBUTTON1 = button 4/5)?
            // If it does, the host can route it straight to requestBack and skip the page injection below.
            browser.addMouseListener(object : org.eclipse.swt.events.MouseAdapter() {
                override fun mouseDown(e: org.eclipse.swt.events.MouseEvent) {
                    if (e.button >= 4) { swtSawMouseBack = true; log("win08: SWT MouseListener saw button ${e.button} (mouse back/forward)") }  // recon: observe only
                }
            })

            // Item 7: new-window requests (target=_blank / window.open). S2.4 routes the URL at the page level (the
            // injection below), where the URL is known; this native listener is the BACKSTOP — it blocks the window
            // itself (required=true, no NewWindow) so none ever opens in-app, whether or not the injection caught it.
            browser.addOpenWindowListener(object : org.eclipse.swt.browser.OpenWindowListener {
                override fun open(event: org.eclipse.swt.browser.WindowEvent) {
                    newWindowRequested = true
                    // Block the popup via SWT's OWN path: required=true + browser left null → Edge.handleNewWindowRequested
                    // calls put_Handled(true) with NO NewWindow, so WebView2 opens nothing and window.open() returns null.
                    // (Leaving browser null WITHOUT required=true does NOT block on Edge — a full chrome window opens,
                    // sharing the WebView2 profile/session. Creating a throwaway Browser here can DEADLOCK — SWT warns.)
                    // Sabotage 'noblock' leaves required=false so the popup appears (the smoke check then reds).
                    event.required = (sabotage != "noblock")
                    log("win08: REPORT OpenWindowListener fired — new-window ${if (event.required) "BLOCKED (required=true, no NewWindow)" else "ALLOWED (sabotage=noblock)"}")
                }
            })

            // After the first load, send a hello over the native→web channel; a ready back proves both ways.
            var lateBridgeDone = false
            var probedOpen = false
            browser.addProgressListener(object : ProgressAdapter() {
                override fun completed(event: ProgressEvent) {
                    // S2.2 recon: track the URL + a navigation count, so the --win08-back recon can tell whether the
                    // mouse back walked exactly ONE entry (preventDefault held) or two (WebView2 double-navigated).
                    navCount++; lastUrl = runCatching { browser.url ?: "" }.getOrDefault("")
                    // B3 sabotage (b): register the bridge only AFTER the load — the page's head script has already
                    // run without window.TroubaStageShellReceive, so no ready is sent and the bridge check reds.
                    if (sabotage == "bridge-late" && !lateBridgeDone) { lateBridgeDone = true; registerBridge() }
                    runCatching { browser.execute("window.__troubastageShell&&window.__troubastageShell.deliver(JSON.stringify({type:'hello'}))") }
                    // S2.2: inject once a window-level mouseup listener for the back mouse button (event.button===3). It
                    // preventDefaults WebView2's own back-nav and forwards {type:"back"} to the host → requestBack (walk
                    // Studio history or leave at the root), like Alt+←. Sabotage nomouseback skips it (WebView2's own
                    // back then takes over and never leaves at the root). The recon measures whether preventDefault holds.
                    if (sabotage != "nomouseback") runCatching { browser.execute(
                        "if(!window.__win08back){window.__win08back=1;window.addEventListener('mouseup',function(e){if(e.button===3){" +
                        "e.preventDefault();var r=window.TroubaStageShellReceive;if(r){r(JSON.stringify({type:'mousebackseen'}));r(JSON.stringify({type:'back'}));}}},true);}"
                    ) }
                    // S2.4: capture new-window requests at the page level (SWT's OpenWindowListener has no URL). Override
                    // window.open and intercept a[href] clicks that WOULD open a new window — target!=_self, OR a middle-
                    // click (auxclick button 1), OR ctrl/meta/shift+click — resolve to absolute, preventDefault, and forward
                    // {openurl,url} to the host, which routes by origin + scheme. <a download> is left alone (WebView2's,
                    // S2.5). A plain same-origin left-click (no target/modifier) is NOT touched, so Studio's SPA nav is intact.
                    runCatching { browser.execute(
                        "if(!window.__win08links){window.__win08links=1;" +
                        "var S=function(u){try{var r=window.TroubaStageShellReceive;if(r)r(JSON.stringify({type:'openurl',url:new URL(u,location.href).href}));}catch(e){}};" +
                        "window.open=function(u){if(u)S(u);return null;};" +
                        "var A=function(e){var x=e.target&&e.target.closest?e.target.closest('a[href]'):null;return (x&&!x.hasAttribute('download'))?x:null;};" +
                        // left-click: route a new-window link — target!=_self, or ctrl/meta/shift (open-in-new-tab/window).
                        "var Hc=function(e){var a=A(e);if(!a)return;var t=a.getAttribute('target');if((t&&t!=='_self')||e.ctrlKey||e.metaKey||e.shiftKey){e.preventDefault();S(a.getAttribute('href'));}};" +
                        // auxclick: ONLY the middle button (1). NOT right (2) or back/forward (3/4) — those must not route a
                        // link (the mouse Back button is S2.2's; a right-click opens the context menu, S2.6).
                        "var Ha=function(e){if(e.button!==1)return;var a=A(e);if(!a)return;e.preventDefault();S(a.getAttribute('href'));};" +
                        "window.addEventListener('click',Hc,true);window.addEventListener('auxclick',Ha,true);" +
                        "var rr=window.TroubaStageShellReceive;if(rr)rr(JSON.stringify({type:'linksready'}));}"   // smoke waits for this
                    ) }
                    // S2.5 sabotage catchdownload: a listener that WRONGLY preventDefaults <a download> clicks (the thing
                    // the capture carefully excludes). saveBlob's temp <a download> click is then cancelled → WebView2
                    // never downloads → the download-lands check reds. Proves downloads must be left to WebView2.
                    if (sabotage == "catchdownload") runCatching { browser.execute(
                        "window.addEventListener('click',function(e){var a=e.target&&e.target.closest?e.target.closest('a[download]'):null;if(a)e.preventDefault();},true);"
                    ) }
                    // S2.1 (R1, refinement 2): the no-ready → cover timer is armed by the HOST load paths only (first
                    // load / ⋮ Reload / Retry), NOT here in completed(). An in-page same-origin navigation — e.g. S2.4
                    // opening a song's PDF in place — also completes with no Studio `ready`; arming here would land the
                    // cover over a perfectly good PDF after 10s. See armNoReadyTimer().
                    // SABOTAGE armoncompleted: re-create that old bug (arm on EVERY completed nav) → an in-page nav to a
                    // never-ready page then trips the cover, so the in-page-nav-no-cover check reds.
                    if (sabotage == "armoncompleted") armNoReadyTimer()
                    // S1.4 positive control (item 7): once, fire a window.open so the OpenWindowListener is exercised —
                    // it must log REPORT + suppress (no OS window). Dormant unless WIN08_PROBE_OPENWINDOW is set.
                    if (System.getenv("WIN08_PROBE_OPENWINDOW") == "1" && !probedOpen) {
                        probedOpen = true
                        log("win08: PROBE firing window.open('_blank') to exercise OpenWindowListener")
                        runCatching { browser.execute("window.open('https://example.com/win08-probe','_blank')") }
                    }
                    // (S2.5 replaced the dormant WIN08_PROBE_DOWNLOAD data: probe with --win08-download, which runs
                    // Studio's real saveBlob against a loopback /pdf and asserts the file lands in Downloads.)
                }
            })

            // S2.3: apply a pending cookie clear (persisted by a sign-out / server change while no Browser was live)
            // BEFORE seeding — clear-then-seed, so signing out and back in as someone else on the same server is clean.
            // Sabotage clearafterseed defers it to AFTER the seed (below): clearSessions() then wipes the fresh session,
            // which proves clear-BEFORE-seed is the load-bearing ordering the re-point's pending path relies on.
            val clearAfterSeed = System.getenv("WIN08_SABOTAGE") == "clearafterseed"
            if (!clearAfterSeed) applyPendingCookieClear()

            // Seed the session cookie AS THE SERVER SETS IT (item 5): Path=/ + HttpOnly (+ Secure on https), so
            // Studio's own scripts cannot read the session (document.cookie), while the request header still carries
            // it. SWT's setCookie parses these attributes via HttpCookie.parse.
            sessionCookie?.let { c ->
                val value = com.troubastack.app.sessionCookieSeed(c, origin) // A98: HttpOnly; SameSite=Lax (+Secure https)
                runCatching { Browser.setCookie(value, origin) }.onSuccess { ok -> log("win08: setCookie=$ok via sessionCookieSeed") }
            }
            if (clearAfterSeed) applyPendingCookieClear()   // WRONG order: wipes the just-seeded session → survival reds
            // S2.1 (R1): the host collapses the SwingPanel to 0.dp while Loading/the cover shows (the hole closes so
            // Compose draws over the Studio area), and restores it on `ready` (fitShellToCanvas refits on the resize).
            browser.setUrl(loadUrl)
            armNoReadyTimer()   // S2.1 (R1): the first load is host-initiated → cover if it doesn't ready in 10s
            log("ctrl: setUrl($loadUrl); entering readAndDispatch loop")

            runLoopUntilDisposed(display, shell)
        } catch (e: Throwable) {
            log("win08: SWT thread failed: ${e.message}")
            onEdt(onEngineMissing)
        } finally {
            runCatching { display?.takeIf { !it.isDisposed }?.dispose() }
        }
    }

    private fun runLoopUntilDisposed(display: Display, shell: Shell) {
        while (!shell.isDisposed) { if (!display.readAndDispatch()) display.sleep() }
    }
}

/** Scheme + host + port of [url], lowercased (default ports normalised), or "" when it cannot be parsed. */
internal fun originOfUrl(url: String): String = runCatching {
    val u = java.net.URI(url)
    val scheme = u.scheme?.lowercase() ?: return ""
    val host = u.host?.lowercase() ?: return ""
    val port = when {
        u.port != -1 -> u.port
        scheme == "http" -> 80
        scheme == "https" -> 443
        else -> -1
    }
    "$scheme://$host:$port"
}.getOrDefault("")

internal fun sameOrigin(url: String, origin: String): Boolean = originOfUrl(url) == origin
