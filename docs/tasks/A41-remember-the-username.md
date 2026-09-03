# A41 — Sign in should remember who you are

**Priority:** normal · **Size:** XS · **Area:** `app/androidApp` (`ConnectScreen.kt`). Lane: Mobile.
**Follows A38** (landed `cdfcab8`), whose wording this makes true.

VLL, 2026-08-23: *"do you think the app should remember the last username after a disconnection?"*
My answer: **yes.**

## Why — A38 already promises this and doesn't deliver it

A38's Disconnect deliberately keeps the server address so that signing back in **resumes** rather
than starts over; the KDoc on `clearSession` and a `SessionTest` comment both say *"Sign in needs only
a password"*. But `ConnectScreen.kt:76` is `var username by remember { mutableStateOf("") }` — the
username field is blank every time, so you retype your name as well. Two places in the source assert
something the code doesn't do; this task is the cheaper way to fix that than deleting the sentences.

The argument on its own merits:

- **The username is not a secret.** It is the same class of data as the server address we already
  persist. The password is the secret, and it stays unpersisted — that line does not move.
- **It's what makes the Guest state honest.** "Guest · the band" plus a **Sign in** button
  says *you are one tap from being back*. An empty name field contradicts the sentence beside it.
- The counter-argument — a shared or borrowed tablet — is weak: a prefilled field is editable in one
  tap. It is a convenience, not a lock.

## What to build

1. Persist the username on a **successful** connect only (not on a failed attempt), next to
   `CORE_URL_KEY`. A `lastUsername` key in the same Storage.
2. `ConnectContent` seeds the username field from it; the **password field is always empty**.
3. **Clear it when the server changes.** A username belongs to a server, and
   `dropSessionIfOriginChanged` already treats session-and-origin as a pair — the remembered username
   rides along with that same rule. Fold this into that function (which A38's review already flagged
   for consolidation onto `clearSession`; doing both at once is fine and preferred).
4. **Sign-out keeps it** — that's the entire point. `clearSession` must NOT clear it, and
   `SessionTest` must assert that, exactly as it already asserts `coreUrl` survives.

## Acceptance criteria

- `SessionTest` gains a case: after `clearSession`, the remembered username is **unchanged** (same
  shape as the existing `coreUrl` assertion). This is the regression guard.
- A pure test that a **failed** login does not persist the username, if the persist point is
  reachable from `commonTest`; if it is only reachable from the Android composable, say so at the
  gate and I will accept a structural argument naming the call site instead.
- The password field is never seeded — assert or argue it structurally.
- Changing the server URL clears the remembered username in the same step that drops the session.
- Device: Disconnect → the row reads Guest → **Sign in** → the username is prefilled, the password
  is empty, and signing in works with the password alone.
- `:shared:check` green; iOS klib if you touch anything shared.

## Out of scope

- Storing the password, "remember me", biometrics, any credential manager integration.
- Multiple remembered accounts / an account picker. One server, one last username. If we ever need
  more than that it will be because the multi-band work found a reason, and it can be specced then.
