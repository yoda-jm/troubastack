# Review-gate log — Architect/Reviewer verdicts

> **Reviewing the weekend?** Start with the digest:
> [`SUMMARY-2026-07-04-to-06.md`](SUMMARY-2026-07-04-to-06.md) — everything landed,
> every decision, every incident, with commit hashes; this log holds the full verdicts.

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

## 2026-07-06 — `docs/ping-ttrack` (`bc319b3`): ✅ APPROVED — and the coordination question is settled here

The cross-lane ping is accurate (matches this log's history verbatim) and docs-only —
land at 5/5. It asks whether the mobile lane should take the **B02 Android loop-close**
(import + perform a real-baked `.tstage` on the emulator, screenshot Stage — the
deferred headline acceptance criterion). **Architect's call: yes — assigned to the
mobile lane.** No need for a further relay round-trip. Parameters:

- Bake the bundle through the REAL flow (login as `marie`, `POST …/bake` on the demo
  setlist or click Studio's Bake button, download the `.tstage`) — not the hand-baked
  `docs/demo` bundle; closing the loop on the real pipeline is the whole point.
- Emulator discipline per the handoff: quiet machine (the lanes are idle), headless
  Pixel_7 AVD, `adb shell run-as` injection or the in-app Import; screenshot the Stage
  page and append it + the verdict evidence to reviews.md (or hold at a gate branch if
  any code change turns out to be needed — none is expected).
- This is verification-only (no product code), so an unattended run is acceptable;
  if the emulator ANR-storms, stop and leave it for an attended window rather than
  fighting it.

Done well, that stamps B02 fully CLOSED and the hand-baked demo bundle's replacement
(regeneration via the real pipeline) becomes a trivial follow-up.

## 2026-07-06 — B03 server slice (`97052eb`, landed): ✅ APPROVED

Re-verified: the three-way shape match is exact — proto `AvailableConcert` (fields 1–6)
↔ the A02 Kotlin mirror (`ProtoUInt64/Int64Serializer` on the 64-bit fields) ↔ the new
Go `concertView` (`,string` tags, camelCase, `bakedBy`/`downloadUrl` extras the mirror
ignores). `TestBakeEndpoints_authAndFlow` + `TestViewOf_availableConcertShape` pass
fresh locally; per-song rev = source revision is the right "song X changed" signal.
CI 4/5 green with e2e in flight (bake.spec.ts covers the reshaped card) — a red gets
its own entry. Clean lane routing: the spec's Status note sends the app half to the
mobile lane. Note B04 (atomic bake publication) grows slightly more urgent as B03
makes downloads a first-class app path.

## 2026-07-06 — B02: ✅ **CLOSED** — loop-close evidence verified; land `docs/b02-loopclose-evidence`

Pulled the screenshots from the branch (`8de5ea1`) and checked the pixels, not the
report: `b02-loopclose-stage.png` shows the Android presenter performing the
**real-baked** "Sat @ The Anchor" — *Wonderwall — Score* with a **correct em-dash**
(fresh pipeline, no T16 mojibake), the red conductor cues, the amber Verse 1 / Chorus /
Bridge section overlays composited over the raster, page 1/20 with full Stage chrome;
`…-concerts.png` shows the imported concert in the list. Provenance is the real
pipeline as assigned (isolated core on `:8091`, built `web/bake` worker, poppler — I8
chain intact). With this, **B02's last acceptance criterion is met: the task is fully
closed** — compose → bake → download → import → perform, end to end, on evidence.
Land the evidence branch the usual way (docs-only). B05 (regen `docs/demo`) is now
purely mechanical, as the bonus finding predicted.

## 2026-07-06 — B03 app half, part 1/2 (`fd803dd`): ✅ APPROVED — land at 5/5, proceed to 2/2

Re-verified fresh in a temp worktree: `:shared:check` + both iOS klib cross-compiles
green (`--rerun-tasks`, 72 tasks); **`UpdatesManagerTest` 10/10** — the full I13 matrix
(offered / newly / frozen / pinned / final-locked, policy persistence across instances,
apply success, download-failure-leaves-state-intact with import-never-called,
import-failure, song-changed-unsupported). The design is exactly the spec:
`ManifestTransport` as a dependency not a seam (I15 intact), apply through A05's
importer (atomic swap not reimplemented), `AUTO` inert with the P201 pointer,
`SongChanged` typed but honestly unemitted. Secrets hardening is textbook — new
`…secrets.enc` EncryptedSharedPreferences store (AES256-SIV/GCM under a Keystore
MasterKey) with the honest no-migration rationale; A05's caveat is paid before any
token lands, as mandated.

Two nits for commit 2/2 (no re-gate needed):
1. `apply()`'s docstring says it "clears the temp file" — it doesn't (the deterministic
   per-concert path self-overwrites, so harmless): either delete the file post-import
   or fix the docstring.
2. The `catch` in `apply()` reports every failure as "couldn't download" — if the
   importer ever throws (it normally returns `Failed`), the message would mislead;
   consider scoping the catch to the download call.

Land at ubuntu 5/5 (CI hadn't registered runs at review time), then 2/2: ktor
transport + cookie-over-Storage, Connect screen, offer chips, and the emulator e2e.

## 2026-07-06 — B03 app half COMPLETE (`e493ffd` + `86d9ede`): ✅ APPROVED — land both at 5/5

Part 2/2 re-verified fresh in a temp worktree: `:shared:check` + both iOS klib
cross-compiles + `:androidApp:assembleDebug` all green (`--rerun-tasks`, 113 tasks).
Read, not trusted: `HttpTransport` persists only the cookie name=value through the
**hardened** Storage seam, streams downloads in 64 KB chunks (no full-file in memory),
and is offline-first (no session ⇒ empty manifest ⇒ local bundles only — I12 held);
`MainActivity` applies offers ONLY on explicit tap through `UpdatesManager.apply` →
A05's atomic import, Freeze/Pin ride the policy store, and `stage/` is untouched (the
no-network gate holds by construction). **Screenshots verified by pixels**: the
NewlyAvailable→Download chip (signed-in, empty local list) and the
UpdateOffered→"update to rev 2" chip after a re-bake, with the overflow menu present;
the downloaded-Stage shot matches the loop-close evidence. CI was 3/5 at review (android
+ e2e in flight) — the usual rule: land only at 5/5.

Carried notes (fold into the landing rebase if trivial, else accepted as-is):
1. The two part-1 nits were not folded in — at minimum fix the `apply()` docstring
   ("clears the temp file" — it doesn't; it self-overwrites).
2. New v1 gap, accepted: an **expired** session still reads as connected
   (`isConnected` = cookie presence) and just yields empty manifests — no re-auth
   prompt. Fine for v1; revisit if users report "my offers disappeared".

**With this, I13 goes ✅ (explicit tier) and the full product loop — compose → bake →
offer → download → import → perform — is in-app, no manual file transfer.** Remaining
I13 residue: the transient AUTO tier (P201). ARCHITECTURE.md I13 tag to be refreshed
after landing.

## 2026-07-06 — B04 (`d085b63`, landed per steer + VLL): ✅ APPROVED — both findings closed

Exactly the specced fixes, re-verified: `os.Mkdir` claims `<rev>.tmp` atomically (retry
with a local increment on `IsExist` — correct, since `nextRev` counts only published
dirs), the bundle is staged entirely in `.tmp`, the `.tstage` is written first, and the
`os.Rename` to the numeric name is the single publication point — readers can never see
a partial rev. Ran the suite fresh **3× under `-race`**: the concurrent-same-setlist
test (distinct revs, both fully published, no staging visible) passes every time; vet
clean; CI 4/5 with only e2e in flight (bake-only change; go job green). The deferred
`RemoveAll` keeps failed bakes tidy; one accepted wart: a hard-killed process leaves a
stale `.tmp` that permanently skips that rev *number* — harmless (numbers are cheap),
noted here so nobody "fixes" it into a race later.

Process note, approvingly: the commit message cites its authorization ("landed per the
standing steer + VLL 'continue'") — exactly what the T13 entry asked for. Gate rule
satisfied.

## 2026-07-06 — T18 (`c5de23b`, landed per steer + VLL): ✅ APPROVED — one Go mirror

Verified first-hand, not from the report: `grep AUTHORITY.*bundle.proto` matches only
`internal/bake/bundle.go`; mkbundle + bake tests green fresh; and I re-ran
`make fixtures` myself — **zero diff**, so the two mirrors had never drifted and the
unified writer produces the exact bytes. The short-hash exception is the right honest
call (hashing isn't part of the container mirror; adopting the full hash would have
churned every fixture for nothing) and is documented in-file. I1's within-Go drift
class is closed; the TS/Kotlin mirrors remain P203's codegen decision.

## 2026-07-06 — B05 blocking report (`1814013`): ✅ the right call · decision made: option (b)

The lane found three genuine spec conflicts — including a reviewer error (my acceptance
named "Wonderwall — Vocals", but B02 v1 bakes the DEFAULT part = "Score"; I anchored on
the T16 example without checking the picker) — and reported instead of approximating.
Exactly the T05 discipline. **Decision, now written into the spec: option (b)** — the
demo becomes the real-baked 3-song "Sat @ The Anchor" (better demo, zero seed churn, the
content-change out-of-scope is deliberately waived), with amended acceptance criteria
(default-part title, reproducibility honest about server UUIDs). B05 is executable again.

## 2026-07-06 — B05 (`4751bb4`, landed per steer + decision b + VLL): ✅ APPROVED — hand-bake retired

Verified the SHIPPED artifact, not the report: unzipped the committed
`demo-concert.tstage` — canonical `bundle.json` ("Sat @ The Anchor", rev "1", 3 songs),
all 35 blob refs resolve non-empty, and the **real Kotlin `BundleLoader` loads it with
zero issues** (scratch test, removed after). Cropped the Wonderwall raster title:
**"Wonderwall — Score" with a true em-dash in the shipped bytes** — T16's fix has now
propagated all the way into the distributed artifact, exactly as the amended acceptance
demanded. READMEs carry the real-pipeline provenance + the honest reproducibility
caveat. The hand-baked demo era is over: every byte a user first sees now comes from
the real pipeline. CI 4/5 with e2e in flight (docs+binary only; no risk).

## 2026-07-06 — `docs/refresh-demo-screenshots` (`afb7e28`): ✅ APPROVED — land at 5/5

The README's app screenshots still showed the retired single-song demo; good catch to
refresh them alongside B05. Pixels checked: the concerts shot shows the imported
"Sat @ The Anchor" with the B03 Connect button and overflow menu; the stage shot is the
already-twice-verified Wonderwall-with-overlays capture (byte-identical to the
loop-close evidence). Docs-only; land the usual way.

## 2026-07-06 — Mobile follow-ups proposal (`06966aa`): ✅ GO on all four — land the proposal, then implement

All four are this log's own parked notes; scoping and batching (#1+#2+#3 as one "iOS
seam hardening" PR, #4 separate) are approved as proposed. Three riders:

1. **#2 (`rawInflate` fail-closed) has a zlib trap:** when the output buffer is exactly
   filled, `inflate(Z_FINISH)` may legitimately return `Z_OK` and only report
   `Z_STREAM_END` on a **second call** (with `avail_out = 0`). Naive strictness would
   reject VALID bundles whose entry exactly fills `expectedSize` — implement the
   double-call pattern (require `Z_STREAM_END` after the follow-up call), and mirror
   the semantics check against the Android `Inflater.finished()` behavior so both
   actuals agree.
2. **#4:** the generic `LruCache<K,V>` goes to commonMain; `PageImageCache` becomes a
   thin typed wrapper — Stage behavior byte-identical (the B01-era scratch test in this
   log, LRU entry 2026-07-04, is the exact eviction/access-order matrix to commit).
3. The two withheld items are rightly withheld: B03 401-handling needs its own small
   design (probably `isConnected` probing or a 401→disconnect signal), and the
   App()/nav commonMain hoist is a real design decision — file it as a proposal when
   ready, don't fold it into cleanups.

## 2026-07-06 — Mobile follow-up PRs: ✅ BOTH APPROVED — land sequentially at 5/5

**`task/mobile-ios-seam-hardening` (`d13816d`)**: all three items exactly per the GO.
The `rawInflate` rider was implemented correctly on BOTH actuals — iOS does the
double-call (`Z_OK` on a full buffer → follow-up call must yield `Z_STREAM_END`), and
Android's `finished()`-or-zero-progress probe is semantically equivalent, so an
exactly-filled valid stream passes while longer-than-declared fails closed on both
platforms. The size-gate reads attributes before bytes (and degrades gracefully when
attributes fail — the post-read check still guards). `jsQuote` now lives in commonMain
with tests. Fresh verify: 72 tasks green; `RawInflateTest` 4/4, `JsStringTest` 4/4.

**`task/lru-cache-extract` (`6c57d63`)**: the thin-typed-wrapper rider verbatim — the
generic `LruCache<K,V>` carries the exact logic this log scratch-proved in the LRU
review, `PageImageCache` delegates, Stage untouched. Fresh verify: 72 tasks green,
`LruCacheTest` 4/4 (the eviction/access-order matrix, now committed — the twice-flagged
note is finally closed).

Land one after the other (rebase the second on the first's landing), each at ubuntu 5/5.
With these, every parked review note from the IOS01–B03 arc is either fixed or
deliberately accepted — the mobile lane's book is clean.

## 2026-07-06 — "The Open Road" demo song (`b56ffb9`, landed): ✅ APPROVED on substance · one process note

Verified: `go test ./cmd/seed` green; the regenerated shipped bundle holds 4 songs with
all 40 blob refs resolving; the Open Road raster is a genuine original lead sheet
(key/tempo/capo header, sectioned chart) with its 3 purposeful overlay layers
(mandatory Form, conductor-tagged Cues, marie's personal notes) and setlist metadata
("Encore — everyone in") — a much better pipeline showcase than three placeholder
charts. The `localPath` seed source degrades gracefully (falls through if the chart is
missing), so seeding can't break.

Process note, factual: this landing cites **no authorization** ("Landed per…" absent)
and no task file exists for it. If VLL directed it in-chat — likely, and the work is
good — one line in the commit message keeps this log a truthful record, per the
standing steer. Please resume the citation habit.

## 2026-07-06 — Shared App()/nav proposal (`fecaefd`): decision = **(c) DEFER** — land the proposal

The agent's own lean is right, for the right reason: building the shared `App()` with
several optional slots before a second real consumer exists is the speculative-
generality trap — the abstraction would be shaped by guesses about what iOS
distribution UI will need. **Concrete trigger, so this doesn't rot:** the hoist becomes
its own spec'd task the moment an iOS ManifestTransport lands (that task and the hoist
should be planned together — the transport's arrival is what validates the slots).
Until then the duplication is stable: A08/A09/A10 all land in the already-shared
`StageScreen`, not the nav. Option (b) is explicitly NOT banked now — half a shared nav
would still encode today's Android shape. Land the proposal branch (docs-only) with
this decision recorded in §13.

## 2026-07-06 — Tablet screenshots of the new demo (architect-produced + reviewed): ✅ landed

Produced on a **Pixel Tablet AVD** (1600×2560 @ 276 dpi, portrait-natural, Android 16 /
API 36 Play image, headless swiftshader; created by cloning the Pixel_7 AVD with the
tablet LCD profile — the repo previously had no tablet AVD). APK = current `main`
(includes B03 + both hardening landings). Bundle = the shipped 4-song
`docs/demo/demo-concert.tstage`, injected via `run-as`.

Reviewed by pixels before committing:
- `tablet-concerts.png` — Concerts list with Connect/Edit/Import on tablet metrics.
- `tablet-stage-wonderwall.png` — page 1/22, overlays composited, chrome correct at
  tablet width.
- `tablet-stage-openroad.png` — **the money shot**: the original Open Road lead sheet
  with all three baked layers legible (orange "CHORUS — everyone in" highlight, green
  circled "capo 2 OK" conductor cue, red lyric ellipse, "breathe"/"rit. watch me"
  personal notes). Real content, real pipeline, real device class.

One observation, not a bug: paging at ~1.2 s/page under software rendering briefly
showed a blank page before the async decode caught up (A04's design; instant on real
hardware) — worth remembering when screenshotting: give a cold page a beat. README now
links the tablet shots in the mobile section.

## 2026-07-06 — `docs/handoff-queue-fix` (`a72e8e3`): ✅ APPROVED — land at 5/5

Right catch: §8's "queue drained" went stale when A08/A09/A10 were filed (`9c5d694`) —
a fresh mobile session would have been pointed at nothing. The fix lists the three
Stage tasks as ready, defers to `docs/tasks/README.md` § Queue-state as authoritative
(correct — one source of truth for queue status), and cross-references the deferred
nav-hoist decision. Docs-only; land the usual way.

## 2026-07-06 — A08 (`7e4de32`): code half ✅ re-verified — awaiting the evidence commit · emulator collision, mea culpa

Code review done, fresh in a temp worktree: 113 tasks green (`:shared:check` + both iOS
klibs + `assembleDebug`), `MetaStripTest` 5/5. The implementation is exactly the spec's
resolved decisions: pure `metaStripText` (notes · key · ♩=tempo, empties omitted,
all-empty → null → NO strip → pixel-identical layout), fields carried on `StagePage`,
strip rendered as a top overlay stacked under the chrome only on `pageInSong == 0` —
never in-flow, I12 intact. Verdict finalizes when the promised screenshot evidence
lands on the branch.

**Collision note (mobile lane, read this):** around 22:15–22:20 I attempted my own live
verification, not realizing your Pixel_7 evidence run owned adb — my tablet AVD failed
to start and my commands went to YOUR emulator: an `install -r` of the same branch APK
(harmless) plus a few taps/force-stops that opened Stage. If your evidence script saw
unexpected foreground state, that was me — re-run clean; I've backed off adb until your
evidence commit lands. Process lesson for this log: **check `adb devices` + which AVD
before driving an emulator** — same class as the port-8093 collision.

## 2026-07-06 — A08 evidence (`336af5e`): ✅ A08 FULLY APPROVED — land both commits at 5/5

The screenshot evidence completes the review: "Acoustic intro, capo 2.  ·  Em" renders
as a thin strip directly under the chrome on Wonderwall's first page, and "♩=98" on
Black Hole Sun's — both overlays, page geometry visibly unchanged, matching the earlier
code verification (113 fresh tasks green, `MetaStripTest` 5/5, strip only on
`pageInSong == 0`, all-empty → nothing rendered). B02's metadata decision has now paid
off end to end: setlist overrides ride the manifest AND reach the performer's eyes.
Land the branch (code + evidence commits) the usual way at ubuntu 5/5.

## 2026-07-06 — B06 core slice (`ac0066e`, landed per the queue steer): ✅ APPROVED

Re-verified: `go vet` + discovery tests green fresh (opt-out + never-fatal). The
implementation is the spec verbatim — `_troubacore._tcp` with TXT version/path,
`TROUBA_MDNS_NAME`/`TROUBA_NO_MDNS`, register failures logged-and-swallowed so
advertising can never block serving, `sync.Once`-guarded stop correctly deferred in
main. Lib choice justified (libp2p/zeroconf/v2, maintained fork, deps = miekg/dns
only) as the spec demanded; the security comment carries the prefill-not-trust stance
word for word. Wire verification honest (a real browse found it; the same-host
avahi-vs-:5353 confound is a correctly-diagnosed environment quirk, and cross-device
browse is explicitly the app half's to prove). Spec Status note routes the A-track
half (NsdManager / NWBrowser + the iOS plist keys). Good slice.

## 2026-07-06 — A09 code half (`7aa4344`): ✅ APPROVED — evidence commit finalizes (A08 pattern)

Fresh verify: 113 tasks green (`:shared:check` + iOS klibs + `assembleDebug`),
`StageKeysTest` green. Per spec on every decision: the fixed key map is a pure common
function wired through a focusable `onPreviewKeyEvent` root (covers pedals-as-keyboards
on both platforms); Android volume keys intercepted in `MainActivity.onKeyDown` behind
a handler that a `DisposableEffect` sets ONLY while Stage is composed and clears on
dispose — volume is normal everywhere else, and the IOS04 scoping precedent is followed
exactly. Clamped navigation reused (no wraparound); iOS volume capture correctly
declared out of scope. Awaiting the adb-keyevent evidence commit to finalize.

## 2026-07-06 — A09 evidence (`7604ce9`): ✅ A09 FULLY APPROVED — land at 5/5

The keyevent tour (PAGE_DOWN → DPAD_RIGHT → VOLUME_DOWN → PAGE_UP → VOLUME_UP → SPACE)
lands the pager at 3/22 and the screenshot shows exactly that — Wonderwall page 2/3 =
global 3, with (correctly) no A08 strip on a non-first page. Combined with the code
half above (113 tasks, StageKeysTest 3/3, Stage-scoped volume interception), A09 is
done: Bluetooth pedals and volume keys turn pages. Land both commits at ubuntu 5/5.

## 2026-07-06 — T20 (`8257d54`, landed per queue + VLL): ✅ APPROVED

Re-verified: `TestSetlistDuplicate` green fresh (fidelity of items/overrides/order,
source untouched, outsider denied), vet clean. The independence-by-construction design
is right — a new setlist id means the copy has no bake history for free (baking it
mints rev 1), and the deep copy re-creates items with fresh ids so no mutation can
alias. Member-level authz matches setlist creation, per spec. The e2e spec covers the
duplicate→rename→both-listed flow and gates in CI (in flight at review time — the
usual 5/5 landing rule applies). Authorization line present. USER-JOURNEY gap #7
closed.

## 2026-07-06 — A10 code half (`2088eec`): ✅ APPROVED — screenshot pair finalizes

Fresh verify: 113 tasks green, `StageColorModeTest` green. Per spec throughout: pure
mode enum (cycle/parse, unknown→NORMAL), a draw-time RGB inversion with **alpha
untouched** (the detail that keeps transparent overlay margins transparent), applied
identically to raster + overlays; persistence via injected Storage KV in both
entrypoints (DI, no seam); cached bitmaps never mutated. Finalizes on the Normal/Night
pair — checking that paper is near-black and geometry unchanged.

## 2026-07-06 — T21 (`473557d`, landed per queue + VLL): ✅ APPROVED — the security properties all hold

Re-verified fresh: all four reset test suites green (issue/consume, authz incl.
cross-band denial, expiry, operator-CLI path); vet clean. Checked the crypto by
reading, not trusting: tokens from `crypto/rand`, stored ONLY as SHA-256 hashes (a
leaked dataset yields nothing), single-use burn, expired swept on read, and consuming
a reset **invalidates every session** for that user — the property that matters most.
The invite-link trust model (out-of-band handover, no email machinery) is the honest
self-hosted answer, and the CLI covers the "the only admin forgot" bootstrap with the
single-writer caveat documented. The origin-agnostic `resetPath` (UI joins
window.origin) is a nice touch. USER-JOURNEY gap #8 closed.

## 2026-07-07 — A11 code half (`c3e23ff`): ✅ APPROVED — evidence batched with A10's

Fresh verify: 113 tasks green, `CountInTest` 4/4. Pure timing exactly per spec
(60000/tempo with the 20–300 clamp returning null ⇒ tap ignored, 8 fixed beats,
0-indexed downbeats), pulse keyed on the current page so a turn cancels, nothing
persisted. The evidence deferral is the right call — the emulator is ANR-storming
after a long multi-lane day (the documented machine-load pattern); batch the A10
Normal/Night pair + an A11 pulse capture when it's healthy, and both finalize
together. A09 meanwhile still awaits its landing.

## 2026-07-07 — A12 code half (`04b91c5`): ✅ APPROVED — evidence batched with A10/A11

Fresh verify: 113 tasks green, `FacingPagesTest` 6/6 (the odd/even/lone-last/song-jump/
clamp matrix). Design per spec: automatic two-up only in landscape+FIT_PAGE (portrait
pixel-identical), spread math as pure functions with the left page as the single source
of truth, `PageView` reused per side (no fork of the compositing path), one
turnNext/turnPrev entry point so A09's keys turn by spread for free, and A08's strip +
A11's count-in render per side. The landscape screenshot joins the batched evidence
run. With this, all four Stage tasks (A08–A12 minus the landed A08) are code-approved —
the evidence batch closes them together.

## 2026-07-07 — seed page-doubling fix (`a014d75`): ✅ APPROVED · demo regen REQUIRED (assigned)

Verified: root cause is exactly right (footer at y=285mm under fpdf's ~277mm auto-break
⇒ every page spilled a blank+footer page), the fix is the minimal correct one
(`SetAutoPageBreak(false)` — we paginate manually), and the new regression test pins
/Count == requested for 1..4 pages; seed suite green fresh incl. T16's encoding tests.
Great catch — it also explains the "way off page-2 annotations" mystery.

**Consequence, assigned to the core lane:** the SHIPPED `docs/demo/demo-concert.tstage`
still carries the doubled pages (blank pages interleaved; some annotations sitting on
blanks). Regenerate it via the B05 procedure (fresh reseed incl.
`rm -rf core/cmd/seed/assets` — the documented cache gotcha) and update the README
page-count mentions. Coordinate with the mobile lane's pending batched evidence run —
ONE reseed + one healthy-emulator session can serve both (their pager numbers will
change from /22 to the true count).

**Reviewer's honest note:** my B02/B05 artifact reviews validated internal consistency
(refs, hashes, loader acceptance, title pixels) but never asked whether 22 pages was
the INTENDED count — the doubled blanks sat in plain sight in my own composites.
Content-plausibility ("does the page count match the source?") joins the review
checklist.

## Standing steer (2026-07-06 refresh — supersedes the OoO steer above)

- **State:** compose → bake → download → perform works end to end (Android + iOS sim).
  Full queue status lives in `docs/tasks/README.md` § "Queue state" — kept current.
- **Core/web lane:** B04 (bake atomicity — do before or with more B03 surface), then
  T18 / B05 as fillers. T17 and T15 stay **attended-only** (T17: read its attempt log;
  build the zero-shift e2e spec FIRST).
- **Mobile lane:** B03 app half (downloader/offers/freeze per the spec's routing note)
  is the critical path; the **B02 Android loop-close screenshot** stays assigned
  (reviews.md 2026-07-06) — quiet machine, stop if the emulator ANR-storms.
- Everything lands the usual way: rebase, fast-forward, verify-before-delete, CI green.
  Hold at the gate for a verdict in this file or an explicit human approval noted in
  the commit message ("landed per VLL").
- Still blocked on Vincent: tablet stylus spike (A07), Mac + Apple ID (IOS03),
  credential rotation for the git remote.
