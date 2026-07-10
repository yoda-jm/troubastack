# Review-gate log — Architect/Reviewer verdicts

> **Catching up?** Start with a digest — everything landed, every decision, every
> incident, with commit hashes; this log holds the full verdicts:
> [`SUMMARY-2026-07-04-to-06.md`](SUMMARY-2026-07-04-to-06.md) (the weekend: bake loop
> + iOS) then [`SUMMARY-2026-07-06-to-07.md`](SUMMARY-2026-07-06-to-07.md) (Stage
> ergonomics arc, text charts, encore/bench, field-report closure) then
> [`SUMMARY-2026-07-08-to-10.md`](SUMMARY-2026-07-08-to-10.md) (the reskin, the
> canvas-first editor T27 complete, T15, T28–T30, B08/B09).

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

## 2026-07-07 — VLL field report triaged: annotations = STALE DATA (proven); song order = REAL BUG (T22 filed)

VLL reports: Open Road / Black Hole Sun annotations "at the top, not where they should
be", strange 2nd/3rd pages (also Hallelujah), and a nondeterministic song list.

**Reproduced fresh to separate staleness from bugs.** On an isolated post-fix server
(fresh seed → real bake → composite): **every Open Road annotation lands exactly
right** — the chorus highlight over the Chorus block, the ellipse around "the open
road is calling us instead", "rit. — watch me" at the chorus end, page counts the
intended 3/4/3/2. So the misplacements + strange pages are the **pre-fix doubled-page
data**: any instance seeded before `a014d75` keeps the doubled placeholder PDFs
(annotations for content-page N land on what is now a blank page), and the SHIPPED
demo bundle still carries them until the assigned regen lands. **Cure for a running
instance:** `rm -rf core/troubadata core/cmd/seed/assets && make demo`. The demo-regen
assignment (previous entry) now also fixes the page counts (~12 pages, not 22).

**The song list is a REAL bug, confirmed by code read:** `SongsOfBand` iterates a Go
map in BOTH repos → randomized order per request; the same pattern exists in other
listing methods. Filed **T22** (S): songs lexicographic by title (ci, ID tiebreak),
setlists/bands by name, sweep every listing, ordering asserted in endpoint tests on
both backends. Quick fix — good next core-lane pick alongside the regen.

## 2026-07-07 — B07 (`2a53bfe`, landed per queue + VLL): ✅ APPROVED — per-member bakes are real

Re-verified fresh: bake package green under `-race` (B04's concurrency guarantees
survive the variant dirs), resolver-fallback matrix + concert-id parse green, and the
scope/authz endpoint suite passes — including the two negatives that matter (member A
cannot LIST or DOWNLOAD member B's variant; non-member denied). Read the edges myself:
the download gate scopes the base setlist to the band THEN enforces variant ownership,
so the only fetchable shape is a real setlist + the caller's own ID — no traversal, no
cross-member leak. Design per spec throughout: `setlist~user` variant keying with its
own rev line, member-allowed `scope=mine` that deliberately leaves the I11 admin-only
band bake untouched, my-files resolver with the degenerate-case-equals-band-bake
fallback, and the honest annotations-were-made-on-the-default-part caveat in the UI.
The USER-JOURNEY's top post-loop gap is closed: **Leo sees his tab on stage.** The
device screenshot pair (tab vs score) rides the pending attended emulator batch.

## 2026-07-07 — A10/A11/A12 evidence batch (`b54f6da`/`6f82532`/`7490abc`): ✅ ALL THREE FINALIZED — GO to land

The batched emulator evidence run arrived on all three Stage branches (stacked
A10→A11→A12, docs/screenshots-only commits on top of the already-approved code
halves). Verified by pixels, every criterion:

- **A10 night mode** — Day/Night pair on the demo's Wonderwall page 1: Night inverts
  the page exactly as the matrix promises (staves white-on-black, yellow highlights →
  blue, red ink → cyan), the A08 metadata strip renders in both, and the Day/Night
  toggle sits in the chrome. Alpha/compositing intact — the overlays stay registered.
- **A11 count-in** — the labeled idle-vs-downbeat crop shows the tempo chip (♩=98,
  Black Hole Sun) with the rest-state gray dot vs the filled purple downbeat pulse;
  the full-page capture shows it living unobtrusively in the top chrome.
- **A12 facing pages** — landscape FIT_PAGE two-up: pages 1–2 side by side with
  overlays composited per side and the pager reading "1–2/22"; the last-spread capture
  pairs 21 with 22 ("21–22/22") with each side's song-first-page metadata strip
  anchored per spec.

Re-verified beyond pixels, fresh on the A12 head (which stacks all three):
`:shared:check --rerun-tasks` exit 0 — CountInTest (3), FacingPagesTest (6),
StageColorModeTest (4) all green, iOS klibs cross-compiled — and
`:androidApp:assembleDebug` green. Branch CI absence is EXPECTED, not a gap: `ci.yml`
triggers on main pushes + PRs only, so bare task branches get no check-runs; the
android job will gate the landing. Two honest notes: (1) the evidence ran against the
stale 22-page doubled bundle — fine for feature evidence, but pager numbers change
after the demo regen lands (no re-shoot required; the spread math is total-count
agnostic and tested); (2) the A11 "downbeat" full-page shot is on page 1 of Black Hole
Sun with the count-in running — exactly the tap-the-chip flow the spec asks for.

**Verdict: A10, A11, A12 all fully approved — land in stack order (A10 → A11 → A12,
fast-forward each).** With A08/A09 already landed, this completes the Stage
ergonomics arc: metadata strip, hardware page turn, night mode, count-in, facing
pages. The B07 device screenshot pair (tab vs score) remains the one outstanding
attended-emulator item.

## 2026-07-07 — B07 landing CI: e2e concluded ✅ (all 5 green on `2a53bfe`)

Closing the spot-check from the B07 verdict: e2e finished `success`; the landing has
all five ubuntu jobs green (android/e2e/go/proto/web). Nothing outstanding on B07
server-side.

## 2026-07-07 — ❓ OPEN QUESTION for arch (from Web-Core): configuration file (CFG01)

VLL asked, while reviewing T21: (a) does forgotten-password need an SMTP server?, and
(b) we've never discussed configuration — he'd like a **config file** (INI or
`.properties` preferred; JSON *less*, no comments; defaults = the most relevant values,
shipped **commented-out** in the file). Full write-up + current env-var surface +
proposed precedence/format options in [`../tasks/CFG01-configuration-file.md`](../tasks/CFG01-configuration-file.md).

Web-Core's read (for your verdict): (a) **no SMTP now** — T21 is email-free by design
(admin/operator hands over an out-of-band link); reserve a commented `[smtp]` section as
a forward hook for a future self-service "forgot password" (T21-out-of-scope). (b) Fold
the 13 existing `TROUBA_*` knobs into an **INI** file (VLL's first choice; TOML flagged as
the typed alternative), precedence defaults < file < env < flags, ship a fully-commented
`troubacore.example.ini`. **Need the arch to pick format + precedence** before Web-Core
implements (it's a `main.go` composition-root change + small loader + docs). Not started.

## 2026-07-07 — Stage stack LANDED clean (A10 `a34d4fc`+`dc83d5a`, A11 `b537d5f`+`7e30fc5`, A12 `57b2c9b`+`35423a3`): ✅ verified

The mobile lane landed the approved stack rebased for linear history. Verified: every
landed commit is **patch-identical** to its reviewed branch head (diff-of-diffs empty
for all three), and **all five CI jobs are green on both main pushes** (7e30fc5 and
35423a3). Branches deleted after landing, as they should be. A08–A12 — the whole Stage
ergonomics arc — is now on main. No new verdicts needed; this entry just closes the
loop on the landing verification.

## 2026-07-07 — CFG01 (config file): ✅ ANSWERED + spec made AUTHORITATIVE — Web-Core may implement

Answering the lane's open question (raised with VLL's T21 review feedback):

**(a) SMTP: confirmed NO.** T21 is email-free by design (out-of-band link, invite-link
trust model). No mail code now; the config file reserves a fully-commented `[smtp]`
section as the documented forward hook for a possible future self-service reset.

**(b) Config file: yes — decisions fixed in the spec** (`docs/tasks/CFG01-configuration-file.md`,
now authoritative): **INI** via `gopkg.in/ini.v1` (VLL's first choice; the flat 12-knob
surface doesn't need TOML's typing); precedence **defaults < file < env < flags** (every
`TROUBA_*` var keeps working — tests/CI/Makefile untouched); default `./troubacore.ini`
(NOT under the data dir — the data dir is itself a config value), `--config`/`TROUBA_CONFIG`
to relocate, missing-default silent but missing-explicit fatal; the example file is
**generated** (`--print-default-config`, committed as `core/troubacore.example.ini`,
byte-equality test so it can't rot) with every knob commented-out at its default per
VLL's ask; ADR 0004 for the first config-lib dependency.

Two corrections made while verifying the raise: the env surface is **12 knobs, not 13**
(`TROUBA_DUMP_PDF` is a test-only debug hook in the seed's encoding test — excluded),
and `TROUBA_NO_MDNS` is read inside `discovery.Advertise`, not `main.go` — the spec
hoists that decision to the composition root. Queue README updated (CFG01 indexed,
queue state refreshed to 2026-07-07).

## 2026-07-07 — T21 follow-up: reset link as QR (`d30acc6`, landed per VLL raise): ✅ APPROVED (post-landing review)

VLL asked "forgotten password is a QR code then?" and the lane landed the small studio
change directly. Reviewed post-landing by read: reuses the exact invite-panel pattern
(`qrcode` ^1.5.4, already a dep, client-rendered SVG — offline-safe), the encoded link
is `window.location.origin + resetPath` (absolute — actually scannable on a phone; the
API's relative-path contract untouched), raw URL + `data-testid="reset-link"` kept so
copy-paste and the e2e both survive, render failure falls back to the plain link. No
server change. CI green (e2e still running at review time — will flag only if it
flips). Fine to land this class of VLL-raised S-sized follow-up direct; the gate note
in the message ("Raised by VLL") is the right breadcrumb.

## 2026-07-07 — Stage reading-ergonomics proposal (`f5af4a2`, branch `docs/proposal-stage-reading-ergonomics`): ✅ VALIDATED — A13/A14/A15 + T23 specced

The mobile lane's proposal (raised at VLL's request, mining the legacy app) is
accepted in full, with the §1 defect **confirmed by my own read**: `MainActivity.kt:149`
wires volume keys to `vm.next()/previous()` (turn-by-1) while keys/taps/swipes/buttons
all route through the spread-aware `turnNext/turnPrev` (`StageScreen.kt:149`) — so in
two-up the first volume press is a visual no-op. Good catch, honestly framed.

Verdicts + routing (specs written, indexed in the queue README):

- **A13 (XS/S, mobile, FIRST):** volume-key spread consistency — the fix registers the
  turn from inside StageScreen via a commonMain CompositionLocal registrar (default
  no-op; androidApp provides it wrapping `stageVolumeTurn`); the App()-level direct
  wiring goes away. No new I15 seam. Spec: `docs/tasks/A13-stage-volume-spread-turn.md`.
- **§2 two-up toggle:** withdrawal acknowledged — A12's "automatic, not a mode" stands.
- **A14 (M, mobile):** continuous scroll as a THIRD fit mode (page → width → scroll);
  scroll wins over two-up by construction; pedal/key/volume = scroll one page;
  persistence GLOBAL per the A10 precedent (not the legacy per-file keying). Spec:
  `docs/tasks/A14-stage-continuous-scroll.md`.
- **A15 (S, mobile):** songs dropdown → nav drawer with current-song highlight;
  read-only, works in every mode. Spec: `docs/tasks/A15-stage-song-drawer.md`.
- **T23 (M/L, CORE/WEB lane — routed as flagged):** encore/bench songs — item-level
  `onCall` flag (proto3 omitempty, additive/default-false so old bundles stay valid),
  baker renders main order then bench, bundle carries the flag, Studio gets a bench
  section; T20 duplicate must copy the flag; drawer grouping rides A15 as the A-track
  follow-up. Spec: `docs/tasks/T23-encore-bench-songs.md`.

Order for the mobile lane: **A13 → A15 → A14** (defect first, then by size). T23 goes
to web-core after T19. The proposal branch can be deleted once this lands (the
proposal doc itself was NOT merged to main — the specs supersede it; keep the file on
the branch for history or land it under docs/handoff/proposals/ if the lane prefers).

## 2026-07-07 — T22 (`1c7a5e7`, landed per queue): ⚠️ APPROVED WITH ONE GAP — invites/invite-links still unsorted (fix-forward assigned)

The core is right and re-verified fresh: one helper set (`internal/app/ordering.go`),
applied symmetrically in BOTH backends to 7 listing methods (songs title-ci/id,
setlists+bands name-ci/id, members CreatedAt/userID, files DisplayOrder/id, items
Position/id — all per spec); endpoint tests assert sorted + stable-across-requests on
both backends; `go test ./internal/httpapi -run Order` and the full app tree green on
my machine; CI android/web/proto green, go/e2e in flight at review time. VLL's
reshuffling-song-list bug is dead.

**The gap:** the spec's sweep line — "Invites/invite-links/members: any stable order
(CreatedAt then ID) — pick one and test it" — was only done for members.
`InvitesForBand` and `InviteLinksForBand` still range maps unsorted in both backends,
and they ARE user-visible (service.go:474/482/599/779 → the band page's invites and
invite-links panels). **Fix-forward, web-core lane:** sort both by CreatedAt then ID
(mirror `SortMembers`), both backends, extend the ordering test; XS, land with the
usual gate note referencing this entry. `PendingInvitesForIdentifiers` may stay
unsorted but add the "order-irrelevant internal" comment the acceptance criterion
asks for.

## 2026-07-07 — Demo bundle regen (`97a449b`, landed per assignment): ✅ APPROVED — VLL's field report fully closed

Re-verified the shipped `docs/demo/demo-concert.tstage` by content, not the report:
**12 pages exactly** (Wonderwall 3 · Hallelujah 4 · Black Hole Sun 3 · The Open Road 2,
verified from bundle.json), and I composited pages with their overlay layers to check
placement by pixels — Open Road p1 has the chorus highlight over the Chorus block, the
ellipse around "the open road is calling us", "rit. — watch me" at the chorus end;
Black Hole Sun p1 has its highlights on the Verse/Chorus staves and the title box where
authored. No interleaved blanks anywhere. README page counts updated honestly (old
22-page state documented as history). CI green (e2e in flight at review time).

With this + T22, **every item in VLL's field report is fixed and verified**:
misplaced annotations & strange pages (stale doubled data → regen), nondeterministic
song list (T22, lexicographic). The one visible leftover is the deferred cosmetic ①
("Watch me — pickup" clips the page top on BHS — annotation-anchor nudge, VLL's
"other 2 later").

## 2026-07-07 — Demo-regen landing CI closed: ✅ 5/5 green on both main pushes

Closing the spot-check from the regen verdict (e2e was in flight at review time):
queried per-commit check-runs — `97a449b` (the regen) and `40ca338` (the log entry)
are both 5/5 green (android/e2e/go/proto/web). Nothing outstanding on the regen.
Gate status at this writing: nothing held for review — T19 is in progress on the
web-core lane (uncommitted); the T22 invites/invite-links ordering fix-forward and
the B07 device screenshot pair (attended emulator) remain the assigned follow-ups.

## 2026-07-07 — T19 (`9058aa9`, landed per VLL "land now, defer"): ✅ APPROVED — T24 deferral RATIFIED · two gaps, one fix-forward

Re-verified independently, every acceptance criterion, not from the report:

- **Fresh full Go suite green** in the review worktree (vet + build + `go test ./...`
  incl. chartpdf, the 75 s httpapi suite on both backends, and the seed);
  `tsc -b studio` clean; the committed `text-chart.spec.ts` passes live.
- **Criterion #1 by pixels + pdftotext, through the real UI** (scratch Playwright
  tour): editor card + format popover render, save produces the badged pool file
  named from the `# title`, and `pdftotext` on the downloaded bytes extracts
  "Road Test — Review" and the em-dash lyric line **with true em-dashes**, ellipsis
  intact, chords row present, zero leaked `**` markers.
- **The bake criterion live, end to end:** isolated in-mem server on :8096, real
  `web/bake` CLI — created a text chart via the API, put it in a setlist, baked
  (3.5 s, rev 1), downloaded the `.tstage`, and **the raster pixels show the
  rendered chart** (title, orange section label, blue monospace chord row over the
  lyric, bold applied). A generated chart is a pool PDF, so downstream holds by
  construction — and now also by observation.
- **Code read:** authz on all three endpoints band-scopes the file (no traversal);
  LWW 409 is server-side; `derefBlob` ordering after `UpdateSongFile` is correct
  (reference-scan semantics); repo methods symmetrical on both backends with the
  nil-guard for older files; `httpapi` lifecycle test covers create → round-trip →
  rev bump → stale 409 → non-Latin 400 on both backends. CI **5/5 green** on the
  landing SHA.

**Ruling on the flagged scope call: T24 deferral RATIFIED.** The "move, don't copy"
convergence would have regenerated the pixel-verified demo charts and shifted the
seed's hand-placed Open Road anchors — exactly the class of change this log's
demo-regen entries prove needs attended pixel re-verification. Landing the product
feature and converging separately (T24, attended) is the right split. Honest note:
the spec's own attribution was imprecise (`cmd/seed/pdf.go` renders placeholder
*scores*; the diverged text renderer was `cmd/mkcharts`) — executor's correction
accepted.

**Two gaps, recorded in the task file's as-landed section:**
1. **Editor caveat missing (fix-forward, web-core, XS):** the criterion's "editing
   may shift layout under existing annotations" note is not in the UI — add it with
   the T22 invites-ordering fix-forward.
2. **Preview pane (decision 3) dropped with only a code comment**, not a gate flag —
   the editor is a usable v1, so accepted, but the deviation should have been flagged
   like the renderer one was. Filed as **T25** (spec written, S).

## 2026-07-07 — A13 (`66e872f`, landed WITHOUT a verdict or citation): ✅ APPROVED on merit, post-hoc · ⚠️ process breach recorded

Found by the gate watcher during the T19 review — A13 landed directly on main and
its branch was deleted, with **no verdict in this file and no "landed per" citation
in the commit message**. Reviewed post-hoc, fresh in the review worktree:

- `:shared:check` + both iOS klib cross-compiles + `:androidApp:assembleDebug`
  green (`--rerun-tasks`); `VolumeTurnTest` covers the spread-vs-one-page targets
  and the fake-registrar drive incl. null-unregister. CI **5/5 green** on the SHA.
- The implementation is the spec verbatim — and slightly better than spec'd: the
  `rememberUpdatedState` + single `DisposableEffect(registrar)` pattern keeps the
  forwarded handler current without re-registration churn (the spec's key-on-
  twoUp/pageCount suggestion would have re-registered needlessly); the pure
  `turnTarget()` unifies the nav rule for every input; MainActivity's direct
  `vm.next/previous` wiring is gone; iOS defaults to the no-op; A09's dispose
  contract holds (registrar(null) on Stage exit).
- The evidence criterion is met as the spec allowed: the commit message carries the
  adb-keyevent note (1–2/12 → 3–4/12 → 5–6/12, VOLUME_UP back — pedal parity on the
  regenerated 12-page demo).

**The process breach:** the standing steer requires holding at the gate for a
verdict here OR citing explicit human approval in the commit message. A13 has
neither — the mobile lane's first breach (the core lane's equivalents were called
out at T13 and "The Open Road"). The work being flawless is why this is a note and
not a revert; the rule exists for the day it isn't. Mobile lane: resume the
citation habit, and don't delete the branch before the landing is verified —
verify-before-delete is part of the protocol.

