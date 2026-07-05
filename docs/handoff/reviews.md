# Review-gate log — Architect/Reviewer verdicts

When the human relay is offline, verdicts land here instead of chat. Executing agents:
treat an entry here exactly like a pasted review. Keep working your lane's queue per the
standing steer at the bottom; **hold every finished task at the review gate** — a fresh
architect session (bootstrapped from [`architect-reviewer.md`](architect-reviewer.md))
reviews and appends verdicts.

---

## 2026-07-04 — T16 (seed PDF em-dash): ✅ APPROVED, closed

Re-verified independently, not from the report: `go test ./cmd/seed -count=1` green;
wiped the gitignored `core/cmd/seed/assets/` cache, reseeded a fresh in-mem server, and
`pdftotext` on the regenerated `wonderwall-vocals.pdf` extracts a correct
"Wonderwall — Vocals". The fix is the right one (cp1252 translator on every drawn
string + the info-dict UTF-8 flag), the new encoding test guards the exact mapping, and
both honest notes were correct calls: the committed demo bundle keeps its mojibake until
B02 regenerates it (per the task), and the assets-cache gotcha is real — the effective
reseed is `rm -rf core/troubadata core/cmd/seed/assets && make demo`.

## 2026-07-04 — IOS01 feasibility checkpoint: ✅ GO — carry to a PR

The spike answered the gating question the strong way (Linux cross-compiles iOS klibs;
local verification is possible) and already paid for itself by catching a real
portability bug in shipped A04 code (`LinkedHashMap(16, 0.75f, true)` is JVM-only).
Proceed with full IOS01, with these conditions:

1. **Land the LRU portability fix first as its own small commit/PR** — it's a bug in
   shipped code, independent of iOS, and deserves a standalone review entry.
2. The zip design is approved as proposed: commonMain **internal** parser
   (EOCD → central dir → local headers) with portable zip-slip + size-cap guards, and a
   tiny `expect fun rawInflate` (JVM `Inflater(nowrap)` / iOS `platform.zlib`
   `inflateInit2(-15)`); JVM-runnable tests crafted with `ZipOutputStream`.
3. If it balloons, the T10 precedent applies: **two commits max** (Storage + zip reader
   first; WebViewHost second), each individually green and held at the gate.
4. `InkOverlay` stays `TODO` (A07-blocked); I15 gates apply verbatim — platform code
   only in the seam actuals; the parser lives in commonMain as spec'd.

## 2026-07-04 — LRU portability fix (`fix/stage-lru-portability`, a8049fd): ✅ APPROVED — land it

Re-verified independently in the `troubastack-lru` worktree, not from the commit message:
fresh `:shared:check --rerun-tasks` green; the diff is the textbook access-order
emulation (remove+re-insert on `get`, remove-before-put, evict `keys.first()`), and I
additionally proved the semantics with a scratch commonTest (evicts least-recently-*used*
not least-recently-inserted; re-put counts as access; miss doesn't perturb order — 3/3
pass on debug+release unit targets, scratch file removed after). A sweep confirms no
other access-order `LinkedHashMap(cap, load, true)` constructors remain anywhere in
commonMain. Mobile agent: land by the usual rebase + fast-forward, then proceed with
IOS01 on top per the GO conditions.

One note, not a blocker: `PageImageCache` still has no committed unit test. Fine for
this fix (it must stay minimal), but IOS01's review will look kindly on one arriving
with the Storage/zip commit if it's cheap to add.

## 2026-07-04 — IOS01 (PR #10, 5d68f92): ✅ APPROVED with one small pre-land condition

Re-verified independently in the `troubastack-IOS01` worktree, not from the report: fresh
`:shared:check :shared:compileKotlinIosArm64 :shared:compileKotlinIosSimulatorArm64
--rerun-tasks` — all green on this Linux host in 43s (72 tasks); `ZipArchiveTest` 4/4 on
both unit-test variants; CI 5/5 green on the head SHA (queried from the API, including
the new klib cross-compile step in the android job); I15 intact (exactly three iosMain
seam files, zero `platform.*` imports outside iosMain, `InkOverlay` still all-TODO). The
GO conditions hold: LRU landed first as PR #9 (approved above), one commit is within the
two-commit budget, and the zip design is exactly as approved — internal commonMain
parser, `expect rawInflate` with `Inflater(nowrap)` / `inflateInit2(-15)`. The
`WKUserScript` shim that lets one `bridge.ts` serve both shells is a genuinely good call.

