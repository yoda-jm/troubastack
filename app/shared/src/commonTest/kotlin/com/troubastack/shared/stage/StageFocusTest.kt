package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * A50 — the Stage key/pedal-focus POLICY: it holds focus only when no focus-stealing surface is open, so
 * the focus effect can RE-request focus each time the Stage becomes unobscured. Each surface must release
 * focus individually, and the identity pick must count in BOTH forms — it is the one the chrome predicate
 * (`overlayOpen`) misses, and it's reached mid-set via Settings→Switch.
 *
 * Guards the POLICY, not the WIRING: re-keying the effect to `LaunchedEffect(Unit)` leaves this green —
 * an accepted, stated blind spot (same class as A49's), verified on the device instead.
 */
class StageFocusTest {

    private fun holds(
        drawerOpen: Boolean = false,
        showSettings: Boolean = false,
        showLayers: Boolean = false,
        showRole: Boolean = false,
        switchIdentity: Boolean = false,
        needsIdentityPick: Boolean = false,
        pickDismissed: Boolean = false,
    ) = stageHoldsKeyFocus(drawerOpen, showSettings, showLayers, showRole, switchIdentity, needsIdentityPick, pickDismissed)

    @Test fun allClear_holdsFocus() = assertTrue(holds(), "nothing open ⇒ the Stage holds the pedal")

    @Test fun drawerOpen_releasesFocus() = assertFalse(holds(drawerOpen = true))

    @Test fun settingsSheet_releasesFocus() = assertFalse(holds(showSettings = true))

    @Test fun layersDialog_releasesFocus() = assertFalse(holds(showLayers = true))

    @Test fun roleDialog_releasesFocus() = assertFalse(holds(showRole = true)) // an OutlinedTextField — certain theft

    @Test
    fun identityPick_viaSwitch_releasesFocus() =
        // Settings → "Switch" — the mid-set case overlayOpen misses. Teeth: dropping the identity-pick term reddens this.
        assertFalse(holds(switchIdentity = true))

    @Test
    fun identityPick_viaUnresolvedRoster_releasesFocus() =
        // The picker's other form: an unresolved roster, not yet dismissed. Teeth: same term drop reddens this.
        assertFalse(holds(needsIdentityPick = true, pickDismissed = false))

    @Test
    fun identityPick_dismissed_holdsFocus() =
        // Dismissed ⇒ the picker is not showing ⇒ focus returns (the `&& !pickDismissed` half).
        assertTrue(holds(needsIdentityPick = true, pickDismissed = true))
}