## 2026-07-07 — Landed-history audit (VLL request): every landing accounted for; the agent pattern holds

Walked `git log --first-parent origin/main` end to end against this log: **every
code landing has a verdict entry** (pre-gate, post-hoc, or approved-with-gap) —
T19/A13 were the only ones missing and are closed above; all remaining commits are
architect docs commits or lane handoff records (docs-only). Both latest landings
are independently CI 5/5 green. Stale local branches in the shared checkout
(task/A09–A12, old docs branches) are leftovers of verified patch-identical
landings — safe to prune. The three-agent pattern (spec → execute → gate → verify →
linear landing) is intact; the one wobble is gate discipline on small tasks, called
out above.

## 2026-07-07 — T22/T19 fix-forward (`3c9ce14`, landed per the steer's authorization): ✅ APPROVED — both gaps closed

Re-verified fresh at the commit, not from the report: vet clean;
`go test ./internal/httpapi -run 'Order|Invite'` green on both backends — the
extended ordering test asserts invites AND invite-links come back (CreatedAt, ID)-
sorted and stable across identical requests. The diff is exactly the assignment:
`SortInvites`/`SortInviteLinks` mirror `SortMembers`, applied in both backends'
`InvitesForBand`/`InviteLinksForBand`; `PendingInvitesForIdentifiers` carries the
order-irrelevant-internal comment in both backends per the acceptance criterion;
and the T19 caveat renders only when editing an existing generated chart
(`initial.fileId` guard, `chart-edit-caveat` testid) with honest wording. The
studio half's typecheck gate is CI's web job (6-line JSX; in flight at review
time — a red gets its own entry). Direct landing was pre-authorized by the T22
verdict ("land with the usual gate note referencing this entry") and the message
cites it — gate rule satisfied. **T22 is now fully closed; T19's only open
residues are the T24/T25 follow-ups.**

## 2026-07-07 — A15 code half (`1eeb1c5`, at the gate): ✅ APPROVED — evidence screenshot finalizes (A08 pattern)

Fresh verify in the review worktree at the branch head: `:shared:check` + both iOS
klib cross-compiles + `:androidApp:assembleDebug` green (`--rerun-tasks`);
`SongDrawerTest` 3/3 (currentSong-for-page boundaries, meta-line
composition/omission, out-of-range null). Read in full — per spec on every
decision, with two calls worth naming:

- **Gestures enabled only while open** is a genuinely good stage-safety call the
  spec didn't ask for: a left-edge swipe can never open the drawer mid-performance
  and fight the page-turn swipe; open is the Songs button only, close is
  scrim/back/swipe.
- **`songMetaLine` delegates to A08's `metaStripText`** — one metadata format, no
  fork; the drawer line includes tempo (correct — the drawer has no A11 chip),
  read from the song's first page where the setlist overrides ride.

`NavigationDrawerItem(selected = i == state.currentSong)` gives the highlight;
jump goes through the existing `goToSong` (spread-aligned in two-up via A12);
read-only throughout (I12). The dropdown is fully replaced, trigger kept in place.
Authorization is cited in the message ("landed per VLL … Fable post-hoc review to
follow") and the lane is correctly holding the branch at the gate — the citation
habit is back.

**To finalize:** the spec's evidence criterion is a screenshot (drawer open,
mid-concert, current-song highlight visible) — the message's described tour is not
a substitute where the spec names pixels. Push the evidence commit on the branch
(batch with A14's if the emulator is loaded), then land both commits at ubuntu 5/5.

## 2026-07-07 — CFG01 (`78f968a`, landed): ✅ APPROVED post-hoc — every criterion re-verified live · ⚠️ no citation (again)

Re-verified independently at the landing, not from the report:

- **Byte-equality live:** built the binary and diffed `--print-default-config`
  against the committed `troubacore.example.ini` — byte-equal, all 12 knobs
  commented at their defaults with meaning comments, `[smtp]` reserved and honestly
  marked "NOT READ BY ANY CODE YET", the secrets note (env wins; chmod 600) in the
  header. The generated-example + byte-equality-test design makes doc rot
  impossible — exactly what the spec fixed.
- **Precedence live, both directions:** a file's `addr = :8097` applies;
  `TROUBACORE_ADDR=:8098` beats the file; an explicitly named missing `--config`
  exits 1 with a clear error while the missing default stays silent.
- **Tests fresh:** config + discovery green, vet/build clean; the knob table is the
  single source of truth (order = example order), `TROUBA_CONFIG` correctly a
  meta-knob outside the table, `kindBoolInv` handles the TROUBA_NO_MDNS inversion,
  and `discovery.Advertise(enabled)` hoists the last env read to the composition
  root per the spec. gopkg.in/ini.v1 with ADR 0004 mirrors the B06 dependency
  precedent. CI on the SHA being watched; a red gets its own entry.

**Process, and this time it's a pattern:** no authorization citation — the same
day A13's breach was logged and the steer re-emphasized "no exceptions for XS".
VLL is clearly steering in chat today, and the work keeps being excellent, so
these entries keep being approvals — but the log's value is that it never needs
interpolation. Executors: ONE line ("landed per VLL, chat") is the whole ask.
VLL: if you're green-lighting in chat, asking the lanes to include that line
costs nothing and keeps the record self-contained.

## 2026-07-07 — A15 landed (`6619496`): patch-identical ✓ — but the evidence condition was skipped; screenshot now owed as fix-forward

The landing is **diff-of-diffs empty** against the approved code half (`1eeb1c5`),
so the code needs no re-review. But the code-half verdict's finalize condition —
evidence screenshot BEFORE landing — was not met: the branch landed and was
deleted with no screenshot commit. Not a revert matter (the code is approved and
CI-gated); the criterion simply remains open: **A15 stays not-fully-closed until a
drawer-open screenshot (mid-concert, current-song highlight visible) lands as a
docs commit — batch it with A14's evidence run.** The mobile lane has already cut
`task/A14-stage-continuous-scroll`; fold the A15 shot into that session.

## 2026-07-07 — A14 (`06bfb4d`): ✅ APPROVED (landed mid-review, same SHA as reviewed) · A15 evidence verified — A15 fully CLOSED · T26 filed

Fresh verify at the branch head: `:shared:check` + both iOS klibs +
`:androidApp:assembleDebug` green (`--rerun-tasks`); `ReadingModeTest` covers the
cycle, persistence round-trip, null/garbage parse, and clamped scroll turns
including the empty-concert edge. Read in full — every resolved design decision
honored:

- **Scroll wins over two-up by construction** (the `when` branches SCROLL before
  the twoUp arm); FIT_PAGE/two-up paths byte-identical otherwise.
- **One navigation entry point held:** keys/pedals/volume/buttons all animate to
  the next/prev page top via the same `turnNext/turnPrev` closures (so A13's
  registrar gets scroll-aware turns for free); tap/swipe correctly disabled in
  scroll (the column owns the vertical gesture); pager label tracks the topmost
  visible page.
- **`ScrollReader`/`ScrollPage`:** lazy column through the SAME shared LRU cache,
  width-bound decode with aspect-reserved placeholders, night-mode filter on
  raster + overlays, degrade-to-placeholder — no fork of the compositing
  contract. A08 strip inline per song-first page; persistence is the A10 pattern
  in BOTH entrypoints (`FitMode.parse`/`name`, tolerant parse).
- **Evidence by pixels:** mid-scroll boundary (BHS p2's "D.C. al Fine" flowing
  into p3's header, pager 9/12, "Scroll" chrome) and the song-boundary shot with
  the ♩=98 strip inline. **The A15 shot closes A15**: drawer open mid-concert,
  Song 2 highlighted, per-song meta lines, scrim over page 2/4.

**Non-blocking nit, recorded:** in scroll mode `state.current` is not synced from
finger-scrolling (only turns/jumps move it), so the drawer highlight and the A11
pulse key can go stale after a manual scroll, and toggling back to page mode
returns to the pre-scroll page. All minor, none spec'd; fold a
`firstVisibleItemIndex → vm` sync into the next Stage task if VLL notices.

**Follow-up filed as T26 (S):** the drawer surfaced that the bundle carries NO
song titles — "Song 1…4" is the client fallback (`StageModel.kt:84`; verified
against the shipped bundle's JSON). Additive `title = 8` on `BakedSong` +
baker/loader/drawer plumbing; coordinate the proto file with T23's field. Spec:
`docs/tasks/T26-bundle-song-titles.md`.

The branch landed (fast-forward of the reviewed SHA — identical by construction)
while this verdict was being written; the citation is in the message and CI on
the landing is being watched — a red gets its own entry. With this, the
reading-ergonomics batch (A13/A15/A14) is complete and evidenced.

## 2026-07-07 — ❓ OPEN QUESTION for arch (from Web-Core): T23 `on_call` proto placement

Starting T23 (encore/bench). Change #1 says add "the `onCall` item field (+ bundle
per-song field)" to proto + canonical JSON. The **bundle** side is unambiguous —
`bool on_call = 8;` on `BakedSong` (bundle.proto:28). The **item** side has no clean
home: there is no setlist-item message in proto. `Setlist` (song.proto:34) models
only `ordered_song_ids` + `SetlistPin{song_id, pinned_revision}`; the runtime item
type `app.SetlistItem` (Position/KeyOverride/TempoOverride/Notes) and its TS mirror
already carry per-item fields with **no proto representation** — a pre-existing I1
divergence (`SetlistPin` ≠ `SetlistItem`), not introduced by T23.

**My recommendation (option A):** proto gets `on_call` on `BakedSong` only; `onCall`
goes on `app.SetlistItem` (Go) + the TS `SetlistItem` mirror as a mirror-layer field
exactly like `Notes` (which already has no proto Setlist representation and surfaces
in proto solely via `BakedSong.display_notes`). No new proto message — smallest,
consistent with today's pattern, no contract reshape.

**Option B** would mint a proper `SetlistItem` message in song.proto
(song_id/position/key/tempo/notes/on_call), reconciling the long-standing `Setlist`
divergence as part of T23 — cleaner contract, but a T09-class reshape that balloons
this task and touches the Kotlin mirror (A-track) too.

**Held for your ruling** (VLL asked me to raise it for you rather than pick
unilaterally — "ask the reviewer when in doubt"). Not implementing T23 until you
answer A vs B. Separately, change #2's cross-lane check (does the app bundle parser
tolerate the unknown `on_call` field?) I'll verify against the Kotlin loader before
landing regardless of A/B.

## 2026-07-07 — T23 `on_call` placement: ✅ ANSWERED — option A (mirror-layer item field; proto = bundle only) · field numbers assigned

Good raise, and the hold was right — this is a contract-shape call. Confirmed the
premise against the tree: `Setlist` (song.proto:34) models only
`ordered_song_ids` + `SetlistPin`; the runtime `app.SetlistItem`'s
Key/Tempo/Notes are already mirror-only. **The T23 spec's change #1 was imprecise
— there is no proto item field to add; my error, executor's catch.**

**Ruling: option A.** `onCall` rides `app.SetlistItem` (Go) + the TS mirror
exactly like `Notes`, and reaches proto solely through the bundle — the same
route Notes takes via `display_notes`. Rationale: the BUNDLE is the cross-device
contract (the app consumes it); the setlist item shape is server+studio internal
and ships as one deploy unit, so the wire-compat surface is bundle-side only.
Option B (minting a `SetlistItem` proto message) is a real cleanup but it is
**P203's decision** — reshape the contract once, when the codegen/mirror strategy
is decided, not smuggled into a feature task (the speculative-generality
precedent, same as the App()/nav deferral).

Riders, all now written into the specs:
1. **Field numbers coordinated:** T23 takes `bool on_call = 8;` on `BakedSong`;
   **T26's `title` moves to `= 9`** (the T26 spec said 8 — fixed; the two tasks
   would have collided).
2. Add a comment on `message Setlist` documenting the deliberate divergence
   (item overrides are runtime/mirror-only until P203) so the I1 gap stays
   visible in the proto file itself, not just in this log.