**The condition:** the acceptance criteria list the **size cap** among the JVM-runnable
zip tests, and the GO said "portable zip-slip **+ size-cap** guards". The zip-slip guard
is portable and tested; the size-cap decision is three lines living only in the iOS
`unpackBundle` — untestable off-device, and the deviation wasn't flagged. Close it the
cheap way before landing: hoist the per-entry cap decision into `ZipReader.kt` (e.g.
`internal fun exceedsSizeCap(written: Long, uncompressedSize: Int, cap: Long): Boolean`
covering the negative-overflow case), call it from the iOS actual, add a JVM test case.
If you believe that's wrong, the T05 route applies — record the deviation in the task
file with rationale instead. Either way, don't drop a listed criterion silently.

Non-blocking notes (carry to IOS02, no action now):
1. `NSData.dataWithContentsOfFile` reads the whole file *before* the 512 MB check — a
   multi-GB picked file is fully loaded then rejected (jetsam risk). A
   `NSFileManager.attributesOfItemAtPath` size check first would be cheap.
2. `WKUserContentController → messageHandler → WebViewHost → view` is a strong cycle
   across the ObjC/Kotlin-Native boundary that the K/N GC cannot collect. Fine for one
   app-lifetime host; IOS02's glue must `removeScriptMessageHandler` on teardown if
   hosts are ever recreated.
3. `rawInflate` (iOS) accepts `Z_OK` with exactly-filled output, silently truncating a
   stream longer than its declared size — contained by the cumulative cap, just lenient.

## 2026-07-04 — B01 (`00a7da5`, landed): ✅ APPROVED post-hoc — excellent work, one process note

B01 landed on `main` without a verdict here. The review happened anyway, independently,
in the review worktree — and everything holds:

- **Parity re-measured, not read off the report:** fresh `npm run build && npm test` —
  L1 99.872% / L2(text) 99.715% within Δ≤3; transparency agreement 99.9993% / 99.9759%
  with worst opaque-side α=129 (genuine AA edges, no misplaced content). Matches the
  commit's claims. The `MAX_DISAGREE_ALPHA` guard is a smart formalization.
- **CLI on bare Node 24:** end-to-end run — stdout exactly 0 bytes, stderr logging,
  transparent PNGs (92.3%/97.3% transparent pixels), z-ordered `index.json`.
- **The Kotlin loader really does accept bake's output:** I assembled a bundle dir from
  bake overlays + a dummy raster and ran the REAL `BundleLoader` against it via a scratch
  JVM test — `Loaded`, zero issues, z-order preserved (scratch removed after). This is
  stronger than the in-repo `bundle-crosscheck.test.mjs` (an honest JS mirror of the
  loader contract) and closes the criterion's literal reading.
- CI 5/5 green on `00a7da5`. Deviations accepted: the transparency-clause relaxation is
  correct (the spec's own Δ≤3 tolerance concedes the same AA reality); `setTextFontFamily`
  with unchanged default keeps studio byte-identical; the OffscreenCanvas shim keeping ink
  on the browser composite path is exactly the right kind of I8 paranoia.
- **One real gap, recorded in the task file:** the acceptance command
  `cd web/bake && npm ci --no-workspaces && npm run build` fails on a *fresh* checkout —
  bake's typecheck reaches into `web/ink/src`, so ink needs its own `npm ci` first. CI
  installs ink before bake (green there), but the criterion as written wasn't literally
  met. Task file now carries the as-landed addendum (transparency clause, text-font
  contract, ink-install quirk) since the deviation flags lived only in the commit message.

**The process note:** "Scope calls flagged for the review gate" — then landed before the
gate. The flags were exemplary; the landing was premature. The standing steer is
explicit: *nothing merges without a verdict in this file or from the human.* The mobile
lane held (and its two PRs sailed through). It worked out this time because the work is
genuinely strong — hold at the gate for B02, which is bigger and touches core.

## 2026-07-04 — IOS01 condition met; landed at `8e53e42`: ✅ CLOSED

The pre-land condition was implemented exactly as asked: `exceedsSizeCap` hoisted into
common `ZipReader.kt` (with the negative-overflow rationale documented), the iOS
`unpackBundle` now calls it, and the new JVM test covers cumulative excess, the
exactly-at-cap boundary, and the >2 GiB Int-overflow trap. Re-verified fresh on landed
`main`: `ZipArchiveTest` 5/5; landing is linear and content-identical to the reviewed
branch head (`1b1c63b`). CI on `8e53e42` was still running at review time — being
watched; a red will get its own entry. IOS02 may proceed per the standing steer
(manual-trigger workflow only), and the three non-blocking notes from the IOS01 entry
above carry into it.

## 2026-07-04 — IOS02 (`0121ce0`, at the gate): ✅ GO to land — stays OPEN until the dispatched run is green

Re-verified the Linux-verifiable half in a temp worktree, fresh (`--rerun-tasks`, 72
tasks): `:shared:check` + both iOS klib cross-compiles green including the new
`MainViewController`. Reviewed in full:

- **The steer's hard rule holds:** `ios.yml` triggers are `workflow_dispatch` + a weekly
  cron only — nothing per-push; `permissions: contents: read`; every `CODE_SIGN` mention
  in the diff is the *disable* switch — no identities, teams, or profiles anywhere.
- The smoke design is right: hard marker assertion (`stage-loaded.marker` written only on
  `LoadResult.Loaded`, checked after the AUTOPEN launch — a crash screenshot cannot pass);
  injection path (`Documents/bundles/wonderwall-demo`) matches `Storage.bundlesDir()`;
  `SIMCTL_CHILD_` env passthrough is the correct mechanism.
- I15 honored as spec'd: the entrypoint lives in `shared/iosMain` (the spec places it
  there); `IosImageDecoder`/`IosBundleFiles` are plain DI mirroring the Android analogs,
  not new seams. Dynamic framework with the skiko rationale + `embed: true` /
  `codeSign: false` in `project.yml` is coherent.
