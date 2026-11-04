package com.troubastack.desktop.studio

import com.sun.jna.Native
import com.sun.jna.Pointer
import com.sun.jna.platform.win32.User32
import com.sun.jna.platform.win32.WinDef.HWND
import com.sun.jna.platform.win32.WinDef.RECT
import com.sun.jna.platform.win32.WinUser.WNDENUMPROC
import com.sun.jna.ptr.IntByReference
import com.sun.jna.win32.StdCallLibrary
import java.awt.Component
import java.awt.Window

/**
 * WIN08 — the small Win32 layer behind the child-HWND embedding (chosen after SWT_AWT's embedded Shell left the
 * WebView2 controller unready — its ready callback is not pumped on the SWT thread, so the first Edge operation
 * times out). A top-level, SWT-thread-owned Shell readies normally (WIN05, ~80 ms); [embed] then reparents its
 * HWND into the heavyweight AWT canvas so it draws inside the TroubaStage window while SWT keeps owning its pump.
 */
internal object Win08Native {
    private const val GWL_STYLE = -16
    private const val WS_CHILD = 0x40000000     // fits in Int
    private const val WS_POPUP = Int.MIN_VALUE  // 0x80000000
    private const val GA_ROOT = 2

    private val user32: User32 get() = User32.INSTANCE

    /** The native window handle of a realized heavyweight AWT [component], or null. */
    fun hwndOf(component: Component): HWND? =
        runCatching { Native.getComponentPointer(component)?.let { HWND(it) } }.getOrNull()

    /** The native window handle of a realized top-level AWT [window], or null. */
    fun hwndOf(window: Window): HWND? =
        runCatching { Native.getWindowPointer(window)?.let { HWND(it) } }.getOrNull()

    /** Wrap an SWT `Control.handle` (a win32 HWND as a long). */
    fun hwnd(handle: Long): HWND = HWND(Pointer.createConstant(handle))

    private const val SWP_NOZORDER = 0x0004
    private const val SWP_NOACTIVATE = 0x0010
    private const val WM_CLOSE = 0x0010

    /** Reparent [child] (a top-level SWT Shell's HWND) into [parent] (the AWT canvas): clear WS_POPUP, set WS_CHILD,
     *  then SetParent. The child stays owned by the SWT thread, so its message pump is unchanged. */
    fun embed(child: HWND, parent: HWND) {
        var style = user32.GetWindowLong(child, GWL_STYLE)
        style = (style and WS_POPUP.inv()) or WS_CHILD
        user32.SetWindowLong(child, GWL_STYLE, style)
        user32.SetParent(child, parent)
    }

    /** Size [child] to fill [parent] in PHYSICAL pixels (item 4): the parent's own client rect → SetWindowPos. */
    fun fitChildToParent(child: HWND, parent: HWND) {
        val (w, h) = clientSize(parent)
        if (w > 0 && h > 0) user32.SetWindowPos(child, null, 0, 0, w, h, SWP_NOZORDER or SWP_NOACTIVATE)
    }

    /** Close a window the way a person does (item 2): post WM_CLOSE to [frame] so the app's onCloseRequest runs. */
    fun postClose(frame: HWND) {
        user32.PostMessage(frame, WM_CLOSE, com.sun.jna.platform.win32.WinDef.WPARAM(0L), com.sun.jna.platform.win32.WinDef.LPARAM(0L))
    }

    /**
     * WIN08 S1.8 check 2 — the embedded browser's top-level window is the TroubaStage frame: find a WebView2 host
     * window (class "Chrome_WidgetWin…") among the frame's descendants and confirm its GA_ROOT ancestor is the frame.
     */
    fun frameOwnsWebView(frame: Window): Pair<Boolean, String> {
        val frameHwnd = hwndOf(frame) ?: return false to "no frame HWND"
        var webview: HWND? = null
        user32.EnumChildWindows(frameHwnd, WNDENUMPROC { hWnd, _ ->
            val buf = CharArray(256)
            val n = user32.GetClassName(hWnd, buf, buf.size)
            val cls = String(buf, 0, maxOf(0, n))
            if (cls.contains("Chrome_WidgetWin")) { webview = hWnd; false } else true
        }, Pointer.NULL)
        val wv = webview ?: return false to "no WebView2 child window under the frame"
        val root = user32.GetAncestor(wv, GA_ROOT)
        return (root != null && root == frameHwnd) to "webview=$wv root=$root frame=$frameHwnd"
    }

    // --- B2: DPI awareness + client rects. Awareness: 0 unaware, 1 system, 2 per-monitor, -1 unavailable. ---