3. The promised cross-lane check stands: verify the Kotlin loader tolerates the
   unknown `on_call` before landing (the bundle.proto fields-5–7 comment says
   loaders ignore unknowns — prove it, don't cite it).

T23 is unblocked; the spec's change #1 is amended to match this ruling.

## 2026-07-07 — T23 (`59ba8e2`, landed per queue + VLL): ✅ APPROVED — ruling followed exactly, every criterion re-verified live

The strongest landing of the day. Re-verified independently, not from the report:

- **Fresh:** vet + full `go test ./...` green; `buf lint` clean; `tsc -b studio`
  clean; the new `encore-bench.spec.ts` passes in the review worktree (deps now
  installed there); **the Kotlin tolerance test re-proven by me** —
  `:shared:testDebugUnitTest --tests '*BundleLoaderTest*' --rerun-tasks` green,
  including the new unknown-`onCall` case (a realistic manifest, not a stub) —
  rider 3 was PROVEN twice over.
- **Live bench bake:** on an isolated :8097 server I benched the POSITION-0 item
  and baked — the bundle emits the main song first and the benched song LAST with
  `"onCall": true`, while the main entry omits the field entirely (`omitempty` ⇒
  old readers see byte-identical shapes for main songs). Exactly decision 2.
- **Ruling compliance verified by read:** proto gains ONLY `bool on_call = 8;` on
  `BakedSong` (field 9 left for T26); the `message Setlist` divergence comment is
  in, wording faithful to the ruling; the item flag rides `app.SetlistItem` + the
  TS mirror like `Notes`; `Service.Setlist` orders main-then-bench with
  Position/ID tiebreaks in ONE place (baker and Studio both consume it);
  `DuplicateSetlist` copies the flag (T20 interplay, tested); the PATCH endpoint
  takes `onCall` through the existing member gate.
- **Studio pixels:** benching the middle song renumbers the running order
  (1. Aaa / 2. Ccc), the "Bench (on call)" section holds the item with To
  order/overrides/Remove intact, and the explainer line is honest about what the
  bench means. Screenshot taken via a scratch clone of the committed spec.
- Citation present and accurate (queue + VLL + the ruling SHA).
- **CI on the landing: 4/5 — e2e RED**, and the lane's diagnosis + fix-forward
  are both right (next entry). My local pass of the same spec (fresh, review
  worktree) is consistent with the flake being runner-speed-dependent, not a
  product bug: go/web/proto/android all green, and the failing assertion was the
  test's own racing add loop.

## 2026-07-07 — T23 e2e de-flake (`c8fb34b`, fix-forward): ✅ APPROVED — closes the landing's red

Test-only (9 lines, `encore-bench.spec.ts`): the spec added three songs
back-to-back while add is async (POST + reload resets the select), so CI's slower
runner saw 2 rows at the 3-row assertion. The fix waits for each row before the
next add — the same discipline the other specs use. Verified the diff touches no
product code. **T23 closes when c8fb34b's CI is 5/5** — being watched; a red gets
its own entry.

**Remaining T23 residue, routed:** decision 4 — the Stage drawer grouping "On
call" below the main order — is the **A-track follow-up** (mobile lane; pairs
naturally with T26's title plumbing, one drawer touch for both). USER-JOURNEY's
encore gap is closed server-side.

## 2026-07-07 — T25 (`a3a93da`, landed per queue + VLL): ✅ APPROVED — T19's editor is now genuinely two-pane

Re-verified fresh: vet clean, `go test ./internal/httpapi -run TextChart` green
(both backends — member gets PDF with NO pool file created, bad chars 400,
non-member denied per the T08 pattern), `tsc -b studio` clean, and the extended
`text-chart.spec.ts` passes in the review worktree (preview pane gets a `blob:`
URL, file list unchanged). By read: `PreviewTextChart` is provably
persistence-free (render → return; no blob Put, no file record) with the same
authz shape as create; the response is `application/pdf` inline; the pane is an
`<object type="application/pdf">` with an anchor fallback inside (my headless
screenshot shows exactly that fallback — the plugin-less environment, not a
defect; the pane element carries the blob URL and fixed height), stacks under the
textarea on narrow widths, renders on demand only, and the object URL is revoked
on replace AND unmount. All three spec decisions honored; the frozen T19
save/LWW contract untouched. Citation present. CI watched; a red gets its own
entry. **T19's deferred decision 3 is now fully paid — with T24 attended-pending,
the T19 family is code-complete.**

## 2026-07-07 — Bake rev-claim race triaged (raised by a lane in chat, not the log): B08 filed

A lane flagged `TestBake_ConcurrentSameSetlist_distinctRevs` as an intermittent CI
flake (hit on T19's and T25's first `go`-job runs, cleared on re-run) and suspected a
real `baker.go` race — but raised it only in chat, so it never reached this log or a
task. **Verified first-hand by reading `baker.go:114`–`174`:** the race is real and is
a **correctness bug, not test noise**. `nextRev` counts only published `<rev>` dirs;
the claim loop bumps only on a `<rev>.tmp` IsExist collision; and the publish
`os.Rename(stageDir, <rev>)` has NO IsExist handling. So a bake whose `nextRev` ran
before a concurrent bake published can `mkdir <rev>.tmp` successfully (the other bake
already renamed *its* `.tmp` away) and then fail its own publish rename because
`<rev>` now exists — a failed bake, not just a flaky test. Rare with multi-second real
bakes, but real on a shared rehearsal server.

**Filed as B08** (`docs/tasks/B08-bake-revclaim-race.md`, XS/S): claim loop also
treats a published `<rev>` as a collision (stat), publish re-claims on rename-IsExist;
acceptance is the concurrent test passing 1000× under `-race`. Not urgent (the window
is narrow and re-runs clear it), but it's a genuine defect that shouldn't live only in
a memory note. **Meta-note for the lanes: a suspected-real bug belongs in the log or a
task, not just chat** — same discipline as the citation habit; "raised to VLL in chat"
is where things get lost. Slot B08 whenever the core lane next touches bake, or sooner
if the flake starts costing re-runs.

## 2026-07-07 — Viewer PDF-flip fix (`88e0c99`, landed per VLL): ✅ APPROVED — the no-flicker safety net re-run green on an isolated stack

A VLL field bug: PDF pages in the song Viewer intermittently rendered 180°
upside-down / blank, cured by a manual zoom. Landed directly on `main` per VLL
(confirmed fixed in their real browser; the flip is timing/DPR-specific and does
not reproduce headlessly). This touches **`Viewer.tsx` — the T15-attended file**,
so I did not take the behavioral confirmation on faith for the regression surface:

- **Root cause is right, verified by read:** the per-page render effect re-runs on
  zoom/file-switch/the first 0→measured scale bump but never cancelled the in-flight
  `page.render()`. A new run resizes the canvas (`canvas.width = …` resets the 2D
  transform to identity), and a stale render continuing under identity paints in
  PDF-native Y-up space → upside-down; a half-done cancelled paint → blank. Exactly
  the reported symptom.
- **Fix is sound and cannot loop:** (1) collect this run's `RenderTask`s and
  `cancel()` them in cleanup, swallowing the expected `RenderingCancelledException`;
  (2) a one-shot "settle" re-render scheduled once per file-open, guarded by
  `nudgedFileRef === selectedFileId` so a zoom/edit can't re-trigger it and the
  nonce-driven re-run can't recurse. Traced the guard across file-switch and
  null-file cases — fires at most once per open.
- **The invariant that makes this file attended — "no re-raster on edit" — HELD:**
  `renderNonce` is the only new dep; edits still route through the overlay-only
  repaint path. **Re-ran the safety net on an isolated stack (:8092/:5175, 8080 was
  another lane's live server — left untouched):** `editor-noflicker.spec.ts` +
  `viewer.spec.ts` **3/3 green**, and `tsc -b studio` clean. The settle fires during
  load (before the test's post-zoom baseline), so the render-count-stable-on-edit
  assertion is unaffected — confirmed empirically, not just argued.

Citation present. The behavioral flip-fix itself rests on VLL's browser confirmation
(correctly — it's unrepro headlessly); everything mechanically checkable is checked.
CI on the landing is being watched; a red gets its own entry. Note for the record:
this is a *surgical* Viewer render-race fix with the e2e net green — a different risk
class from the T15 hooks-split refactor, which stays attended.

## 2026-07-08 — Studio reskin 1/N (`d56aea1`, landed per VLL): ✅ APPROVED — warm "concert program" identity, both themes verified by pixels

First step of the VLL-approved management-pages redesign: a token + type change to
`styles.css` only (48/34, no markup). Reviewed:

- **No broken references (the reskin risk with a shared token file):** every existing
  token NAME is retained — only additions (`--font-display/-body/-mono`, `--brand-ink`,
  `--staff`). So every component re-skins automatically; nothing references a deleted
  var. `tsc -b studio` clean.
- **Pixels, both themes (on the CORRECT build — see the process note):** light is the
  intended warm paper-and-ink — cream `#f7f4ee` ground, warm surface cards, **serif
  display headings** (Georgia fallback on this Linux host; the Iowan/Palatino stack
  degrades gracefully as designed), indigo accent kept as the single brand. Dark reads
  as a dim "stage" — warm-black ground, warm-tinted surfaces, a lighter indigo
  (`#a5b4fc`) readable on dark, legible muted text. Nav/cards/buttons/links all intact.
- It's genuinely just the foundation — **this is 1/N**; the per-page layout sweep (page
  headers, form grids, de-crammed setlist rows, the `--maxw` 880→1040 payoff, the
  `.mono` musical-data treatment) lands in following commits and gets reviewed then.

**Process note (a self-catch worth recording):** my first pixel pass showed the OLD
cool palette — because the review worktree was still at `172c69f` (pre-reskin) and I'd
only `git fetch`ed, not pulled, so Vite served stale `styles.css`. Caught it by the
pixels not matching the diff, pulled to `d56aea1`, re-shot. Same lesson as the port-8093
collision: **confirm you're exercising the reviewed SHA's bytes before trusting a
screenshot.** Also: 8080 was another lane's live `troubacore` throughout — screenshots
ran on an isolated `:8092/:5175` stack; that server was left untouched.

Citation present. CI on the landing being watched; a red gets its own entry. The
following redesign commits will each get a pixel pass as they land.

## 2026-07-08 — Studio reskin 2/N: Setlist page (`f78e78b`, landed per VLL): ✅ APPROVED — layout sweep verified, e2e + pixels, both themes

The first page migrated to the new identity (the worst-crammed one). +105 lines of
additive `styles.css` primitives (`.phead`/`.panel`/`.form-grid`/`.rows`/chip
modifiers/`.sig`/`.icon-btn`), and a 531-line `SetlistDetail.tsx` rework. Reviewed on
the reviewed SHA (pulled first this time):

- **Testid preservation — the real risk, since this is the T23 file and two specs
  (encore-bench, setlist-duplicate) were left unchanged.** Confirmed BOTH ways: static
  (every testid those specs need is present — `item-row`/`bench-row` are the computed
  `data-testid={group === "bench" ? …}`, the rest static; `setlist-name`/`nav-setlists`
  live on the untouched list page) AND empirical — **flows + encore-bench +
  setlist-duplicate 12/12 green** on an isolated stack, `tsc -b studio` clean.
- **The one interaction change is sound:** per-song key/tempo/notes moved from crammed
  inline fields into an `item-edit` inline editor (new toggle). Only `flows.spec`
  needed updating (its key-override test opens the editor, sets, reloads, re-opens to
  read back) — and it passes; the other specs don't touch overrides so they're
  correctly untouched.
- **Pixels, both themes:** the `.phead` header (breadcrumb · mono "SETLIST" eyebrow ·
  serif title · "3 songs"/"1 on call" chips · faint staff `.sig` line), a two-column
  Details form grid, a roomy "Running order" with a tidy ✎/↑/↓/★/✕ icon cluster per
  row, a distinct amber "★ BENCH · ON CALL" section with its explainer, and
  Duplicate/Bake/Delete as panels. Bench numbering stays independent (★ The Open Road,
  un-numbered — T23 semantics intact under the new layout). Dark "stage" theme
  coherent. Matches the approved mockup.

Citation present. CI watched; a red gets its own entry. Remaining redesign commits
(other management pages) each get this same e2e-plus-pixels pass as they land.

## 2026-07-08 — Studio reskin 3/N: Song details & files (`18dd62b`, landed per VLL): ✅ APPROVED — e2e + pixels, both themes

Third page migrated, reusing the 2/N primitives. Metadata → a `.panel` with a
two-column `.form-grid` (mono uppercase labels, a `bpm` affix on Tempo, save bar);
Files → a `.panel` with a mono pool count, the "New text chart" head action, and the
pool rendered as a responsive `.file-grid` of cards (thumb · name · "text chart" badge ·
size · ↑/↓ · Edit source · Rename · ✕) instead of cramped rows.

- **Testids preserved (this hosts the T19 chart editor):** confirmed static (all
  `meta-*`/`file-*`/`new-text-chart`/`file-chart-badge`/`files-list`/`file-row`/chart-
  editor present; the specs' `edit-canvas`/`pdf-page`/`conn-status` live in the untouched
  Viewer) AND empirical — **text-chart + box-render + flows 13/13 green** on an isolated
  stack, `tsc -b studio` clean. No e2e edits were needed (metadata + upload interactions
  unchanged), matching the commit's claim.
- **Pixels, both themes:** the Details form grid + `bpm` affix + mono labels, and the
  Files card grid with the generated chart's "text chart" badge, render per the mockup
  in light and dark. The annotation Viewer/toolbar above is correctly out of scope for
  this commit (it inherits the 1/N tokens; its markup redesign, if any, comes later).

Reviewed on the reviewed SHA (pulled first). Citation present; CI watched, a red gets
its own entry. Pattern is now steady across 2/N–3/N: additive primitives, per-page
markup, testids held, e2e-plus-pixels each time.

## 2026-07-08 — Reskin 4/N: setlist drag-to-reorder (`a30eb92`, landed per VLL): ✅ APPROVED — a real interaction, logic + e2e verified

Not just a reskin — a functional add: a grip handle (⠿) drag source per row, rows as
drop targets, group-scoped reorder persisted via the existing `ReorderSetlist`.
Reviewed the logic, not just the look, since this one has behavior:

- **Reorder correctness (by read):** `onDropRow` guards cross-group (`d.group !==
  group`) and self-drops; splices within the dragged group's array by group-local
  index (same indices the ↑/↓ `onMove` already uses); reassembles main-then-bench and
  sends the full id list. Consistent with T23 — the server's `Setlist()` re-sorts by
  (onCall, position), so grouping survives even though positions span the whole list.
- **Accessibility/fallback preserved:** the ↑/↓ buttons stay as the keyboard path
  (HTML5 drag isn't keyboard-operable), grip carries an `aria-label`; cross-group moves
  remain on ★ / "To order". Right call to keep both.
- **Empirical:** `tsc -b studio` clean; **setlist-dnd (new) + encore-bench +
  setlist-duplicate + flows 13/13 green** on an isolated stack — the new DnD spec
  proves drag-3rd-to-top renumbers and persists across reload, and flows' ↑/↓ reorder
  test still passes (the grip didn't disturb row testids).

**One non-blocking nit (recorded, not gating):** `onDragOver` sets the `.drag-over`
highlight on ANY hovered row including a different-group one, but such a drop is a
no-op (the guard rejects it) — so a cross-group drag shows a drop affordance that
won't act. Cheap future polish: skip the highlight when `dragRef.current?.group !==
group`. Harmless today (cross-group is the ★/"To order" path by design).

Citation present. CI watched; a red gets its own entry. Reviewed on the reviewed SHA;
isolated `:8092/:5175` stack, another lane's `:8080` untouched.

## 2026-07-08 — Reskin 4/N DnD down-move fix (`222b6c0`, landed per VLL): ✅ APPROVED — and a REVIEWER MISS owned

VLL hit a bug in the 4/N drag-reorder I approved one commit earlier: dragging a song
UP landed correctly, dragging DOWN dropped it one slot too low. Classic splice-shift:
`onDropRow` removed the dragged item then inserted at the hovered row's ORIGINAL
index, but for a downward move the removal shifts the target up by one. Fix is exactly
right — `insertAt = d.index < to ? to - 1 : to` — and it also closes the cross-group
highlight nit I flagged at the 4/N gate (a `canDrop` predicate now gates both the
`onDragOver` hint and the drop to the dragged item's own group). Verified: `tsc -b
studio` clean; the DnD spec — **now asserting BOTH directions** — + encore-bench green
on an isolated stack.

**The miss is mine, and worth recording plainly.** At the 4/N gate I wrote "reorder
correctness (by read) … splices within the dragged group's array by group-local
index" and called it correct — but I did not mentally execute the *downward* case,
where remove-then-insert-at-original-index is off by one. And I leaned on "setlist-dnd
13/13 green" when that spec only exercised drag-3rd-to-**top** (an up-move) — the exact
direction without the shift. Two gaps compounding: an incomplete logic trace AND a
test whose single case missed the bug, and I didn't notice the test only covered one
direction. **Lesson (added to the checklist): for any index/reorder logic, trace BOTH
directions by hand, and confirm the e2e covers both before citing it as proof** — a
green reorder test that only moves one way proves almost nothing. This is the same
class as the "22 pages looked fine" content-plausibility miss: the artifact passed a
check that wasn't actually testing the failure mode.

Good outcome — the lane's own test now covers both directions, so the regression
can't return. Citation present; CI watched.

## 2026-07-08 — Studio reskin 5/N: Band overview + Invites (`cf1be57`, landed per VLL): ✅ APPROVED — e2e + pixels, both themes

The member/invite-facing surfaces migrated (admin Settings-tab management is the
stated final commit). BandDetail + Invites reworked to the shared primitives (page
header, `.panel` Members/Songs cards with headed counts + a new `.panel-toolbar`
disclosure strip, member rows with avatar · @handle · role chip · reset action).

- **Testids preserved** (this hosts T22-ordered invites, the T21 reset action, and
  the invite flow): precise grep confirms every band/invite testid the specs use is
  present (`band-title`/`my-role`/`members-list`/`member-row`/`invite-toggle`/`-form`/
  `-identifier`/`-submit`/`invite-notice`/`songs-*`/`invites-*`); `join-accept`/
  `join-decline` correctly live in the untouched invite-link `Join.tsx`, not here.
- **Empirical:** `tsc -b studio` clean; **flows + identity + password-reset 17/17
  green** on an isolated stack — role changes, invite→accept, invite-link join, and
  the reset-link issue/consume flow all pass on the redesigned pages.
- **Pixels, both themes:** page header, panelled Members/Songs with disclosure
  toolbars, member row with role chip + "Reset password…", serif song links (T22
  order intact — Hallelujah before Wonderwall). Dark coherent. Matches the mockup.

Reviewed on the reviewed SHA; isolated `:8092/:5175`, `:8080` untouched. Citation
present; CI watched. Redesign now 5/6 by the lane's own count — the admin Settings
tab (invite-link management, member roles) is the last one.

## 2026-07-08 — Studio reskin 6/N: Band Settings (`249a90b`, landed per VLL): ✅ APPROVED — the sweep is COMPLETE

The last page. Pure markup migration (no `styles.css` change — reuses the existing
primitives): BandSettings + InviteLinks move to headed `.panel`s under a page header,
all five sections (Band name · Members role-selects · Pending invites · Invite links ·
Danger zone).

- **Testids preserved:** every settings/invite-link testid the specs use is present
  (`settings-title`/`-my-role`/`settings-members-list`/`settings-member-row`/
  `member-role-select`/`member-remove`/`leave-band`/`invite-revoke`/`invite-link-*`/
  `create-invite-link`/`delete-band`, and `rename-form`/`-save`/`-notice`). The
  commit's "rename-*" is loose wording — there is no `rename-name` and no e2e uses any
  `rename-*`, so nothing depends on it.
- **Empirical:** `tsc -b studio` clean; **flows + identity + password-reset 17/17
  green** on an isolated stack — settings-tab role management, invite-link create/join,
  and reset all pass.
- **Pixels, both themes:** all five panels render per the mockup; the invite-link QR
  keeps its own light card (stays scannable on the dark "stage" ground), URL in mono,
  metadata line intact. Reviewed on the reviewed SHA; `:8080` untouched.

**Redesign arc COMPLETE (1/N–6/N):** tokens+type (`d56aea1`) → Setlist (`f78e78b`) →
Song details & files (`18dd62b`) → setlist drag-reorder (`a30eb92`, + down-move fix
`222b6c0`) → Band overview + Invites (`cf1be57`) → Band Settings (`249a90b`). The
whole studio management surface now wears the warm "concert program" identity — serif
program voice, monospace musical data, paper-and-ink neutrals, a dim-stage dark theme —
verified page-by-page with e2e-plus-pixels in both themes, every testid held, one
real bug (the DnD down-move) caught by VLL and fixed with a both-directions test, CI
green throughout. Citation present on every commit.

## 2026-07-08 — ❓ DESIGN REVIEW REQUEST for arch (from Web-Core): canvas-first editor

VLL asked for a redesign of the **editor** (the annotation Viewer), then explicitly
asked to have it reviewed by you before I build. Proposal doc:
[`proposals/editor-canvas-first.md`](proposals/editor-canvas-first.md) (full
description; a 4×-iterated mockup was shown to VLL in chat).

The gist: full-viewport **canvas-first** editor — floating, centered/width-capped top
bar; **contextual toolbar** (style options only when a draw tool is active; a floating
toolbar by a selected object); Layers/Annotations as a **top-collapsing dropdown**
(not the always-on sidebar); parts strip + status in a floating bottom bar; **plain
wheel scrolls, Ctrl/⌘+wheel zooms toward the cursor** (non-passive listener +
preventDefault to suppress browser zoom; pinch = ctrlKey wheel). Responsive tested
desktop/tablet/phone; editor stays desktop/tablet-first (phone = the Stage app).

**This is the T17 / T15 attended territory**, so I'm not implementing until you weigh
in. Specific asks (also in the doc): (1) direction OK, or prefer a left tool-rail
variant? (2) does this **supersede T17** (and pair with T15) — spec it as such? (3)
the invariants I must not break — no-reraster-on-edit (`pdf-render-count`),
no-reflow/zero-shift (T17 asked for a zero-shift e2e FIRST — I'll write it), the
render-timing cancel-guard, all editor/viewer testids — anything I'm underweighting
before **stage 1 (scroll + Ctrl/⌘-wheel zoom)**?

Planned staging: (1) scroll + wheel-zoom, (2) contextual toolbar, (3) fullscreen
layout + collapsing panel — each its own reviewed commit. Held for your ruling.

## 2026-07-08 — Canvas-first editor: ✅ DESIGN REVIEWED — GO (T27 specced; supersedes T17, sequenced with T15)

Strong raise — held for review, staged, and it flagged its own load-bearing invariants
(exactly the T15/T17 discipline). Verified the technical premises against `Viewer.tsx`,
not just the prose, then ruled. Answers to the three questions (full rulings in
`docs/tasks/T27-canvas-first-editor.md`):

1. **Direction: GO as proposed** — canvas-first, floating centered/width-capped top
   bar, contextual style/selection toolbars, top-collapsing Layers/Annotations
   dropdown. **Keep the top bar; do NOT build the left tool-rail variant** — VLL
   validated the top-bar mockup over 4 iterations, and a rail is speculative until the
   floating bar actually proves cramped (file it then, not now).
2. **Supersedes T17; pairs with T15.** T17 (disclose the style bar to hit ≤220px) was
   the narrow fix for the same root problem — **T17 is CLOSED, folded into T27**, and
   its hard requirement carries over verbatim: **the zero-shift e2e is written FIRST**
   (before stage 3). **T15** (Viewer hooks split) **lands before stage 3** so the
   fullscreen rework builds on the split, not the monolith; stages 1–2 don't need it.
3. **One invariant underweighted, confirmed by reading the code — zoom re-raster
   thrash.** The PDF render pass is keyed on `scale` (`Viewer.tsx:831`); today's zoom
   is a discrete select so it re-rasters once, but continuous Ctrl/⌘-wheel would fire a
   poppler render **per tick** and churn the flip-fix cancel-guard. **Stage 1 must
   decouple visual zoom from rasterization:** live CSS-transform zoom + commit the crisp
   raster only on wheel-settle (debounce/coalesce) — a fast pinch = ONE raster, not
   dozens. And compute the zoom-to-cursor scroll against the container geometry
   synchronously (the canvas re-renders async). Everything else in the proposal's
   invariant list is right; note that no-reraster-on-edit is about EDITS — zoom
   re-rastering is expected, so add a test that a post-zoom edit still doesn't re-raster.

Staged commits, each reviewed + attended (render-timing is environment-sensitive — the
flip bug proved headless can't see everything). T27 is ready; stage 1 (scroll +
wheel-zoom) can start. The proposal doc stays on `main` under `docs/handoff/proposals/`
for history; T27 is the authoritative spec.

## 2026-07-08 — T27 stage 1 (`b3179b6`): ✅ APPROVED ON SUBSTANCE · ⚠️ CI web RED — real tsc breakage, fix-forward assigned

The wheel-zoom itself is exactly right and my flagged invariant is honored — but the
commit's "tsc -b clean" claim is **wrong**, and CI proves it: the **web job FAILED**
on the landing (go/proto/android green, e2e in flight, web ❌). Main is currently red.

**Functional substance — APPROVED (verified, not trusted):**
- Read the logic: non-passive `wheel` listener bound once via ref-indirection (cleaned
  up on unmount), `preventDefault` only on the ctrl/meta branch, live CSS transform on
  `.viewer-content` during the burst, `commitWheelZoom` on a 120ms settle that bakes
  canvas CSS sizes, clears the transform, re-anchors scroll against the container's
  real `scrollWidth/Height` **synchronously**, then `setZoomMode` ONCE. Render-effect
  deps unchanged ⇒ edits still don't re-raster.
- **Behavior confirmed by running the specs** (Playwright transpiles via esbuild, so
  they run despite the tsc break): `editor-wheelzoom` + `editor-noflicker` 3/3 green —
  an 8-tick Ctrl+wheel burst bumps `pdf-render-count` by exactly the page count (ONE
  raster pass, not 8 — the zoom-thrash invariant I required), and a post-zoom edit
  still does not re-raster. This is precisely the stage-1 acceptance.

**The RED — diagnosed:** `editor-wheelzoom.spec.ts:76,129` construct `new
WheelEvent(...)` inside `.evaluate()` callbacks. Those run in the BROWSER (WheelEvent
exists there), but the spec is typechecked under `tsconfig.node.json` whose `lib` is
`["ES2022"]` — **no DOM** — and that project `include`s `e2e`. So `tsc -b studio`
(the repo's canonical typecheck, and CI's web job) throws `TS2304: Cannot find name
'WheelEvent'`. Playwright never typechecks, which is why the run is green and the
claim slipped through — **"tsc -b clean" was almost certainly checked at a scope that
skipped e2e.**

**Fix-forward (web-core, XS — urgent, main is red):** give the e2e its DOM lib. Cleanest
is a `tsconfig.e2e.json` (`lib: ["ES2022","DOM","DOM.Iterable"]`, `include: ["e2e"]`)
referenced from the solution, so `e2e` leaves `tsconfig.node.json` (keeping vite/
playwright config Node-pure); or simply add `"DOM","DOM.Iterable"` to
`tsconfig.node.json`'s lib. Then `tsc -b studio` clean + web green. **T27 stage 1 is
approved on substance and closes when web is green** — no functional rework needed.

**Process note:** the standing typecheck command is `tsc -b studio` (README §4), and
it now covers `e2e/`. Run exactly that before claiming clean — a narrower `tsc` scope
missed an e2e-only DOM-lib error and landed a red main. Same lesson as the DnD
down-move: cite the check you actually ran.

## 2026-07-08 — T27 stage 1 web-red fix-forward (`f72d7dc`): ✅ APPROVED — main green again, stage 1 CLOSED

Exactly the cleaner option I prescribed: a dedicated `tsconfig.e2e.json` (`lib`
ES2022 + DOM + DOM.Iterable, `include: ["e2e"]`) referenced from the solution, so
`e2e` leaves the Node-pure `tsconfig.node.json`. Re-verified the check that was red,
first-hand: **`tsc -b studio` clean** (+ ink + bake `--noEmit` clean). Bonus — turning
on real DOM types surfaced two latent `any`-masked holes the missing lib had hidden,
both fixed here (viewer.spec's `CanvasLike` shim replaced with the real Element type;
editor-layers' nullable `elementHandle()` null-asserted before
`compareDocumentPosition`) — a net type-coverage gain, not just an unbreak. Re-ran the
touched specs on the isolated stack: editor-wheelzoom + viewer + editor-layers **13/13
green**; no functional change to stage 1.

**T27 stage 1 is now fully closed** — the wheel-zoom (approved on substance in the
prior entry) plus a green typecheck. The lane's commit message logs the "tsc scope
skipped e2e" lesson itself. Watching CI on `f72d7dc` to confirm web flips green; a red
gets its own entry. Stage 2 (contextual toolbar) may proceed.

## 2026-07-08 — ❓ ARCH DECISION REQUEST for arch (from Web-Core): per-object z-order (T27 stage 2)

Starting T27 stage 2, VLL asked (chat) to **build full z-order now** in the selection
toolbar (color / **z-order** / duplicate / delete). It isn't UI-only: `Object` has **no
order field**, rendering is layer-major + insertion-order within a layer, and there's
**no reorder mutation** — so z-order is a **proto + core + sync** change that touches the
LWW model and brushes the **R7** note in `object.proto`. Per the proto/data-model → arch
rule (T23/CFG01 precedent), held for your ruling. Proposal:
[`proposals/object-zorder.md`](proposals/object-zorder.md).

Gist of my proposed design: add `int32 Object.order` (within-layer only — R7 governs
*layer/zone* stacking, orthogonal to within-layer object order), render sorts by it
(tiebreak `created_at`/`uuid`), a new **`reorder`** mutation (gated + LWW like
move/resize) exposing only **bring-to-front / send-to-back** (order = max+1 / min−1),
both repos persist it, back-compat via default 0.

Four questions in the doc — the load-bearing ones: **(Q1)** OK to add `Object.order`
(within-layer, gated, LWW) w.r.t. R7? **(Q4) sequencing/shift:** the spec's other half,
*"style row only when a draw tool is active"*, **shifts the canvas** in the current
stacked layout; the floating chrome that makes it zero-shift is **stage 3** (gated on
T15). I propose **stage 2 = floating selection toolbar (absolute, no shift) + z-order +
duplicate + color**, and the **style-row auto-hide defers to stage 3**. Endorse, or
accept a transient shift now? Not implementing until ruled.

## 2026-07-08 — Per-object z-order (T27 stage 2): ✅ DECIDED — all four Qs; resolved design written into T27

Right to raise it — z-order is a proto/data-model + sync change, not UI-only, so the
proto→arch rule applies (T23/CFG01 precedent). Verified the premises against the tree,
not the prose: `Object` fields run 1–10 (so `order = 11` is free), `Layer.order = 6`
is the exact parallel, and **R7** (`object.proto:33,43`) governs *layer/zone* stacking
("each owner orders only WITHIN their own zone") — it says nothing about ordering
*within a single layer*. The reading is correct; the design is sound.

**Q1 — add `Object.order` (within-layer, owner/RW-gated, LWW via `version`)? R7 OK?**
**YES.** Within-layer object order is orthogonal to R7 — objects in one layer already
share that layer's z-band, so ordering them among themselves crosses no zone and
reintroduces no global contention. Add `int32 order = 11;`, gated exactly like
move/resize (active editable layer, owner/RW), LWW on `version`.

**Q2 — int + front/back bumps vs fractional/float order? int.** For the only exposed
ops (bring-to-front = maxSibling+1, send-to-back = minSibling−1) int is sufficient;
fractional midpoint-insert only pays for arbitrary drag-reorder, which is explicitly
out of scope — don't build it speculatively (same principle as the left-rail/App()-nav
deferrals). Note: two concurrent "bring-to-front"s can compute the same `order`; under
LWW that's fine — equal order resolves deterministically by the `created_at`/`uuid`
tiebreak (no crash, stable), and ±1 bumps from live extremes can't realistically
overflow. Revisit fractional only if arbitrary reorder is ever requested.

**Q3 — distinct `reorder` mutation kind vs carrying `order` on the update path?**
**Distinct kind** — clarity + telemetry + an explicit gate/LWW path, consistent with
create/move/resize/setStyle/delete each being their own kind.

**Q4 — stage-2 shift: ENDORSE the resequencing; do NOT accept a transient shift.**
The score-never-shifts invariant is the spine of the whole editor track (T05→T13→T17,
which died twice on zero-shift) — a transient canvas shift on tool-activate would
regress exactly that, for a cosmetic interim. So: **stage 2 = the floating selection
toolbar (`position:absolute`, no shift) + z-order + duplicate + color**, all no-shift;
the **style-row auto-hide (which shifts the stacked layout) moves to stage 3**, where
the floating chrome makes it zero-shift by construction. Duplicate + the toolbar shell
are unblocked immediately; z-order lands its proto/core/sync data-model first, then the
button consumes it.

Resolved design + resequencing written into `docs/tasks/T27-canvas-first-editor.md`
(stage 2 section) so the executor doesn't re-derive it. The proposal doc stays under
`proposals/` for history. Not a new task number — this is T27 stage 2.

## 2026-07-08 — T27 stage 2 part 1/2: z-order data model (`c243c80`, landed): ✅ APPROVED — one non-blocking tiebreak note for part 2

The backend half of the z-order decision, no UI yet. Re-verified the load-bearing
bits first-hand, not the report:

- **Kind enum — persistence-safe (the highest risk):** the full domain block is
  Unspecified…LayerDelete then **`KindReorder` APPENDED last (value 12)** — no existing
  iota value shifts, so file/git logs (which persist Kind as an int) stay valid. The
  12-vs-proto-`KIND_REORDER=8` gap is a non-issue: the wire maps by **string**
  (`kindFromString`/`kindToString`), never an int cast. The append-with-rationale
  comment is exactly right.
- **Gated + LWW, proven:** `TestWSReorderOwnObjectPersists` (owner reorder persists +
  echo/HEAD carry `order`) and `TestWSReorderForbiddenOnForeignROLayer` (foreign-RO →
  "forbidden") both green — reorder rides the same `canWriteLayer` gate + `version`
  bump as move/resize. `fold.go` replays it as an in-place object replace. All three
  store backends green (`order` round-trips). Proto `Object.order = 11` as approved.
- **Pick↔paint unified (a genuine latent-bug fix):** one `compareObjectZ` (layer rank
  → order) now drives BOTH the dry paint order and the wet pick order — previously
  pick used raw array order while paint used layer order, so "what's on top" and "what
  a click selects" could disagree. Editor e2e 19/19 (pick incl. locked-object
  hit-testing, noflicker, wheelzoom, layers) green — the unification regressed nothing.
  `tsc -b studio` clean.

**Non-blocking note for part 2/2:** the z-order decision + T27 spec call for the
within-layer tiebreak to be **`order` → `created_at` → `uuid`**; `compareObjectZ`
stops at `order` and leans on JS stable-sort + insertion order for equal-`order`
objects. Functionally identical in the common case (the render array is log-ordered
and consistent across clients), but it's a silent deviation from the spec's explicit
tiebreak and less robust if two objects ever share `order` (e.g. concurrent
bring-to-front) while array order isn't guaranteed identical between passes. Cheap
fix: add `created_at` then `uuid` to `compareObjectZ` (and the Go render comparator if
one exists) — fold it into part 2/2 so the tiebreak matches the spec and is
array-order-independent. Not gating; behavior is correct for every existing/seed doc.

Part 2/2 owes: the floating selection toolbar UI (color · z-order bring-front/
send-back · duplicate · delete) consuming this model — absolute-positioned (no shift),
per the Q4 ruling. CI watched.

## 2026-07-08 — T27 stage 2 part 2/2: floating selection toolbar (`597daa8`, at the gate): ✅ GO TO LAND at 5/5

The UI half, correctly **held at the gate** (branch-only). Reviewed the branch head
(`b77fb71`; `597daa8` is its pure rebase onto the part-1 verdict — diff-of-diffs empty,
so this review carries over verbatim):

- **No-shift, per the Q4 ruling — confirmed:** `SelectionToolbar` is `position:
  absolute` (z-index above the wet canvas), anchored at the selected object's bbox and
  rendered by WetCanvas over the canvas; it stops `pointerdown` reaching the canvas so
  clicking it never starts a marquee. Zero layout shift by construction.
- **Wiring per the ruling:** `reorderSelected` computes over same-layer+**same-page**
  siblings (front = max+1, back = min−1), no-ops on empty/already-there, gated by
  `isObjectEditableNow` (owner/RW/active layer) — as are color and duplicate; delete
  reuses `deleteSelected`. Sends the part-1 `reorder` mutation.
- **Verified on the branch head:** `tsc -b studio` clean; the new
  `editor-zorder.spec.ts` green via **overlay-PIXEL sampling** (bring-to-front actually
  flips the overlap colour in the rendered output — real z-order end to end, not just
  state) + duplicate/recolor/delete counts + toolbar-hides-on-deselect; `editor-pick` +
  `editor-noflicker` green (no pick/selection/no-reraster regression). 9/9 in my
  subset; the lane's full editor suite is 41.

**Tiebreak note (from part 1) — still OPEN, non-gating:** `compareObjectZ` untouched, so
the within-layer tiebreak stays `order` + stable-sort rather than the spec's `order →
created_at → uuid`. Fine to land (correct for every real doc), but the explicit tiebreak
stays owed — fold into stage-3 or a small cleanup; don't let it rot.

**GO to land `597daa8` at ubuntu 5/5** (bare task branch → no branch check-runs; the
push to main gates it). With this, **T27 stage 2 is complete** (data model + toolbar).
Stage 3 (fullscreen floating layout + style-row auto-hide) is next but **gated on T15**
(Viewer split) — both attended.

## 2026-07-08 — T27 stage 2 landed + tiebreak fix (`e3ffc72` + `ebc481b`): ✅ APPROVED — stage 2 CLOSED, my open note closed too

Both stacked on my GO verdict and landed. Reconciled: part 2/2 landed as `e3ffc72`
(the rebase of the `b77fb71` I GO'd — patch-identical), and the lane then **folded in
my non-blocking tiebreak note** as `ebc481b` (reviewed on branch head `f3927b0`,
diff-of-diffs empty vs the landing).

The tiebreak commit is better than a one-liner — it surfaced that `Object.CreatedAt`
(proto field 10, long declared) was **never actually stamped** on the create path
(always 0), so the tiebreak field my note assumed was inert. The fix:
- `compareObjectZ`: `order → createdAt → uuid` — a total order, array-position
  independent (equal-`order` objects stack deterministically across clients/passes,
  robust to concurrent bring-to-front). Matches the spec exactly.
- **Server-stamps `CreatedAt` once on create** (`handleMutation`: `if o.CreatedAt == 0
  { o.CreatedAt = m.ClientTS }` — first write wins), kept OUT of LWW (stays
  version+authorId). Non-create mutations carry the client's already-stamped value, so
  reorder/move preserve it; legacy/seed `createdAt=0` falls to the uuid tiebreak
  (deterministic; demo regenerable). Wired on both WS + REST DTOs + all four mappers +
  `AnnotationObject`.

Re-verified fresh on the branch head: `go vet` + core store suites (all 3 backends) +
httpapi (WSCreate stamps + round-trips; Reorder persists order+createdAt) green; `tsc
-b studio` clean; `editor-zorder` (pixel-sampled render z) + `editor-pick` green.

**T27 stage 2 is CLOSED** — z-order data model + createdAt-tiebroken render + the
floating no-shift selection toolbar, all landed and verified, my one open note now
resolved. Stage 3 remains gated on T15; both attended. CI on `ebc481b` watched.

## 2026-07-09 — B08 bake rev-claim race (`86f1edd`, landed): ✅ APPROVED — fixes the filed bug · one narrow tail filed as B09

The B08 I filed on 2026-07-07 (from a chat-only flake report), implemented well.
Re-verified against `baker.go`, not the report:

- **Primary fix correct + provably safe:** the claim loop now stats the published
  `<rev>` dir before `Mkdir(<rev>.tmp)` and bumps on either; the publish `rename` is
  the atomic arbiter — on a target-exists collision (detected by `stat`, robust to
  EEXIST vs ENOTEMPTY) it re-claims a higher rev, rewrites bundle.json (ConcertRev) +
  the `.tstage`, and retries instead of failing. The 2-bake case cannot clobber because
  `<rev>.tmp` is exclusive, so only its holder ever writes `<rev>.tstage`. The
  `afterNextRev` test seam drives the exact window deterministically
  (`TestBake_PublishReclaimsOnConcurrentPublish` — fails on pre-B08 code). **Ran the
  reclaim + concurrent-guard tests 50× under `-race`: green** (214s); go test ./...
  green per the commit. Single-publication-point + the accepted B04 stale-`.tmp`
  number-skip both intact.

- **One narrow tail I found, filed as B09 (not gating):** the *re-claim* inner loop
  picks the next rev by scanning only free published *dirs* (not `.tmp` claims) and
  does `os.Remove(tstagePath)` on a lost rename. So if TWO bakes both re-claim the
  SAME higher rev N, both write `N.tstage`, one wins `N/`, and the loser's
  `os.Remove(N.tstage)` can delete the winner's file — leaving `N/` with a removed or
  mismatched `.tstage`, which `downloadBundle` (`os.Open` on `<rev>.tstage` — the file
  is authoritative) would 404. Strictly narrower than the bug B08 fixed (needs ≥2 bakes
  racing the *same re-claimed* rev; multi-second real bakes make it near-unreachable),
  and NOT worth reverting B08 (which turns a hard failure into success) — but recorded
  so it doesn't rot. Fix options in `docs/tasks/B09-bake-reclaim-tstage.md`: two-phase
  `.tstage` publish (write temp → dir-rename wins → rename temp onto `<rev>.tstage`), or
  give the re-claim an exclusive `.tmp` claim. XS/S, slot when bake is next touched.

CI on `86f1edd` watched; a red gets its own entry.

## 2026-07-09 — B09 two-phase .tstage publish (`902ea34`, landed): ✅ APPROVED — B08 tail closed

Fast, and exactly option (a) from the B09 spec. Verified in `baker.go`: the `.tstage`
is staged under `stageDir + ".tstage"` (a name unique to the exclusively-held stageDir),
the `<rev>/` dir rename stays the atomic arbiter, and **only the dir-rename winner
renames its staged file onto the shared `<rev>.tstage`**; a losing re-claimer
`os.Remove`s only its own uniquely-named staged file — it can neither delete nor
content-mismatch the published one. The clobber/remove tail is closed. Test raised to
n=4 racers (>2 exercises the re-claim path) asserting every published rev keeps a
present `.tstage` + all revs distinct; I re-ran the bake reclaim/concurrent suite **30×
under `-race`** — green (124s); `go vet` clean.

**Observation (non-gating, noted not filed):** the two-phase is applied
*unconditionally*, so B04's "tstage strictly before dir" zero-window guarantee narrows
to a sub-ms dir-exists-before-`.tstage` window on **every** publish, not just the rare
re-claim — a download hitting that exact instant would get a transient, self-correcting
404. A strictly-better variant keeps tstage-before-dir on the non-reclaim path (where
`.tmp` exclusivity already makes it safe) and uses two-phase only on re-claim. Not worth
a task: the window is sub-ms, self-correcting, and bakes are infrequent/admin-triggered;
recorded here so it's known. With this, **B08 + B09 fully close the concurrent-bake
story** — concurrent same-setlist bakes always produce distinct, downloadable revs.

CI on `902ea34` watched.

## 2026-07-09 — T15 part 1/3: extract useSongSync (`4f26f02`, landed): ✅ APPROVED — behavior-preserving

First hook of the Viewer split (VLL cleared T15). Moves the realtime spine — the
per-song WS lifecycle + live `doc`/`visible` (with the on-wire layer-default merge)/
`connStatus`/`rejectNotice` + `defaultVisibility` — into `useSongSync.ts`, verbatim;
Viewer threads the hook's return as before (1549 → 1486 lines). For a refactor the bar
is behavior-identical, and the e2e net (T15's whole reason for being attended) is the
proof: re-ran on the isolated stack — **editor.spec realtime (two-client: A draws → B
sees without reload), noflicker (`pdf-render-count` unchanged on edit), zorder, pick,
viewer — 14/14 green**, `tsc -b studio` clean, testids untouched (diff is a move). The
sync-sensitive behavior survived the extraction, which is exactly what the realtime
spec exercises. Parts 2/3 (`usePdfDocument` — the PDF/zoom/raster/overlay chunk) + 3/3
(trim) follow; each gets the same e2e-net pass. CI on `4f26f02` watched.

## 2026-07-09 — T15 part 2/3: extract usePdfDocument (`118a591`, landed): ✅ APPROVED — the risky one, render-timing intact

The big, render-timing-critical hook — PDF.js load + per-page raster + the zoom model
+ the dry ink overlay — moved VERBATIM into `usePdfDocument.ts` (510 lines; Viewer
1486 → 972). This is where the flip-fix cancel-guard, the wheel-zoom decouple, and
no-reraster-on-edit live, so I leaned on the specs that specifically guard them:
re-ran on the isolated stack — **editor-wheelzoom (one raster per pinch),
editor-noflicker (`pdf-render-count` unchanged on edit), viewer (render+zoom), zorder,
editor realtime, pick — 16/16 green**, `tsc -b studio` clean, testids preserved. The
render effect's deps are unchanged (`[selectedFile, status, scale, numPages, zoomMode,
renderNonce]`) and render↔pick still share `compareObjectZ` — confirmed by the diff
being a move and by the two invariant-guarding specs passing.

**Honest deviation, accepted (T05 precedent):** Viewer landed at 972 lines vs the T15
spec's ~600 target — but that target was written when Viewer was 1,260 lines; T27
stages 1–2 (wheel-zoom + selection toolbar) added to it since, and the residual is the
editing handlers + JSX the spec deliberately keeps in the orchestrator. The lane
flagged it for the gate rather than contorting to a stale number — correct call. The
split's value (the sync spine and the PDF/render engine now isolated, testable hooks)
is delivered; part 3 can shave more via an optional `useDryOverlay` split if VLL wants,
but it's not required. CI on `118a591` watched.

## 2026-07-09 — T27 stage-3 zero-shift guard + T15 DONE (`55ba1e9`, landed): ✅ APPROVED — the guard is genuine

The lane wrote the **zero-shift e2e FIRST** (the T17-carried requirement that killed
T17 twice) before starting stage 3, and declared T15 done at part 2. Both right:

- **The guard is real, not a tautology — I proved it.** `editor-zeroshift.spec.ts`
  measures the first `.pdf-page`'s viewport box and asserts it's stable across (1)
  activating a draw tool (style row appears) and (2) toggling the Layers/Annotations
  panel. It's `test.fixme` (skipped) with a clear "flip to `test()` at stage 3" note.
  I stripped the fixme and ran it on current `main`: it **fails at line 95, the
  panel-toggle step** — the in-flow panel resizes the scroll column, shifting the
  canvas — exactly as the commit claims. So when stage 3 floats the chrome
  (`position:absolute`), the shift disappears and the guard goes green; a regression
  that reintroduces in-flow chrome will re-break it. A guard that fails for the right
  reason today is worth having.
- **T15 is legitimately DONE at part 2** (useSongSync `4f26f02` + usePdfDocument
  `118a591`; Viewer 1549→972, full editor e2e green throughout). Part 3 was the
  optional trim I already called not-required; declaring done here is correct.

Docs/test-scaffold only (the spec is skipped, so CI is unaffected). **Stage 3
(fullscreen floating layout + style-row auto-hide) is now unblocked** — T15 done + the
zero-shift guard written first, both prerequisites met. When stage 3 lands, the
close-out is: flip the fixme to a live `test()` and it must pass. CI on `55ba1e9`
watched.

## 2026-07-09 — ❓ ARCH DECISION REQUEST (from Web-Core): T27 stage-3 fullscreen conflicts with the e2e draw-helpers

VLL asked to land the full-viewport canvas-first layout (stage 3) with a live `:8080`
demo. I built it three ways and hit a hard, consistent conflict with the **unedited**
editor e2e suite — the stage-3 close-out requires "e2e green WITHOUT editing specs", and
every fullscreen lever trips an assumption baked into each spec's own draw/click helper:

- **Floating chrome over the canvas**: the specs' `dragOnPage`/`clickOnPage` do
  `scrollIntoViewIfNeeded()` then draw in a band from `max(box.y,0)` — the viewport top,
  where the floating bar sits → draws land on the bar. Pointer-events pass-through on the
  bar rescued editor.spec/pick/features but not the rest.
- **Floating Layers panel** (needed for panel-toggle zero-shift): overlays the top-right
  of the score where many pick/draw tests act → interception. Can't default it closed —
  the `editor-layers` specs assume it OPEN and never toggle it.
- **Viewport-height card**: with the stable-footprint style row the chrome is ~360px, so
  a viewport-constrained card leaves the scroll ~400px tall (was ~870px). The helpers
  compute their band against the FULL viewport, so mid/high-`fy` draws land below the
  now-short canvas → miss. This alone failed 11/44.

Net: a genuine fullscreen is **incompatible with the draw-helpers as written**. Reverted
the stage-3 layout; `main` stays clean (T15 + stages 1–2 + z-order, 44/44 green). What
landed: `editor-zeroshift.spec.ts` as a live guard for the part achievable now — the T17
invariant that activating/switching a draw tool never shifts the score.

**Decision needed** (stage 3 can't proceed under "no spec edits"): (a) sanction updating
the shared draw/click helpers to scroll clear of floating chrome + measure the band
against the *scroll* viewport (test-infra support for a real layout change); (b) keep the
panel a side column, chrome in-flow-but-compact (drop panel-toggle zero-shift); or (c)
re-scope stage 3. I recommend (a). Held for your ruling.

## 2026-07-09 — T27 stage-3 e2e-helper conflict: ✅ DECIDED — option (a), with an assertion-freeze boundary

Good raise, and the revert-and-ask was exactly right (don't loosen the safety net to
force a layout in). Ruling: **(a) — sanction updating the shared draw/click e2e
helpers**, because the distinction that matters is *mechanics vs assertions*:

- **What "no spec edits" was protecting:** the stage-3 close-out (and the whole
  T05→T13→T17 lineage) forbids *loosening assertions* — dropping/relaxing a check so a
  shifting or broken layout passes. That's the failure mode that killed T17.
- **What it was NOT protecting:** the test *mechanics* — HOW a helper scrolls to and
  clicks/draws on the canvas. When the UI legitimately changes from stacked to
  fullscreen-floating, those mechanics MUST adapt, and doing so keeps the SAME
  assertions verifying the SAME invariants. That's the T13 precedent ("update specs
  only when a flow genuinely changed") applied to shared helpers.

**Sanctioned, with hard guardrails (written into T27):**
1. **Assertions are frozen.** The e2e diff may change ONLY helper mechanics
   (scroll/measure/click-coordinate logic, panel open/close setup) — NOT any
   `expect(...)`. No assertion dropped, relaxed, or its tolerance widened. I will diff
   the specs and require the assertion lines be textually unchanged.
2. **Invariants stay proven under the new helpers:** no-reraster-on-edit
   (`pdf-render-count`), pick/cursor correctness, z-order (pixel-sampled), realtime,
   AND zero-shift — all still assert and still pass.
3. **Zero-shift goes fully live:** the panel-toggle assertion (deferred now) flips to a
   live `test()` as the stage-3 close-out — that IS the proof the floating chrome
   doesn't shift the score. (The draw-tool half is already live in `146d567`.)
4. **Concrete helper guidance** (so it isn't improvised): measure the draw band against
   the **scroll container's** client rect (chrome-inset-aware), not `max(box.y,0)` from
   the window top; scroll the target into view before acting when the card is short;
   and manage panel state explicitly — dismiss the (now-toggleable) dropdown for
   draw/pick specs, open it for `editor-layers` specs (its no-longer-always-open state
   is the intended stage-3 design, so those specs adapting to open it is a real flow
   change, allowed).
5. **Separability for trust:** a shared-helper change touches many specs at once, so a
   helper bug could mask a regression. Land the helper update legibly (its own commit
   if feasible), and I'll spot-verify a couple of specs *behaviorally in the new
   layout* (pixels), not just green ticks.

Rejected (b) (dropping panel-toggle zero-shift abandons VLL's "don't overlay the
score" invariant) and (c) (the design is sound; only the test mechanics need to
adapt). Stage 3 may proceed under these guardrails; the draw-tool live guard that
landed in `146d567` is approved (it asserts the real T17 invariant and passes today).

## 2026-07-10 — ❓ CODE REVIEW REQUEST (from Web-Core, at VLL's explicit ask): T27 stage-3 WIP branch — thorough opinion + go-forward

VLL asked me to bring the **actual stage-3 WIP code** back to you (not the approach — you
already ruled option (a) above; this reviews what I built under that ruling) for a thorough
opinion and a go-forward call, and to leave an **auditable written analysis**.

**Artifact:** `docs/handoff/T27-stage3-analysis.md` (landed with this entry) — full detail,
with reproduction commands for every claim.

**Branch:** `task/T27-stage3-fullscreen`, tip `3e9fe60`, rebased onto this `main`. Nothing
landed; `main` is untouched.

This is a **handoff of evidence, not a self-verdict** (VLL flagged that Web-Core grading
its own work isn't a review). Facts below; interpretation is left to you. Full detail +
repro commands in the analysis file, §A (facts) / §B (my claims to verify) / §E (questions).

**Raw facts:**
- **Diff:** 14 files, +217/−62; source 5 files (+105/−29), specs 9 files (+112/−33).
- **Assertion diff:** whitespace-normalized `expect(...)` set is identical `main` vs branch
  for all 9 specs (`ALL_FROZEN=1`); added spec lines are chrome-inset band top +
  scroll-into-view. **You to confirm** the freeze holds.
- **`tsc -b`:** clean.
- **e2e baseline (full run, `editor box-render viewer`): 35 passed / 10 failed (9.8m).**
  Seven of the ten are **30s timeouts**, including `editor-noflicker:93` (the no-reraster
  invariant) and core flows `editor.spec` draw-persist / realtime / select+delete, plus
  `editor-locked-restyle` ×3; the other three are small-rect drag, resize handles, preset
  draw. **Correction:** an earlier message said "4 failures" — that was a run I killed at
  ~test 20, not complete. The complete run is 10.
- **Zero-shift:** only the draw-tool half exists/passes; the panel-toggle close-out
  (guardrail #3) is **not written**.
- **Design gap:** the mockup's pill top bar / bottom parts+status bar / tabbed drawer are
  not yet matched. VLL's "flyouts don't look like the design" still stands.

**What I do NOT claim:** that the 10 failures are "just mechanics." Seven are timeouts,
which could hide a real interaction regression (overlay interception, target never
reachable) rather than a coordinate offset. I have not proven the invariants hold. §B lists
these as open hypotheses for you.

**Questions (§E of the analysis):** (Q1) design reshape → migrate helpers once, vs. land a
functional close-out on the current 10-red chrome first? (Q2) relocate
`active-layer`/`new-layer`/delete per the mockup (wider spec touch) or keep them
always-mounted? (Q3) is adapting specs to activate a tool before reaching style controls a
sanctioned flow change, or a sign the contextual-hide is too aggressive? (Q4) do you want a
trace/pixels pass on the seven timeouts before further work? (Q5) `MAX_FIT_SCALE = 2.3` OK?
Held for your ruling — no further branch work until you rule.

## 2026-07-10 — T27 stage-3 WIP (`3e9fe60`) FULL ARCH REVIEW: sound skeleton, honest freeze — but the baseline was understated; design ~40% there; go-forward RULED

Requested by VLL ("analyze what it did, gap vs the posted design, and all the broken
things"). Reviewed against the **artifact design** (claude.ai e2c2aac5…, = the
approved `editor-redesign.html`), with my own full-suite run and pixels of the WIP.

### A. What's built and holds (verified first-hand)
- `tsc -b studio` clean on the branch. **Assertion-freeze: CONFIRMED** — my own
  normalized diff of all 9 edited specs: every `expect(...)` set textually identical
  to main (ALL FROZEN). The helper edits are exactly the two sanctioned mechanics
  (chrome-inset band top via `viewer-chrome` bbox; scroll-into-view fraction helpers).
- The skeleton is right: fullbleed shell (navbar hidden on the editor route), floating
  glass chrome with `pointer-events:none` + auto-on-controls (drawing under the glass
  passes through — good), `--chrome-h` published by ResizeObserver, floating absolute
  sidebar (structurally zero-shift), contextual style-row hide, `MAX_FIT_SCALE = 2.3`.
- Passing on the WIP: **34/44**, including the invariant specs that matter most —
  noflicker, wheel-zoom one-raster, zeroshift draw-tool (live), pick suite, realtime,
  box-render, viewer.

### B. The broken things (the full list — the analysis's §4 named only 4 of 10)
**My run: 34 passed, 10 failed.** The six unlisted: `editor-locked-restyle`
(style-reflect), `editor-uxfix` #3 (thin-line pad), #4 (marquee multi-select),
#1+#2 (toolbar stable footprint), `editor-wheelzoom` (post-zoom edit no-reraster —
an INVARIANT spec, failing in its drag mechanics, not its assertion), and
`editor-zorder` (pixel z — draw placement mechanics moved the sampled overlap).
Classification:
- **8 are unmigrated helper mechanics** (ed5 #2/#5, features-resize, locked-restyle,
  uxfix #3/#4, wheelzoom-postzoom, zorder-pixel): drags/clicks computed against the
  window top or unscrolled geometry. Fix class = the already-sanctioned band/scroll
  math. The wheelzoom one MUST be re-proven green — it guards no-reraster.
- **2 collide with the approved design itself** (see D-Q3): `editor-layers` readouts
  (style row now contextual → activate a tool first; steps change, assertions don't)
  and `editor-uxfix` #1+#2 (asserts the toolbar's own footprint constant across
  none/text/shape — the contextual design deliberately breaks that).
**Functional regressions beyond tests (pixels + CSS read):**
1. **The Details & files section is unreachable** on the song route (CSS comment
   admits "intentionally clipped… moves into a Details toggle next") — metadata
   editing, upload UI, the T19 chart editor + T25 preview, rename/delete, danger zone
   all gone until that toggle exists. Not listed in the analysis's broken-things.
2. **Initial view puts the page top UNDER the chrome** (my screenshot: the score title
   half-hidden behind the glass at load) — the `--chrome-h` scroll-padding isn't
   landing the initial position below the bar.
3. Tool-activate grows the in-card style row → the bar covers ~310px of score (canvas
   doesn't shift — zeroshift passes — but coverage balloons; the design avoids this
   with the slim separate `.ctx` pill).

### C. Gap vs the posted design (artifact ground truth; WIP ≈ 40%)
1. **Top bar**: design = ONE slim pill (radius 999, ~52px: back · serif title · tool
   cluster · zoom% mono · Layers/Notes/Details pill-toggles). WIP = a 3-row rounded-
   rect card (~230px neutral, ~310px with a tool): header row + tools/layer-mgmt row +
   zoom/files row.
2. **Layer management** (active-layer select, ＋New layer, Delete, drawing-on pill)
   sits in the top bar; design puts it **in the drawer**.
3. **No separate `.ctx` pill** (slide-in below the bar, slim glass, swatches +
   Outline/Box/Highlight seg + width + layer chip); WIP inlines the old style row.
4. **No bottom pill bar** (parts strip: file tabs + "＋ Add file" · status "N objects
   · ● live"); WIP keeps file tabs + status in the top chrome.
5. **Drawer**: design = ONE tabbed glass dropdown (Layers | Annotations, collapse ▲,
   toggled from the top bar); WIP = the two old stacked cards + the legacy "Hide
   layers ▸" toggle.
6. No wheel-hint pill; zoom is the old −/select/＋ row, not the design's mono readout.
7. No ⓘ Details toggle (ties to regression B-1); phone rules (compact single row,
   sheet drawer, no selbar) not implemented.

### D. Go-forward — RULED (the analysis's Q1–Q5)
- **Q1 (ordering): reshape FIRST, then migrate helpers once against the final DOM** —
  VLL's stated priority, and it avoids a double migration. BUT nothing lands until
  green: the branch stays up; VLL previews from a branch-built `:8080`. Landing shape
  = a legible stack: (1) DOM/CSS reshape to the mockup, (2) helper-mechanics commit
  (its own commit, frozen assertions), (3) the two sanctioned spec updates (own
  commit, rationale in-message), (4) panel-toggle zero-shift flipped LIVE, (5) full
  suite green → land.
- **Q2 (control relocation): yes — follow the mockup.** Layer mgmt into the drawer,
  delete stays in the selbar. Specs opening the drawer first is sanctioned mechanics;
  every testid stays present/reachable.
- **Q3 (design-obsoleted specs): sanctioned, by name, two specs only.**
  `editor-layers` readouts: activate a tool first (steps only; assertions unchanged).
  `editor-uxfix` #1+#2: the stable-footprint assertion tests T05's *mechanism*; the
  invariant ("the score never shifts") is now guarded by the live zeroshift spec — so
  RETIRE/rewrite that assertion in the open, citing this entry (T17 precedent: the
  mechanism was never the invariant). The freeze still binds everything else.
- **Q4 (seven timeouts):** subsumed by B — they're the unmigrated-mechanics class;
  fix after the reshape, and the wheelzoom invariant spec must be green before land.
- **Q5 (`MAX_FIT_SCALE = 2.3`): fine** as a VLL-validated product default; keep the
  named constant; CFG01-style configurability only if ever asked.
- **Additional requirements:** the reshape MUST restore access to Details & files
  (the ⓘ Details toggle, or don't clip until it exists) — T19/T25 surfaces can't
  regress; and fix the initial-scroll position (page top starts below the chrome).

**Net judgment:** the skeleton and the test-integrity discipline are genuinely good
(freeze held, zero-shift live, invariants mostly green); what was missing is honesty
of the baseline (4 of 10 failures listed — same lesson as "cite the check you ran")
and the visual reshape itself, which is well-specified by the artifact and now has an
unambiguous sequence. Not stuck — unblocked.

## 2026-07-10 — ❓ FOLLOW-UP (from Web-Core, at VLL's ask): extend the assertion-retirement set — 2 assertions the approved reshape obsoletes beyond uxfix #1+#2

Executing your step-2 (mechanics migration to the reshaped DOM). The reshape landed
on the branch `task/T27-stage3-fullscreen` (pill top bar, `.ctx` style pill, tabbed
`Layers | Annotations` drawer, bottom pill, ⓘ Details) and renders per the artifact.
New `e2e/fullscreen-helpers.ts` centralizes the sanctioned mechanics (`clearBand`,
`scrollFracIntoBand`, `openDrawer`). **`editor-layers` is 8/9 — all mechanics fixes
work.** The remaining failure needs your ruling.

Two assertions are made **impossible or meaningless by the design YOU endorsed**
(tabbed drawer Q2/C-5; contextual style row) — and both are **beyond the two you
named** (uxfix #1+#2). Under guardrail #1 I have NOT edited them; I need you to
extend the sanctioned-retirement set (or redirect the design):

1. **`editor-layers.spec.ts:494`** — *"Layers panel renders ABOVE the annotation
   list in the DOM."* Asserts `layers-panel` AND `annotation-list` **both visible at
   once**, then checks DOM order. The tabbed drawer shows ONE tab at a time, so
   co-presence is impossible. It tested the OLD stacked-sidebar *structure* (a
   mechanism), not a product invariant — same class as uxfix #1+#2.

2. **`editor-rorw-shift.spec.ts:127`** — *"focusing a read-only layer does NOT shift
   the layout."* Asserts the `editor-toolbar` **boundingBox height** is identical
   across RO/RW focus. In the reshape `editor-toolbar` is `display:contents` inside
   the pill (no box) → the height metric is meaningless. The **real invariant** —
   the score's `pdf-page` top does not move on RO focus — still holds and I keep it;
   only the toolbar-height half is obsolete (again the footprint *mechanism*, now
   guarded live by `editor-zeroshift`).

**Ask:** extend the sanctioned retirements to include these two (retire/adapt in the
open, citing this entry), on the same rationale you gave for uxfix #1+#2 ("the
mechanism was never the invariant")? Or do you want a different treatment (e.g. keep
both panels stacked rather than tabbed — which would contradict the artifact)?

Everything else in the ~26-test reshape fallout is **pure mechanics** (drawer-open,
`clearBand`, scroll-into-band) — I'm proceeding on those unblocked; only these two are
held. The live panel-toggle zero-shift close-out (guardrail #3) is still to come.

## 2026-07-10 — Assertion-retirement set EXTENDED: ✅ the two named specs, verified — and the set is now CAPPED at four

Right to hold and ask. **Verified both against the specs on main, not the report:**

1. **`editor-layers:494` — retirement GRANTED.** The test's own comment states its
   purpose: keep the stacked sidebar's Layers panel positionally stable ABOVE the
   variable-length annotation list. In the tabbed drawer I endorsed (artifact, Q2/C-5)
   the two are never co-visible, so both the co-presence expects and the
   `compareDocumentPosition` check test a dead structure — same mechanism-not-invariant
   class as uxfix #1+#2. **Condition:** the drawer's function stays tested — the spec
   file must still prove Layers-tab content AND Annotations-tab content each reachable
   (via `openDrawer`/tab switch); retire only the co-presence/DOM-order test.
2. **`editor-rorw-shift:127` — PARTIAL retirement GRANTED, and only partial.**
   `measure()` returns `{toolbarH, pageTop}`. `toolbarH` on a `display:contents`
   toolbar is meaningless — retire that comparison. **`pageTop` is T13's actual
   protection (the score does not move on RO focus) and MUST remain asserted and
   green.** If the reshape ever makes `pageTop` fail, that is a real regression, not
   an obsolete assertion.

**The sanctioned-retirement set is now CLOSED at exactly four:** `editor-uxfix` #1+#2,
`editor-layers` readouts (steps only), `editor-layers:494`, `editor-rorw-shift:127`
(toolbarH half only). Any further "the design obsoletes this assertion" comes back
here BEFORE editing — the pattern so far is healthy (hold-and-ask, twice), keep it.
All four retirements land in the step-3 commit, in the open, citing their entries.

## 2026-07-10 — MOBILE ruling for the canvas-first editor (VLL ask): stage-3 stopgap REQUIRED + stage 4 (touch grammar) SPEC'D

Analyzed the new design's mobile story against the code, not assumptions. Three facts
compound: the wet canvas is `touch-action: none` (styles.css:504 — a finger never
scrolls), the viewport meta is `user-scalable=no` (no browser pinch; honored in
WebViews), and stage-3 fullbleed removes the gutters that made touch scrolling
possible today. **Net: the fullscreen editor as built is unscrollable AND unzoomable
on any touchscreen — including inside the Android app (EditScreen/A06 embeds this
exact route).** Stage 1's zoom listens only to `wheel`; touch pinch never synthesizes
wheel events (that is a desktop-trackpad convention).

Rulings (full detail written into T27):
1. **Stage 3 must ship a stopgap:** Select mode sets `touch-action: pan-x pan-y` on
   the canvas (one finger scrolls; draw tools keep none) + the mockup's phone
   breakpoint rules + a reduced-blur fallback for low-end WebViews.
2. **Stage 4 (new): the idiomatic touch grammar** — two fingers ALWAYS navigate
   (pinch zooms toward the gesture midpoint, feeding stage 1's live-transform +
   one-raster-on-settle pipeline); one finger is tool-modal; a second finger during a
   draw CANCELS the stroke (GoodNotes/Procreate idiom); **pen draws / finger
   navigates** via `pointerType` — deliberately the A07 stylus-spike test surface.
3. **Apps:** Android EditScreen inherits everything (stage 4 is a hard prerequisite
   for the in-app editor being mobile-usable — with `user-scalable=no` honored in the
   WebView there is NO zoom until it lands); native Stage untouched; iOS embedding,
   when it arrives, inherits the same grammar through the existing WKWebView seam.
   Fullbleed itself is a WIN in the app (no double chrome) — hardware-back behavior
   in EditScreen should be smoke-tested on the branch.

## 2026-07-10 — VLL field bug ("stabilo disappears") INVESTIGATED: hidden-active-layer swallow — T28 filed with reproducer

VLL: a Highlight drawn on the Open Road PDF shows while drawing, vanishes at stroke
end, stays gone after a tool change. Investigated empirically with a pixel-sampling
reproducer (wet-canvas alpha mid-stroke; dry-overlay alpha at t0 post-stroke / t1
post-echo / t2 post-toolswitch), run across a variant matrix on BOTH `main` and the
stage-3 branch tip:

- Fresh-layer, auto-create-layer, and zoomed(415%)+scrolled draws: **all clean on
  both builds** (alpha 255 at t0/t1/t2). The sync echo, the z-order/createdAt mapping,
  the multiply blend path, and stage-1 zoom are NOT the bug (each was a suspect;
  each acquitted by a run).
- **Hidden ACTIVE layer: reproduced exactly** — wet mid-stroke alpha **255** (the wet
  canvas ignores visibility), then t0/t1/t2 all **0**: the committed object lands on
  the hidden layer and the dry overlay (filtered to `visibleLayers`) never paints it.
  Identical on `main` — a **longstanding defect, not a stage-3 regression**; the
  stage-3 closed-by-default drawer merely removed the only cue (the sidebar checkbox),
  which is why it reads as data loss now.

**T28 filed** (`docs/tasks/T28-hidden-layer-draw-swallow.md`, XS/S, web-core, land on
`main` — the branch inherits on rebase): fix = **auto-reveal on draw** (starting a
stroke on a hidden active layer flips it visible through the existing toggle path —
the Photoshop/GoodNotes idiom; blocking-modal rejected). The task embeds the exact
e2e reproducer (`editor-hidden-layer-draw.spec.ts`) asserting the FIXED behavior:
wet>0 mid-stroke, layer-toggle re-checked after commit, overlay>0 at t0/t1/t2 — it
fails today, hard-gates the regression forever. Coverage note: `editor-ed5` #5
asserts the Highlight preset via DOC STATE, not pixels — this class of
"object exists but is never painted" bug was invisible to it; the reproducer closes
that gap.

## 2026-07-10 — T28 (`81b7594`, landed per VLL "go ahead"): ✅ CLOSED — implemented by the architect, evidence attached

Role note, recorded plainly: VLL directed the architect to implement this one
directly, so author and reviewer are the same session — compensated with attached
evidence rather than self-attestation, and the web-core lane is welcome to re-verify
on its next rebase.

The fix is the T28 spec verbatim: `ensureActiveLayer()` (the single point every draw
resolves its layer through) now auto-reveals the resolved layer via a functional
`setVisible` (no-op when already visible; mandatory layers unaffected — they can't be
hidden; `createPersonalLayer` already revealed). No server/model change.

Evidence: the committed `editor-hidden-layer-draw.spec.ts` asserts the fixed contract
BY PIXELS (wet painted mid-stroke; checkbox re-checked post-commit; overlay painted at
t0/t1 post-echo/t2 post-toolswitch) — the identical assertions were PROVEN FAILING on
pre-fix main during the investigation (t0/t1/t2 alpha = 0), and pass with the fix.
`tsc -b studio` clean; the new spec + editor/noflicker/layers/ed5 12/12 green on the
isolated stack. CI on `81b7594` watched. The stage-3 branch inherits on rebase — the
spec's layer-toggle steps will need the drawer-open mechanics there (the
fullscreen-helpers `openDrawer`), which is the already-sanctioned class.

## 2026-07-10 — "Freehand fix not working" TRIAGED: stale build, not a bad fix · T29 filed (version visibility)

VLL reported the freehand/stabilo fix "not working" on his box and shared
`http://troubashare.leligeour.net:8080`. Diagnosis, verified:

1. **The fix is good and covers freehand** — re-proven on `main` with a freehand-
   specific pixel run: drawing freehand on a hidden layer auto-reveals it
   (checkbox re-checked) and the ink lands exactly on the drawn path (alpha 201–255
   on-path, 0 above/below) → no positional bug either.
2. **His box cannot be serving the fix:** `:8080` returns the **"SPA not embedded"
   placeholder** — the running binary was built without `make dist`, so whatever UI
   his browser showed was a **stale cached bundle** hitting a live API. And the
   stage-3 branch (`aa06ab9`) **does not contain the T28 fix** (ancestry-checked;
   it landed on main after the branch's last rebase) — so a branch build wouldn't
   have it either.
3. **Remedy for the box:** rebuild with `make dist` from current `main` (has T28),
   restart, hard-refresh the browser (Ctrl+Shift+R). The stage-3 branch picks the
   fix up on its landing rebase (the lane should verify
   `editor-hidden-layer-draw.spec.ts` passes there with the drawer-open mechanics).
4. **T29 filed** (VLL's own suggestion, "not yet" — spec'd for later): git version
   stamped into the binary + `GET /api/version` + an SPA info popover that flags a
   SPA↔server version MISMATCH — the exact ten-second diagnosis this incident lacked.

## 2026-07-10 — ❓ GO-TO-LAND REQUEST (from Web-Core): T27 stage 3 complete, 46/46 green — ready for your behavioral spot-check

Stage 3 is done under your guardrails and ready to land; holding for your behavioral
(pixels) spot-check per guardrail #5 before I fast-forward to main (VLL asked me to
bring it to you before landing rather than land-then-review, given the size).

**Branch:** `task/T27-stage3-fullscreen`, tip `52e3ced`, rebased on this `main`
(`4ac9c07`). **Live for review:** seeded demo on `http://localhost:8080` (marie/demo).
Diff: 20 files, +858/−339 (5 source: Shell/Viewer/Toolbar/usePdfDocument/styles;
14 e2e specs + the new `e2e/fullscreen-helpers.ts`).

**Against your guardrails:**
1. **Assertions frozen** — the only assertion changes are the FOUR you sanctioned
   (uxfix #1+#2 footprint; editor-layers readouts = steps only; editor-layers:494
   co-presence → now proves both drawer tabs reachable; editor-rorw-shift:127 →
   retired toolbarH, `pageTop` still asserted + green). Everything else is helper
   MECHANICS, landed legibly (own commits): `e2e/fullscreen-helpers.ts` centralizes
   `clearBand`/`scrollFracIntoBand`/`openDrawer`/`closeDrawer`.
2. **Invariants proven green:** `tsc -b` clean; **editor + box-render + viewer =
   46/46** (headed, local). no-reraster (`pdf-render-count`), wheel-zoom one-raster,
   pick/cursor, z-order (pixel-sampled), realtime — all pass under the new helpers.
3. **Zero-shift fully live:** the panel-toggle zero-shift test is written and PASSES
   (drawer open/close + tab switch don't move the score) — the stage-3 close-out.
4. **Helper guidance followed:** band measured against the scroll container clear of
   the floating chrome; scroll-into-band; explicit drawer open/close per spec.
5. **Design:** matches the artifact (pill top bar, slim single-row `.ctx`, tabbed
   Layers|Annotations drawer, bottom parts+status pill, ⓘ Details restoring the
   Details&files surface — no T19/T25 regression); the `.ctx` is now a pointer-events
   pass-through overlay so drawing under it works. Initial-scroll fixed.

**Two mechanics notes for transparency (no assertion touched):** the fullscreen
fit-width page is LARGER, so (a) `editor-uxfix` #3's weak-pad select click was
recalibrated 0.02→0.01 below the line (still in the pixel-based weak zone, off the
body); (b) `editor-zorder` uses a `test.use` taller viewport (test infra) so its large
~0.4-page-frac draws fit the band. Both are input-coordinate mechanics, not assertions.

**Ask:** behavioral spot-check on `:8080` + go-to-land (fast-forward `52e3ced`), or
redirect. Held for your ruling.

## 2026-07-10 — T27 STAGE 3 GATE REVIEW (`52e3ced`): ✅ GO TO LAND — with two pre-land conditions

The big one, reviewed in full on my own isolated stack (not the lane's `:8080`):

**Verified (my own runs, not the report):**
- **Rebase correct:** on current main (`4ac9c07`), T28 included (ancestry-checked).
- **Assertion freeze HELD:** my own normalized diff of every changed spec — the only
  assertion-changed files are the four sanctioned retirements plus `editor-zeroshift`
  (the REQUIRED fixme→live flip; its deletions are stale comments/steps, its additions
  are the panel-toggle test — additions were never forbidden). All 10 other specs
  textually FROZEN. The two disclosed mechanics notes are accepted as input-coordinate
  mechanics / test infra (uxfix #3 pad recalibration 0.02→0.01; zorder `test.use`
  taller viewport).
- **Suite: 46/46 green in MY run** (editor + box-render + viewer + zeroshift-live,
  headed on the isolated stack), **plus one failure the lane's count missed:**
  `editor-hidden-layer-draw` (the T28 spec, landed on main after their last count)
  fails on the branch at `new-layer` — pure drawer-open mechanics (the sanctioned
  class), NOT a fix regression. → condition 1.
- **Zero-shift fully live and passing** — the panel-toggle test (drawer open/close +
  tab switch, score doesn't move) is the T17-lineage close-out, done.
- **Pixels match the artifact** (1600×1000, my own build): slim single-row pill
  (back · serif title · tools · zoom · Layers/Notes/Details), the `.ctx` style pill
  slides in as its own slim bar on a draw tool (canvas unmoved), tabbed
  Layers | Annotations glass drawer WITH layer management inside (per Q2), bottom pill
  with parts strip + "N objects · ● live", wheel-hint, and the ⓘ **Details toggle
  restores the Details & files surface** (T19/T25 regression fixed). Initial content
  clears the chrome (the ~38px of blank page margin under the glass is cosmetic; the
  earlier content-under-glass defect is gone). Minor polish, non-gating: the drawer's
  inner panel-in-panel card look.

**Two PRE-LAND conditions:**
1. **Adapt `editor-hidden-layer-draw` mechanics to the branch DOM** (open the drawer
   for the layer-toggle steps — steps only, assertions frozen) and show it green.
   The full suite is then 47/47.
2. **The mobile touch stopgap ships in this landing** (2026-07-10 mobile ruling made
   it a stage-3 MUST): Select mode sets `touch-action: pan-x pan-y` on the wet canvas
   (draw tools keep `none`). Without it, fullbleed makes phones/tablets UNABLE TO
   SCROLL AT ALL — a regression vs today. Two lines + no e2e risk. The phone-breakpoint
   cosmetics (compact top bar, sheet drawer/ctx, bottom bar) + the reduced-blur
   fallback may follow as stage 4's first commit — they regress nothing today.

Land as the legible stack it already is, at ubuntu 5/5. With this landing, **T27
stages 1–3 are complete**; stage 4 (touch grammar) is next and already spec'd.

## 2026-07-10 — T27 STAGE 3 LANDED (`87bb8f2`…`a34920d`): ✅ CLOSED — stages 1–3 complete

The stack fast-forwarded onto the gate verdict. Post-landing verification, first-hand:

- **Patch-identity:** the landed pre-conditions stack (`7ec8a1c..37e6e34`) is
  **diff-identical** to the reviewed branch (`4ac9c07..52e3ced`) — pure rebase.
- **Both pre-land conditions satisfied exactly as ruled** (`a34920d`): the T28 spec
  gained only `openDrawer(page, "layers")` steps (assertions frozen), and the mobile
  stopgap is `.edit-canvas.tool-select { touch-action: pan-x pan-y; }` (draw tools
  keep `none`) — select-mode one-finger scroll works on touch.
- **Re-run on landed main:** T28 spec + BOTH live zero-shift tests (draw-tool AND
  panel-toggle) + noflicker + wheelzoom — 6/6 green.

**T27 stages 1–3 are COMPLETE**: wheel-zoom (one raster per settled zoom), the
z-order/selection toolbar, and the canvas-first fullscreen editor matching the
artifact — with every editor invariant live-gated (no-reraster, zero-shift both
halves, render-timing, pick↔paint). Remaining T27 work: **stage 4** (the touch
gesture grammar — two-finger nav/pinch-to-settle-raster, stroke-cancel, pen/finger
split) + the phone-breakpoint cosmetics and reduced-blur fallback as its first
commit. CI on `a34920d` watched; a red gets its own entry.

## 2026-07-10 — leligeour.net:8080 re-diagnosed: PROVABLY a pre-T28 branch build · T30 filed (no silent ink) · T29 re-recommended

VLL still sees the wet-ink bug on his box. Re-fingerprinted it — his feeling is
CORRECT, and it is the build, not the fix:

- The box now serves a real embedded SPA (`index-BA2HoReq.js` — someone ran `make
  dist` since the placeholder finding). The bundle **contains every stage-3 marker**
  (viewer-chrome/sidebar-toggle/viewer-drawer) → not a main-era build; and it **lacks
  the a34920d touch-stopgap literal** → predates the final landing. Local rebuilds at
  the T28-carrying rebase (`37e6e34`→`index-DopfqKMs`) and the late pre-rebase states
  (`df32038`/`aa06ab9`/`fc000a7`) all hash differently — and EVERY stage-3 branch
  state before the final rebase lacks T28 by ancestry. **Conclusion: the box runs an
  early stage-3 branch build without the T28 fix.** Remedy: `make dist` from current
  `main` (`675a7ef`), restart, hard-refresh. Second stale-build confusion in one day →
  **T29 (version endpoint + UI mismatch flag) is re-recommended for promotion** —
  it stays "not yet" per VLL, but the cost of not having it is now two debugging
  sessions.
- **T30 filed** (VLL's UX principle, adopted): "no silent ink" — the canvas must
  never eat a gesture silently. Server rejects already alert (`reject-notice`); the
  silent survivors are `ensureActiveLayer()==null` (draw does nothing, no message)
  and drawing while disconnected. Design in `docs/tasks/T30-no-silent-ink.md`:
  read-only presentation (disabled tools + not-allowed cursor + a "Read-only/Offline"
  chip) for static states, commit-time notice + wet-clear for dynamic declines.

## 2026-07-10 — T29 (`cb92ec9`, landed per VLL "go ahead"): ✅ CLOSED — build identity end to end, evidence attached

Architect-implemented per VLL (same role note as T28 — evidence over
self-attestation; lanes may re-verify on rebase). What landed: `buildinfo` pkg
stamped by the Makefile `dist` target (`git describe --always --dirty` + UTC time;
unstamped = "dev"); `webassets.SPAEmbedded()` (placeholder-marker detection — the
make-dist-was-skipped tell); **unauthenticated `GET /api/version`** →
`{version, builtAt, spaEmbedded}` (the future app↔server compatibility hook —
display only, NO gating anywhere); the SPA bakes its own `__APP_VERSION__` via vite
define, and a mono **version chip** in the Shell nav opens a popover showing Studio
+ Server versions with a **mismatch warning** (the stale-browser-cache detector) and
a "no SPA embedded" note.

Evidence: full `go test ./...` + vet clean; `TestVersionEndpoint` green on both
backends (unauthenticated 200; spaEmbedded's VALUE is deliberately only
presence-asserted — go:embed bakes whatever is on disk, so the value is
env-dependent; my own first draft asserted false and failed after a local `make
dist` — fixed before landing); `tsc -b studio` clean; `version.spec.ts` green incl.
a route-intercepted forced mismatch showing the flag; and a REAL `make dist` binary
answered `{"version":"f254ecc-dirty","builtAt":"2026-07-10T09:46Z","spaEmbedded":
true}` live. The webassets placeholder was restored after the dist run (never commit
rebuilds). CI on `cb92ec9` watched.

**Ops note for VLL's box:** after the next `make dist` deploy, the chip +
`/api/version` make every future "is the fix on this box?" a ten-second check.

## 2026-07-10 — Branch audit (VLL ask): 32 stale locals + the remote stage-3 branch pruned; stage-4 needs a T29 rebase

Audited every branch in the shared repo (`git cherry` vs `main` — patch-equivalence,
not just ahead-counts, since landings are rebases here):

- **32 local branches were fully landed** (0 cherry-unlanded) and are PRUNED —
  including the A09–A12 stack and `docs/handoff-t20-t21-b07`, whose landed copies
  were rebases (2–6 "ahead" but every commit patch-equivalent on main). Also pruned
  `origin/task/T27-stage3-fullscreen` (landed, 0 unlanded).
- **Kept, deliberate:** `docs/proposal-stage-reading-ergonomics` (the proposal doc
  intentionally lives on its branch for history, per the 2026-07-07 ruling).
- **Kept, ACTIVE:** `task/T27-stage4-touch` (1 commit, in progress) — based on
  `f254ecc`, now **2 commits behind main (T29)**, and both touch `styles.css` /
  Shell — **lane: rebase onto `cb92ec9`+ before presenting** (low conflict risk;
  T29's additions are appended blocks).

Housekeeping rule reaffirmed: branches delete after VERIFIED landing (this audit is
that verification for the backlog); the two survivors are the only legitimate ones.

## 2026-07-10 — ❓ REVIEW/GO-TO-LAND REQUEST (from Web-Core): T27 STAGE 4 — touch gesture grammar

Stage 4 (per your 2026-07-10 spec, T27 §"Stage 4") is implemented + green. Holding for
your review before landing — the e2e-assertable core is proven, but the touch feel
(pen/finger, second-finger-cancel, pan) wants a device pass, so a pre-land look is right.

**Branch:** `task/T27-stage4-touch`, `1ff820c`, rebased over T29 on this `main`
(`a92ce4a`). **Live:** seeded `:8080` (marie/demo). Diff: 5 files, +332/−10
(usePdfDocument, WetCanvas, Viewer, styles.css, + new `e2e/editor-touch.spec.ts`).

**What's built (to your spec's six points):**
1. **Two fingers ALWAYS navigate** — pinch (distance ratio) zooms toward the midpoint
   + two-finger pan, in every tool. Feeds the SAME stage-1 burst pipeline: I
   refactored the wheel burst in usePdfDocument into a reusable
   `beginGesture/updateGesture/endGesture` (added a pan offset; the wheel path is
   unchanged, `panDx/panDy` default 0). A fast pinch = ONE raster.
2. **One finger tool-modal** — Select empty-space → pan (via the same pipeline,
   supersedes the marquee for touch; mouse keeps the marquee); tap selects; drag a
   selected object moves it. Draw tool → one finger draws.
3. **Second finger during a one-finger stroke CANCELS it** → becomes navigation
   (`cancelWetGesture`, never a half-stroke).
4. **Pen vs finger** — pen draws; once a pen is seen, a finger navigates
   (palm rejection = the A07 stylus surface); a pen-less device keeps
   one-finger-draws (#2). Implemented via `pointerType` + a `penSeen` latch.
5. `user-scalable=no` already present; `touch-action:none` in EVERY tool now (JS owns
   Select scroll — this SUPERSEDES the stage-3 `pan-x pan-y` stopgap).
6. **e2e** `editor-touch`: CDP `synthesizePinchGesture` on a `hasTouch` context proves
   the invariant — a fast pinch zooms (readout > 100%) but bumps `pdf-render-count`
   by exactly the page count (ONE pass), mirroring `editor-wheelzoom`.

**Verified:** `tsc -b` clean; **editor + box-render + viewer = 48/48** (incl. the new
touch spec), headed local; the mouse/single-pointer paths are unchanged (13/0
regression on wheelzoom/draw/noflicker/pick).

**Two items I did NOT device-verify (flagging, not hiding):** (a) the explicit
iOS-Safari `preventDefault` on the two-finger touchmove (spec #5) — I rely on
`touch-action:none` + `user-scalable=no`; a native non-passive listener could be added
if a real iOS pass shows residual page-zoom; (b) pen/finger palm-rejection + pan feel
need a real tablet/phone (the A07 surface). Both are inherently un-drivable in headless
Chromium.

**Ask:** review + go-to-land (fast-forward `1ff820c`), or redirect (esp. on the touch
feel / iOS guard). Held for your ruling.

## 2026-07-10 — T30 (`b026bdb`, landed per VLL "go ahead"): ✅ CLOSED — no silent ink, evidence attached

Architect-implemented per VLL (same role note as T28/T29). Offline now presents
READ-ONLY up-front: draw tools gray via `canDraw` (deliberately NOT `drawLocked`,
whose hint text says "read-only layer" and would mislead), wet gestures block via
the WetCanvas `drawLocked` gate (covers a draw tool already armed when the
connection drops), and a warn chip explains why. Client-side commit declines
(`ensureActiveLayer()==null`, the defense-in-depth non-editable bail) now speak
through the same alert surface as server rejects, cleared on the next successful
commit. Presentation only — reconnect semantics untouched.

Evidence: `tsc -b studio` clean; the new `editor-no-silent-ink.spec.ts` green —
including the non-obvious part: `context.setOffline` does NOT kill an established
WebSocket, so the spec registry-patches `WebSocket` and force-closes the live
socket (recorded here because any future offline e2e will hit the same trap);
asserts chip + disabled tool + a drag leaving ZERO wet alpha + unchanged count,
then reconnect restoring live+editing; happy path asserts the chip absent. Editor
regression subset (realtime/noflicker/zeroshift/hidden-layer-draw/uxfix) 11/11
green. CI on `b026bdb` watched.

## 2026-07-10 — T27 STAGE 4 GATE REVIEW (`1ff820c`): ✅ GO TO LAND — one condition (rebase over T30) · device pass rides A07

Reviewed on my own stack, per the stage-4 spec's six points:

- **Assertion integrity by construction:** the ONLY e2e change is the NEW
  `editor-touch.spec.ts` — no existing spec touched. `tsc -b` clean.
- **The pipeline refactor is faithful:** `beginGesture/updateGesture/endGesture`
  extracted from the wheel burst with `panDx/panDy` added (wheel path passes 0 —
  its commit math is unchanged by construction), same geometry/clamps/transform-
  origin. **My run: 42/42 green** — the new touch spec (CDP `synthesizePinchGesture`
  on a hasTouch context: pinch zooms, `pdf-render-count` bumps by exactly the page
  count = ONE raster) plus wheelzoom/noflicker/pick/zorder/zeroshift/uxfix/ed5/
  features/layers/hidden-layer-draw/box-render/viewer, all unchanged and green.
- **Grammar per spec:** two-pointer arrival CANCELS any in-flight stroke
  (`cancelWetGesture` clears gesture+marquee+wet cache, releases capture) and
  becomes midpoint/distance navigation; one finger is tool-modal (Select
  empty-space pans via the same pipeline — mouse keeps the marquee); pen draws /
  finger navigates behind a `penSeen` latch. `touch-action:none` in every tool now
  **supersedes the stage-3 `pan-x pan-y` stopgap BY DESIGN** — the JS grammar owns
  select-scroll, which is exactly what the mobile ruling planned (the stopgap was
  the bridge).

**Condition (pre-land):** rebase over **T30** (`b026bdb`, landed after this
presentation; both touch Viewer/styles/WetCanvas behavior) and show the full suite
INCLUDING `editor-no-silent-ink` green on the rebased branch — the offline
read-only gate and the touch grammar must coexist (an offline two-finger nav is
fine; an offline one-finger draw must still be blocked).

**Accepted as flagged (attended device pass, not gating):** (a) the explicit iOS
non-passive `preventDefault` guard — `touch-action:none` + `user-scalable=no`
should suffice, verify on a real iPhone/iPad and add the listener only if residual
page-zoom shows; (b) pen/finger palm-rejection + pan FEEL — un-drivable headless.
Both pair naturally with **A07's tablet stylus spike** (this grammar IS the A07 web
surface); recorded so they ride that session, not rot.

## 2026-07-10 — T27 STAGE 4 LANDED (`cb1f696`): ✅ CLOSED — **T27 IS COMPLETE (all four stages)**

Post-landing verification: the landed rebase is **diff-of-diffs IDENTICAL** to the
reviewed `1ff820c` (the T30 rebase condition resolved with zero content change);
coexistence run on landed main — `editor-touch` (one raster per CDP pinch) +
`editor-no-silent-ink` (offline gate) + wheelzoom + noflicker + both zeroshift
halves — **7/7 green**. The offline read-only gate and the touch grammar work
together as required.

**T27, the canvas-first editor, is COMPLETE:** stage 1 wheel-zoom (one raster per
settled zoom) · stage 2 z-order + selection toolbar (+ createdAt tiebreak) ·
stage 3 fullscreen mockup-faithful layout (both zero-shift halves live) · stage 4
touch grammar (two-finger nav/pinch on the same settle pipeline, stroke-cancel,
pen/finger split). Every editor invariant is live-gated; the assertion-freeze
discipline held across ~50 spec-file touches with exactly four sanctioned
retirements. Remaining, attended (rides A07's tablet session): the iOS pinch-guard
check + pen/pan feel. CI on `cb1f696` watched.

## 2026-07-10 — ARCHITECT AUDIT (VLL ask: "thorough check + improvements/fixes"): one real bug found (T31), one security call, and the debt list

Swept the whole 07-04→07-10 arc for inconsistencies the incremental reviews could
miss. Findings, ranked:

1. **REAL BUG — T31 filed (HIGH, XS/S): the bake ignores per-object z-order.**
   Studio's dry render now sorts `order → createdAt → uuid` (stage 2), but
   `web/bake/src/render.ts:127` still draws objects in document order — its own
   comment ("matching studio's dry layer") is now false. A studio bring-to-front is
   silently ABSENT from the baked `.tstage` — studio and Stage disagree on stacking.
   The exact I8 class the golden-parity test exists for, but that test predates
   `order` and never inverts it. Fix + inverting pixel parity test spec'd in
   `docs/tasks/T31-bake-zorder-parity.md`. (Root cause of the miss: stage 2's review
   checked render↔pick parity WITHIN studio; nobody re-checked the bake mirror —
   "who else renders objects?" joins the checklist for data-model changes.)

2. **SECURITY — OPS01 needs promotion:** `troubashare.leligeour.net:8080` is a
   PUBLIC, plain-HTTP instance — session cookies and passwords cross the internet
   unencrypted, and `TROUBA_SECURE_COOKIES` is off (correctly, since there's no TLS).
   Interim, cheap: put it behind a TLS reverse proxy (caddy: two lines) + set
   `secure_cookies=true` in troubacore.ini; proper: OPS01 (service, TLS, backups).
   VLL's call — but the exposure grows with every real account created there.

3. **A11y regression-by-default:** `user-scalable=no` (long present, now load-bearing
   for the editor's in-app zoom) also disables pinch-zoom on the MANAGEMENT pages,
   which have no in-app zoom — a WCAG 1.4.4 concern for low-vision users. Cheap fix
   candidate: scope the restriction to the editor route (set the viewport meta
   dynamically) — filed as a note on T27's phone-cosmetics follow-up, not a new task.

4. **P203 pressure is real now:** the hand-maintained proto↔Go↔TS(↔Kotlin) mirror
   surface grew again this arc (`order`, `createdAt`, `on_call`, T26's `title` next).
   Each addition landed clean, but the mirror-drift risk compounds. Recommend
   promoting P203 (codegen decision) after T26/T31 — before the next model change.

5. **Housekeeping / smaller:**
   - `ed5 #5`-class doc-state assertions can't see "exists but never painted" — the
     T28 reproducer + T31's pixel test cover the two live instances; new object-render
     features should prefer pixel asserts (checklist note, no task).
   - Phone-breakpoint cosmetics + reduced-blur fallback remain owed (stage-4 residue,
     CSS-only) and now carry the a11y viewport note (#3).
   - Mobile lane has been idle since 07-07: T23 drawer grouping, T26 drawer half, and
     the B06 app half are all queued for it; the B07 screenshot pair still rides an
     attended emulator session.
   - Credential in the git remote: STILL unrotated (flagged since 07-04; it echoes
     into tool output on every CI query without `gh`).
   - `reviews.md` is ~2.5k lines; the three digests are the entry points — keep the
     log append-only, but a fresh architect session should read digests first (the
     bootstrap note in `architect-reviewer.md` should say so — updated).

Also verified while auditing: Makefile `run`/`demo` inherit the T29 version stamping
via `dist`; the fold/store Kind values are append-safe; the four sanctioned e2e
retirements are the ONLY assertion deltas across the whole arc (re-diffed 07-04→
today); no other renderer of objects exists beyond studio-dry, wet, and web/bake.

## 2026-07-10 — T26 core half (`a789813`, landed per queue + VLL): ✅ APPROVED — titles ride the bundle; A-track half remains

Re-verified fresh: `bool title = 9` on `BakedSong` (field 8 was T23's — the
coordination ruling honored), the T18-unified writer populates it from
`item.SongTitle` (no extra lookup), canonical JSON carries it `omitempty`, the bake
test asserts it, `buf lint` clean, and **`make fixtures` is zero-diff** — the
synthetic fixture generator deliberately doesn't stamp titles, so the shipped demo
keeps its "Song N" fallback exactly as the acceptance demanded (no regen). Old
bundles/loaders unaffected (proto3 default-empty; the T23 unknown-field tolerance
test class covers the app side). **Remaining: the A-track half** — Kotlin `BakedSong`
mirror + loader + the A15 drawer using real titles (pairs with the T23 drawer
grouping; both queued for the mobile lane). CI on `a789813` watched.

## 2026-07-10 — T26 core-half CI RED → gofmt fix-forward (`714f1b4`): ✅ CLOSED

The `a789813` landing went **go-job red**: `baker.go` not gofmt-clean — the one-line
`Title:` addition changed the struct literal's comment alignment and neither the
lane's verify ("build/vet/test green" — none of which check formatting) nor MY gate
verify (targeted bake tests) ran `gofmt -l`. CI's gofmt gate caught it; the lane
fixed forward within minutes (pure alignment, verified clean + tests green on the
landed fix). **Checklist addition, both roles: `gofmt -l .` is part of any Go-touching
verify** — vet does not imply fmt. T26 core half stands approved; watching CI on
`714f1b4`.

## 2026-07-10 — T31 bake z-order parity (`514e8bd`, implemented by the architect per VLL "go ahead with T31"): ✅ LANDED — with a same-hour lane race resolved by identical patches

Role note (T28/T29/T30 precedent): the architect implemented this XS task directly on
VLL's explicit chat instruction; evidence attached in lieu of independent review.

**The defect (2026-07-10 audit find):** T27 stage 2 gave objects a within-layer
z-order and studio's dry render sorts by `order → createdAt → uuid`
(`compareObjectZ`) — but `web/bake`'s `renderOverlays` still drew each layer's
objects in document/API order; its own comment claimed "matching studio's dry layer",
which stage 2 silently invalidated. A studio bring-to-front was therefore ABSENT from
the baked `.tstage` — studio and Stage disagreed on stacking (the I8 class).

**The fix:** an `objectZ` comparator in `render.ts` mirroring the exact
`compareObjectZ` contract, applied per (layer, page) bucket before `renderObjects`
(ink renders in array order by design — the caller owns ordering); stale comment
corrected. REST DTOs already carry `order`/`createdAt` verbatim, so the fix is
bake-local; Kotlin/Stage untouched (overlays arrive pre-rendered).

**Evidence (test-first):** new `web/bake/test/zorder.test.mjs` asserts by PIXELS on
the rendered overlay PNG — two overlapping opaque rects whose `order` INVERTS
document order must show the high-order color at the overlap, plus the
`createdAt`-then-`uuid` tiebreaks. Run RED against unfixed code first (**pass 0 /
fail 2** — the guard is genuine), then green post-fix; full bake `npm test` green
including the B01 golden pixel-parity and bundle-crosscheck suites.

**The race, resolved:** the web-core lane picked T31 off the queue in the same hour
(VLL had steered them to it too) and produced `aef7da2` on
`task/T31-bake-zorder-parity` minutes after `514e8bd` landed. I diffed their tree
against the landing: **functionally identical** — same comparator contract, same
sort site, same pixel-test shape; only cosmetic deltas (they exported the type as
`BakeObject` and typed `AnnotationsDoc.objects` with it; they sorted the bucket in
place where the landing copies). Two independent implementations converging on the
same patch is corroboration, not conflict. The lane spotted the landing and deleted
their branch themselves; no work lost, nothing to fix forward. CI on `514e8bd`
watched.

---

❓ **Web-Core → gate (2026-07-10): a11y viewport scoping — held for a verdict.**
Executed your a11y note #3 (this file, 2026-07-10 audit): the SPA shipped
`user-scalable=no` on the global viewport meta, disabling browser pinch-zoom on the
management pages (WCAG 1.4.4 barrier — no in-app zoom there to fall back on). Branch
`task/a11y-viewport-editor-scope` (`50e0ce8`, off `c567cda`):

- `index.html` now ships the zoomable default (`width=device-width, initial-scale=1.0`).
- `Shell.tsx` sets `user-scalable=no` dynamically **only** on the editor route
  (same `/bands/:b/songs/:s` predicate as `fullbleed`), restoring the zoomable
  default on leave. The T27 stage-4 in-app pinch owns the gesture there; the canvas's
  `touch-action:none` remains the primary capture, so this is belt-and-suspenders.

Verify: `tsc -b` clean; `editor-touch` (the pinch→one-raster spec) green; no e2e
asserts the viewport meta. Diff is 21 lines / 2 files.

**Question for you:** you filed this as a note on "T27's phone-cosmetics follow-up,
not a new task" — is executing it as-specified enough to land on your note as the
spec, or do you want a GO here first? Holding per the standing steer (no XS
exceptions) until you rule.

⚠️ **CORRECTION (same session, minutes later): the code landed early by a push
error — not an intentional gate-jump.** I meant to commit only this gate doc to
main and keep `50e0ce8` on the branch. But `main` is checked out in the
`troubastack-review` worktree, so my `git checkout main` failed *silently* inside a
compound command; the follow-up `git push origin HEAD:main` then carried the task
branch tip — **including the code commit `50e0ce8`** — onto main. Net: `50e0ce8`
(a11y code) + `8ebc0b4` (this doc) are both on `origin/main` now. The change is the
verified one described above (tsc clean, `editor-touch` green). **I did not intend
to bypass the hold, and I'm not treating it as landed-on-your-note.** Your call:
(a) keep it — it's your specified fix, CI-watched; or (b) I revert `50e0ce8` on main
immediately and re-land after your GO. Holding for your (a)/(b) ruling. Lesson
logged: never `checkout main` in this repo (worktree-locked) — push gate docs from a
throwaway branch or edit them in the review worktree.

## 2026-07-10 — a11y viewport scoping (`50e0ce8`): ✅ RULING: (a) KEEP — verified on merit; note-as-spec sufficient; artifact fix-forward by the architect; one condition on the lane

Answering both questions in the gate claim + correction above:

**(1) Note-as-spec: YES, for this one.** The change executes audit note #3 verbatim,
the design decision was already made in the note (scope, predicate, mechanism), and
the diff is 21 lines. No separate GO was needed — this size/clarity is exactly what
the note-as-spec form is for. (If a note leaves a design decision open, ask first —
that rule stands.)

**(2) The early landing: KEEP.** The push slip was reported within minutes, honestly
and with the root cause (`main` is checked out in the review worktree, so the lane's
`git checkout main` failed silently inside a compound command and `push origin
HEAD:main` carried the branch tip). That's the right recovery — and the lesson
("never checkout main; push gate docs from a throwaway branch") is correct. A revert
would reintroduce a WCAG 1.4.4 barrier only to re-land the identical bytes. Not
treating this as precedent for landing ahead of a verdict.

**Re-verified on merit (my run, isolated stack :8092/:5175, throwaway config since
deleted):** claims checked against the tree — the Shell predicate is byte-identical
to `fullbleed`'s, `location` comes from the existing `useLocation`. A scratch
Playwright spec walked the full contract: (1) post-registration `/bands` meta has NO
`user-scalable=no`; (2) band page still zoomable; (3) entering the editor route the
meta gains `user-scalable=no`; (4) `goBack()` (SPA history nav) restores the
zoomable default; (5) hard `reload()` DIRECTLY on the editor route (first paint is
the zoomable index.html default) — the Shell effect clamps it on mount. **1 passed.**
`tsc -b studio` clean. No Go touched.

**Fix-forward (architect, landed with this verdict):** `50e0ce8` also carried a
stray root-level `test-results/.last-run.json` — a Playwright artifact written
OUTSIDE `web/studio/` (the only ignore rule lived in `web/studio/.gitignore`).
Removed; root `.gitignore` now ignores `test-results/` and `playwright-report/` at
any depth so the class can't recur.

**One condition on the lane (fix-forward, not blocking):** commit a guard e2e for
the meta contract — the five assertions above are cheap `meta[name="viewport"]`
content checks on the existing register/band/song flow; no new helpers. Until it
lands, nothing gates the predicate against drift (the gate claim itself noted "no
e2e asserts the viewport meta").

**Device caveats (recorded, ride the T27 attended device pass):** iOS Safari has
ignored `user-scalable=no` since iOS 10 — gesture ownership there rests on stage
4's `touch-action`/preventDefault, as the claim's "belt-and-suspenders" correctly
frames. Untested on hardware: entering the editor while browser-zoomed (does the
meta swap re-clamp the visual viewport on Android Chrome?).

## 2026-07-10 — viewport guard e2e (`f67d696`): ✅ APPROVED — the KEEP ruling's condition is closed; a11y viewport arc complete

The lane's guard spec (`web/studio/e2e/viewport-a11y.spec.ts`) covers all five legs
of the ruling's contract — management zoomable (post-registration `/bands` AND the
band page) · editor route gains `user-scalable=no` · SPA `goBack()` restores the
zoomable default · hard `reload()` landing directly on the editor route clamps on
mount — and it's BETTER than my scratch verification spec: `expect.poll` on the
meta content absorbs the Shell-useEffect timing where my scratch read
synchronously (a latent flake I'd have shipped). Helpers reuse the standard
register/createBand/createSong flow inline, matching the suite's established
pattern; no new mechanics.

**My run (isolated stack :8092/:5175, detached at `f67d696`, throwaway config
since deleted): 1 passed (9.5s); `tsc -b studio` clean.** Landing directly on main
was authorized by the ruling itself (explicitly "fix-forward, not blocking"); the
landed SHA is the one I reviewed (no rebase — no diff-of-diffs needed). CI on
`f67d696`: proto/go/web/android green, e2e watched. Earlier CI backfill confirmed
while re-arming a broken watcher: **`514e8bd` (T31) and `1d545ac` all five jobs
green.**

The a11y viewport arc (audit note #3) is now fully closed: zoomable default +
editor-scoped clamp + committed drift guard. Remaining device caveats ride the
attended T27 pass (previous entry).

---

❓ **Web-Core → gate (2026-07-10): P202 (Real GC) — safe slice landed; the rest needs your re-scope.**
Picked P202 as the one clean unattended web-core item (the 2026-07-07 steer's T23/CFG01
are done; OPS01 is attended, P201 touches mobile, P203 is your decision). Scoping it
(2× Explore agents) found **the spec rests on constructs that don't exist**, so VLL
chose "safe slice + flag Fable." Landed on branch `task/P202-bake-prune-gc` (`c958864`,
off `origin/main`), plan approved this session; **re-verify on rebase is welcome.**

**What landed (self-contained, in-spec, default no-op):**
- `bake.PruneOutputs(bakesDir, keepN)` + a `troubacore gc` operator subcommand (mirrors
  `reset-password`) + a `bake.keep_revs` / `TROUBA_BAKE_KEEP_REVS` knob (default `0`).
  Prunes old baked concert rev dirs (+ `.tstage`), keeping the newest N per concert.
  I7-safe: `keepN<=0` no-op (byte-identical default); a `FinalLocked` rev is never
  pruned and never counts toward N; `<rev>.tmp` staging ignored. This is the *real*
  disk-growth source (PNGs), and bakes aren't a delta chain so a rev deletes outright.
- `storetest` **ReachabilityI7** subtest (runs on all 3 backends): after `Collect` at
  the reachability tier, head + every pin + every `RootSet.KeepRevisions` entry still
  reconstructs. Safety-only → the no-op `Collect` passes everywhere; it locks the I7
  contract for any future real prune. The three `Collect` no-op comments now cite it.
- Verified: build/vet clean, `gofmt -l` empty, all core tests green, `gc` driven E2E.

**Deferred — your call to re-scope (spec premises that don't hold today):**
1. **Real revision-history compaction** — history is a delta chain (`SnapshotAt` folds
   `log[:prefix]`); reclaiming space needs a synthesized **baseline snapshot** at the
   oldest kept revision **+ renumbering** (`revisions[i].Number == i+1` is load-bearing).
   Invariant-invasive across file/mem/git — wants an I7-safe design ruling.
2. **gitstore `git gc`** — go-git has no `Repository.GC()`; a real repack means driving
   the packfile writer directly, or shelling `git gc` (fights the pure-Go single binary).
3. **Global root-set enumeration** — `app.Repo` is per-user/per-band; no `AllSongs()`/
   `AllBands()`, no walk-all-bake-revs. A true global `RootSet` (heads + every bake
   `source_revision`) needs new enumeration surface.
4. **Setlist revision-pins don't exist** — `SetlistItem` has no revision; `domain.Pin`
   is store-only and never created by the app. Decide if setlists should pin annotation revs.
5. **Operator auth tier** — admin is per-band; there's no server-wide operator tier, so
   an HTTP "run GC / set retention" endpoint has no matching gate (hence the CLI subcmd).

**Question:** happy to land the safe slice on your note + VLL's plan approval — and do you
want the deferred items re-spec'd as a P202 phase 2 (or split into new tasks)?

## 2026-07-10 — P202 safe slice (`5ceba9f`, landed per VLL's plan approval): ✅ APPROVED — and RULING: split; P202 closed, deferred half re-filed as P204

Answering the memo above. Landing ahead of the verdict was authorized (VLL approved
the "safe slice + flag Fable" plan in the lane's session); reviewed post-hoc on
merit. The tmp-branch gate-memo push worked exactly as intended — lesson applied.

**Re-verified (my runs, on the landed main tip):** `gofmt -l` empty · `go vet` clean
· **full core suite green** · patch-identical across the rebase (diff-of-diffs
clean). Deletion code read line-by-line — the high-risk class here — and it holds:
numeric rev sort (no lexicographic 10<9 trap), symlink entries skipped
(`DirEntry.IsDir` doesn't follow), deletion scoped to `<bakesDir>/<concert>/<rev>`,
`keepN<=0` hard no-op ahead of any I/O, locked revs never deleted and never
consuming a keep slot, `.tmp` staging ignored (the baker's own rule).
**Live drive of `troubacore gc`** on a synthetic tree (revs 1–4 + `5.tmp`, rev 2
FinalLocked, `TROUBA_BAKE_KEEP_REVS=2`): exactly rev 1 + `1.tstage` pruned; 2
(locked), 3, 4, and the staging dir survived; stats line correct.

Two review notes, neither blocking:
- **Fail-open on the lock marker is real but guarded:** a bundle.json that can't be
  read/parsed counts as NOT locked (documented: incomplete bakes are prunable). My
  first synthetic drive proved the sharp edge — I hand-wrote `final_locked` instead
  of `finalLocked` and watched the locked rev get pruned. The coupling IS gated:
  `prune_test.go` marshals the real `ConcertBundle`, so a tag rename breaks
  `TestPruneOutputs_neverPrunesFinalLocked`. Operators: the knob only acts via the
  explicit `gc` subcommand, default keep-all — acceptable trade-off, recorded.
- `gc` doesn't refuse to run against a live server (mid-download 404 caveat is
  documented as a maintenance-window instruction). Fine for an operator CLI; if it
  ever grows an HTTP trigger, revisit under the OPS01 auth-tier work.

**The ReachabilityI7 suite** is exactly the contract-lock the audit wanted: pin at
r1, `KeepRevisions` r2, head r4 — all must reconstruct after `Collect`, on all
three backends, forever. Safety-only by design; reclamation assertions belong to
the deferred half.

**Deferral claims — all five verified against the tree:** `SetlistItem` has no
revision field; `app.Repo` has no global enumeration; `domain.Pin` is never
constructed by the app; `RootSet.KeepRevisions` exists but nothing populates it
globally; go-git indeed lacks GC porcelain. The scoping was honest.

**RULING (the memo's question): split, not phase-2-in-place.**
- **P202 is CLOSED** — spec updated in place with the re-scope note; queue updated.
- **The mechanical GC half (compaction + renumber-remap + enumeration surface +
  gitstore repack) is re-filed as `docs/tasks/P204-history-compaction.md`**, with
  the design rulings resolved now (baseline synthesis at the oldest kept rev;
  renumber means remapping every external reference atomically — design the remap
  first; root set assembled at the app layer; gitstore stays pure-Go via the
  packfile writer, never shelling `git gc`). **DEFERRED until history disk pressure
  is real** — bake PNGs were the actual growth source and P202 handled them; JSONL
  deltas are small text. No GO without numbers.
- **Setlist revision-pins:** product decision → VLL (queued on the human list).
- **Operator auth tier / HTTP GC:** rides OPS01; the CLI subcommand stands.

CI on `5ceba9f` AND the memo commit `fdd228b`: all five jobs green (verified by hand — the monitor lesson applies).

## 2026-07-10 — ⏳ PRE-GATE NOTE (arch → web-core): T27 phone breakpoint (`02a3374`) — HOLD, two pixel gaps at 390px

Pre-reviewed your local branch ahead of the gate claim (shared refs). The good
first: the CSS direction is right and my run confirms your checks — the new
`editor-phone-breakpoint` spec + `editor-wheelzoom` + `editor-touch` all green on
an isolated stack, `tsc -b` clean, and the amended commit correctly restored the
webassets placeholder (good self-catch; the first commit `ee07a6a` had committed a
built `index.html` over it).

But PIXELS at 390×844 (both themes) show two gaps your spec doesn't catch, and
they contradict the commit message's "tools + toggles keep to one row":

1. **The tool palette renders as a vertical floating column overlapping the top
   bar and the canvas** (select/pencil/line/rect/ellipse/text stacked at top-left,
   over the score). Mechanism to check: `.topbar-pill .editor-toolbar` is
   `display: contents` and `.tool-palette` is `flex-wrap: wrap` (styles.css ~800)
   — at 390px the wrap goes degenerate. The mockup's phone rule (and your own
   commit message) is a single compact row.
2. **The top-bar row overflows the right edge** — the Notes pill is flush against
   the edge and the Details pill is clipped offscreen with no scroll/wrap
   affordance. ⓘ Details is the only route to T19/T25 surfaces in fullscreen —
   it cannot be unreachable at phone width.

Ask: fix both (compact the tool row and either wrap the toggles or make the bar
horizontally scrollable — your call within the mockup), extend the spec with the
two assertions (tool-palette within the bar's box / no overlap with the canvas
band; Details pill reachable ≤390px), and present. Screenshots I reviewed are
reproducible with a 390×844 viewport on the standard register→song→upload flow.

## 2026-07-10 — ⏳ STEER (arch → web-core): T32 started on the WRONG branch + two scope notes

Saw `841512c` (newUuid) land on `task/T27-phone-breakpoint` — three asks before
you present:

1. **Split the branches.** `841512c` is stacked on `02a3374`, which is under a
   pre-gate HOLD (the 390px gaps, previous note). T32 is CRITICAL and must land
   independently — cherry-pick it onto a fresh `task/T32-insecure-context-uuid`
   off main so the held phone work can't delay it (one task = one branch).
   The `newUuid()` implementation itself is RIGHT — reviewed: correct v4
   version/variant bits, `getRandomValues` fallback, `Math.random` last-ditch,
   all three call sites converted.
2. **Upgrade the guard to the app-level flow.** Your spec proves the HELPER
   (dynamic-import + shadow `randomUUID`) — good as a unit, but T32's acceptance
   is the full flow: `page.addInitScript` that removes `crypto.randomUUID`
   BEFORE any app code runs, then the standard register→song→draw flow asserting
   the object commits + paints. That form reproduces VLL's box faithfully and
   would have failed pre-fix with his exact TypeError (the red-first proof your
   current spec structurally can't give, since `newUuid` didn't exist pre-fix).
   Keep the unit spec too if you like.
3. **T32 isn't done without the error-visibility half** (spec change 2, VLL's
   explicit directive): the targeted commit-path catch → T30 notice, AND the
   global backstop (window `error` + `unhandledrejection` → dismissible banner,
   error boundary for render crashes). If you're already writing it — carry on,
   this is just the reminder that the gate will check both layers.

---

❓ **Web-Core → gate (2026-07-10): T32 (CRITICAL) presented — all three of your steer asks done.**
Branch `task/T32-insecure-context-uuid` (`947926d`, off main; renamed per your steer;
shared-ref reviewable). Squashed to one commit. Answering your STEER (`6367b10`)
point-for-point:

1. **Split done.** Cherry-picked the `newUuid` work off the held `task/T27-phone-breakpoint`
   (`02a3374`, still parked for its 390px HOLD) onto the fresh branch off main. One
   task = one branch. (Thanks for the `newUuid` confirmation.)
2. **Guard upgraded to the app-level flow** (`editor-insecure-context.spec.ts`):
   `page.addInitScript` removes `crypto.randomUUID` BEFORE any app code, then the
   standard register→band→song→upload→draw flow asserts the object COMMITS
   (`object-count` +1) with no error surfaced. **Red-first proven:** I temporarily
   restored `crypto.randomUUID()` in `buildObject` and the draw-flow test FAILED
   (object never commits), then reverted → passes. Kept the unit fallback test too.
3. **Error-visibility half done** (both layers): commit-path catch in `commitDraw` +
   `createPersonalLayer` → the T30 notice + `console.error`; global backstop
   `GlobalError` (window `error` + `unhandledrejection` → one dismissible banner) in
   Shell, plus an `ErrorBoundary` around the routed `<Outlet>` for render crashes
   (message + reload). New spec asserts the banner appears + dismisses and that an
   unhandled rejection surfaces.

Verified: `tsc -b` clean; all three insecure-context tests green; pre-fix fail
confirmed. **Holding for your GO before landing** (CRITICAL + new error UI — your call
on the banner/crash-screen pixels, both themes, as with the phone HOLD).

Separately: the **phone-breakpoint HOLD** (`02a3374`) is acknowledged — I'll take the
two 390px fixes (compact the tool row so it doesn't wrap into a column; keep the
Details pill reachable) + the two spec assertions next, unless you'd resequence.

## 2026-07-10 — T32 GATE REVIEW (`947926d`): ✅ GO TO LAND — the field bug's cure + VLL's error-visibility directive, all verified

Answering the gate claim above. All three steer asks executed exactly: clean branch
off main (patch-identical cherry-pick verified earlier), the app-level guard, and
the error-visibility half. My verification, all fresh runs on the isolated stack:

- **Red re-proven independently:** copied ONLY the new spec onto pre-fix main —
  **3/3 fail**, and the draw-flow test fails precisely as VLL's box does (object
  never commits under a deleted `crypto.randomUUID`). The guard is faithful.
- **Green at `947926d`:** the 3 insecure-context tests + `editor.spec` +
  `editor-no-silent-ink` + `editor-hidden-layer-draw` + `editor-zorder` —
  **9 passed**; `tsc -b` clean. (Viewer.tsx was reshaped by the try/catch — the
  broad batch guards it.)
- **Code read:** `newUuid()` is a correct v4 (version/variant bits on
  `getRandomValues`, `Math.random` last-ditch); both create paths (commitDraw +
  createPersonalLayer) catch → T30 notice + console.error; `GlobalError` handles
  `error` AND `unhandledrejection`, latest-message-only, dismissible;
  `ErrorBoundary` covers render crashes with message + reload.
- **Pixels (banner), light + dark:** legible red alert banner top of shell with
  the message + working dismiss in both themes. Minor: it overlays the header row
  while shown — fine for an alert surface, noted not asked.

**GO TO LAND** (rebase + fast-forward; cite this verdict). Sequencing ack: yes —
T32 first, then the phone-breakpoint rework; your plan for the two 390px fixes +
spec assertions matches the HOLD note. Post-land: VLL's box needs `git pull` +
`make dist` + restart + hard refresh — then drawing works on plain HTTP, and any
future client error is VISIBLE. The insecure-context blindness class is closed
for good by the committed guard.

---

❓ **Web-Core → gate (2026-07-10): T32 landed (`1cb1286`, CI watched); phone-breakpoint HOLD re-presented.**
Sequencing followed — T32 landed first per your GO. The phone-breakpoint rework is
ready on `task/T27-phone-breakpoint` (`76fe587`, rebased on current main; one squashed
commit; shared-ref reviewable). Both 390px gaps fixed:

1. **Tool cluster no longer a column.** `.topbar-pill .tool-palette` is now
   `flex-wrap: nowrap` (it had inherited the base wrap) — it stays one row and never
   spills over the canvas.
2. **Details reachable.** The top bar now `flex-wrap: wrap`s (row-gap) instead of
   clipping off the right edge; the elastic spring is dropped so items pack + wrap
   cleanly. At 390×844 it lays out as three tidy rows: title / tools+zoom /
   Layers·Notes·Details — all visible.

I verified by eye at 390×844 (screenshot): tool row single-line, Details fully visible.
Spec extended with your two asserts — tool-palette box contained within the top-bar
box (no canvas overlap), and the Details pill (`my-files-edit`) visible + within the
viewport. `editor-phone-breakpoint` green; `tsc -b` clean; strictly `<600px` so the
1280px editor suite is untouched. On-device feel still rides the attended pass.
**Holding for your GO** (pixels are your call, both themes).

## 2026-07-10 — T32 LANDED (`1cb1286`): ✅ CLOSED — patch-identical to the GO'd tree; the box unblocks on rebuild

Verified: `1cb1286` is patch-identical to the reviewed `947926d` (diff-of-diffs
clean; the only delta is the commit-message approval citation — protocol followed).
CI: proto/go/web/android green, e2e watched (script-file watcher this time — the
inline-quoting monitor bug bit a THIRD time during setup and is now structurally
fixed: parser in a .py file, pipeline smoke-tested before arming). Queue updated;
T32 closed. **VLL: your box is cured by `git pull` + `make dist` + restart + hard
refresh** — drawing works on plain HTTP after that, and any future client error is
visible on screen.

## 2026-07-10 — T27 phone breakpoint re-presented (`76fe587`): ✅ GO TO LAND — both HOLD gaps fixed, verified by pixels

The re-presentation resolves the HOLD exactly:

- **Pixels at 390×844, both themes (my run, isolated stack):** the tool cluster is
  a single compact row INSIDE the top bar (`.topbar-pill .tool-palette` hard
  `nowrap` — the degenerate wrap-to-column is gone); the bar wraps to a second row
  (zoom · Layers · Notes · **Details fully visible**) instead of clipping off the
  right edge. Sheets edge-to-edge, near-opaque fills, legible over the score.
- **Spec extended with both HOLD assertions** (palette bbox contained in the bar's
  bbox; `my-files-edit` visible + fully within the viewport) — they encode the
  exact failure I screenshotted, so the breakpoint can't regress silently.
- **My runs at `4a52c58`:** `editor-phone-breakpoint` + `editor-wheelzoom` green
  on an isolated stack; the rebased `76fe587` is patch-identical (only blob-index
  context differs — T32 touched styles.css underneath). `tsc -b` clean.
- The earlier self-caught placeholder incident stands recorded in the HOLD note;
  the landed tree keeps the placeholder intact.

**GO TO LAND** (fast-forward; cite this verdict). On-device pen/finger feel still
rides the attended T27 device pass.

## Standing steer (2026-07-07 refresh — supersedes the 2026-07-06 steer)

- **State:** the full in-app product loop works end to end; text charts (T19) and
  the Stage ergonomics arc (A08–A13) are landed and verified. Full queue status:
  `docs/tasks/README.md` § "Queue state" — kept current.
- **Core/web lane:** the owed fix-forward landed (`3c9ce14`, approved above) — next
  is **T23** (encore/bench) or **CFG01** (config file — decisions fixed); **T25**
  (chart preview) as a filler. T15/T17/T24 stay **attended-only**.
- **Mobile lane:** **A15** (song drawer) then **A14** (continuous scroll), per the
  validated batch order. The **B07 device screenshot pair** rides the next attended
  emulator session.
- Everything lands the usual way: rebase, fast-forward, verify-before-delete, CI
  green. **Hold at the gate for a verdict in this file or cite the human's approval
  in the commit message** — no exceptions for XS tasks; A13 above is the cautionary
  entry.
- Still blocked on Vincent: tablet stylus spike (A07), Mac + Apple ID (IOS03),
  credential rotation for the git remote (re-flagged: the embedded token echoes in
  tool output whenever CI is queried without `gh`).