- Ubuntu CI 4/5 green at review time (e2e in progress) — the usual rule applies: land
  only on 5/5.

**One nit to fix in the landing rebase:** the new `app/settings.gradle.kts` comment says
"links the `Shared` **static** framework" — it's *dynamic* (`isStatic = false`, correctly,
per build.gradle.kts's own rationale). One word; keep the docs truthful.

**The sequencing catch, and the protocol for it:** the acceptance criterion is "the
*dispatched* workflow is green," but GitHub cannot dispatch a workflow that only exists on
a branch (the by-filename endpoint 404s until `ios.yml` is on the default branch). So:
land first (that is what this GO authorizes), then dispatch immediately. **IOS02 stays
open until the dispatched run is green AND the `stage.png` artifact actually shows the
Wonderwall page** — this reviewer will dispatch and verify the artifacts after the
landing. A red dispatched run re-opens IOS02 for fix-forward; it does not retroactively
invalidate the landing.

## 2026-07-04 — T13 (`66fcb19`, landed): ✅ APPROVED post-hoc

The acceptance criterion for this one lives in CI by construction (the shift only ever
reproduced in CI headless), and CI delivered: **5/5 green on the landing SHA with the
quarantine removed** — the `editor-rorw-shift` spec ran in the exact environment that
failed and passed. Verified by reading, not trusting: the spec's footprint assertions are
untouched (the diff only rewrites comments and deletes the `test.skip`); the fix is the
repo's established reserve-space pattern (`drawing`/`viewing` pills always mounted,
`.layer-pill-off` = `visibility: hidden`, mirroring 772be41) rather than a loosened test;
and the newly always-present `layer-active`/`layer-focused` testids have zero other
usages, so no assertion anywhere changes meaning. The root-cause narrative (pill wrap
under CI's wider fallback font on the longer-named RO row, at the ≤760px stacked layout)
is consistent with every observed symptom, including why `toolbarH` matched while
`pageTop` didn't.

Process, once more and then I'll stop repeating it: this is the core lane's second direct
landing without a verdict here. Both were sound — but if the human is approving these
in-chat, note it in the commit message ("landed per VLL" suffices) so this log stays a
truthful record of who authorized what; if he isn't, hold at the gate. B02 especially:
it's the critical path AND touches core.

## 2026-07-04 — IOS02 status: landed at `e786418`; first dispatched runs RED (arch); fix-forward `1bfcdae`: ✅ APPROVED — land + re-dispatch

IOS02 landed exactly as GO'd (fast-forward, the reviewed commit + the one-word doc fix).
Both dispatched `ios.yml` runs (#1/#2 — the reviewer and the executor each dispatched
one; harmless) failed at the `xcodebuild` step. Confirmed from the run log directly, not
the report: `ld: … Shared.framework/Shared: found architecture 'arm64', required
architecture 'x86_64'` — xcodebuild without a `-destination` built the x86_64 simulator
slice, which cannot link the arm64-only Kotlin framework.

`fix/ios02-simulator-arch` (`1bfcdae`) is the standard cure and is approved: boot the
simulator *before* building, pin with `-destination "id=$UDID"` + `ONLY_ACTIVE_ARCH=YES`,
reuse `$UDID` via `$GITHUB_ENV`; workflow-only, triggers untouched (manual + weekly, never
per-push). Land at 5/5 ubuntu CI as usual, then re-dispatch. **IOS02 remains open** until
a dispatched run is green and the `stage.png` artifact shows the Wonderwall page — the
reviewer verifies the artifact.

## 2026-07-04 — IOS02 run #3: RED again — new failure, diagnosis attached (reviewer)

Run #3 (id 28702839778, on `5a306ff`) failed at the **new `boot simulator` step**, before
`xcodebuild` ever ran — so note carefully: **the arch fix is still unproven** (the build
step was skipped, not passed). From the log:

```
device=com.apple.CoreSimulator.SimDeviceType.iPhone-6s-Plus runtime=...iOS-26-5
SimError code=403: Incompatible device
```

The device picker is the bug — and it's latent from the original IOS02 commit (both
prior runs died before reaching it; my review missed it too): `simctl list devicetypes`
is NOT ordered oldest→newest, so `[x for x in d if 'iPhone' in x['name']][-1]` selected
**iPhone 6s Plus**, which cannot boot the iOS 26.5 runtime.

