# A50 — Stage key focus: re-request it when an obscuring surface closes

**Lane:** Mobile · **Severity:** the audit's own words — *"the exact failure this product cannot have"*
(§4.3, "Pedal focus is requested exactly once") · **Verified against `05fd00e`**
**Files:** `app/shared/src/commonMain/kotlin/com/troubashare/shared/stage/StageScreen.kt` (+ a new pure
seam and its test)

## The defect

`StageScreen.kt:283`:

```kotlin
val keyFocus = remember { FocusRequester() }
LaunchedEffect(Unit) { runCatching { keyFocus.requestFocus() } }
```

Keyed on `Unit` — it runs **once per composition, ever**. Nothing re-requests focus. Every surface below
takes focus and, on dismissal, hands it back to nothing:

| Surface | Line | Gated by | In `overlayOpen`? |
|---|---|---|---|
| `ModalNavigationDrawer` (song drawer, A15) | `:288` | `drawerState.isOpen` | yes |
| Settings `ModalBottomSheet` | `:747` | `showSettings` | yes |
| `LayersDialog` (`AlertDialog`) | `:597` | `showLayers` | yes |
| `RoleDialog` — **contains an `OutlinedTextField` (`:1235`)** | `:598` | `showRole` | yes |
| **`WhoAreYouDialog`** | `:603` | `switchIdentity \|\| (needsIdentityPick(...) && !pickDismissed)` | **NO** |

Open the drawer to jump a song, close it — the pedal is dead for the rest of the set, silently. A
performer has no way to tell except that pressing it does nothing.

**The omitted row is the sharp one.** `overlayOpen` (`:243`) is
`drawerState.isOpen || showSettings || showLayers || showRole` — it does **not** include the identity
pick, which is reached mid-set via Settings → "Switch": exactly what a player does when they realise
they're reading the wrong part. So the one surface the existing predicate misses is one of the more
likely ones to be used during a performance.

**Existing coverage:** `StageKeysTest` (3 tests) covers the key *mapping* — which key means which turn.
Nothing covers focus. There are zero Compose UI tests.

## Deliverable

### 1. Pure seam + test (the A47/A48/A49 pattern — 3 for 3)

Extract "should the Stage hold key focus?" as a **named pure function** taking the surface flags and
returning a `Boolean`, and unit-test it. Shape it as reads best; the point is that the *policy* — which
surfaces count as focus-stealing — becomes testable off-device, exactly as `decodeStagePosition` and
`cacheThrough` did.

The test must cover **each** surface individually and the all-clear case, and must include the identity
pick in both of its forms (`switchIdentity`, and `needsIdentityPick && !pickDismissed`).

### 2. Wire the focus request to it

Key the focus effect on the predicate instead of `Unit`, so focus is re-requested each time the Stage
becomes unobscured. **Keep `runCatching`** — `requestFocus()` throws if the `FocusRequester` is not
attached, and that must stay non-fatal.

### 3. Teeth-check

Named mutation: **drop the identity-pick term from the predicate.** A named test must redden. **Report
the reddened count**, not just "it fails".

## Decisions — PINNED, do not re-litigate

1. **A separate predicate. Do NOT reuse `overlayOpen`, and do NOT widen it.** One predicate with two
   consumers is what caused this: for chrome auto-hide an omission is cosmetic (the chrome hides a beat
   early); for key focus the same omission is a dead pedal. Widening `overlayOpen` would also change
   chrome behaviour as a side effect of a safety fix. Keep them separate even though they will overlap
   almost entirely — the duplication is the point, because the two lists are allowed to diverge and one
   of them is safety-critical.
2. **No Compose UI test rig.** Zero exist; standing one up eight days before the concert is the wrong
   trade. The pure seam is how this gets tested.
3. **No change to key mapping.** `StageKeys.kt` and `StageKeysTest`'s 3 tests stay untouched.
4. **No change to `overlayOpen`, chrome auto-hide, or the dismissal logic** of any dialog.

## The limit of this guard — stated up front, not discovered later

The pure test guards the **policy** (which surfaces count). It does **not** guard the **wiring** — if
someone later re-keys the effect back to `LaunchedEffect(Unit)`, the suite stays green and the pedal
dies again. That is the same class of blind spot A49 shipped with (its arbiter covers `cacheThrough`,
not the call sites), for the same reason: no portable way to assert Compose effect wiring without a UI
test rig.

**This is accepted, deliberately.** Do not attempt to close it — say in the submission that you know it
is open. Recording a known limit beats pretending to a guard we don't have.

## Because the wiring can't be unit-tested: verify it on the device

State plainly in the submission whether you exercised it on the real device with the real pedal:
open the drawer → close it → confirm the pedal still turns pages; then the same for the identity pick
(Settings → Switch → pick/dismiss). **If you could not test on hardware, say so** — an untested claim
here is worse than an absent one, and this is the one part a green suite cannot speak to.

## Gate

`:shared:testDebugUnitTest` + `:androidApp:test` green — **read the results XML, never the exit code**
(264 executions on `05fd00e`; state the new total and reconcile it). Cite the new test by name, the
teeth-check's reddened count, and the device result or its absence.
