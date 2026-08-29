# A58 — Perform a concert offline, on the hardware it will be played on

**Lane:** Mobile · **Origin:** VLL, 2026-08-29 — *"spec the offline performance journey on real
hardware"* · **Verified against `b64d254`** · **Concert: 2026-09-05**

**Files:** device evidence (no code change required to pass), plus one regression fixture under
`app/shared/src/androidUnitTest/` and its test.

## Why this task exists

The flow check (`web/studio/flowcheck/flowcheck.spec.ts`) ends the moment a `.tstage` exists on
disk. **Nothing downstream of that artefact is verified by anything.** The bundle is the product's
whole thesis — I12: perform offline, account-less, on hardware you own — and the path from the
baked file to a musician reading a page has never been exercised end to end, on hardware, once.

Two facts I established before writing this, both worth knowing on their own:

1. **The tablet has never held a bundle.** `run-as com.troubashare.app ls files/bundles` on the
   device (`23073RPBFG`, Android 15, current-main APK installed 2026-08-29) returns an **empty
   directory**. Not stale, not damaged — empty. The download path has not run on this hardware.
2. **The demo server has nothing to download.** Its only setlist has a bake directory
   (`bakes/<setlistId>/`, created 2026-08-24) containing **no revision file at all**, where a
   successful bake writes `<rev>.tstage` there — as the flow-check run did (39,933 bytes). So the
   demo cannot currently serve a bundle to anything.

That second one is the gig-relevant fact. **Leg 0 below exists because of it.**

## What the code already promises — this is a verification task, not a repair task

I read the path before specifying the work, and it is deliberately built for this. Do not go
looking for a bug to fix; go looking for whether the promise holds on metal.

- **The Perform tile is never gated.** `HomeScreen.kt:437-441` — `Card(onClick = onPerform, …)` has
  **no `enabled =` parameter at all**, in deliberate contrast to the Studio tile at `:485-487`
  (`enabled = studioEnabled`). The comment at `:481-482` says why: *"TroubaStage above stays enabled
  always (offline perform is I12)."*
- **The local concert list is network-free.** `MainActivity.kt:799-810` `listConcerts` is a plain
  non-`suspend` `listFiles()` over `bundlesDir()` + `BundleLoader.load` per directory; a directory
  that will not load becomes a `damaged = true` entry rather than throwing.
- **The probe cannot block anything.** `MainActivity.kt:214-223` short-circuits with no cookie
  before any request; with a stale cookie `probePresence` runs `withTimeout(3000)` and
  `.getOrDefault(Presence.Unreachable)` inside a `LaunchedEffect` — nothing awaits it, and the
  offline status line is a reassurance, not an error (`HomeScreen.kt:117`: `" · concerts on device
  still work"`).