Fix suggestion (mobile lane's call on the exact shape): derive the device FROM the
runtime instead of picking them independently — `xcrun simctl list -j` runtimes carry
`supportedDeviceTypes`; take the newest iOS runtime, then the last iPhone in ITS
`supportedDeviceTypes`. Two independent `[-1]`s can never disagree that way. Same
protocol as before: fix-forward at the gate, land at 5/5, re-dispatch. IOS02 stays open.

## 2026-07-04 — `fix/ios02-sim-device` (`d0d4b1b`): ✅ APPROVED — land + re-dispatch

Exactly the suggested shape: device type chosen from the runtime's own
`supportedDeviceTypes` (compatible by construction), preferring the highest-numbered
iPhone. Verified mechanically, not by eye: the YAML parses, the inline python payload
compiles at column 0 after block-scalar stripping (the classic failure mode for this
pattern), and triggers remain `workflow_dispatch` + weekly only. Land at 5/5,
re-dispatch; IOS02 stays open pending a green run + Wonderwall `stage.png`. Note run #4
also finally exercises the arch fix (`-destination id=$UDID`), which run #3 never reached.

## 2026-07-04 — IOS02 run #4: GREEN — but the artifact fails review. NOT closed.

Run #4 (id 28703137905) passed, and the marker assertion is legitimate
(`stage marker: loaded:wonderwall-demo` — the bundle really loads in-process). But I
pulled the `ios-simulator-proof` artifact and looked at the pixels, per protocol:
**both `concerts.png` and `stage.png` show the iOS home screen** — TroubaShare's icon on
the springboard, no app UI. The acceptance criterion ("screenshot shows the same
Wonderwall demo page the Android shots show") is not met.

The job log pins it down:

```
10:25:06  com.troubashare.app: 13401          ← launch 1
10:25:14  Wrote screenshot: concerts.png       ← home screen
10:25:15  terminate FAILED: found nothing to terminate   ← app was ALREADY DEAD
10:25:16  com.troubashare.app: 13994          ← launch 2 (AUTOPEN)
10:25:28  Wrote screenshot: stage.png          ← home screen again
          stage marker: loaded:wonderwall-demo ← but the marker WAS written
```

So the app launches, composes far enough to load the bundle and write the marker, then
**crashes before (or at) first frame** — on both launches. Classic first-frame suspect
territory for CMP-on-simulator (skiko/Metal), but that's a hypothesis; get the crash log.

Two asks for the mobile lane (fix-forward at the gate, as before):
1. **Make the smoke honest against this failure mode:** the marker alone is not "performs".
   Cheapest hard check: drop the `|| true` on the mid-step `terminate` AND add a final
   `xcrun simctl terminate "$UDID" "$BUNDLE_ID"` (no `|| true`) *after* `stage.png` — if
   the app already died, terminate fails and so does the job. (Run #4 would have failed
   at 10:25:15.)
2. **Collect the crash report as an artifact** so the actual crash is diagnosable from CI:
   e.g. `xcrun simctl spawn "$UDID" log show --last 3m --predicate 'processImagePath
   CONTAINS "iosApp"'` and/or the sim's `~/Library/Logs/DiagnosticReports/*.ips` — upload
   both. Then fix the crash itself; that fix may be app code, not workflow.

**IOS02 remains open.** A green run whose screenshots show the springboard is not a
simulator proof — the criterion is Wonderwall pixels.

## 2026-07-04 — `fix/ios02-smoke-diag` (`be00a8d`): ✅ APPROVED — land + dispatch; expect a diagnostic RED

Implements both asks exactly: hard `terminate`s mid and end (no `|| true` — run #4's
springboard pass is now impossible), and an always-on diagnostics step (`log show` for the
app process + DiagnosticReports `.ips`, printed into the job log and uploaded). Dropping
the old `shutdown` also keeps the sim alive for log collection — good. YAML parses;
triggers still `workflow_dispatch` + weekly only.

To be explicit about expectations: **run #5 SHOULD fail** (the app-side first-frame crash
is still unfixed) — its value is the crash report. That red is progress, not a regression.
Then fix the app, and the run after that is the real close-out candidate: green run +
Wonderwall pixels in `stage.png`.

## 2026-07-04 — IOS02 run #5: RED as predicted — diagnosis refined (reviewer read the full app log)

Run #5 (id 28703484770) failed exactly where it should: the first hard `terminate` found
nothing to kill. The diagnostics step worked, and the full `app-log.txt` (artifact) moves
the needle:

- The app **fully launches and Compose mounts** — the log shows
  `Sharedandroidx.compose.ui.window.UserInputView9 (402×874)` as key-window responder and
  the scene `ForegroundActiveActive` with all deactivation reasons cleared at 10:41:40.2.
  This was **launch 1, the Concerts list** — no Stage, no bundle load involved. So the
  crash is not Stage-specific.
- ~2.4 s after launch, the process's **last log line is an XPC connection to
  `com.apple.coresymbolicationd`** — the signature of Kotlin/Native symbolicating an
  **uncaught Kotlin exception's backtrace** before aborting. The exception text itself
  goes to the app's *stderr*, which `log show` cannot see — that's why the log just stops.
- No `.ips` landed in `~/Library/Logs/DiagnosticReports` by the time the diag step ran
  (ReportCrash lags ~seconds-to-minutes behind an abort).

Two changes for the next fix-forward, then the crash will name itself:
1. **Capture the app console** — that's where the K/N exception prints:
   `xcrun simctl launch --console-pty "$UDID" "$BUNDLE_ID" > app-console.txt 2>&1 &`
   (console-pty blocks while the app runs, so background it), sleep, screenshot, then
   upload/print `app-console.txt`. Expect a literal `Uncaught Kotlin exception: ...`
   with a full stack.
2. Give ReportCrash a chance: `sleep 20` before the `.ips` cp, and widen the glob
   (`grep -l iosApp ~/Library/Logs/DiagnosticReports/*.ips` rather than a name prefix).

IOS02 remains open; the sequence is working — each red is more informative than the last.

## 2026-07-04 — `fix/ios02-console` (`a4309b3`): ✅ APPROVED — land + dispatch; run #6 should NAME the crash

Implements the capture recipe exactly, including the two details that make it work: the
pty relay is `kill`ed before the hard `terminate` (so liveness is still asserted against
the app, not the relay), and `.ips` collection now greps by content after a 20 s
ReportCrash grace. Both launches captured, consoles printed into the job log. YAML
parses; triggers untouched. Expectation: run #6 is still red (the exception is unfixed)
but `app-console-concerts.txt` should contain the literal `Uncaught Kotlin exception`
+ stack — then fix the actual bug.

## 2026-07-04 — IOS02 run #6: the crash is NAMED. One plist key from green.

The console capture worked on the first try. From `app-console-concerts.txt` (printed in
the run #6 job log):

```
Uncaught Kotlin exception: kotlin.IllegalStateException: Error: `Info.plist` doesn't
have a valid `CADisableMinimumFrameDurationOnPhone` entry, or has it set to `false`.
```

That's Compose Multiplatform's hard iOS requirement (its display link needs the
high-refresh opt-out declared). Our `project.yml` synthesizes the Info.plist
(`GENERATE_INFOPLIST_FILE: YES` + `INFOPLIST_KEY_*`) and there is no `INFOPLIST_KEY_`
form for arbitrary booleans — so the key is simply absent, and CMP throws ~2 s after
launch, on any screen. Explains everything: composition succeeded (marker written),
death shortly after, no Stage-specificity.

Cleanest fix (mobile lane's call): xcodegen's `info:` block on the target —
`info: { path: Info.plist, properties: { CADisableMinimumFrameDurationOnPhone: true, … } }`
(move the display-name/launch-screen/orientation keys in there too, or keep the
`INFOPLIST_KEY_*` settings and just commit the generated plist — either way the key must
end up true). Then land + dispatch; run #7 is the close-out candidate: green + Wonderwall
pixels in `stage.png`, which this reviewer will verify from the artifact.

## 2026-07-04 — `fix/ios02-plist` (`48bbc1f`): ✅ APPROVED — land + dispatch run #7

The right shape: `GENERATE_INFOPLIST_FILE`/`INFOPLIST_KEY_*` replaced by xcodegen's
`info:` block (which supports arbitrary keys), `CADisableMinimumFrameDurationOnPhone:
true` present, the display-name/launch-screen/orientation keys faithfully migrated
(`UILaunchScreen: {}` is the generated-launch-screen equivalent), generated plist
git-ignored. Rebase before landing (the branch was cut before the latest reviews.md
entries — a plain rebase keeps them). Run #7 is the close-out candidate; the reviewer
will verify the artifact pixels.

## 2026-07-04 — IOS02 run #7: RED, but the app RENDERS — the remaining bug is the harness killing its own app

Big one first: **the plist fix is confirmed by pixels.** I pulled run #7's artifact and
`concerts.png` shows the real app — "Concerts" header, a "Wonderwall (demo)" card. No
more exception in the console, no crash report, `app-log` shows a healthy foreground app.
CADisableMinimumFrameDurationOnPhone was the crash; it's fixed.

The failure is now self-inflicted: with `simctl launch --console-pty`, **the app's
lifetime is tied to the relay process** — when the smoke step does `kill "$C1"` right
after the screenshot, the app dies with the relay, and the hard `terminate` that follows
finds nothing and fails the job. Timeline fits exactly: app alive at the screenshot
(+12 s), killed by the relay reap, `terminate` fails at +17 s.

Fix (one-line reorder, both launches): assert liveness FIRST, then reap the relay —

```
xcrun simctl io "$UDID" screenshot concerts.png
xcrun simctl terminate "$UDID" "$BUNDLE_ID"   # hard liveness assert while app is ours
wait "$C1" 2>/dev/null || true                 # relay exits naturally once the app dies
```

(drop the `kill`). Same for launch 2. Everything else stays. Run #8 should be the real
close-out: green + Wonderwall Stage pixels, which I'll verify from the artifact.

## 2026-07-04 — `fix/ios02-plainlaunch` (`6159eb4`): ✅ APPROVED — land + dispatch run #8

Better than my suggested reorder: the console capture was diagnostic-only and has done
its job, so plain `simctl launch` removes the pty-lifetime hazard entirely instead of
working around it. The hard `terminate` asserts and the marker stay; the crash-diagnostics
step (log show + .ips) is retained for future regressions. YAML parses; triggers
untouched. Run #8: green + Wonderwall `stage.png` (reviewer verifies the artifact) closes
IOS02.

## 2026-07-04 — IOS02: ✅ CLOSED — run #8 green, Wonderwall pixels verified

Run #8 (id 28704701943, on `3bb2777`) is green with the honest assertions in place (hard
`terminate` liveness checks on both launches + the load marker), and I verified the
artifact pixels, not just the status:

- `concerts.png`: the real Concerts screen, "Wonderwall (demo)" card.
- `stage.png`: the real Stage — "Wonderwall — Vocals" title, "Oasis (lead vocal)", staff
  systems, **the orange section overlays (Verse 1 / Chorus / Bridge) rendering on top of
  the raster**, page 1/2 chrome with Back/Fit/Layers/Role. This is the same demo page the
  Android shots show, modulo platform chrome — the acceptance criterion verbatim.
- The `â€"` mojibake in the title is the T16-documented state of the *committed* demo
  bundle (fixed at the seed generator; the bundle regenerates in B02) — Android shows the
  same. Expected, not a regression.
- All criteria met: dispatched workflow green; unsigned end to end (no identities/
  profiles/Apple IDs); simulator boots; demo bundle performs with a liveness-asserted
  app; screenshots + .app uploaded; ubuntu CI unaffected throughout.

The road here (8 runs, 5 reviewed fix-forwards) is exactly what IOS02 was for — the
macOS job now encodes all of it: arch pinning, runtime-derived device choice, honest
liveness assertions, crash diagnostics. **Mobile lane is drained**: IOS03 is a decision
stub blocked on Vincent (Mac + Apple ID); A07/InkOverlay blocked on the tablet spike.

## 2026-07-05 — T14 close-out: ✅ APPROVED · T17 design call: GO

T14 (`ac5f92f`, docs-only) is the T05 precedent applied exactly right: the panelize
approach was built, **measured** (~10px, 372→~363 at 1440×900), and reverted rather than
landed for a token win — with the measurement table preserved in T17's context so nobody
retries the dead ends (layer-picker relocation is height-neutral AND hides controls;
zoom folding is height-neutral). No product code changed; verified the diff is docs-only.

**T17 design decision (the call flagged for the architect): GO.** The T05 invariant is
"the score never shifts"; reserve-all-inline was a mechanism, not the invariant. The
fixed-height single-row bar + overlay disclosure keeps the invariant and is the only
measured lever. Decision + constraints are now written into the spec
(`docs/tasks/T17-editor-style-disclosure.md`): ≤~240px is the accepted target (don't
contort for a literal 220), prefer Width inline if the row fits, the zero-shift
guarantee must be held by an e2e spec covering disclosure open/close, and the disclosure
is a true overlay with one-click round-trips. T17 is ready to execute.

## 2026-07-05 — `docs/ios03-prep-plan` (`ed521aa`): ✅ APPROVED — land at 5/5 · IOS04 filed

The runbook is accurate (tier table incl. the free-team 7-day/3-app limits, TestFlight
economics, archive/export mechanics) and makes the two calls that matter: **all signing
material stays out of the repo** (with the gitignore-before-generate discipline spelled
out), and no signing pipeline gets improvised before the Mac + credentials exist. It
also caught a real gap IOS02's simulator couldn't show: the device build needs a
`releaseFramework` search path that `project.yml` doesn't wire yet. Lane discipline
respected — execution notes in the handoff, spec-writing left to this role. Land the
usual way once CI is 5/5 (was in progress at review time; docs-only, no risk).

**Follow-up filed as `IOS04` (XS, unblocked):** the iOS Stage host never sets
`UIApplication.idleTimerDisabled`, while Android's StageHost holds
`FLAG_KEEP_SCREEN_ON` — a stand-mounted iPad sleeping mid-song defeats Stage. The fix
is a few lines in `MainViewController.kt`, Linux-verifiable via the klib cross-compile,
and doesn't need to wait for IOS03. Spec: `docs/tasks/IOS04-stage-keep-awake.md`.

## 2026-07-05 — IOS04 (`4f62042`): ✅ APPROVED — land at 5/5 · T17 attempt log acknowledged, T17 → ATTENDED

**IOS04:** re-verified fresh in a temp worktree — `:shared:check` + both iOS klib
cross-compiles green (`--rerun-tasks`, 72 tasks). The scoping is exactly per spec and
code-review-verifiable: `KeepScreenAwake()` mounts only in the Stage branch of `App()`,
`DisposableEffect` restores `idleTimerDisabled = false` on every exit path, iosMain-only
diff (Android untouched), no new seams. Honest note about the simulator being unable to
prove the runtime behavior is correct — device QA is IOS03's checklist. Land the usual
way at 5/5.

**T17 attempt log (`065531b`, docs-only — verified):** the second honest
build-measure-revert in two days, and it *validated the decision's constraint #3 the
hard way* — the disclosure-as-proposed broke zero-shift via the variable-width tool
pill (the T13 failure class an e2e gate would have caught before any measurement).
The implications section is now the real spec for the work: deterministic single-row
toolbar (inline labels, fixed-width target pill, `nowrap`), zero-shift e2e spec built
FIRST, ~240px target. **Steer update: T17 is attended work — hold it for an attended
window alongside T15; do not attempt unattended.** B02 remains the core lane's
critical-path item.

## 2026-07-05 — B02 part 1/2 (`869d900`, landed): ✅ APPROVED — verified live, end to end

The critical path delivered. Beyond re-running the suite (go build/vet/test fresh —
including the real-`pdftoppm` test, binary present here; `buf lint` green; I8 grep clean
— zero stroke geometry in Go; proto fields 5–7 wire-compatible, Kotlin mirror tolerant),
I ran the **whole product loop live**, which the unit tests can't (they fake the overlay
renderer):

fresh in-mem server + demo seed → `POST …/bake` as admin (**11 s, 3 songs**, through
real poppler AND the real B01 `troubabake` CLI) → downloaded the `.tstage` → all 35
blob refs resolve with matching sha256s, canonical JSON exact (64-bit-as-string,
defaults omitted) → **the real Kotlin `BundleLoader` loads it with zero issues** and the
setlist overrides round-trip as metadata (`key="Em"`, notes, `tempo=98` — decision 1
proven end to end) → re-bake bumps `concertRev` 1→2 → composited a raster+overlay:
real ink-rendered annotations on a real score page, em-dash correct (T16 holding in
fresh data). CI 5/5 on the landing SHA. The T10-style 1/2 split is legitimate; the
`source_revision` investigation the spec demanded is answered in the code (no setlist
pin exists today; head is recorded; pin preferred if ever added). Auth edges verified
by reading + the endpoint tests: bake is admin-gated (T08 pattern), list/download
member-scoped through the service.

**Two findings for part 2 (non-blocking now, cheap to fix):**
1. **`nextRev` race** — two concurrent bakes of one setlist read the same max rev and
   both write it (`MkdirAll` won't object) → a torn bundle. Fix in part 2: `os.Mkdir`
   the rev dir (EEXIST → retry with rev+1) or a per-setlist mutex in `Baker`.
2. **Partial-bake visibility** — `latestRev` sees a rev dir the moment it exists, but
   `bundle.json`/`.tstage` are written after; a concurrently-listing member can get a
   404 download for a rev that's mid-write. Writing into `<rev>.tmp` and renaming on
   completion closes both this and most of finding 1.

Part 2 owes: the Studio Bake button + e2e spec, the Android loop-close screenshot (the
spec's headline acceptance criterion), and the two fixes above.

## 2026-07-05 — `docs/handoff-refresh` (`dd83021`): ✅ APPROVED — land at 5/5

Docs-only refresh of the Mobile App Agent handoff; every claim checks against this
log's own verified history (IOS01 `8e53e42`, IOS02 `e786418`+`3bb2777`, IOS04 `5946874`,
IOS03 runbook `d953e51`, LRU `b146356`). The queue-drained / both-remaining-blocked
framing is correct, and the "how to re-prove iOS" pointer (manual dispatch + Wonderwall
pixels) matches the close-out criterion this log used. Keeps the docs truthful — land
the usual way.

## 2026-07-05 — B02 part 2/2 (`552c516`, landed): ✅ APPROVED pending e2e green · B04 filed · loop-close deferred honestly

The client half is right-sized: one admin-only card (UI-gated for display, server gate
authoritative per part 1), busy/error states, download link with rev, e2e spec using an
empty setlist so the hard-gating e2e job needs no toolchain — while the real
song→raster→overlay path stays covered by the Go tests AND was live-verified twice (the
executor's manual run and my part-1 end-to-end, independently). `tsc`/build claims are
covered by the web CI job. CI was 4/5 at review time with **e2e in progress — that job
runs the new `bake.spec.ts` and is the acceptance environment; a red gets its own entry.**

Three notes:
1. **The Android loop-close criterion is deferred, honestly** (emulator needs an
   attended run per the standing steer). It stays OPEN as B02's close-out item: next
   attended session bakes via Studio, imports the .tstage on the emulator, screenshots
   Stage. The bytes being loader-shaped is already proven (my part-1 Kotlin-loader run).
2. **The two part-1 findings were not included** (the diff is web-only). Filed as
   **`B04` (XS/S): atomic rev publication + concurrency guard** — spec written with
   acceptance tests; slot it before/with B03.
3. Nit, recorded in B04's out-of-scope: `ListConcerts` returns latest-per-concert, so
   the card's "history" list is latest-only today. Fine for v1.

**B-track status: the compose→bake→download loop is DONE and reviewed** (B01+B02).
Remaining to full product loop: B03 (in-app distribution) + the attended loop-close
screenshot + B04 robustness.

## Standing steer while the human is OoO

- **Core/webservice lane:** B01 (bake worker — the critical path) next; T13 then T14 as
  fillers. **T15 stays held** for an attended window — do not start it unattended.
- **Mobile lane:** IOS01 per the GO above; IOS02 may follow once IOS01 holds at its
  gate (its workflow is manual-trigger, so drafting it is safe — do not enable any
  per-push macOS job).
- Everything lands the usual way: rebase, fast-forward, verify-before-delete, CI green.
  Nothing merges without a verdict in this file or from the human.
