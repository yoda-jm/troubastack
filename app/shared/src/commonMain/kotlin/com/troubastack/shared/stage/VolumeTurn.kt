package com.troubastack.shared.stage

import androidx.compose.runtime.staticCompositionLocalOf

/**
 * A13 — bridge for hardware volume-key page turns.
 *
 * Android volume keys never reach Compose, so the Activity intercepts them and must call back into
 * the Stage. Rather than the entrypoint owning turn logic (which caused the two-up turn-by-1 defect
 * — the Activity called the VM's page±1 while every other input used the spread-aware turn),
 * [StageScreen] publishes its own spread-aware turn through this registrar and the entrypoint just
 * forwards the intercepted press. A CompositionLocal keeps this as pure Compose plumbing — no new
 * I15 seam. The value is a function called with a handler to register (or `null` to unregister on
 * Stage dispose). The default is a no-op, so a platform that never provides it — iOS, which has no
 * volume-key turn — simply does nothing.
 */
val LocalVolumeTurnRegistrar = staticCompositionLocalOf<(((PageTurn) -> Unit)?) -> Unit> { {} }