    private interface User32Dpi : StdCallLibrary {
        fun GetThreadDpiAwarenessContext(): Pointer?
        fun GetAwarenessFromDpiAwarenessContext(ctx: Pointer?): Int
        companion object { val I: User32Dpi by lazy { Native.load("user32", User32Dpi::class.java) } }
    }

    private interface Shcore : StdCallLibrary {
        fun GetProcessDpiAwareness(hProcess: Pointer?, value: IntByReference): Int
        companion object { val I: Shcore by lazy { Native.load("shcore", Shcore::class.java) } }
    }

    /** The calling thread's DPI-awareness value (call on the thread you want to measure). */
    fun threadDpiAwareness(): Int =
        runCatching { User32Dpi.I.GetAwarenessFromDpiAwarenessContext(User32Dpi.I.GetThreadDpiAwarenessContext()) }.getOrDefault(-1)

    /** The process's DPI-awareness value. */
    fun processDpiAwareness(): Int =
        runCatching { val r = IntByReference(); Shcore.I.GetProcessDpiAwareness(null, r); r.value }.getOrDefault(-1)

    /** A window's client size in physical pixels. */
    fun clientSize(h: HWND): Pair<Int, Int> =
        runCatching { val r = RECT(); user32.GetClientRect(h, r); (r.right - r.left) to (r.bottom - r.top) }.getOrDefault(-1 to -1)

    /**
     * WIN08 item 7 — count VISIBLE TOP-LEVEL WebView2 host windows (class "Chrome_WidgetWin…"). The embedded browser
     * is reparented WS_CHILD so it is NOT enumerated by EnumWindows (top-level only); a value > 0 therefore means a
     * window.open/target=_blank popup opened its own chrome window. Blocked correctly, this is 0.
     */
    fun topLevelWebViewWindows(): Int = runCatching {
        var count = 0
        user32.EnumWindows(WNDENUMPROC { h, _ ->
            if (user32.IsWindowVisible(h)) {
                val buf = CharArray(256); val n = user32.GetClassName(h, buf, buf.size)
                if (String(buf, 0, maxOf(0, n)).contains("Chrome_WidgetWin")) count++
            }
            true
        }, Pointer.NULL)
        count
    }.getOrDefault(-1)

    private const val GWL_EXSTYLE = -20
    private const val WS_EX_TOPMOST = 0x00000008

    /** Item 8 — is the OS foreground window the given [frame]? With the ⋮ menu open this must hold (the menu, a
     *  non-activating JPopupMenu, must not take activation from the frame). */
    fun foregroundIsFrame(frame: Window): Boolean = runCatching {
        val fg = user32.GetForegroundWindow(); val fh = hwndOf(frame)
        fg != null && fh != null && fg == fh
    }.getOrDefault(false)

    /** Item 8 — does any VISIBLE window of THIS process carry WS_EX_TOPMOST? The ⋮ menu must not be topmost. */
    fun anyProcessWindowTopmost(): Boolean = runCatching {
        val myPid = com.sun.jna.platform.win32.Kernel32.INSTANCE.GetCurrentProcessId()
        var topmost = false
        user32.EnumWindows(WNDENUMPROC { h, _ ->
            if (user32.IsWindowVisible(h)) {
                val pid = IntByReference(); user32.GetWindowThreadProcessId(h, pid)
                if (pid.value == myPid && (user32.GetWindowLong(h, GWL_EXSTYLE) and WS_EX_TOPMOST) != 0) topmost = true
            }
            true
        }, Pointer.NULL)
        topmost
    }.getOrDefault(true)

    private const val WS_CAPTION = 0x00C00000
    private const val MONITOR_DEFAULTTONEAREST = 0x00000002

    /**
     * WIN08 F11 diagnosis — the window's REAL state (not Compose's isFullScreen flag): the full window rect, whether
     * the style still carries WS_CAPTION (a title bar → not full screen), and the monitor's whole rect to compare.
     */
    fun frameState(window: Window): String = runCatching {
        val h = hwndOf(window) ?: return "no frame HWND"
        val wr = RECT(); user32.GetWindowRect(h, wr)
        val style = user32.GetWindowLong(h, GWL_STYLE)
        val hasCaption = (style and WS_CAPTION) == WS_CAPTION
        val mon = user32.MonitorFromWindow(h, MONITOR_DEFAULTTONEAREST)
        val mi = com.sun.jna.platform.win32.WinUser.MONITORINFO(); user32.GetMonitorInfo(mon, mi); val m = mi.rcMonitor
        "winRect=[${wr.left},${wr.top},${wr.right},${wr.bottom}] wsCaption=$hasCaption style=0x${Integer.toHexString(style)} monitor=[${m.left},${m.top},${m.right},${m.bottom}]"
    }.getOrElse { "frameState error: $it" }
}