- **Loading is total by construction.** `BundleLoader` is documented as a total function (I12: *"one
  bad page must never take down a performance"*) — a missing or empty blob is a per-page `issue`, not
  a failure.

**So the expected outcome of this task is that it passes.** Say so in the submission if it does. A
green result here is not a null result: it is the first evidence that the central promise is true.

## The one method problem — solve it before you start

**`adb` reaches the tablet over wifi** (`192.168.2.64:5555`; the device's only route is via `wlan0`).
Disabling wifi to test offline **kills the harness that would observe the test.** A run that quietly
avoids this by leaving wifi up and merely stopping the server is *not* this task — that tests an
unreachable host, not an offline device, and it exercises none of the DNS/mDNS/no-route behaviour a
venue actually produces.

Pick one and **name it in the submission**:

- **(a) USB adb, then disable wifi.** The honest instrument. Note the known re-enumeration gotcha
  when the device sits at 100% battery and drops to charge-only.
- **(b) Airplane mode with deferred evidence.** Schedule the window on-device
  (`nohup sh -c 'svc wifi disable; sleep N; svc wifi enable' &`), perform the legs **by hand** during
  it, then reconnect and pull `logcat` (its ring buffer survives) plus the app's own state. The
  evidence is gathered *after*, which is fine — provided it is the *record*, not your memory of the
  screen.

**Do not inject input into the app without a fresh ask from VLL.** Legs that need taps are his or a
human's to perform; your job is to prepare them and to gather the artefacts afterwards.

## The legs

### Leg 0 — make a bundle exist on the demo (precondition, and a finding in its own right)

Bake the demo's setlist so `bakes/<setlistId>/<rev>.tstage` exists. **Report whether the bake
succeeds and why it had not before** — a setlist with items is bakeable under T124, so its empty
bake directory predating that fix is a loose end worth one sentence.

⚠️ **The demo's content is real band material. Nothing derived from it may be committed.** Names,
song titles, and setlist names from that server stay out of the repo, out of test fixtures, out of
file names, and out of pasted logs — refer to it generically. This applies to screenshots too.

### Leg 1 — download it to the tablet, online

Signed in, over wifi, via the Home Download/Update row. The download route is authed
(`GET /api/bands/{bandId}/concerts/{concertId}/bundle`, `bakeapi.go:39`) — so this leg *must* be
online, and it is the only leg that may be.

**Artefact:** `files/bundles/<concertId>/` exists with `bundle.json` and a populated `blobs/`;
report the byte count and the blob count, both read off the device.

### Leg 2 — go offline, cold-start, and perform

The real one. In this order, because the order is what makes it real:

1. Device offline by your chosen method — **verified offline**, not assumed (`airplane_mode_on`, or
   no route, evidenced).
2. **Force-stop the app.** Process death is the point: `me` is a plain
   `remember { mutableStateOf<CurrentIdentity?>(null) }` (`MainActivity.kt:195`), so a cold start
   offline has no last-known identity. A run that only backgrounds the app tests a warm cache and
   proves less than it appears to.
3. Launch, open TroubaStage, reach a rendered page, turn pages, toggle a layer.

**Watch for this specifically:** with `me` null and no *stored* pick, `resolveIdentity` returns `""`
(`IdentityPicker.kt:22-24`) and `needsIdentityPick` is true — so the **"Who are you?" picker should
appear on the first offline open** of a rostered concert. That is the designed fallback, not a
defect. Confirm it appears, confirm it works with no network, and confirm the pick persists so the
*second* cold start goes straight to the page. **If it does not appear, something is wrong** —
report that as the finding.

### Leg 3 — the awkward legs, which are the ones worth doing

- **A stale session while offline.** The cookie is 30 days (`webapi.go:176`), so this will not bite
  at the concert — but confirm an expired/invalid session still performs. The `Unauthorized` branch
  deliberately keeps `me` (`MainActivity.kt:351`); prove Perform is unaffected.
- **A damaged bundle.** Corrupt one blob inside an installed bundle, offline, and confirm the
  contract: that page degrades, **the performance does not**. This is the one leg where the total-
  function claim gets tested rather than read.
- **Airplane mode from launch, with no bundle at all.** Confirm the empty state says something true
  and actionable rather than failing silently — the "not dying without messages" property.

## Deliverable B — one durable guard, because a hand-run leg decays

Everything above is a one-shot. Add exactly one thing that keeps holding:

**A real server-baked bundle as a test fixture, asserted performable.** `FixtureBundleTest` today
loads fixtures produced by `core/cmd/mkbundle` — a *separate generator*. **Nothing anywhere asserts
that the bundle the real baker produces loads in the app.** That seam is exactly where a format
drift would land, and it would be silent until a musician found it.

- Use the **flow check's** artefact, not the demo's — its content is synthetic by construction
  (a generic band/song/setlist created by the harness), so it carries no band data. ~40 KB.
- Assert it loads with **zero issues**, and assert the things a performance needs: song count, page
  count, at least one overlay layer, and a non-empty roster.
- **Say in the submission whether the fixture differs structurally from the `mkbundle` ones.** If it
  does, that difference *is* the finding this guard exists to catch.

## Explicitly NOT in this task

**Instrumented / Compose UI tests.** There are none in the repo — no `androidTest` source set, no
`androidTestImplementation`, no test runner (verified: zero matching paths under `app/`). That gap is
the standing audit §5 row and it is real, but standing it up is its own task and **must not be
smuggled in here**. It follows that the Perform tile's always-on property cannot be pinned by a test
today: removing it would redden **zero** tests. **State that limitation in the submission rather than
inventing a tautological seam to have something to assert** — a pure function that returns "always
enabled" guards nothing, and I would reject it.

Also out: making Studio work offline · sync · iOS · changing the download UI · the scanner's idle
timeout · re-recording DEMO-VID.

## What counts as passing

**Artefacts, not gestures** — the rule T124 pushed into the server applies to a device report too.
"I opened the concert and it worked" is not evidence. Evidence is: the bundle directory listing with
sizes, the `logcat` span covering the offline window, the verified-offline state, and — for the
render — something that could only exist if a page actually decoded from a local blob. A screenshot
is admissible; a screenshot **of the hard case** (offline, cold-started, layer toggled) is what is
being asked for.

And if a leg cannot be run, **say which and why**. An unrun leg reported honestly is worth more than
a green one that quietly tested something easier.
