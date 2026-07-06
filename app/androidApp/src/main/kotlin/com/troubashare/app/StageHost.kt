package com.troubashare.app

import android.app.Activity
import android.content.Context
import android.content.ContextWrapper
import android.view.WindowManager
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.ui.platform.LocalContext
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat

/**
 * Wraps the Stage screen with the two performance-time window behaviours, SCOPED to the screen's
 * lifecycle (entered on compose, restored on dispose — every exit path):
 *  - keep the screen on (FLAG_KEEP_SCREEN_ON) so the display never sleeps mid-performance,
 *  - immersive: hide the system bars, revealed transiently by an edge swipe.
 *
 * Boundary: this hides OUR chrome only. Silencing other apps' notifications is Do-Not-Disturb — the
 * user's responsibility (no DND permission, no notification listener). Both live here in androidApp,
 * not in a seam: they are one-liners on the Activity window (I15).
 */
@Composable
fun StageHost(content: @Composable () -> Unit) {
    val activity = LocalContext.current.findActivity()
    DisposableEffect(activity) {
        val window = activity?.window
        if (window != null) {
            window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
            WindowInsetsControllerCompat(window, window.decorView).apply {
                systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
                hide(WindowInsetsCompat.Type.systemBars())
            }
        }
        onDispose {
            if (window != null) {
                window.clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
                WindowInsetsControllerCompat(window, window.decorView).show(WindowInsetsCompat.Type.systemBars())
            }
        }
    }
    content()
}

/** Walk the ContextWrapper chain to the hosting Activity. */
internal fun Context.findActivity(): Activity? {
    var ctx: Context = this
    while (ctx is ContextWrapper) {
        if (ctx is Activity) return ctx
        ctx = ctx.baseContext
    }
    return null
}
