package com.troubastack.desktop.studio

import com.sun.jna.Native
import com.sun.jna.Pointer
import com.sun.jna.platform.win32.User32
import com.sun.jna.platform.win32.WinDef.HWND
import com.sun.jna.platform.win32.WinUser.WNDENUMPROC
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
    private const val WS_CHILD = 0x40000000.toInt()
    private const val WS_POPUP = 0x80000000.toInt()
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

    /** Reparent [child] (a top-level SWT Shell's HWND) into [parent] (the AWT canvas): clear WS_POPUP, set WS_CHILD,
     *  then SetParent. The child stays owned by the SWT thread, so its message pump is unchanged. */
    fun embed(child: HWND, parent: HWND) {
        var style = user32.GetWindowLong(child, GWL_STYLE)
        style = (style and WS_POPUP.inv()) or WS_CHILD
        user32.SetWindowLong(child, GWL_STYLE, style)
        user32.SetParent(child, parent)
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
}
