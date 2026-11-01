package com.troubastack.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberUpdatedState

/**
 * WIN03 — the desktop's system Back: Esc, Alt+Left and the mouse Back button (outside Stage) all dispatch here. It keeps
 * Android's rule, which A79 and every screen's Back rely on: **the handler composed last wins**, and disposing it hands
 * Back to the one before. Pure, so the rule is unit-tested (the Compose side only registers and disposes).
 */
class BackDispatcher {
    private val handlers = ArrayList<Registration>()

    /** One registered handler; [dispose] removes it, whatever its position. */
    inner class Registration internal constructor(internal val onBack: () -> Unit) {
        fun dispose() { handlers.remove(this) }
    }

    fun register(onBack: () -> Unit): Registration = Registration(onBack).also { handlers += it }

    /** Run the newest handler. False when none is registered (the window then does nothing). */
    fun dispatch(): Boolean {
        val newest = handlers.lastOrNull() ?: return false
        newest.onBack()
        return true
    }

    /** The AppPlatform.BackHandler shape: register while composed, always calling the latest [onBack]. */
    @Composable
    fun Handler(onBack: () -> Unit) {
        val current by rememberUpdatedState(onBack)
        DisposableEffect(this) {
            val registration = register { current() }
            onDispose { registration.dispose() }
        }
    }
}
