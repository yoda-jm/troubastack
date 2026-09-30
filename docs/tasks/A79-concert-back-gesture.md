# A79 — In concert mode, Back never leaves the concert

**Lane:** mobile · **Status:** specced, not started · **Origin:** VLL, 2026-09-30: *"while in concert mode
the back gesture should be disabled if possible, I already exited several times a concert while wanting to
swipe left (next page/song)"*.

## 1. Cause

In `MainActivity.kt`, the Stage host registers `BackHandler { selectedDir = null }`. That makes **any**
system Back leave the concert at once, including an edge swipe meant as a page turn. Stage runs in immersive
mode (`StageHost`: bars hidden, `BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE`), but on VLL's tablet the side-edge
back gesture still fires. A page swipe that starts near the edge is therefore read as Back.

## 2. ⟨D1⟩ Back does nothing while performing; ✕ is the way out

While the performing pager is shown, **consume** system Back: the concert stays open, on the same page.

- **Leaving is ✕ only.** The red ✕ in the Stage top bar already calls `onExit()`. That is the deliberate
  exit, and it stays exactly as it is.
- **The gesture gives no feedback**: no toast, no snackbar. A message on the page in the middle of a song is
  worse than the lost swipe.
- **Dialogs and sheets that close on Back keep doing so** (the ⚙ sheet, confirmations, menus). They handle
  Back themselves, above the Stage handler. Only the step "leave the concert" is disabled.
- **Note mode:** Back does nothing there either. Leaving note mode is its ✓ button. Back must not discard a
  note that is still open.
- **Screens that are not performing keep Back as today:** the failure screen and the empty state have a
  "Back" button, and system Back still leaves them. There is nothing to protect there, and they have no ✕.

VLL said *"if possible"*. It is possible: the app's own handler runs before the system's default
"leave". It does not depend on the gesture-exclusion API.

## 3. Not required, and not to be done blind

**Turning that edge swipe into a page turn** is out of scope. Android lets an app exclude only a limited
height of each edge from the back gesture (`setSystemGestureExclusionRects`, capped by the system,
nominally 200 dp per edge), and MIUI may differ. Consuming Back (⟨D1⟩) fixes the actual harm, leaving the
concert, on every platform. If the lost swipe itself still annoys VLL, that is a follow-up, and it starts
with **measuring on his tablet**, not with a guess.

## 4. Acceptance

- In performing mode, a system Back leaves the concert open **on the same page**. Assert the page index, not
  only that the screen stayed.
- ✕ still leaves the concert.
- With the ⚙ sheet open, Back closes the sheet and **not** the concert, then a second Back does nothing.
- In note mode, Back does nothing, and the note in progress is still there.
- On the failure screen, Back still leaves.
- **Device pass on VLL's tablet:** a side-edge swipe during a concert, several times, from both edges. The
  concert is never left. **Ask VLL before using adb.**
