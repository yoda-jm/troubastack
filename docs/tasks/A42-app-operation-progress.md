# A42 — Real progress in the app: a determinate update bar, and one-tap bake from Home

**Priority:** ① high (it retires the "Updating…" black box) · ② normal, and **blocked on T99 landing**
**Size:** ① M · ② M · **Area:** `app/` (Mobile lane). Requested by VLL via Mobile's relay (`ca38abe`).

Two halves. **They are separate deliverables and ① lands alone first** — it needs no server change and
no contract, so it must not be held hostage to ②.

---

## ① The InFlight state shows real download progress

Today `UpdateStatus.InFlight` is a `data object` rendered as a bare indeterminate spinner plus
"Updating…" (`HomeScreen.kt:303`). It says nothing for what can be a multi-minute transfer on venue
wifi.

**It is computable app-side with no server change**, which I verified rather than took on trust:
`downloadBundle` (`HttpTransport.kt:180`) already streams in a 64 KB `readAvailable` loop, and the
server serves the bundle via `http.ServeContent` (`bakeapi.go:264`) — which sets `Content-Length` for a
normal (non-Range) GET. Total and running bytes are both in hand at the loop.

### What to build

- `UpdateStatus.InFlight` becomes a **data class** carrying a phase and an optional fraction. It stays
  cancellable in every phase — bundles are large and this is venue wifi.
- **Downloading:** determinate bar from bytes-read ÷ `Content-Length`, with a human byte readout.
- **Installing:** `BundleImporter`'s unpack→validate→swap tail is coarse and genuinely not a
  percentage. Label it "Installing…" and **go back to indeterminate**. Do not manufacture a fraction
  for it.

### The rules that matter more than the bar

- **A missing or unparseable `Content-Length` ⇒ indeterminate, exactly as today.** A proxy or a future
  chunked response must degrade to a spinner. **Never synthesise a fake fraction** — a bar that
  invents its own progress is strictly worse than an honest spinner, because it destroys the one
  diagnostic this task is for.
- **Never show 100% before the swap has actually completed.** The bar hitting 100% and sitting there is
  the exact failure mode users read as "hung". Cap the download phase below full, and let the terminal
  state come from the install completing.
- Progress emission must be **pure and testable** — a fake transport emitting known byte counts drives
  the state machine; no device needed for the unit test.

### This is NOT a fix for the A39 stall

It makes the stall *legible* ("stuck at 40%" instead of a shrugging spinner), which is worth having
while the root cause is chased. **It must not be presented as, or mistaken for, the fix.** The A39
hang is a separate defect with its own root-cause work in flight, and closing it on the strength of a
nicer spinner would be exactly the wrong outcome.

### Acceptance

- Determinate bar advances against a known `Content-Length`; unit-tested off a fake transport.
- No `Content-Length` ⇒ indeterminate, no crash, no fabricated number — **its own test**.
- Cancel still works mid-download **and** mid-install.
- 100% is never displayed before the swap completes.
- `:shared:check` + APK; **add `:shared:compileKotlinIosSimulatorArm64`** (neither `check` nor the APK
  covers the iOS klib).
- Device demo: a real update showing the bar move. Per A39's lesson, **a state no human has executed
  does not get waived.**

---

## ② One-tap bake from Home

A39 punted this on four grounds: (a) admin-only, (b) no dirty/stale signal, (c) the re-bake race,
(d) slow and opaque. T99 dissolves (d). Here are rulings on (a)–(c) so this isn't re-litigated.

**Blocked until T99 lands** — it is T99's contract or nothing; do not fork a second progress mechanism.

**(a) Admin-only — enforced, and the affordance follows.** `bakeProgress` already requires
`RoleAdmin` (`bakeapi.go:139`) and 403s otherwise, so the server is not trusting the client. The app
must additionally *hide* the control for non-admins — a button that always fails is not an affordance.
If Home has no role signal today, plumbing one is part of this task.

**(b) There is no staleness signal, and you must not invent one app-side.** I checked: the baker
records `song.SourceRevision` at bake time (`baker.go:286`) and **nothing ever compares it**. So the
app cannot honestly badge "needs update". Ship ② as an **unconditional, explicitly-labelled re-bake
action** ("Re-bake concert"), never as a notification implying staleness was detected. A genuine
needs-bake signal is core-side work; it is not in this task's scope and must not be faked in its UI.

**(c) The race is already handled.** B08/B09 make concurrent bakes of the same setlist produce distinct
revs, and T99's `progressRegistry.claim` refuses an id held by a live entry, so two bakes cannot
clobber each other's readout. No new guard — but **prove it**: a test where an app bake and a studio
bake of the same setlist overlap, both succeed, and each reads only its own progress.

**Minting the id:** Kotlin `java.util.UUID.randomUUID()`. Note the studio's constraint does **not**
apply here — Home is native Compose, so the secure-context `crypto.randomUUID` trap that blocked T99
is a browser-only concern. It *would* apply to anything driven from the app's WebView host
(`WebViewHost.kt`), which is worth remembering but is not this surface.

### Acceptance

- Admin sees the action; a non-admin does not, and the server still refuses if asked.
- Live "Baking song N of M" on Home from T99's endpoint, with the same "Finishing…" rule for the
  `done == total`, no-song tail — a frozen "N of N" is the thing T99 exists to avoid.
- Progress unavailable (404/expired) ⇒ degrade to a plain "Baking…"; **the bake still completes**.
- No timer outlives the screen.
- Overlapping app+studio bake test per (c).
- Same build matrix as ①, plus a device demo of a real bake driven from Home.

## Out of scope

Any bake-staleness signal; deleting or identifying concert rows (that's the separate concert-row
identity + delete spec); changing the bake pipeline itself.
