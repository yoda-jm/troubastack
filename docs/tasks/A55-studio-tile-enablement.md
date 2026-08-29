# A55 — TroubaStudio must look disabled when it is disabled

**Lane:** Mobile · **Origin:** VLL, testing live — *"TroubaStudio should be greyed with a reason (no data
connection or not logged in … it should be clear it is disabled and cannot click on it)"* ·
**Verified against `3386b58`**
**Files:** `app/shared/src/commonMain/kotlin/com/troubashare/shared/home/HomeScreen.kt` + a pure seam and its test.

## The defect

`HomeScreen.kt:422` wires the Studio tile as `onClick = onStudio`, **unconditionally**. Tapping it while
signed out, unconfigured, or offline lands on a manage screen that cannot reach the server — a dead end
reached by a control that looked live.

## Deliverable

### 1. A pure `studioEnablement(identity)` seam

The Home `Identity` is already computed for the status line; derive the tile's state from it rather than
introducing a second source of truth. Put the function next to the existing `identityLine`-style helpers in
`home/` so it is unit-testable off-device, and shape the result so the reason travels with the state
(`Enabled` vs `Disabled(reason)`) — the caption must come from the same decision that disables the tile,
not from a parallel `when`.

| identity | tile | caption |
|---|---|---|
| `Connected` | enabled | — |
| `SignedOut` / `NotSetUp` | disabled | "Sign in to manage concerts" |
| `Offline` | disabled | "No connection" |
| `Checking` | disabled | **neutral** — see below |

### 2. Disabled means *not clickable*, not merely grey

VLL said *"cannot click on it"*. A `Button(enabled = false)` is the contract; do not settle for an alpha
change on a still-clickable surface.

### 3. `Checking` must not flash a wrong reason

The probe runs on every resume. A tile that reads *"Sign in to manage concerts"* for a moment on every
return to Home, then enables itself, is worse than one that is briefly and quietly unavailable. Keep it
disabled during `Checking` with a neutral caption (or none) — and say in the submission what you chose.

### 4. TroubaStage stays enabled, always

Performing is offline-capable by design (I12). This task is **Studio-only**; a change that greys the Stage
tile is a regression, not a fix.

## Tests

Every row of the table, by name. The `Checking` row matters as much as the others — it is the one a person
sees most often.

## Teeth-check

Make `studioEnablement` return `Enabled` for `SignedOut`. A named test must redden. Report the count.

## Out of scope

Making Studio work offline · changing what Studio does once entered · the Concerts list inside it.
