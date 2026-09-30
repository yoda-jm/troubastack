package com.troubastack.shared.stage

/** A79 — what a system Back does on the Stage host. */
enum class StageBack { STAY, LEAVE }

/**
 * A79 ⟨D1⟩ — while PERFORMING (a loaded concert with pages, note mode included) Back is consumed: the concert
 * stays open on the same page and ✕ is the only exit. On VLL's tablet the side-edge back gesture fires even in
 * immersive mode, so a page swipe that starts near the edge used to leave the concert mid-song. Note mode is
 * covered too — Back must never discard a note in progress; its ✓ is the way out. The failure and empty
 * screens have no ✕ and nothing to protect, so Back still leaves them. Dialogs and sheets handle Back
 * themselves, above this handler, so they keep closing on Back.
 */
fun stageBackAction(state: StageState): StageBack =
    if (state.failure == null && state.pages.isNotEmpty()) StageBack.STAY else StageBack.LEAVE
