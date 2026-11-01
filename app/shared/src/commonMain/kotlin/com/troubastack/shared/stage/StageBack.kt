package com.troubastack.shared.stage

/** A79 — what a system Back does on the Stage host. WIN03: [EXIT_FULL_SCREEN] for Esc on a desktop window. */
enum class StageBack { STAY, LEAVE, CLOSE_DRAWER, EXIT_FULL_SCREEN }

/**
 * A79 ⟨D1⟩ — while PERFORMING (a loaded concert with pages, note mode included) Back is consumed: the concert
 * stays open on the same page and ✕ is the only exit. On VLL's tablet the side-edge back gesture fires even in
 * immersive mode, so a page swipe that starts near the edge used to leave the concert mid-song. Note mode is
 * covered too — Back must never discard a note in progress; its ✓ is the way out. The failure and empty
 * screens have no ✕ and nothing to protect, so Back still leaves them. Dialogs and sheets handle Back
 * themselves, above this handler, so they keep closing on Back.
 *
 * WIN03 D4 — on a desktop window, Esc goes through this same decision: it closes the drawer if one is open,
 * otherwise it leaves [fullScreen], otherwise it does what Back does. It never leaves a concert; ✕ does. Android
 * never passes [fullScreen], so its Back is unchanged.
 */
fun stageBackAction(state: StageState, drawerOpen: Boolean = false, fullScreen: Boolean = false): StageBack = when {
    // A79 follow-up (Fable): the ☰ song drawer is an overlay like a sheet, so Back closes it — and only it.
    drawerOpen && state.failure == null && state.pages.isNotEmpty() -> StageBack.CLOSE_DRAWER
    fullScreen -> StageBack.EXIT_FULL_SCREEN
    state.failure == null && state.pages.isNotEmpty() -> StageBack.STAY
    else -> StageBack.LEAVE
}
