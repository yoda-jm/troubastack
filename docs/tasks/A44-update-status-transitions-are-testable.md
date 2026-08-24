# A44 — The update-status transition belongs in shared, where it can be tested

**Priority:** normal — it prevents a recurrence of a bug that shipped twice · **Size:** S
**Area:** `app/` (Mobile lane). From the A39/A42① review (`review/a39-a42a`).

## 1. Why

The A42① deadlock — a successful update leaving the row on `InFlight("Installing…")` forever, because
the re-diff that would resolve it is guarded by `homeUpdate !is InFlight` — was **not** a transport bug
and **not** a hard bug to reason about. It was a state-machine mistake: *a terminal state must be set
before handing off to a guarded refresh.*

It went undetected by 410 passing tests for one structural reason: **the transition lives inline in a
Composable lambda in `MainActivity.kt`**, where no test can reach it. Only a device demo found it.

The lane asked (honestly) whether to add `ktor-client-mock` and an `androidApp` test source set. That is
the wrong lever for this particular bug, and the right lever is already in the codebase:
`inFlightStatus(p)` in `shared/home` is a **pure** mapping with pure tests
(`UpdateProgressTest`) that hold two real invariants. The terminal transition deserves the same
treatment.

## 2. What to build

Extract the update outcome → `UpdateStatus` decision out of `MainActivity`'s lambda into a **pure
function in shared**, beside `inFlightStatus`. It takes what the outcome actually is — which concerts
succeeded, which failed, whether a newer rev appeared — and returns the `UpdateStatus` to display. The
Composable then only *applies* the result.

Keep the behaviour exactly as landed; this is a move plus tests, not a redesign.

## 3. Acceptance criteria

- **A test that fails against the pre-fix behaviour**: "all succeeded ⇒ the returned status is terminal,
  never `InFlight`". That is the assertion that would have caught the deadlock, and it must be written
  so that reverting to `InFlight("Installing…")` reddens it.
- Partial failure still yields the failure status (the existing result-driven path, not an optimistic
  `UpToDate`).
- The transition is reachable from `commonTest` — no device, no Android instrumentation.
- `:shared:check` + `:androidApp:assembleDebug` + **`:shared:compileKotlinIosSimulatorArm64`**.

## 4. Explicitly NOT in scope

Adding `ktor-client-mock` or an `androidApp` unit-test source set **for the A39 timeout**. See the gate
ruling: a timeout *configuration* is not worth a mock harness that would mostly assert the configuration
back to itself. If a future change makes transport behaviour (retry, backoff, partial-read handling) our
logic rather than ktor's, that decision should be revisited on its own merits.
