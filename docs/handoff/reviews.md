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

## 2026-07-10 — T27 phone breakpoint LANDED (`f90a7ca`): ✅ CLOSED — the stage-4 residue is done

Patch-identical to the GO'd `76fe587` (diff-of-diffs clean; only the approval
citation added — protocol followed). CI on `f90a7ca` watched (script-file watcher).
With this, T27's unattended scope is fully complete: the ONLY remaining T27 item is
the attended device pass (iOS pinch-guard + pen/finger feel, rides A07). Queue
updated.

## 2026-07-10 — USER-JOURNEY tidy (`7fb3c42`, docs-only, landed direct): ✅ APPROVED post-hoc — register is truthful again

Spot-verified against the gate record: every ✅ ref is real (`ac0066e` B06 core
slice, `2a53bfe` B07, A08–A12 within the landed set, T20 `8257d54`, T21, the P202
safe slice, T32) and the still-open rows match the queue exactly (B06 app half →
mobile; P204 deferred; I11 widening = product call). The refreshed bottom line is
the right read: with the ergonomics arc, B07, and T32 all landed, **OPS01 is now
the single blocker between "demo" and "my band actually uses this"** — consistent
with the audit's OPS01-urgency call. Good practice note: the tidy split the
register into actionable vs resolved instead of deleting history.

## 2026-07-10 — ⏳ PRE-GATE HOLD (arch → web-core): T33 (`e435794`) — the bar is RIGHT, the ⋯ popover is invisible + unreachable to real users

The headline is earned: ctx bar **41.6px vs topbar 46.7px** (my measure), one slim
row, both themes, all 28 specs in my batch green, `tsc -b` clean, assertion freeze
honored to the letter (zero `expect()` lines changed — verified mechanically).

But the ⋯ popover FAILS the pixel check, and it's the ed5-#5 class again — worse:
**green-spec-broken-UI**. Evidence from my run at `e435794`:

- Screenshot with `style-blend` asserted visible: **no panel paints anywhere** —
  the ⋯ button shows its active state, the area below is bare canvas.
- `elementFromPoint` at the blend select's own center returns `DIV.viewer-scroll`
  (`hitIsBlendOrChild: false`) — a real user's click lands on the CANVAS.
- Cause: the panel is a child of `.style-controls`, which is `overflow-x: auto`
  (the bar's scroll container) — the popover is clipped out of paint entirely.
  Your spec passes only because Playwright's actionability machinery scrolls the
  overflow container to reach it; humans can't.

Fixes required before the gate:
1. Render the panel OUTSIDE the overflow clip — anchor it to `.ctx-bar` itself
   (sibling of `.style-controls`), or `position: fixed` / a portal. Mind the two
   ctx-bar traps: `pointer-events: none` on the bar (the panel needs its own
   `auto`) and z-index above the chrome.
2. Add the class-killer assertion to `editor-ctx-thin.spec.ts`: after opening ⋯,
   `elementFromPoint` at `style-blend`'s center must resolve to the select (or a
   descendant) — the exact probe that caught this. That kills the whole
   "Playwright-reachable but human-unreachable" class for popovers.
3. Re-present; I'll re-run pixels + the probe.

## 2026-07-10 — ⏳ T33 HOLD UPDATE (arch → web-core, on `1c9aafc`): clip fixed and probe-verified — two items left

Re-ran at `1c9aafc` ahead of your memo. The `position: fixed` escape WORKS: the
panel paints (FILL/BORDER/BLEND/HEX all visible, both my probe and pixels agree)
and my `elementFromPoint` check at the blend select's center now resolves to the
select — the human-unreachable defect is gone.

Two items before GO:

1. **The panel is mis-anchored ~300px left of its trigger.** The ⋯ button sits at
   the bar's right end (x≈930 at 1280 wide); the panel renders at x≈430–620,
   visually disconnected from what was clicked. Check the measured-coords math —
   for a right-aligned anchor, `style.right` must be
   `window.innerWidth - buttonRect.right` (a raw `rect.right`/`rect.left` there
   lands exactly where I saw it). Should hang directly under the ⋯.
2. **HOLD ask #2 is still owed:** the committed `elementFromPoint` probe in
   `editor-ctx-thin.spec.ts`. Without it, re-parenting the panel back into the
   scroll container someday would go green again — the probe is the class-killer,
   not a nicety. (My scratch version, for reference: bbox center of `style-blend`
   → `document.elementFromPoint` → `closest('[data-testid="style-blend"]') !==
   null`.)

Everything else stands from the first HOLD review: 41.6px bar, 28-spec batch
green, freeze honored. Fix the anchor, commit the probe, present.

## 2026-07-10 — ⏳ T33 HOLD, round 3 (`8f3da4d`): probe committed ✅ — but the anchor bug has a ROOT CAUSE your math can't fix from where the panel lives

The committed `elementFromPoint` probe is exactly right (ask #2 closed — nice
form, the `hit.contains(sel)` case included). But the panel is still mis-anchored,
measured at `8f3da4d`: panel right edge **622** vs the ⋯ trigger's **946**
(324px off), panel top **167** vs button bottom **98** (69px gap instead of ~6).

**Root cause — not your arithmetic:** `.ctx-bar` has `transform:
translateX(-50%)`. A transformed ancestor becomes the CONTAINING BLOCK for
`position: fixed` descendants (CSS spec), so your viewport-based coords
(`window.innerWidth - b.right`, `b.bottom + 6`) resolve against the BAR's box,
not the viewport — that double-offset reproduces the 324/69 numbers exactly.
No right/left formula fixes this while the panel stays inside the transformed
subtree.

Fix (pick one):
a. **Portal the panel to `document.body`** (`createPortal`) — `fixed` becomes
   truly viewport-relative and your existing math is already correct. The
   outside-click/Esc logic keeps working (it checks containment via refs).
   Recommended.
b. Keep it in-tree but switch to `position: absolute` with coords computed
   relative to `.ctx-bar`'s padding box (fiddlier; the overflow container must
   stay out of the ancestry — it does, the containing block is the bar).

And gate the anchor too: add to the spec `|panel.right − btn.right| ≤ 8` and
`0 ≤ panel.top − btn.bottom ≤ 12` (bounding boxes) — my screenshots are the only
thing catching this today, and screenshots don't run in CI.

---

❓ **Web-Core → gate (2026-07-10): T33 re-presented — all three HOLD rounds addressed.**
Branch `task/T33-thin-ctx-bar` (`ed1c032`, off main; shared-ref reviewable; one squashed
commit). Your round-1/2/3 findings were exactly right — including the transformed-
ancestor root cause. Point-for-point:

1. **Clip (round 1):** the popover no longer lives inside `.style-controls`' overflow
   scroll container. **Portaled to `<body>`** via `createPortal` — so it's also outside
   the ctx bar's `transform: translateX(-50%)`, which (round 3) was making
   `position: fixed` resolve against the bar, not the viewport (the ~300px offset).
   `pointer-events: auto` on the panel (trap #1); `z-index: 40` clears the chrome +
   drawer (trap #2). Coords are the ⋯ button's `getBoundingClientRect` (viewport-space,
   now correct post-portal).
2. **Class-killer probe (round 1):** `editor-ctx-thin.spec.ts` — after opening ⋯,
   `elementFromPoint` at `style-blend`'s centre must resolve to the select/descendant.
   **Red-first proven:** reverted to the clipped `position: absolute` → the probe FAILS
   with "must be the top element"; restored `fixed` → passes.
3. **Anchor gate (round 3):** the two bounding-box assertions you specified —
   `|panel.right − btn.right| ≤ 8` and `0 ≤ panel.top − btn.bottom ≤ 12` — are in the
   spec. They fail on the mis-anchored (pre-portal) version.

Self-caught in parallel: a dark-mode screenshot check flagged the clip before I saw your
HOLD (the phone-breakpoint lesson applied); the anchor offset I initially missed by eye,
your probe + my anchor asserts now both catch it. Verified: `tsc -b` clean; the 6
affected specs 24/24 (ed5/layers/locked-restyle/uxfix/hidden-layer-draw got open-⋯ /
close-drawer mechanics — the slim bar puts presets+⋯ at the right end under the drawer;
no `expect()` changed); full editor suite was 51/51 before these popover-only tweaks.
Bar: 41.6px (your measure) vs 46.7px top. **Holding for your re-run (pixels + probe).**

## 2026-07-10 — T33 GATE REVIEW (`ed1c032`): ✅ GO TO LAND — three rounds converged; every finding fixed AND gated

Final verification at `ed1c032`, all my own runs on the isolated stack:

- **The number:** ctx bar 41.6px vs topbar 46.7px — one slim row, thinner than
  the target. (Was 96.5px — a 2.3× reduction.)
- **The popover, fully resolved:** portaled to `document.body` (the correct
  escape from the `transform` containing-block trap), anchored EXACTLY —
  measured hGap **0.0px**, vGap **6.0px** — paints and hit-tests clean in both
  themes (`elementFromPoint` at the blend select's center resolves to the
  select).
- **Every regression class from this review is now COMMITTED as an assertion:**
  height ≤ topbar+2px (shape + text targets), the `elementFromPoint`
  reachability probe, and the anchor bounds (|Δright| ≤ 8, gap ≤ 12). The
  "Playwright-reachable but human-unreachable" class and the mis-anchor class
  both die here.
- **Batch:** 27 specs green (ctx-thin + the four mechanic-updated + hidden-layer
  + editor core); `tsc -b` clean; assertion freeze held all three rounds (zero
  `expect()` changes in the mechanic edits — verified each round).

Review-history note for the log: round 1 caught paint-clipping via pixels, round
2 caught mis-anchoring via bbox measurement, round 3 identified the containing-
block root cause the lane's coord math couldn't fix — each finding produced a
COMMITTED guard, which is the point of the gate. **GO TO LAND** (fast-forward;
cite this verdict).

## 2026-07-10 — T33 LANDED (`0e4381a`): ✅ CLOSED — the ctx pill is one slim row

Patch-identical to the GO'd `ed1c032` (diff-of-diffs clean; only the approval
citation added). CI: proto/web/android green at check time, go/e2e watched
(script-file watcher). Queue updated — T33 done. The editor chrome is now fully
mockup-faithful: 46.7px top pill, 41.6px ctx pill, one row each, with the height,
reachability, and anchoring all e2e-gated.

## 2026-07-10 — T33 CI RED on `0e4381a` → fix-forward `8dabe08`: ✅ the height guard caught a REAL platform defect

The e2e job failed on the NEW height guard itself: CI measured the ctx bar at
**52.06px** vs the 48.7 ceiling — locally it's 41.6 and green (mine and the
lane's runs). Diagnosis: CI's Linux headless uses **classic scrollbars** (like
Windows); `.style-controls`' `overflow-x: auto` grew an ~11px horizontal
scrollbar INSIDE the slim pill. Local overlay-scrollbar Chromium never shows it.
So this was NOT a flaky assertion — the guard caught that on classic-scrollbar
platforms the pill genuinely fattened for users (T13's environment-sensitivity
class, but a real UI defect this time, not a test artifact).

Fix-forward by the architect (red main; T26-gofmt precedent): hide the scrollbar
on the strip (`scrollbar-width: none` + `::-webkit-scrollbar{display:none}`) —
it still scrolls by wheel/touch/drag, and a visible scrollbar inside a 46px pill
was never the design. `editor-ctx-thin` green locally, `tsc -b` clean, CI on
`8dabe08` watched. Lane: no action needed; noted here so the round-trip is in
the log.

## 2026-07-10 — T33 red, CORRECTION (`6d6b19e`): my scrollbar diagnosis was WRONG — the real cause was font-dependent text WRAP, proven to the hundredth

Owning it: the `8dabe08` fix-forward (hide the scrollbar) did nothing — CI
measured **exactly 52.0625 again**, and an identical-to-the-hundredth number is a
deterministic layout, not scrollbar noise. I had pattern-matched to the
scrollbar-takes-space class without proving the mechanism. The proof pass that
should have come first: instrument per-child heights, then try to REPRODUCE the
CI number locally. Doing that showed the real cause immediately — under
CI/Windows **fallback fonts the "draw: rect" target chip wraps to two lines**;
forcing the wrap locally produces `wrapped=52.06` — the CI number exactly.

Fix (`6d6b19e`): `white-space: nowrap` on the pill's text elements — wide glyphs
now overflow into the strip's scroll instead of growing a second line, and the
`8dabe08` scrollbar-hiding (kept: correct defensively) makes that overflow
scroll invisible — the two compose. Verified under the CI-emulating constraint:
41.59px where pre-fix hit 52.06. `editor-ctx-thin` green, `tsc -b` clean, CI on
`6d6b19e` watched.

Checklist lesson (both roles): **before fixing a CI-only layout delta, reproduce
the exact number locally** — a delta you can't reproduce is a diagnosis you
don't have. (Same family as the stale-build rule: confirm the mechanism, not
the resemblance.)

---

❓ **Web-Core → gate (2026-07-11): T34 (HIGH) presented — touch-editor stuck-in-nav fixed + reproduced.**
Branch `task/T34-stuck-nav` (`7186ef5`, off main; shared-ref reviewable). Executes your
root-cause spec exactly:

1. **Self-heal (load-bearing):** `WetCanvas.onPointerDown` now clears `pointersRef` +
   `navRef` when `e.isPrimary` — a primary pointer is by spec the only active pointer of
   its type, so any lingering same-type entry is a stale missed up/cancel. `penSeenRef`
   left sticky (out of scope; flagged for the A07 device pass).
2. **Capture the nav pointers:** throw-safe `capturePointer` on both nav ids + extra
   touches, so real devices deliver the up/cancel to the canvas even lifting over chrome
   or off-window.
3. **Reproducer** (`editor-touch-stucknav.spec.ts`): raw touch PointerEvents +
   `setPointerCapture` throw-shim; F2's lift dispatched on `document.body` (the missed
   up) → fresh single-finger stroke must commit. **Red-first proven** — stashed the fix,
   the missed-lift test FAILS (`object-count` stays 0, the exact symptom); restored →
   passes. Clean-lift control passes both (anti-overcorrect).

Verified: `tsc -b` clean; `editor-touch` (CDP pinch → one raster) stays green (capture
didn't break it); **full editor suite 53/53**. **Holding for your GO.**

Sequencing note: after T34, I'm resuming the **OPS01 unattended deploy slice** (plan
approved by VLL: `deploy/` compose + Caddyfile + backup + docs, tested backup/restore;
attended live-TLS + mobile APK deferred) — will present that separately.

## 2026-07-11 — T34 GATE REVIEW (`7186ef5`): ✅ GO WITH ONE PRE-LAND CONDITION — the heal must close a live gesture

Verified (my runs, isolated stack): the two-variant reproducer + the CDP pinch
test — 3 passed at `7186ef5`, `tsc -b` clean; **red re-proven independently**
(spec copied onto pre-fix main: the missed-lift test fails exactly as the field
bug, the clean-lift control passes both sides). The `capturePointer` throw-safe
wrapper and the isPrimary self-heal match the spec; penSeenRef correctly left
out of scope.

**The condition — one line plus one test.** The self-heal does
`navRef.current = null` WITHOUT `endGesture()`. In the BOTH-lifts-missed flavor
(both fingers lift over chrome/off-window simultaneously — realistic on a
tablet, fingers ending at the pill edges), `navRef` is still LIVE when the next
primary touch heals: `beginGesture` has set a live CSS `transform` +
`willChange` on the content and `wheelBurstRef` is populated —
`commitWheelZoom` never runs, so the score is left CSS-zoomed/panned and blurry
(no crisp re-raster) until some later pinch happens to settle. Fix in the heal:

    if (e.isPrimary) {
      pointersRef.current.clear();
      if (navRef.current) { navRef.current = null; endGesture(); }
    }

…and reproducer variant #3: nav → BOTH ups dispatched on `document.body` → next
single-finger stroke commits AND the content wrapper has no residual inline
`transform` (or equivalently the settle raster committed). Pre-fix-of-condition
this variant leaves the transform stuck — assert on it so the flavor is gated.

Everything else is GO as presented. Land after the condition (fast-forward,
cite this verdict).

## 2026-07-11 — ⏳ PRE-GATE NOTE (arch → web-core): OPS01 slice (`96f53e4`) — two build-breakers the missing docker hid; everything else verified

Pre-reviewed ahead of your claim. The verified-good first:

- **backup.sh re-run end-to-end by me** (file backend, :8094): register→band→song
  → server stop → `backup.sh backup` → wipe → `backup.sh restore` → restart →
  login 200, `RESTORED BANDS: ['OpsBand']`, `RESTORED SONGS: ['OpsSong']`. The
  script's safety posture is right (refuses a non-empty restore target, no
  destructive ops, server-stopped guidance).
- All nine `TROUBA_*` names in the compose/Dockerfile exist in the config
  registry (typo'd env vars silently default — checked each). `/healthz` exists;
  the node-based healthcheck works in a node-slim runtime. Caddyfile is
  idiomatic (`reverse_proxy` does the WebSocket upgrade natively — important,
  the editor is WS). No secrets; `.gitignore` covers `.env` + archives.

**The two build-breakers (the honest "authored to spec, not run" gap made
flesh — this is what the attended bring-up would have hit on VLL's box):**

1. **`web/ink` is never copied into stage 1.** Studio resolves
   `@troubastack/ink` from SOURCE via the Vite alias to `../ink/src` (repo
   quirk: deliberately not an npm dep), and bake's esbuild does the same —
   `npm run build` for BOTH fails on the missing directory. The image cannot
   build anywhere. Fix: `COPY web/ink web/ink` before the studio build (ink
   needs no install of its own — perfect-freehand is deliberately listed in
   studio's and bake's own deps for exactly this).
2. **`core/internal/gen/` is gitignored (0 tracked files).** `COPY core core`
   copies the BUILD CONTEXT, so the go build works only where codegen output
   already exists locally — a fresh `git clone && docker build` fails on missing
   packages. Fix: either a buf-generate step in the Go stage, or (pragmatic)
   document `make proto` as a hard prerequisite in deploy/README + a Dockerfile
   comment. Your call; the README currently doesn't mention it.

Both are contained in the Dockerfile/README — the compose/Caddy/backup design
needs no changes. Fix, re-present; the live `docker compose up` on a real host
stays the attended acceptance step (correctly deferred to VLL's box).

## 2026-07-11 — T34 LANDED (`c2d75a9`): ✅ CLOSED — the stuck-in-nav field bug is dead, all three flavors gated

The pre-land condition delivered exactly: the heal now `endGesture()`s a LIVE
nav (both-ups-missed flavor), and variant #3 gates it properly — it first
asserts the live CSS transform was actually applied mid-pinch, then that a fresh
stroke commits AND the transform settles to "" post-heal. My run at the landing:
all three stucknav variants + the CDP pinch test green (4 passed), `tsc -b`
clean, delta vs the reviewed `7186ef5` is precisely the condition (+ the memo),
citation correct. CI watched. VLL's tablet: after the next `make dist` +
restart, a two-finger gesture can no longer jam the editor — however the
fingers lift.

---

❓ **Web-Core → gate (2026-07-11): OPS01 slice re-presented — build-breaker #1 fixed; #2 respectfully refuted with evidence.**
Branch `task/OPS01-deploy` (`d37aca9`, rebased on main; shared-ref reviewable). Thanks
for the pre-review + the backup.sh re-run.

1. **web/ink not copied — FIXED.** `COPY web/ink web/ink` at the top of stage 1
   (before both builds); its only dep, perfect-freehand, comes from studio's + bake's
   own `npm ci`. Verified locally that `vite build` (studio) + `npm run build` (bake)
   both succeed with only web/ink source + those deps present — the exact state the
   image has after the COPY.

2. **core/internal/gen — NOT a build-breaker (evidence).** core imports **no** codegen:
   `grep -rn internal/gen core --include=*.go` → **0 hits**; `core/internal/gen/` is
   **absent** on my tree right now, and `cd core && go build ./...` → **BUILD OK**. The
   proto types are hand-mirrored (I1/P203, the "four hand-written mirror sets"), so the
   buf `gen` target is unused by the go build — `COPY core core` + `go build` works from
   a fresh clone with no buf/codegen step. (Same for web: no `proto-gen` import — the TS
   types are hand-mirrored too.) I added a Dockerfile comment to preempt the confusion
   rather than a needless buf-in-image stage. If you're seeing a core import of `gen` I'm
   not, point me at it and I'll add codegen — but I couldn't find one.

Unchanged from your verified-good review: compose/Caddyfile/backup/secrets. The live
`docker build` / `docker compose up` on a real host stays the **attended** acceptance
(no docker in my env — authored to spec, not run; deploy/README has the exact steps).
**Holding for your re-review.**

## 2026-07-11 — OPS01 pre-gate round 2 (`d37aca9`): ✅ GO TO LAND the unattended slice — finding #1 fixed, finding #2 was MINE to retract

- **Finding #1 (web/ink missing): FIXED** — `COPY web/ink web/ink` with the
  correct rationale comment (ink resolves from source via the aliases;
  perfect-freehand rides studio's/bake's own deps).
- **Finding #2 (gitignored core/internal/gen): RETRACTED — I was wrong.** The
  lane's counter-claim checked out under my own verification: NOTHING in core
  imports `internal/gen` (not even tests), and `go build ./...` succeeds with
  the directory physically removed. The proto types are hand-mirrored (the I1
  codegen debt, P203's territory); gen is a drift-check target, not a build
  input. I pattern-matched "gitignored generated code" to "build input" without
  checking imports — the Dockerfile comment they added documents the reality.
- Everything else stands verified from round 1 (backup e2e re-run good, env
  names, Caddyfile/WS, healthcheck, no secrets).

**GO TO LAND** the unattended slice (fast-forward, cite this verdict). Remaining
OPS01 scope after landing: the ATTENDED acceptance — live `docker compose up`
HTTPS bring-up on VLL's box (docker exists nowhere in the agent envs; the
Dockerfile is authored-to-spec and now inspection-clean, but only a real build
proves it) — and the release-APK half (mobile lane + VLL's keystore).

## 2026-07-11 — OPS01 unattended slice LANDED (`3662468`): ✅ CLOSED (slice) — the adoption blocker is now on VLL's bench

Patch-identical to the GO'd `d37aca9` (only the approval citation added). CI
watched. **What ships:** the production Dockerfile (multi-stage, SPA embedded,
bake-capable runtime, non-root), compose + Caddy (automatic TLS, WS-native,
domain as the only variable), tested backup/restore, deploy/README. **What
remains of OPS01 — both on VLL:**

1. **The attended bring-up:** on the box — `git pull`, set `DOMAIN` in
   `deploy/.env`, point DNS, `docker compose up -d` from `deploy/`. First real
   `docker build` anywhere (no docker in any agent env); if it breaks, bring the
   error to the gate.
2. **The release APK:** mobile lane + VLL's keystore decision (spec item 3).

With the slice landed, the box also gets the T32 (silent-error) and T34
(stuck-nav) fixes on its next rebuild — one `git pull` + bring-up covers all
three arcs.

## 2026-07-11 — T26 app half + T23 drawer grouping (`604ab37`, mobile lane): ✅ GO TO LAND — both A-track halves done, verified

The queue-sanctioned pairing (T23 §4 explicitly pairs with T26's drawer touch) in
one commit — right call, noted. My verification:

- **Fresh `--rerun-tasks` `:shared:check`: green** (exit 0, only the known
  expect/actual Beta warnings). **SongDrawerTest 5/5, BundleLoaderTest 11/11** —
  exactly the claimed counts, re-extracted from the XML reports.
- **Code read:** `BundleModel` gains `title`/`onCall` additive-with-defaults (old
  bundles stay valid); `buildLoaded` names songs from the baked title with the
  "Song N" fallback on blank; the drawer partitions on `onCall` keeping each
  row's ORIGINAL index (the jump-lands-right subtlety handled — `withIndex`
  before partition).
- **Test hygiene is right:** the stale "unknown onCall" tolerance case became a
  real mapping assertion (both fields, mixed songs) PLUS a genuinely-unknown
  field case — the back-compat guarantee stays gated instead of silently
  becoming a mapped-field test.
- **Pixels:** the committed `t26-song-titles-drawer.png` (emulator, real bake
  driven end-to-end per the message — titles + a PATCHed onCall song baked last)
  shows real titles with metadata lines, the current-song highlight, and
  Hallelujah under the "On call" divider. The shipped old demo keeps "Song N" —
  back-compat visually confirmed too.
- Rebase over the README landing is patch-identical (delta = my README files).

**GO TO LAND** (fast-forward, cite this verdict). This closes BOTH A-track
follow-ups: T26 is fully done (core + app), T23 is fully done (server + drawer).

## 2026-07-11 — T26 app half + T23 drawer LANDED (`309f06f`): ✅ CLOSED — T26 and T23 are both fully done (core + app)

Patch-identical to the GO'd `604ab37` (fully identical — same commit, fast-
forwarded). CI watched. **Process nit for the mobile lane** (logged, no action):
the landing does NOT cite the verdict in the commit message — the
cite-approval-at-landing rule applies to the A-track too; amend before pushing
next time (the web-core lane's `Approved: <verdict>` trailer is the pattern).
The GO itself is unambiguous in this log, so the record stands.

Queue effect: **T26 CLOSED** (proto field 9 → bundle → Kotlin mirror → drawer)
and **T23 CLOSED** (encore/bench: server + Stage drawer grouping). The mobile
lane's remaining queue item is the **B06 app half** (Connect-screen browse).

---

❓ **Web-Core → gate (2026-07-11): P203 Stage 0 done — decision-prep + proposed verdict for you/VLL to ratify.**
Web-core queue was drained (T34/OPS01 landed; rest mobile/attended/deferred), so per VLL
I took P203 Stage 0 (the "cheap, do first" decision-prep). Branch `task/P203-stage0`
(`14e5532`, off main; findings written into P203's Stage 0 section). **Decision-only — no
client code touched.**

**Prototype:** `buf generate` (buf 1.71.0, protocolbuffers/go, scratch tree). Concrete
delta, `BakedSong.source_revision` (uint64):
- generated: `json:"source_revision,omitempty"` (snake_case, numeric) for `encoding/json`;
- canonical (docs/design/08, the fixture oracle): `"sourceRevision": "5"` (camelCase,
  string) — which the mirrors produce via `,string` tags / custom serializers.
- The camelCase name lives only in the `protobuf:` tag that **`protojson`** reads. So
  canonical JSON from generated types needs `protojson` — the **transport change P203
  excludes**. Same per client (TS `bigint` vs `string`; Kotlin runtime vs custom
  serializers). Plus: protos have **no `go_package`** (Stage-1 prereq); `sync`'s 2 oneofs
  → wrapper types vs the hand `Kind` discriminators.

**Proposed verdict (yours to ratify): RE-AFFIRM mirrors for another phase** — the mirrors'
whole job is the canonical JSON that generated types can't emit without the excluded
protojson/runtime; types-only adoption re-adds the same layer for ~5 tidy message
families the discipline has kept aligned. IF you'd rather adopt (moving the bundle onto
protojson too), the gate must settle that encoding policy first; the migration order is
in the file. **Holding — this is your/VLL's call, not mine to land.**
## 2026-07-11 — P203 Stage 0 (`14e5532`): ✅ ANALYSIS ENDORSED — arch recommends RE-AFFIRM; the final call is VLL's

Verified the three load-bearing claims against the tree before endorsing:
zero of the five protos carry `option go_package` (protoc-gen-go emits nothing
without it); the hand mirror's tag is literally
`json:"sourceRevision,string,omitempty"` — the camelCase + string-uint64 shape;
and `docs/design/08` pins exactly that proto3 canonical JSON as the byte-for-byte
fixture contract. The protobuf-go split (generated `json:` tags are snake_case;
the canonical name lives in the `protobuf:` tag that only `protojson` reads) is
textbook-correct.

**The analysis nails the real decision:** the mirrors' entire job is the
canonical-JSON encoding, and types-only codegen cannot produce it with the
standard serializers — you'd re-add the same hand layer on top of generated
structs. Adopting honestly means switching serialization to `protojson`
(+ equivalents per client), which is the transport change P203 itself excludes.
Against ~5 well-behaved message families guarded by AUTHORITY comments, review
discipline, and the fixture oracle, the cost/benefit is clear.

**Arch recommendation to VLL: RE-AFFIRM mirrors-with-discipline for another
phase.** The re-visit trigger is written into the analysis: if/when we WANT the
serialization swap (e.g. protojson everywhere) or the message-family count grows
past what discipline holds, the staged adoption order in the file is the plan.

**GO TO LAND the Stage-0 text** (docs-only) with one edit: mark the verdict line
"arch endorsed 2026-07-11 — awaiting VLL's decision" so the file stays honest
about who has ruled. When VLL answers (chat), the queue + this file get the
final state.

## 2026-07-11 — B06 app half (`25e3cf3`, mobile lane): ✅ GO TO LAND — scope call ACCEPTED; the distribution loop is type-no-IP

My verification:

- **Fresh `--rerun-tasks` `:shared:check` green; DiscoveryTest 5/5** (url/label
  derivation + dedup/stable ordering — the pure logic).
- **I15 holds:** zero `expect/actual` added — `ServerDiscovery` is a pure
  commonMain fun-interface + data class; `NsdServerDiscovery` lives in the
  ANDROID APP layer (DI glue), not a fourth native seam in shared. Correct
  structure, correctly argued.
- **The Android impl reads right:** built-in NsdManager (no new dependency),
  resolve serialized under a mutex (NsdManager's one-at-a-time quirk), Wi-Fi
  multicast lock held only while the flow is collected (Connect-screen lifetime,
  no background scanning), permission documented in the manifest.
- **Security posture is the right one:** tap PREFILLS the URL only — no
  auto-connect, no credentials sent on discovery; the spoof-risk honesty is in
  the code comments (mDNS is unauthenticated; TLS/OPS01 is the mitigation).
- **Pixels:** the committed `b06-connect-discovered.png` shows the discovered
  rows ("Rehearsal Mac", "Venue PC") above the URL field with Connect still
  disabled — the described UX exactly (fake-source injection, reverted; honest
  about the emulator's mDNS NAT limitation).

**Scope call RULED — ACCEPTED:** no iOS `NWBrowser` impl now. iOS is Stage-only
(no Connect screen), so a browse impl would be dead code; shipping the
`NSBonjourServices`/usage-description plist keys NOW (done) means the capability
is ready when the shared Connect screen reaches iOS. Recorded as part of the
IOS-track future work, not a gap.

**Attended item added:** a LIVE two-host mDNS check (real device + a real
server on the same LAN) — the emulator cannot deliver host multicast; rides
VLL's next device session alongside the T27 pass and the two viewport caveats.

**GO TO LAND** (fast-forward, cite the verdict IN THE COMMIT — see the citation
nit on your T26 landing). This empties the mobile queue's unattended items and
closes B06 fully (core + app): the band member types NOTHING to find the server.

## 2026-07-11 — B06 app half LANDED (`4e0c024`): ✅ CLOSED — B06 fully done; the mobile unattended queue is EMPTY

Patch-identical to the GO'd `25e3cf3`, and the landing CITES the verdict (the
T26 citation nit took — good). CI watched. **B06 is fully closed** (core mDNS
advertise + app Connect-screen browse): a band member on the LAN types nothing
to find the server. The live two-host mDNS check rides VLL's attended device
session.

**Queue state after this:** every unattended agent-executable item is DONE.
What remains is attended (VLL: OPS01 bring-up + release APK/keystore, the T27
device pass + viewport caveats + live mDNS, A07 stylus spike, T24, B07
screenshots), decisions (P203 re-affirm/adopt — arch recommends re-affirm;
LICENSE; credential rotation), and deferred-until-pressure (P204, P201 pending
B03-app... P201 check: rides VLL's priority call).

## 2026-07-11 — P203 DECIDED: RE-AFFIRM (VLL) — closed as decided-not-adopted

VLL ratified the Stage-0 recommendation (chat, 2026-07-11: "re-affirm P203").
Recorded in the task file (verdict header rewritten from proposed→DECIDED with
the re-visit triggers), queue updated. The stage-0 landing (`375c6c4`) was
verified docs-only and consistent with the endorsed analysis. I1 stays 🎯 with
the mirror-discipline reality documented — that's T12-honest, not debt denial.

## 2026-07-11 — Mobile handoff refresh (`ea703f3`, docs-only, landed direct): ✅ APPROVED post-hoc — accurate and usefully self-critical

Spot-verified: all five A08–A12 commit refs in the new arc table are real and
correctly described; the queue-empty claim matches the gate record; the attended
list (A07, IOS03, live mDNS, B07 shots, release APK) is exactly right. Best part:
the refresh encodes the citation rule AS PROCESS — owning the A13 breach and this
session's T26 nit, and documenting the message-only-amend trailer technique
(diff stays patch-identical to the reviewed SHA). That's the lane teaching its
future self the lesson — the handoff doing its job.

---

❓ **Web-Core → gate (2026-07-11): field regression fixed — song metadata was unreachable in the editor.**
VLL field report: "where can I edit song info (default key, author, tempo) — nothing in
Details, no ⋯". Root cause: the T27 full-bleed reshape set the page
`height:100dvh; overflow:hidden`, which **clipped** SongEditor's "Details & files"
section — including the `<Metadata>` form — off-screen (not scrollable-to), and the
in-editor "Details" pill was wired only to `<MyFilesEditor>` despite its "Song details &
files" label. So there was no working way to edit song metadata. (Fixed on
`task/song-details-reachable`, `84ab1f3`, off main.)

Fix (reachability of existing UI — no new form, same `PATCH …/songs/{id}`): `Viewer`
takes `song` + `onSongSaved` and renders the existing `<Metadata>` in its Details panel
above `<MyFilesEditor>`; `SongEditor` threads them + drops the now-duplicate clipped
`<Metadata>`. Screenshot-verified: the pill opens a clean Title/Artist/Key/Tempo/Tags/
Notes/Save panel.

Guard: `editor-song-details.spec.ts` — Details pill → metadata form visible **scoped to
the panel** (so the still-clipped copy can't false-pass), edit+save+reload persists.
**Red-first proven** (fails without the wiring). `flows.spec` test 6 got an open-the-pill
mechanic — it had been silently editing the CLIPPED form (the exact
"Playwright-reachable / human-unreachable" class from the T33 lesson). tsc clean; editor
suite + flows **65/65**.

**Flagged for your call (broader gap, left as-is):** the same clipped section still holds
`<Files>` (shared-pool/chart management) + `<DeleteSong>` (admin) — also unreachable in
the full-bleed editor, and `flows` test 7 reaches `<Files>` only via the clipped path.
Want those surfaced too (in the Details panel), or a separate task? **Holding for your GO
on the metadata fix.** Next up per VLL: T35 (wet alpha-stacking).

## 2026-07-11 — Song-metadata reachability (`84ab1f3`): ✅ GO TO LAND — a T27 regression fixed the right way; the broader gap is now a queue item

VLL's field report ("where can I edit song info — nothing in Details") traced to
the T27 full-bleed reshape clipping SongEditor's old Details section (with the
<Metadata> form) off-screen, unreachable and unscrollable. The Details pill
opened only the file manager despite its label.

Verified (my runs, isolated stack): the new `editor-song-details` spec + the
full flows suite — **11 passed**; `tsc -b` clean; **red re-proven independently**
(the spec fails on pre-fix main — form not in the panel); **pixels**: the panel
now shows the full metadata form (Title/Artist/Key/Tempo/Tags/Notes + Save)
above the file manager, exactly the pill's promise. Two things done especially
right: the spec asserts the form INSIDE the panel (scoped — the clipped copy
can't false-pass), and flows.spec's metadata test was found to be silently
editing the CLIPPED form (the Playwright-reachable/human-unreachable class
again) and got the open-the-pill mechanic. Assertion freeze honored.

**The flagged follow-up is accepted as a queue item, VLL's design call:** the
clipped section ALSO held the shared-pool <Files> editor and <DeleteSong> —
both still human-unreachable in the full-bleed editor. Where should they live?
(Candidates: more panel sections; a song-settings surface reachable from the
band page; restore page scroll below the editor.) Filed in the queue as an open
decision — the metadata fix was the urgent, unambiguous slice.

**GO TO LAND** (fast-forward, cite this verdict).

❓ **Web-Core → gate (2026-07-11): T35 (wet alpha-stacking) presented — both fixes done, reproducer red-first proven, but a real wet-path render cost I want your call on.**
Slow freehand at reduced opacity showed periodic **dark bands while wet**. Cause:
the T06 incremental wet path bakes overlapping segments (`WET_OVERLAP`) each at
`globalAlpha=opacity`, so every seam double-coats (α + α(1−α)) into a darker
stripe. Dry/committed render is a single closed outline → looked fine; wet-only,
but jarring mid-stroke. (On `task/T35-wet-alpha`, `071c61b`, rebased on main.)

**Fix 1 — uniform alpha (per your spec's compose approach):** build the cache +
live tail OPAQUE, compose them on a scratch surface, blit the whole stroke to the
wet canvas ONCE at the object's opacity. One alpha application over the union → no
per-seam stacking (mirrors ink's `paintShape("Box")` precedent). **Fix 2 —
capture-time diet:** drop freehand points closer than `WET_MIN_STEP` (~0.15% page
width) to the last KEPT point, filtered at CAPTURE so wet/dry/bake/hit-test see
identical geometry; always keep the final point.

**Reproducer** (`editor-wet-alpha.spec.ts`, **red-first proven** — I stashed the
fix and both failed): (1) at opacity 0.5 no wet core pixel exceeds ~single-coat —
**pre-fix max alpha 192 → post 127** (0.5·255≈127; the 192 is the predicted
0.75 stacked band); (2) a dense 200-move stroke stores far fewer points than
dispatched — **pre-fix 201 → post thinned**. Full editor suite (incl. wet-path
regression specs) **56/56**; tsc clean; dist placeholder untouched.

**⚠️ The call I owe you — inkPerf regression.** Your spec estimated the added cost
as "one drawImage and the same short tail," but the compose approach actually adds
**two full-canvas ops per frame** (clear compose + blit compose→wet at α) on top of
the existing cache blit. Measured (near-worst-case wiggly half-page stroke, single
run, `localStorage.inkPerf=1`):
- **pre-fix:** per-frame render first 3.0ms → last 0.2ms (max 3.0ms); mean event→paint **8.9ms**
- **post-fix:** per-frame render first 11.0ms → last 4.8ms (max 11.0ms); mean event→paint **12.4ms**

Both stay under the 16.6ms/60fps budget (no dropped frames), but the render cost
is a real 3–5×. **Options:** (a) **accept as-is** — bounded, under budget, matches
your specced approach; (b) I restrict the compose clear+blits to the stroke's
**bounding box** (a thin stroke → cheap; full-page stays full-canvas); (c) a
**persistent compose** that only re-strokes the tail region (cheapest, most code).
I lean (a) unless you consider the render delta meaningful — happy to do (b) fast.
**Holding for your GO** (and which perf option).

## 2026-07-11 — T35 GATE REVIEW (`071c61b`): ✅ GO WITH OPTION (b) — bbox-restrict the compose ops; the tablet headroom is the point

My verification first, all green: the reproducer + editor + touch-stucknav +
hidden-layer batch **9 passed** at the fix; `tsc -b` clean; **red re-proven
independently** (both tests fail on pre-fix main); the pixel math checks out
(pre-fix 192 ≈ the predicted 0.75 double-coat on white; post 127 ≈ the single
0.5 coat). The compose implementation mirrors the spec + the `paintShape("Box")
` precedent exactly; the capture filter is page-relative and keeps the final
point. Good catch flagging the perf delta instead of burying it — and yes, my
spec's "one drawImage" estimate undercounted; the full-canvas clear+blit pair is
real.

**The ruling: (b) — restrict the compose clear + blit (and the wet-canvas clear)
to the stroke's padded bounding box.** Rationale: this desktop has 4ms headroom
left at near-worst-case, but the T06 low-latency invariant exists for TABLETS —
the A07 stylus decision hinges on web ink feeling instant on hardware ~3–4×
slower than this machine, where an 11ms render max likely blows the 16.6ms
budget. (b) bounds the cost by STROKE size instead of canvas size — typical
annotation strokes are small, so it recovers most of the regression where it
matters; a page-wide stroke degrades gracefully to (a)'s cost. (c) is rejected:
the persistent-compose invalidation complexity is exactly where wet-render bugs
breed, for marginal gain over (b).

Mechanics are yours (track the stroke's device-px bbox padded by stroke width;
clear/blit that region on both surfaces); correctness stays gated by the
existing reproducer + the full suite. **Re-present with fresh inkPerf numbers**
(same wiggly half-page methodology) — I expect the typical-stroke case to land
near pre-fix; then land with both measurements in the commit message for the
record.

❓ **Web-Core → gate (2026-07-12): VLL field report — the clipped-Details gap is now a live blocker (can't add PDF, can't add a text chart, can't delete a song) + a lyrics-import wish. Requesting a spec for both before I implement.**
VLL: "we cannot add a pdf or a typing text file, we cannot delete a song; it could
be nice to import lyrics from an azlyrics link." The first three are exactly the
broader clipped-`<Details>` gap you accepted as a queue item after the metadata
fix — now confirmed as a live blocker, with VLL's design input attached.

**Diagnosis (grounded, current main):** the full-bleed editor clips `SongEditor`'s
`<Details>` section off-screen (unscrollable). Everything the user needs lives
there: `<Files>` — PDF/image **upload** (`SongDetails.tsx:281`), **"＋ New text
chart"** (`:273`), rename/reorder/delete — and `<DeleteSong>` (admin, `:512`). The
ONLY file UI reachable in the editor is the `my-files-edit` pill → `MyFilesEditor`,
which is per-member **selection** (exclude/reorder/reset of the existing pool) — it
cannot add, create, or delete. So all three actions are human-unreachable in the
editor (the same Playwright-reachable/human-unreachable class as the metadata bug).

**Item A — reachability (recommend, your call on placement):** surface `<Files>` +
`<DeleteSong>` the same way the approved metadata fix surfaced `<Metadata>` — the
Viewer Details panel already renders `<Metadata>` + `<MyFilesEditor>` (`Viewer.tsx:
1134-1138`); add `<Files>` (add/create/manage) and a guarded `<DeleteSong>` there.
Open questions for you/VLL: (1) is the Details panel the right home, or a separate
song-settings surface off the band page, or restore page-scroll below the editor?
(2) `<DeleteSong>` is destructive + admin — least-prominent slot + keep its confirm?
Guard e2e would assert each action reachable **scoped to the panel** (so the clipped
copy can't false-pass), red-first — the metadata precedent.

**Item B — lyrics import (recommend paste-first; flagging azlyrics honestly):** the
outcome VLL wants is "a song's lyrics in as a text chart, fast." I'd steer away from
an azlyrics-specific scraper: (1) azlyrics is Cloudflare-gated and ToS-prohibits
automated access — a server-side fetch from troubacore will likely 403 and is
brittle to DOM changes; (2) it's third-party copyrighted text (low practical risk
for a private self-hosted band tool, but a real dimension — VLL's call, not mine).
**Recommend (a):** a **paste-based** "New text chart from lyrics" — the user pastes
lyrics from anywhere, we normalize into the chart format (strip trailing site cruft,
keep verse/chorus breaks). Zero scraping, no external dependency, robust, works for
azlyrics or any source. **Optional (b):** a *generic* best-effort URL fetch
(readability-style text extract) with azlyrics explicitly flagged as likely-to-fail
— more moving parts. Which do you want, and if (b), server- or client-side fetch?

**Ask:** spec A + B (or split into tasks). I'll implement after your spec — this is
new design, so I'm holding rather than guessing at placement/scope. **VLL also asked
to relaunch the demo once this is done** — I'll regenerate `demo-concert.tstage` after
the implementation lands.

## 2026-07-12 — GATE ANSWER: T36 + T37 specced — both your recommendations ratified, with rulings

Answering the ask above. Two tasks, both landed in the queue:

- **T36 (HIGH — the blocker):** placement RULED — the Details panel, four
  sections (Metadata → Files → My files → Danger zone with `<DeleteSong>` last,
  confirm kept, navigate to the band page on success). The panel gets
  `max-height` + `overflow-y:auto` (it must scroll now), and the clipped
  `<Details>` section is DELETED from SongEditor — the dead copy is the
  substrate of the human-unreachable class, remove it. Guard: panel-scoped
  red-first e2e for all three actions + an `elementFromPoint` probe on the
  Danger-zone tail (the panel's own scroll must not hide it). Rejected
  placements recorded (band-page surface, page scroll).
- **T37 (normal, depends on T36):** **paste-first ratified; the azlyrics
  scraper is DECLINED** — Cloudflare + ToS + it puts the server in the copying
  business; paste keeps the human in the loop and covers every source. Spec has
  the minimal-normalizer contract (CRLF, blank-line collapse, conservative
  cruft blacklist — when in doubt KEEP) + a name-prefilled dialog into the T19
  chart editor. A generic URL fetch stays a recorded later option — VLL's
  copyright call, not ours.
- **Demo regen after both land: approved in principle** — follow the B05
  protocol (regen + pixel-verify + present; the placeholder/verify-before-
  delete disciplines apply).

Sequencing: T36 first (blocker), T37 after, T35's option-(b) rework whenever
ready — three small landings, usual gates.

## 2026-07-12 — T35 LANDED with option (b) (`21698ac`): re-presented per your conditional GO

Done per the GO WITH OPTION (b). The compose clear + both blits + the wet clear
are now restricted to the stroke's padded device-px bounding box (monotonic —
the path only grows, so clearing the current box clears every prior frame).
Correctness re-gated: `editor-wet-alpha` reproducer 2/2 (red re-proven), full
editor suite 56/56, tsc clean, dist placeholder restored.

**Fresh inkPerf (same wiggly half-page methodology, `inkPerf=1`, median of 3
back-to-back runs/config — single runs were too noisy to trust):**
- **pre-fix (no T35):** mean event→paint ~12.8ms · per-frame render steady ~5.1ms
- **post (uniform-alpha + bbox (b)):** mean event→paint ~9.4ms · per-frame render steady ~0.7ms

(b) lands **at/below pre-fix**: pre-fix re-blits the *full* cache every frame,
while (b) bounds clear+blit to the stroke bbox (a "half-page" wiggle is a thin
band → small bbox). It fully recovers the full-canvas variant's ~4.8ms/12.4ms
cost you flagged; a page-spanning stroke degrades gracefully toward that. Both
measurements are in the commit message for the record. CI polling. **This closes
the T35 line** — the "option-(b) rework" in your sequencing note is landed.

## 2026-07-12 — T35 LANDED (`21698ac`): ✅ CLOSED — option (b) exactly as ruled, and FASTER than pre-fix

Verified at the landing: the bbox restriction is precisely the ruling (all four
ops — compose clear, cache blit, wet clear, final blit — bounded to the stroke's
padded bbox via sub-rect drawImage; monotonic-growth note in the comment);
reproducer + editor + no-flicker suites green on my run; `tsc -b` clean;
citation present with BOTH inkPerf measurements recorded (median-of-3). The
numbers beat the ruling's expectation: post-fix ~9.4ms mean event→paint /
~0.7ms steady render vs pre-fix ~12.8/~5.1 — (b) not only recovered the
uniform-alpha cost, it removed pre-fix's full-cache-blit-per-frame, so the
tablet headroom IMPROVED overall. VLL's slow-stroke dark bands are dead
(pre-fix 192 → post 127 at the seams), the capture diet thins dense strokes,
and both are e2e-gated. CI watched.

❓ **Web-Core → gate (2026-07-12): VLL OVERRIDES the T37 azlyrics decline — "azlyrics is a must." Requesting you spec the fetch mechanics; I'll build to spec (honest fetch only, no evasion).**
I showed VLL the T37 ruling (paste-first ratified, azlyrics scraper declined on
Cloudflare/ToS/copyright grounds). VLL's response, verbatim: *"push for azlyrics
link"* then *"eventually also spec/generic, but azlyrics is a must."* So the human
owner is overriding the decline: **azlyrics-link import is a required feature**;
a generic-URL fetch is a wanted follow-on; paste is the fallback. VLL owns the
ToS/copyright call (their private self-hosted tool, their content).

**What I'll build vs won't** (so the spec is realistic): a server-side HTTP GET
with an honest User-Agent + an azlyrics HTML→text parser (their lyrics sit in a
known unlabeled `<div>` after a comment marker), normalized through the same T37
paste pipeline. I will **NOT** build anti-bot evasion (rotating fingerprints,
headless-browser Cloudflare-challenge solving) — it's fragile, ToS-hostile, and
detection-evasion I won't author. **Consequence to state plainly:** azlyrics is
Cloudflare-gated, so an honest server GET will *often* hit a 403/JS-challenge and
the import will fail; client-side fetch is CORS-blocked. So the azlyrics path is
**best-effort** and the paste fallback must ship alongside so VLL is never stuck.

**Ask:** re-spec T37 (or a T38) to include the best-effort fetch — I'd want your
call on: (1) server-side fetch endpoint on troubacore (`POST /api/.../lyrics-import`
taking a URL, returning normalized text or a clear "blocked, paste instead")?
(2) azlyrics-specific parser now + generic readability extract as the follow-on,
or both behind one endpoint? (3) fallback UX when the fetch is blocked. I'll cite
VLL's override + your spec at landing. **T36 (the blocker) I'm starting now** per
your sequencing; this is downstream.

## 2026-07-12 — T37 RE-SPECCED per VLL's override: azlyrics fetch is IN (honest, best-effort, SSRF-guarded)

VLL overrode my paste-only decline ("azlyrics is a must"). The human owner owns
the ToS/copyright call for his private self-hosted tool — the gate accepts it and
specced the mechanics. Answered the lane's three questions in the task file:
(1) server-side endpoint `POST /api/bands/{bandId}/lyrics-import` (client is
CORS-blocked; authed + band-scoped); (2) ONE endpoint, host-dispatched azlyrics
parser + generic readability extract shipped together; (3) fetch is an
accelerator that FILLS a paste-native dialog — block/error shows an honest
message, never dead-ends.

**Two hard boundaries written into the spec, both roles aligned:**
- **No evasion.** Honest GET + truthful UA only; NO anti-bot / Cloudflare-
  challenge / fingerprint-rotation. That's detection-evasion tooling — out on
  principle, and the spec states plainly that azlyrics will therefore OFTEN
  return blocked, so the paste fallback is mandatory shipping. Best-effort by
  construction, and honest about it in the UI.
- **SSRF guard is the load-bearing security control** (a server-fetches-arbitrary-
  URL endpoint is an SSRF vector): scheme allowlist + reject private/loopback/
  link-local/ULA resolution (re-checked across redirects) + timeout + size cap.
  Its Go test table gates the endpoint's existence; parser tests run off-network
  against committed fixtures (no live fetch in CI). Made it the #1 acceptance
  criterion.

Sequencing unchanged: T36 (blocker) first — the lane is on it — then T37. Size
bumped S→S/M (core endpoint added). Landing cites VLL's override + this spec.

❓ **Web-Core → gate (2026-07-12): T36 presented — file management + Delete song reachable in the Details panel; clipped copy removed; 85/85; pixels attached.**
Built to the RULED spec (`task/T36-details-panel`, `1ca469c`, off main). VLL's three
blockers ("can't add a pdf or a typing text file, can't delete a song") are fixed by
surfacing the existing UI in the editor's Details panel.

**Design as ruled:** the panel renders top→bottom **Metadata → Files (upload / ＋ New
text chart / rename / reorder / delete) → My files → Danger zone** (`<DeleteSong>`,
admin-only, confirm kept, navigates to the band page on success). The panel already
carried `max-height` + `overflow:auto`, so it scrolls with the four sections.
**SongEditor's clipped `<Details>` is DELETED** (and its now-unused imports) — the
clipped copy was the substrate of the Playwright-reachable/human-unreachable class,
so the substrate is gone.

**Guard** (`editor-files-delete.spec.ts`, **red-first proven** — stashed the components,
`details-panel` has no `file-input` → red): every action asserted **scoped to the
panel** so no stray copy can false-pass — upload a PDF, ＋ New text chart opens the T19
editor, delete a file, delete the song → lands on the band page; plus an
`elementFromPoint` probe on the **Danger-zone tail** after scrolling the PANEL (kills
the clip/occlusion class for the panel's own scroll). `tsc -b` clean; **full e2e 85/85**.

**Test churn (assertion freeze honored):** 24 specs uploaded via the old always-in-DOM
clipped copy; each got the open-the-Details-panel mechanic before its upload/file
actions (20 via an exact-block edit of the identical local `uploadPdf`; flows test 7,
text-chart, viewer, and the two inline uploaders by hand). Only the open/close mechanic
was added — no `expect()` lines changed.

**Pixels (I verified all four; `emulateMedia` for theme):** panel with all four sections,
**light + dark**, **desktop (1280) + phone (390)** — clean vertical stack at 390, correct
dark palette, Danger zone at the tail. (Note: the "My files" pool list in a fresh-upload
screenshot lagged one async fetch — cosmetic screenshot race in the unchanged
MyFilesEditor, covered green by viewer.spec; not a T36 change.)

**Holding for GO.** On GO I land (cite this verdict) and then **relaunch the demo** per
VLL (`make demo`). T37 (azlyrics/paste import) is next per your sequencing.

## 2026-07-12 — T36 GATE REVIEW (`1ca469c`): ✅ GO TO LAND — the blocker is fixed and the bug-class substrate is gone

VLL's blocker ("can't add a pdf/text file, can't delete a song") fully resolved.
My verification, all on the isolated stack:

- **Red re-proven independently:** the new `editor-files-delete` spec fails on
  pre-fix main (no file-input in the panel) — the four actions really were
  unreachable.
- **Green at the fix (broad batch — 24 specs were touched):** files-delete +
  song-details + flows + text-chart + viewer + editor + zorder — **19 passed**;
  `tsc -b` clean.
- **All four VLL actions reachable + pixels:** the panel scrolls Metadata →
  Files (upload / ＋ New text chart / rename / reorder / delete file) → My files
  → **Danger zone** (Delete song), both themes. The Danger-zone `elementFromPoint`
  probe resolves to the delete button after scrolling the PANEL — the class-
  killer holds at the tail.
- **The substrate is actually removed:** SongEditor's clipped `<Details>` is
  DELETED (not bypassed) — verified in the diff. That's the right fix: a clipped
  human-unreachable copy is exactly what let this class recur (metadata, then
  files/delete).
- **Assertion freeze verified mechanically:** across all 24 touched specs, ZERO
  `expect()` lines changed — the additions are pure open/close-panel mechanics
  (the file UI moved into the panel, so specs that upload now open it first).
  The churn is large but correct and load-bearing (the old always-in-DOM clipped
  uploader was itself the Playwright-reachable/human-unreachable trap; removing
  it forces every spec onto the real path).

**GO TO LAND** (fast-forward, cite the verdict in the commit). This closes the
clipped-Details gap entirely. T37 (lyrics fetch + paste) is unblocked next.

## 2026-07-12 — T36 LANDED (`fa08727`): ✅ CLOSED — the clipped-Details gap is fully closed; VLL's blocker cleared

Patch-identical to the GO'd `1ca469c`, verdict cited in the commit. CI watched.
Song composition inside the fullscreen editor is whole again: add PDF/text
files, create charts, delete files, delete the song — all reachable in the
Details panel. The "Playwright-reachable/human-unreachable" class that produced
this bug three times (metadata, files, delete) is structurally dead — its
substrate (the clipped in-DOM copy) is removed and the `elementFromPoint` probe
guards the panel tail. **T37 (lyrics fetch + paste) unblocked.**

❓ **Web-Core → gate (2026-07-12): T37 presented — "New chart from lyrics" (honest URL fetch + paste fallback), SSRF-guarded; the live azlyrics fetch actually returned OK. Pixels + hand-run attached.**
Built to the re-spec (VLL's azlyrics override), `task/T37-lyrics-import` `58c55f8`,
off main. The no-evasion boundary held: a plain GET with a truthful User-Agent, no
anti-bot/Cloudflare-challenge machinery.

**Core** — `POST /api/bands/{bandId}/lyrics-import` (authed, band-scoped):
- **SSRF guard at DIAL time** (`safeDialContext`): http/https only, and every
  connection — including redirect hops — resolves the host and refuses loopback /
  private / link-local / unspecified / multicast. Defends DNS-rebinding + redirect
  SSRF + cloud-metadata (169.254.169.254). 5s timeout, 1 MB cap, ≤2 redirects.
  `isBlockedIP` is tabled exhaustively (v4+v6, the 172.16/12 edges, metadata IP).
- Host-dispatch parser: azlyrics `<!-- Usage of azlyrics.com -->` marker div; else a
  readability-ish `<p>` extract. Both → `normalizeLyrics`. `{status: ok|blocked|
  error}` — a 403/Cloudflare wall maps to **blocked**, never a 500.
- `normalizeLyrics` pure + tabled (CRLF, collapse-blanks, exact cruft blacklist,
  keep-when-in-doubt); mirrored minimally in TS for the paste path (studio has no
  unit runner — e2e covers the mirror; the Go table is authoritative).

**Studio** — "＋ New chart from lyrics" in the Details-panel Files section: name
(prefilled from the song title), a Fetch-from-URL accelerator, and a paste textarea.
Fetch **fills** the textarea (user reviews); block/error shows an honest message and
**leaves focus in the textarea** — never dead-ended. Create → normalized chart opens
in the T19 editor.

**Verification:** Go tables green (SSRF incl. non-http schemes + metadata IP;
parsers vs committed fixture HTML — azlyrics-shaped + generic + a Cloudflare-block
page; classify mapping; normalizer); `go vet` + `gofmt` clean. e2e **red-first
proven** (the button doesn't exist pre-fix): paste→normalized chart→saved; a stubbed
blocked fetch shows the fallback + keeps the box focused; an ok fetch fills the box.
`tsc` clean; **full e2e 88/88**. Pixels: the dialog in **light + dark, desktop +
390px** (I fixed a phone-width tall-input quirk — scoped `.lyrics-fetch-row` rule, no
other form touched).

**Hand-run (not in CI):** a LIVE azlyrics fetch through the endpoint returned
**`ok`** for two songs (Wonderwall ~1.4 KB, Bohemian Rhapsody ~1.9 KB extracted) —
so the honest GET works against azlyrics *today*; it's still best-effort and may
bounce to "blocked" whenever Cloudflare tightens, which is exactly why paste ships
alongside.

**Holding for GO.** On GO I land (cite the verdict), poll CI, then **relaunch the
demo** (`make demo`) — and I'll fold in **B10** (seed a lyrics text-chart into the
demo) since it's meant to ride this regen.

## 2026-07-12 — T37 GATE REVIEW (`58c55f8`): ✅ GO TO LAND — the SSRF guard is genuinely solid; honest-fetch boundary held

VLL's override implemented to spec. I read the security core line-by-line (not
from the table) and it's correct:

- **SSRF guard — the #1 criterion, verified:** `safeDialContext` resolves the
  host, rejects if ANY resolved IP is blocked, then dials the EXACT validated
  `ips[0]` (no re-resolve → no DNS-rebinding TOCTOU window); it's the transport's
  DialContext so it covers every redirect hop automatically; `isBlockedIP`
  refuses loopback/private/link-local(+multicast)/ULA/unspecified/multicast;
  http/https-only; 5s timeout, 1MB cap, ≤2 redirects. **`TestIsBlockedIP` is
  exhaustive** including the boundary cases (172.32 + 172.15 just OUTSIDE
  172.16/12 → allowed; the 169.254 metadata range → blocked; v6 ULA/link-local/
  multicast). `go vet` + `gofmt` clean; the Go tables (`IsBlockedIP`,
  `NormalizeLyrics`, `ClassifyFetch`) + parser fixtures pass.
- **Honest-fetch boundary HELD:** truthful UA (`troubacore/1.0 (+repo; lyrics
  import)`), plain GET, no evasion anywhere — a Cloudflare/403 wall maps to
  `status:"blocked"`, never a 500, never a retry-with-tricks.
- **Parsers off-network:** committed fixtures (azlyrics-shaped marker div,
  generic `<p>` extract, a Cloudflare-block page) — no live fetch in CI.
- **Studio + pixels:** red-first e2e (button absent pre-fix); paste→normalized
  chart→saved; a stubbed blocked fetch shows the honest fallback and keeps the
  paste box focused; the dialog pixel matches the spec ("＋ New chart from
  lyrics" beside New text chart, name prefilled, Fetch-from-URL accelerator over
  the paste area). My batch (lyrics + files-delete + text-chart) green; `tsc -b`
  clean.
- **Hand-run honesty:** the lane reports live azlyrics returned OK for two songs
  today (best-effort works now, may bounce later) — exactly the honest
  best-effort posture the spec required; the paste fallback covers the bounce.

**GO TO LAND** (fast-forward, cite VLL's override + this verdict). After it
lands: **B10** (seed a text-chart lyrics file) + the demo regen, one pass.

## 2026-07-12 — T37 LANDED (`5966b78`): ✅ CLOSED — lyrics import (fetch + paste) shipped, SSRF-guarded

Patch-identical to the GO'd `58c55f8`; cites both VLL's override and the verdict.
CI watched. VLL's must-have is delivered: honest best-effort azlyrics/URL fetch
into the T19 chart editor with a paste fallback that never dead-ends, behind a
line-by-line-verified SSRF guard. Next (architect-implemented per VLL): **B10 +
the demo regen**, one pass.

❓ **Web-Core → gate (2026-07-12): VLL follow-on — auto-label verses/choruses from the lyric stanzas on import. Requesting a spec/steer; it cuts against T37's "minimal normalizer" ruling, so I'm not bolting it on.**
VLL: "for azlyrics maybe also match paragraphs — lyrics are grouped so we can
detect verses and choruses." **What already works:** the stanza GROUPING survives
import — azlyrics' `<br><br>` → a blank line, and `normalizeLyrics` keeps single
blank lines, so imported lyrics arrive as blank-line-separated stanzas. **What's
new:** turning those groups into the T19 dialect's `## Verse N` / `## Chorus`
section labels so the chart is structured, not just spaced.

**The tension (why I'm asking, not doing):** T37's `normalizeLyrics` is
deliberately minimal — the spec says "do NOT touch section labels … when in doubt
KEEP." Auto-sectioning is the opposite: it INVENTS structure and will sometimes be
wrong (bridge, pre-chorus, intro, spoken tag). So it must NOT live in
`normalizeLyrics`; it's a separate, ideally opt-in step, and the user still edits
in the T19 editor after.

**Proposed design (my recommendation):** a pure `detectSections(text)` applied at
CREATE, **client-side only** (TS) — it runs on both paste and fetched text in one
place, so no Go/endpoint change (the endpoint keeps returning normalized-but-
unlabeled text). Algorithm: split on blank lines → stanzas; a stanza whose text
repeats 2+ times → `## Chorus` (all occurrences); the rest → `## Verse 1`, `## Verse
2`, … in order; keep the `# {title}` heading. e2e-covered via the paste path (no
unit runner in studio).

**Questions for you / VLL:**
1. **Automatic, or a dialog toggle** ("Label verses & choruses", default on/off)?
   Automatic is smoother; a toggle respects the keep-when-in-doubt posture for
   pages where the guess would be noise.
2. **Chorus by exact-repeat** OK, or start simpler — number every stanza `## Verse
   N` with no chorus guess (zero false "Chorus")? Repeat-detection is a good
   azlyrics fit (choruses repeat verbatim) but can mislabel a repeated verse.
3. Confirm **client-side TS, separate from the normalizer** is the right seam.

New task (T38?) or a T37 amendment — your call. I'll implement to your spec.

❓ **Web-Core → gate (2026-07-12): VLL — "can we edit the pseudo-md directly in preview mode?" Requesting a spec; it reverses a deliberate T25 decision, so flagging before I build.**
VLL: "is it possible to edit the pseudo md file in the preview mode directly?"
**Current state:** the T19/T25 chart editor is already a split pane
(`chart-editor-panes`) — the `chart-source` textarea beside the `chart-preview`
PDF `<object>` — but the preview only re-renders when you click **Preview**. That
on-demand refresh was a DELIBERATE T25 choice (the ChartEditor comment: "Preview
renders on demand — no per-keystroke round-trips"), because each preview is a
server render (chartpdf) + a blob swap.

**Interpretations (need VLL's intent):**
1. **Live preview** — the source stays editable (as now) but the PDF
   auto-re-renders as you type (debounced ~400–600ms). "Editing in preview mode"
   = the preview just keeps up. **My recommendation** — it's the natural reading
   and low-risk, but it REVERSES T25's on-demand decision, so it needs your OK
   (debounced + coalesced keeps the render rate sane; still N renders per edit
   session vs 1 today).
2. **WYSIWYG on the rendered PDF** — click text in the preview and edit it there.
   I'd **advise against**: the preview is a rasterized/served PDF with no
   source-position mapping, so this is a large build (round-tripping PDF regions →
   dialect source) for little gain over a live split view. Flagging as likely
   out-of-scope unless you specifically want it.
3. **Just make the split editor nicer** — bigger/toggleable preview, a "render"
   keyboard shortcut, source stays primary. Cheapest; no T25 reversal.

**Questions:** (1) which of the three is the intent? (2) if live (1), is the extra
render load acceptable (debounced, only while the editor is open, same
no-persist endpoint)? (3) new task (T39?) or fold into a T25 follow-up?
I'll implement to your spec — not building until you rule, since (1) undoes a
prior decision and (2) is a big lift.

## 2026-07-12 — GATE ANSWERS: two VLL follow-ons specced (T38 auto-sections, T39 live preview) — intents confirmed with VLL

Both asks flagged prior decisions honestly (T37's minimal normalizer; T25's
on-demand preview) — right call to hold. Confirmed VLL's intent on both:

- **T38 — auto-label verses/choruses on import:** VLL picked **toggle, default
  ON**. Ruled: a pure client-side `detectSections` (TS), SEPARATE from
  `normalizeLyrics` (its keep-when-in-doubt contract stands untouched) — runs on
  both paste + fetched text in the dialog, no Go/endpoint change. Chorus by
  **exact verbatim repeat** (VLL's pick, the right azlyrics fit), verses numbered
  in order, already-labeled input returned untouched (idempotent). Lane's seam
  recommendation (client-side, separate) ratified.
- **T39 — edit in preview:** VLL picked **live debounced preview** (interp 1).
  Ruled: ~500ms debounce + in-flight coalescing (always render the LATEST source,
  never swap a stale blob) + no error-retry loop; manual Preview button stays.
  **WYSIWYG-on-the-PDF (interp 2) explicitly DECLINED** — rasterized served PDF
  has no source mapping, disproportionate. The T25 on-demand reversal is
  sanctioned by VLL; the debounce/coalesce keeps render load sane, editor-open
  only, same no-persist endpoint.

Both are S, studio-only, independent — either lane order. Sequencing note: T38
needs T37 (landed); T39 is standalone.

❓ **Web-Core → gate (2026-07-12): reconciliation — VLL's intent (asked directly) DIVERGED from the GATE ANSWERS above on BOTH. T38 default flips OFF (implementing); T39 is a different feature (rich editor, not live preview) — requesting a re-spec.**
Heads-up so we're not building against a stale confirmation. While you were away I
asked VLL the two design calls directly; his answers came back **different from
what the GATE ANSWERS record**, so I re-asked him to reconcile side-by-side. His
settled calls:

- **T38 (auto-sections): BUILD IT, but the toggle DEFAULT is OFF** (you specced
  default ON). VLL first said "no automatic," then reconciled to "build it but
  default off" — i.e. grouping-only by default, opt-in labeling. Everything else
  in your T38 spec stands (client-side `detectSections`, separate from the minimal
  normalizer, chorus-by-verbatim-repeat, numbered verses, idempotent). **I'm
  implementing T38 with default OFF now** — flag if the OFF default is a problem.
- **T39: NOT live preview.** VLL's actual intent is a **rich pseudo-md SOURCE
  EDITOR** (syntax highlighting for the dialect — `#`/`##` headings, chord lines,
  section labels; source stays primary; preview stays **on-demand**). So your T39
  (live debounced auto-render, interp 1) is **superseded** — and T25's on-demand
  decision **stands after all** (no reversal needed). This is a genuinely different
  build, so I'm **holding T39 for your re-spec.** The open question is the editor:
  a real code-editor lib (CodeMirror ≈ a new studio dependency + bundle weight) vs
  a lightweight custom highlighter over a textarea (no dep, less capable). Your
  call on the dependency; I'll build to it.

Not editing your GATE ANSWERS text — this is the reconciliation on top. T38 lands
the usual way; T39 waits for your re-spec.

## 2026-07-12 — RECONCILIATION ACCEPTED: VLL's direct calls supersede my GATE ANSWERS on T38 + T39; specs updated

The lane correctly caught that my AskUserQuestion confirmations didn't match
VLL's settled intent (he answered the lane directly and reconciled side-by-side)
— exactly the right move: build against the human owner's latest word, not a
stale tool answer. **VLL's direct calls win; both specs updated to match:**

- **T38: default OFF** (was ON in my spec). Grouping-only is the default; labeling
  is opt-in. Everything else in the spec stands; the lane is implementing.
- **T39: re-specced entirely.** NOT live preview — VLL wants a **rich pseudo-md
  SOURCE editor** (dialect syntax highlighting; source primary; **T25's on-demand
  preview STANDS**, no reversal). Answered the lane's dependency question:
  **NO CodeMirror/editor lib** — a ~4-token decoration-only dialect doesn't
  justify the dependency + bundle weight against studio's minimal-dep posture.
  Ruled: a custom highlighter via the **overlay technique** (transparent-text
  textarea over a highlighted `<pre>`), pane made **monospace** (correct for
  chords-over-words AND makes overlay alignment reliable — the technique's one
  failure mode gone at fixed advance width). Gotchas written as hard requirements;
  honest fallback (flag before adding a dep if alignment can't hold).

Process note (mine to own): I used AskUserQuestion and got answers that turned out
not to reflect VLL's settled intent — possibly a mid-thought reading. No harm (the
lane held and reconciled before building the wrong thing), but the lesson is that
a design confirmation is only as good as its currency; the lane's
reconcile-before-build instinct is the safety net working.

❓ **Web-Core → gate (2026-07-12): VLL naming input for T39 — the "chart editor" label reads as misleading for what feels like a rich-text / lyrics editor.**
VLL: *"chart editor is misleading for a rich-text, most-likely lyrics editor?"* The
T19 editor (getting the T39 highlighter) edits the whole chart dialect — `#` title,
`##` sections, chord lines, lyrics — so "lyrics editor" is a bit narrow (chords +
structure are the point of a *chart*, not just lyrics). But VLL's read is fair:
"chart editor" is opaque for what looks like text editing. Requesting a
naming/framing call as part of the T39 build — options: (a) keep "chart" but label
the pane clearly (e.g. "Chart source" / a one-line "title · sections · chords ·
lyrics" hint); (b) lean into "song text"/"lyrics & chords" framing. Your call; it's
cheap (a visible label — **testids stay frozen**), and I'll apply it when I build
T39. Sequencing: I'm finishing **T38** now, then T39.

## 2026-07-12 — T39 naming: RULED "Lyrics & chords" (VLL flagged "chart editor" as misleading)

VLL's right that "chart editor" is opaque; the lane's right that "lyrics editor"
undersells it (chords + sections are the point). Ruled: user-facing label
**"Lyrics & chords"** — the plain-language name for exactly the dialect's content
(title · sections · chords · lyrics), with a one-line hint. LABEL-only (testids
frozen; `chart`/`text-chart` stays the internal/API/file-type term). Folded into
the T39 spec so it lands with the highlighter build. Cheap, no data model touch.

❓ **Web-Core → gate (2026-07-12): T38 presented — opt-in verse/chorus section labeling (toggle DEFAULT OFF, per VLL's reconciled call). Red-first, 90/90, pixel attached.**
Built to spec + VLL's default-OFF (`task/T38-auto-sections`, `2d49b77`, off main).

- **`detectSections`** (pure, in `lyrics.ts`, SEPARATE from `normalizeLyrics` — its
  keep-when-in-doubt contract untouched): split on blank-line stanzas; a stanza that
  repeats **verbatim** → `## Chorus`, others → numbered `## Verse N`; **idempotent**
  (already-`##`-labeled input or a single stanza returned as-is). Client-side only,
  runs on paste + fetched text, no Go/endpoint change — as you ratified.
- **Dialog:** a "Label verses & choruses" checkbox, **DEFAULT OFF**; on → create runs
  detectSections over the normalized body before opening the T19 editor for review.
- **Guard** (`editor-lyrics-sections.spec.ts`, red-first — toggle absent pre-fix):
  default OFF → grouped, no `##`; ON → `## Verse 1/2` + the repeated stanza `## Chorus`
  (and NOT a third numbered verse). `tsc` clean; **full e2e 90/90**; dist untouched.

**One implementation note worth flagging:** the toggle first used a nested
`<label><input><span>` and Playwright's post-check click silently didn't fire the
create handler (traced it to blank-render then isolated it to the checkbox
interaction). Switched to the **id/htmlFor** pattern (input + sibling label) — clean
fix, and arguably more correct markup anyway. Human behavior was never affected;
just flagging the gotcha.

**Holding for GO.** On GO I land (cite verdict), poll CI, then relaunch the demo with
T38 (and I'll use VLL's azlyrics Wonderwall URL for a real fetch-then-label sanity
check). **T39** (the "Lyrics & chords" highlighter, per your ruling) is next.

## 2026-07-12 — T38 GATE REVIEW (`2d49b77`): ✅ GO TO LAND — default OFF, normalizer untouched, red re-proven

Verified (my runs, isolated stack):
- **Red re-proven independently:** the guard fails on pre-fix main (2/2 — the
  toggle doesn't exist); passes at the fix.
- **Green + pixels:** guard + lyrics-import + text-chart green; `tsc -b` clean;
  the dialog shows the "Label verses & choruses" checkbox **default unchecked**
  (asserted `not.toBeChecked()`).
- **`normalizeLyrics` UNTOUCHED (the load-bearing separation):** `lyrics.ts` only
  ADDS `detectSections`; SongDetails only imports it + wires the toggle. No Go,
  no endpoint change — exactly the client-side seam.
- **`detectSections` correct on read:** idempotent (already-`##`-labeled / single
  stanza / empty → returned untouched — the `/^##\s/m` guard); chorus by verbatim
  repeat (normalized key, count ≥ 2 → `## Chorus` at every occurrence); others
  numbered `## Verse N` (choruses don't consume a number). **The `# title` concern
  from my spec is moot by construction** — `detectSections` runs on the title-less
  normalized BODY and `# heading` is prepended AFTER, so the title can never be
  mislabeled (cleaner than I specced).
- The checkbox uses id/htmlFor (not a nested label) — the lane noted a nested
  `<label><input>` broke the click under Playwright; good catch, correct fix.

**GO TO LAND** (fast-forward, cite the verdict). T39 (the rich "Lyrics & chords"
editor) is next.

## 2026-07-12 — T38 LANDED (`2c213d6`): ✅ CLOSED — opt-in verse/chorus labeling shipped (default OFF)

Patch-identical to the GO'd `2d49b77`, verdict cited. CI watched. Imported lyrics
can now be auto-structured into `## Verse N`/`## Chorus` on opt-in, without
touching the minimal normalizer. T39 (rich "Lyrics & chords" editor) is the last
of the T37→T39 chart-editing arc still open.

## 2026-07-12 — go-job RED on `2c213d6` (T38 landing) → B10 gofmt fix-forward `502a569`: ✅ CLOSED — my miss, owned

Not T38 (studio-only) — the red was the **gofmt gate** flagging `cmd/seed/main.go`
from **B10 (`c9af912`, mine)**. My B10 edit added a multi-line `textChartPath`
field to a songDef literal, realigning `src:`; I ran `gofmt -l <file> && echo
CLEAN` at land time — but **`gofmt -l` exits 0 even when it lists a dirty file**,
so the echo fired and I landed dirty. CI's gofmt gate (runs after vet/test, over
the whole tree) caught it on the next go job (T38's). Same class as the T26 red,
plus a misread-the-tool wrinkle. Fixed forward (`gofmt -w`, build clean); memory
updated (never chain `gofmt -l` with `&& echo`; a listed filename is the failure
regardless of exit code). All Go TESTS were green throughout — formatting only.

## 2026-07-12 — gofmt double-fix (harmless): `502a569` (mine, landed) == `9e8bcfe` (lane, on branch)

The lane independently produced the same one-line `cmd/seed/main.go` gofmt fix on
`task/T38-auto-sections` (`9e8bcfe`) while my fix-forward (`502a569`) was already
landing. Both roles caught the B10 misalignment — good. Mine is on main and
CI-green; theirs is now an empty diff vs main and prunable. No action; noting so
the double-commit isn't mistaken for two separate issues.

❓ **Web-Core → gate (2026-07-12): T39 presented — "Lyrics & chords" syntax-highlighted source editor (overlay technique, no dep). Red-first, 92/92, pixels both themes.**
Built to the re-spec (`task/T39-chart-highlight`, `78b15fe`, off the gofmt-fixed main).

- **`chartHighlight.ts` — pure `tokenizeChartLine`, rules MIRROR the renderer**
  (`chartpdf/chart.go`): `# title` / `## section`, a line whose tokens are ALL chords
  (the **same chord regex** as chart.go) → chord, `**bold**` inline on plain lines,
  else plain. **Token text is char-for-char preserved** (bold keeps its `**`) — the
  invariant the overlay needs to stay glyph-aligned.
- **Overlay technique, NO editor lib** (per your dependency ruling): a colored
  `aria-hidden` `<pre>` behind a transparent-text `<textarea>` (caret+edit from the
  textarea, color from the pre); the WRAP owns the border so both boxes share one
  model; the pre mirrors the textarea scroll; pane is **MONOSPACE** (chords-over-words
  + reliable alignment — the technique's one failure mode gone at fixed advance
  width). `package.json` unchanged (no dep added).
- **Naming (your ruling): "Lyrics & chords"** — editor heading + a one-line dialect
  hint. **Label-only**: testids frozen, `chart`/`text-chart` stays the internal term.
- **Guard** (`editor-chart-highlight.spec.ts`, red-first — no `.chart-src-hl` pre-fix):
  token classes correct (title/section/chord-line/bold/plain), editing round-trips
  through the textarea, and **preview stays ON-DEMAND — typing does NOT auto-render**
  (the anti-regression of the reverted live-preview idea; T25 stands). `tsc` clean;
  **full e2e 92/92**; **pixels both themes** (indigo headings, green chord lines
  aligned over the words, `**bold**` kept, plain lyrics — alignment holds).

**Alignment fallback honored:** monospace + overlay holds cleanly in the pixels; I did
NOT need to reach for CodeMirror (would've stopped + flagged first if it drifted).

**Holding for GO.** On GO I land (cite verdict), poll CI, then relaunch the demo with
T39. This closes VLL's editor arc (T36 files/delete → T37 lyrics import → T38 opt-in
sections → T39 highlighted "Lyrics & chords").

## 2026-07-12 — T39 GATE REVIEW (`78b15fe`): ✅ GO TO LAND — all three rulings honored; the chart-editing arc is done

Verified (my runs, isolated stack) — every ruling met:
- **NO new dependency** (the ruling): the diff is 4 files (spec, SongDetails,
  `chartHighlight.ts`, styles.css) — **no `package.json` change**. The highlighter
  is the custom overlay (transparent-text `<textarea>` over an aria-hidden colored
  `<pre>`, monospace, scroll-mirrored). Custom over CodeMirror, exactly as ruled.
- **Preview stays ON-DEMAND** (the anti-regression): typed a full dialect sample →
  **zero auto-render** (my probe: no preview object after typing); the Preview
  button path is untouched. T25's decision held.
- **"Lyrics & chords" relabel** (the naming ruling): the editor heading reads
  "Lyrics & chords" + the one-line dialect hint; `chart-source` and the file-type
  term stayed frozen (label-only).
- **Token correctness by PIXELS, both themes:** `#`/`##` headings in the section
  color, the `G   C` line green (chord class), lyrics plain, `**bold**` bold with
  its markers preserved char-for-char (so the overlay stays glyph-aligned). The
  TS `CHORD` regex is **character-identical to chart.go's `chordToken`** — the
  highlight matches what the PDF renders (I8-spirit parity for the editor).
- **Red re-proven**, editing round-trips through the textarea, `tsc -b` clean.

**GO TO LAND** (fast-forward, cite the verdict). This closes the T37→T39
chart-editing arc: fetch/paste lyrics import (SSRF-guarded) → opt-in
verse/chorus labeling → a highlighted "Lyrics & chords" source editor.

## 2026-07-12 — T39 LANDED (`febb592`): ✅ CLOSED — the T37→T39 chart-editing arc is COMPLETE

Patch-identical to the GO'd `78b15fe`, verdict cited. CI watched. The
"Lyrics & chords" editor (dialect syntax highlighting via a dependency-free
monospace overlay; preview stays on-demand) ships. **Arc complete:** T37
(lyrics import — SSRF-guarded fetch + paste) → T38 (opt-in verse/chorus
labeling) → T39 (highlighted source editor). Chart authoring in Studio is now a
first-class experience.

## 2026-07-12 — P201 STARTED (VLL: "go ahead with P201"), stage 1a landed (`eaa393f`, architect-implemented)

VLL directed the architect to build P201 (rehearsal live mode, L/staged). Executing
per the spec's mandate (core → studio → app, never big-bang); I further split stage 1
into 1a (state) + 1b (autobake) so the concurrency-sensitive piece reviews on its own.

**Stage 1a (landed, evidence attached):** `Setlist.LiveUntil` (persisted),
`LiveModeWindow=3h`, `app.SetlistLive` computing liveness at READ time (self-expiring,
no sweeper); `SetSetlistLive` (admin-only, clock-driven) + `SetlistLiveNow`; a
`WithClock` test hook; `POST …/setlists/{id}/live`. Inert until 1b (like B03's AUTO
enum). Tests: toggle+persistence, boundary+past-deadline expiry (clock-injected),
admin-only, pure predicate. Full core suite + gofmt + vet clean; CI watched.

**Next — stage 1b (the meaty part):** a debounced autobaker observing annotation
commits (the `sync/apply.go` realtime path + the `annotations.go` import path both
funnel through `eng.Apply`) → for live setlists containing the committed song,
coalesce ~5–10s then bake via the existing Baker (B08/B09 make concurrent bakes safe).
Clock-injected. This is the concurrency-sensitive stage — it gets extra scrutiny + a
debounce/expiry test (N commits → 1 bake). Then stage 2 (studio banner) + stage 3
(app transient toggle + R10).

## 2026-07-12 — P201 stages 1b + 2 landed (architect-implemented): rehearsal live mode is real in web+core

Continuing the P201 build per VLL. Since stage 1a:
- **1b (`95db8e8`, CI green):** the debounced AutoBaker — annotation commits on a live
  setlist's songs coalesce (~8s quiet) into ONE autobake via the existing Baker, as the
  enabling admin; policy/clock separated for flake-free tests (burst→1 bake, expiry,
  non-live never bakes; -race clean). Fixed a goroutine-leak I introduced (Router now
  takes a ctx so the ticker stops on shutdown/test-cleanup).
- **2a (`3952840`):** setlist-page admin toggle ("Go live (rehearsal)") + a red LIVE
  banner + pulsing chip; POST …/live; e2e toggle+persist; pixels both themes.
- **2b (`b49694e`):** in-editor LIVE banner — a new membership-gated GET
  …/live-setlists signal, the Viewer polls it (load + 20s) and shows a fixed red strip
  ('publishing to performers'); the top pill shifts below it (no overlap, pixel-checked);
  e2e + editor regression batch green.

**STAGE 2 COMPLETE (web + core).** Rehearsal live mode works end-to-end on the web
side: an admin flips a setlist live, the banner shows in the setlist page AND the
editor, and edits auto-bake. **Stage 3 remains** — the app's TRANSIENT auto-update
toggle (I13: in-memory, resets on leaving Stage) + the R10 viewport-preserving swaps
(facing pages/scroll mode). That's mobile-lane Kotlin work + an attended device pass;
not architect-implementable solo. I'll spec the stage-3 handoff for the mobile lane
next unless VLL redirects.

❓ **Web-Core → gate (2026-07-13): VLL — annotations seem shared across a member's files on a song; wants each file its own annotation. I investigated: per-file scoping IS in the model on current main (couldn't repro "shared"). Requesting your design ruling — bug repro, or a UX gap to close?**
VLL: *"it seems the annotations are the same for all the files of a single band
member on a song — not so nice; each one its own annotation?"*

**What I found (grounded, current main):** annotations ARE per-file already, via
`Layer.FileID` (`domain.go:159`):
- The viewer shows ONE file at a time (file tabs → `selectedFileId`,
  `Viewer.tsx:1087`); layer visibility + the editable/active/focused sets all filter
  `l.fileId === selectedFileId` (`:292/:320`).
- `createPersonalLayer` binds a new "My notes" layer to `fileId: selectedFileId`
  (`:399`) — a member drawing on file A gets a layer scoped to A; switching to file B
  shows B's layers (or none → "New layer" offered).
- **Demo data confirms it:** every seeded layer is bound to exactly ONE fileId —
  Wonderwall's 3 layers all on `Score` (nothing on Vocals/Guitar/Bass/Lyrics); The
  Open Road's 3 on `Lead sheet` (nothing on the text chart). So switching tabs shows
  DIFFERENT (or empty) annotations per file, not the same.
- In fact "same on ALL files" is **not reachable** in the current filter: an
  empty-`fileId` layer matches only when `selectedFileId == null`, i.e. shows on
  NONE once a tab is picked, never all.

**So I can't reproduce VLL's symptom on current main** — which means it's one of:
(a) an **older build** before per-file scoping; (b) a **specific repro** I'm missing
(e.g. a member with a layer whose fileId points at a since-deleted/re-uploaded file,
or the my-files reorder path, or a mobile-app/bake view that composites differently
than studio); or (c) a **UX-clarity gap** — per-file works, but the flow (per-file
"New layer", empty canvas on tab-switch, no visual "these notes belong to <file>"
cue) makes it FEEL shared/confusing.

**Your ruling, please** (you own the layer/annotation architecture):
1. Is per-file-per-member layers the confirmed intended design (I read it as yes)?
2. Is there a known gap where annotations bleed across a member's files, or should I
   file a **repro task** and get VLL's exact steps first (studio? mobile? which
   song/files)?
3. If it's (c) UX-clarity, want a small task — e.g. a "notes for: <filename>" label on
   the active-layer/annotations panel + a clearer per-file empty state — or leave it?

I've NOT changed anything — this is diagnosis + a routed design question. Happy to
file the task once you point at bug-fix vs UX vs already-satisfied. (Also asking VLL
for the exact repro so we're not guessing.)

## 2026-07-13 — RULING: per-file annotations (VLL "annotations same for all a member's files")

Thorough investigation by the lane — and the read is right. **My ruling:**

**1. Per-file-per-member layers ARE the intended + implemented design.** A layer
binds to `Layer.FileID`; the viewer filters everything by `selectedFileId`;
`createPersonalLayer` scopes a new personal layer to the open file. Switching file
tabs shows THAT file's layers (or none). This is correct and I'm not changing it —
a member's Score notes should NOT bleed onto the Vocals part. Confirmed.

**2. The symptom isn't reproducible on current main, so it's one of two things — get
the repro before building.** Given this box's history, **rule out a stale build FIRST**:
have VLL click the version chip / hit `/api/version` — per-file scoping post-dates
several of the builds his box has served, and "annotations shared across files" is
exactly what a PRE-scoping build would show. If the version is current, get the exact
steps: studio or the app? which song + which two files? Is it the SAME layer visibly
on two tabs (a real bug — I'd want the layer's `fileId` + whether the file was
re-uploaded/deleted), or two files that are simply both empty and so LOOK identical?
Don't file an implementation task on an unreproducible "feels shared" — that's how we'd
chase a ghost.

**3. Most likely landing (pre-authorized IF #2 comes back "current build, works, but
confusing"): a small UX-clarity task, not an architecture change.** Per-file is right
but SILENT — an empty canvas on tab-switch and a generic "New layer" give no cue that
notes are per-file. A "Notes for: <filename>" label on the active-layer/annotations
surface + a clearer per-file empty state ("No notes on <file> yet") would make the
existing correct behavior legible. File it as T40 (studio, XS) only after VLL confirms
it's clarity, not staleness or a real cross-file bleed. Routing back to VLL for the
version check + exact repro.

**Web-Core follow-up (2026-07-13): the seed finding you didn't have — it reframes this as a DEMO gap, not a product bug.** VLL then guessed *"maybe in the demo the same
annotation is on all files."* I checked the seed: it's the OPPOSITE but equally telling
— `cmd/seed` attaches **every** annotation layer to `firstPDFFileID(...)` (`main.go:496`,
helper `:547`), i.e. the song's FIRST PDF only. So Wonderwall's cues/markings/chords all
sit on **Score**; Vocals/Guitar/Bass/Lyrics get **zero** annotations. Not duplicated —
concentrated on file[0], parts bare. **So the demo never SHOWCASES per-file annotations
at all** — every song looks annotated-on-one-file, which is almost certainly what read to
VLL as "the annotation situation isn't smart." Likely not staleness or a bug — it's the
demo under-selling a feature that already works.

**Proposed: a B-track seed improvement (demo design → your gate) to actually demo per-file.**
Give a couple of demo songs DIFFERENT annotations on DIFFERENT parts — e.g. chords on the
Guitar part, bowing/breath marks on the Vocals part, section form on the Score — so
switching file tabs visibly shows each part carrying its OWN notes (the feature, on
display). Small (`cmd/seed` + a demo regen), showcases per-file, and answers VLL's "not
smart" directly. This likely supersedes the need for T40's UX label in the demo context,
though the label still helps real bands. **Want this as a task (B11?), and does VLL still
want the T40 clarity label for the product** — or is the demo fix enough? Holding on both
(and VLL's version check still worth doing to fully rule out a stale build on his box).

## 2026-07-13 — RULING (updated): the annotation report is a DEMO gap — B11 filed; T40 optional

The lane's seed finding resolves it: per-file scoping WORKS, but `cmd/seed` attaches
every layer to `firstPDFFileID` — so the demo concentrates all marks on file[0] and
NEVER shows per-file. That's what read to VLL as "not smart." **Not a bug, not
(necessarily) staleness — the demo under-sells a working feature.**

- **B11 filed (approved):** spread demo annotations across PARTS (form on Score,
  breath marks on Vocals, chords on Guitar, …) so switching tabs shows each part's
  own notes. Small (`cmd/seed` + demo regen); it directly answers VLL and puts the
  feature on display. Supersedes T40 in the DEMO context.
- **T40 (the product "Notes for: <file>" clarity label): OPTIONAL, VLL's call.** Even
  with distinct per-part notes, a first-time user gets no explicit cue that notes are
  per-file; the label helps real bands. But it's polish, not needed to answer the
  report — hold unless VLL wants it.
- **Version check still worth doing** to fully rule out a stale build, but B11 is the
  likely real answer. Routing B11 + the T40 question to VLL.

❗ **Web-Core → gate (2026-07-13): CORRECTION — I was WRONG. It IS a real, reproducible cross-file annotation bleed, and it's MY lane (studio overlay render). VLL clarified: a regular browser on Android (studio), NOT the app.**
My earlier "per-file works, can't repro" checked the wrong layer. The layer PANELS
(`editableLayers`/`fileLayers`) DO filter by `selectedFileId` — but the actual
**dry-overlay RENDER does not.** `usePdfDocument.ts:222-226`:
```
const objs = objects.filter(o => o.page === page)          // page INDEX only
  .filter(o => { const l = layersById.get(o.layerId); return l && visible[l.id]; }); // visibility only
```
No `fileId` filter, and `usePdfDocument` isn't even passed `selectedFileId`. So a layer
bound to *Score* paints its objects onto *Vocals* / *Guitar* (any file sharing that page
index). VLL's exact symptom: "select Vocals/Guitar/Score → same annotations, different
PDF." **This IS the repro you asked for** — the same layers (all on Score's fileId) drawn
on every part. Studio's file tabs switch the PDF (raster respects selectedFileId) but the
overlay doesn't. My "can't repro" was checking the panel filter, not the canvas — my
mistake, correcting the record. (So it's NOT a stale build, NOT B11/demo-only, NOT the
app — though B11 still makes the demo showcase the feature once this is fixed.)

**Fix (my lane, clear + central):** filter `doc.objects` to the selected file's layers
ONCE in Viewer — `objectsForFile = objects.filter(o => layersById.get(o.layerId)?.fileId
=== selectedFileId)` (selectedFileId==null → all) — and pass THAT to both `usePdfDocument`
(the dry overlay, line 279) and `EditCanvas` (line 929, hit-test/wet). Red-first e2e:
2-file song, annotate file A, switch to B → B's overlay must NOT show A's objects (fails
today). This also fixes the page-index collision (Score p2 bleeding onto any file with a
p2). **Implementing now as the real T40** (was pencilled as a UX label; the label is
secondary — the bug is the render). Will present with red-first + full suite.

**Second bug (also studio-on-Android, also mine): the 2nd page often renders fully BLACK.**
VLL: same session, regular Android browser. The baked-bundle raster for p2 is fine (I
verified the pixels), so it's the **live studio PDF.js canvas render on mobile** — almost
certainly a **mobile canvas-area cap** (Android Chrome limits canvas to ~16MP / device
memory; a full-res page canvas can exceed it → the browser returns a blank/black surface).
"Often the second page" fits a cumulative-canvas-memory ceiling (page 1 fits, page 2 tips
over). Needs on-device confirmation (hard to repro headless). **Filing as T41** (studio,
mobile): clamp per-page canvas pixel area to a safe mobile budget (cap devicePixelRatio /
raster dimensions on small viewports), with a repro on a real Android viewport. Flagging
before building since the exact cap is device-dependent — will confirm the mechanism first.

**Ask:** any objection to me taking T40 (the fileId render fix — clear correctness bug)
now, and T41 (mobile black-page) after? Otherwise proceeding on T40.

## 2026-07-13 — CORRECTION ACCEPTED: cross-file annotation bleed IS a real render bug (T40); my earlier ruling was wrong

The lane caught it and I VERIFIED it: `usePdfDocument`'s dry-overlay render filters
objects by `o.page === page` (page INDEX) + layer visibility only — **no `fileId`
filter, and `selectedFileId` isn't even passed in** (`usePdfDocument.ts:222-226`). So a
layer bound to Score paints its objects onto Vocals/Guitar/any file sharing that page
index. That IS VLL's "select a different part → same annotations, different PDF." **My
2026-07-13 ruling ("per-file works, it's a demo gap, check for staleness") was WRONG** —
I trusted the panel-filter (which DOES scope by fileId) and didn't check the canvas
render path. The lane owning + reproducing it is the gate working; correcting the record.

- **T40 = the render fix (APPROVED, lane implementing):** filter `doc.objects` to the
  selected file's layers ONCE in Viewer and pass THAT to BOTH `usePdfDocument` (dry
  overlay) AND `EditCanvas` (hit-test/wet) — the fix must cover both or a click still
  hits a bleed object. `selectedFileId==null` → all. Review bar: **red-first e2e**
  (2-file song, annotate A, switch to B → B's overlay must NOT show A's objects — fails
  today); assert BOTH render (pixel/object count on the overlay) AND that a click on B
  doesn't select A's object. This also kills the page-index collision (Score p2 → any
  file's p2). The earlier "UX label" idea is secondary — the bug is the render.
- **T41 = mobile 2nd-page-black (APPROVED to investigate):** likely a mobile canvas-area
  cap (Android Chrome ~16MP / memory ceiling; page 2 tips a cumulative budget). Correct
  to confirm the mechanism on a real Android viewport BEFORE the clamp fix — device-
  dependent cap. Not blocking; file the repro first.
- **B11 still stands** (demo per-file showcase) but is now clearly SECONDARY — it makes
  the demo DISPLAY the feature once T40 makes it actually render per-file. Not the bug.

Proceed on T40 now, T41 after. VLL: this was studio-in-a-browser on Android, not the app.

## 2026-07-13 — T40 LANDED (`c7050d2`, VLL "land it"): cross-file annotation bleed fixed — post-hoc review requested

VLL directed "land it and relaunch the demo," so this landed on his approval (cited
in the commit) rather than holding for a verdict — the field bug he reported +
reproduced red-first. **Post-hoc review invited.**

Fix (central, Viewer): an `objectsForFile` memo filters `doc.objects` to the selected
file's layers (`layersById.get(o.layerId)?.fileId === selectedFileId`; null → all),
passed to the dry overlay (usePdfDocument), both EditCanvas paths, and the
AnnotationList — one filter so canvas/interaction/list agree with the already
file-scoped panels. Root cause was `usePdfDocument:222` filtering objects by
page-index + visibility but never fileId. Also kills the page-index collision
(Score p2 → any file's p2).

Red-first PROVEN: `editor-annotation-fileid.spec.ts` — 2-file song, filled highlight
on file A, switch to B → B empty where A's rect was (**pre-fix alpha 255 → post 0**),
A retains it on switch-back. `tsc -b` clean; **full e2e 95/95**; dist untouched. CI
polling. Relaunching the demo per VLL next. **T41** (mobile black 2nd page — canvas
cap) still queued after.

## 2026-07-13 — T40 POST-HOC REVIEW (`c7050d2`, landed on VLL "land it"): ✅ APPROVED — the bleed fix is correct and covers every path

VLL asked for the review after landing. Verified:
- **Red re-proven independently:** the guard fails on pre-fix (`c7050d2~1`) — the
  cross-file bleed is real (file A's filled highlight shows on file B).
- **Green at the fix + BOTH paths (my ruling bar):** the `objectsForFile` memo
  (`layersById.get(o.layerId)?.fileId === selectedFileId`, null→all) is passed to
  `usePdfDocument` (dry overlay), BOTH `EditCanvas` props (wet + hit-test), AND the
  `AnnotationList` — so render, interaction, and the list all agree with the
  already-file-scoped panels. My 18-spec batch (guard + editor + ed5 + **pick**, the
  hit-test path + files-delete) green; `tsc -b` clean. The one-filter-in-Viewer
  placement is the right central fix; it also closes the page-index collision.
- Citation present (VLL "land it and relaunch the demo" + presented for post-hoc).

**GOOD post-hoc.** This is the actual fix for VLL's "different part, same annotations"
— studio in a browser. **T41** (mobile 2nd-page-black) is next; **B11** (demo
per-file showcase) now has a working feature to display. My earlier
misdiagnosis is on the record; the lane's reproduction + this fix close it right.

## 2026-07-13 — T40 Layers-drawer follow-up (`edb5cac`): ✅ APPROVED — the last un-scoped surface, now filtered

VLL caught the gap right after T40: the CANVAS/EditCanvas/AnnotationList were
file-scoped, but the Layers DRAWER still listed the whole song's layers (Score's
cues showing while viewing Vocals). Fix: `sortedFileLayers` (sortedLayers filtered
to `selectedFileId`) feeds the LayersPanel; `layerRank` correctly stays over ALL
layers (z-order only — only the current file's objects render, so stacking is
preserved). Verified: **red re-proven** (the drawer assertion fails on
`edb5cac~1` — pre-fix it lists file A's layer after switching to B); green at the
fix (guard + editor-layers, 10 passed); `tsc -b` clean. Per-file scoping is now
complete across every surface — render, hit-test, annotation list, AND the layers
drawer.

## 2026-07-13 — B11 LANDED (`c76a0cf`, VLL "prove it"): Wonderwall parts now carry distinct annotations — post-hoc pixel review

VLL, after T40: "in Wonderwall only Score has layers — to prove it we need different
annotations in different files." Your B11 proposal, VLL-requested; landed on his ask
(cited). Demo-only (`cmd/seed`), so it rides the running demo, not the bundle.

The seed attached every layer to firstPDFFileID (Score), so the demo never DISPLAYED
per-file scoping. Now Wonderwall's parts each carry their own layer (Score keeps the
section-form set): **Part-Vocals** = marie's personal "Breath & phrasing" (green —
breath ticks, phrase highlight, soft-dynamic cue); **Part-Guitar** = shared "Chords &
capo" (blue — Capo 2 + Em7/G/Dsus4/A7sus4 over the staves). Two builders in
annotations.go (file-distinct keys so ids don't collide with the Score's);
`pdfFileIDByFilename` resolves the parts; imports are additive + idempotent.

**Pixel-verified (my screenshots vs the live demo, all three tabs):** Vocals shows
only the green breath/phrase ink over the *Vocals* PDF; Guitar shows only the blue
chords + Capo over the *Guitar* PDF; Score unchanged. Switching tabs = different ink,
different PDF, no bleed — T40 on visible display. (Fixed a capo/subtitle overlap after
the first render.) API confirms 3/1/1 layers split across Score/Vocals/Guitar. gofmt
(root) + vet + `go test ./cmd/seed` clean; dist untouched. CI polling. **Post-hoc pixel
review invited.** T41 (mobile black 2nd page) still queued.

## 2026-07-13 — B11 LANDED (`c76a0cf`): ✅ APPROVED — the demo now SHOWCASES per-file annotations

Verified live (seed on an isolated core): Wonderwall's files now each carry DISTINCT
annotation layers — **Score**: Conductor cues + Section markings + Chords; **Part -
Vocals**: "Breath & phrasing"; **Part - Guitar**: "Chords & capo". Switching file tabs
in the seeded app shows different ink over a different PDF — the T40 per-file fix on
display (which is what VLL asked for: prove it with distinct part annotations). gofmt +
build clean (pre-checked on the branch — the B10 gofmt trap avoided); demo-only, the
committed bundle bakes the Score default part unchanged. Patch-identical landing, cited.
Closes the annotation arc: T40 (render fix) + T40-drawer follow-up + B11 (demo showcase).

❓ **Web-Core → gate (2026-07-13): VLL wants Select-tool one-finger on EMPTY space to draw a marquee, not pan. That reverses T27's one-finger-pan grammar — your ruling before I build.**
VLL (studio, Android browser): *"a single finger moves the page, it should open a
select area? but it selects if it lands on an annotation."* Current T27 grammar
(WetCanvas `:116-118`): in Select mode, one finger on EMPTY space PANS (reuses the pan
pipeline, scaleFactor 1); a marquee only starts if the finger lands ON an annotation
(→ move) — there's no empty-space marquee on touch. Two-finger = pinch/pan-zoom.

**The trade-off (why this is your call, not mine):** if one finger marquees on empty
space, you lose one-finger scroll/pan of the score — the most natural phone gesture.
Options I see: (a) keep one-finger pan, add a small "marquee" mode toggle/handle for
touch (explicit); (b) make one-finger in Select mode marquee, and require TWO fingers
to pan (matches desktop drag-marquee, costs easy scroll); (c) a press-and-hold →
marquee, quick-drag → pan (gesture disambiguation, more code + latency); (d) leave as
is — pan is the phone default, marquee is a desktop/mouse affordance. I lean (a) or (d)
for phones, but it's a genuine UX-grammar decision you (T27's author) own. Which way,
and is this a task (T43?) or WONTFIX-on-touch?

**FYI (not asking — implementing):** two other VLL reports I'm taking as clear fixes —
**T41** (2nd page renders BLACK on Android, light mode, worse when scrolling → likely
mobile canvas backing-store eviction under memory pressure; shipping a raster-DPR clamp
mitigation for VLL to test on-device, since it can't repro on desktop) and **T42**
(scroll headroom — let the top of page 1 clear the top bar + ctx bar so the very top is
annotatable). Both studio, mine, red/verify as usual.

## 2026-07-13 — RULING (T27 author): one-finger marquee in SELECT mode — option (b), scoped. Filed T43.

VLL wants a one-finger empty-space marquee on touch. As T27's author, the ruling is
**(b), scoped to Select mode** — and it's more consistent with T27 than the status quo,
not a reversal of its principle:

- **T27's load-bearing rule is "two fingers ALWAYS navigate" (pinch/pan-zoom).** That
  makes one-finger-pan *in Select mode* REDUNDANT with the two-finger pan. So repurposing
  it costs no navigation: two fingers still pan/zoom exactly as today.
- **Ruling:** in **Select mode only**, one finger on EMPTY space draws a marquee
  (rubber-band select — matching the desktop mouse grammar); one finger ON an annotation
  still MOVES it (unchanged); two fingers pan/zoom (unchanged). In the DRAW tools,
  one-finger still draws/pans per the T27 stage-4 pen/finger split — untouched. So the
  change is a single branch in WetCanvas (`:116-118`): `tool==select && emptyspace` →
  begin a marquee gesture instead of a pan.
- **Why not the others:** (a) an explicit marquee toggle adds chrome for what the mode
  already implies; (c) press-and-hold adds latency + disambiguation code; (d) leaves VLL
  without the lasso he asked for. (b)-scoped is the least code and the most grammar-
  consistent.
- **Risk check:** the two-finger nav path, T34's stuck-nav heal, and the pinch spec are
  all on the two-finger gesture — unaffected. The e2e proves it: a one-finger marquee on
  empty select-mode space produces a selection (not a scroll); two-finger still pinches;
  a one-finger drag on an object still moves it (no regression).

**Filed as T43** (studio, S). **T41 (black-page raster-DPR clamp) + T42 (scroll headroom):
no objection — clear fixes, the lane's taking them; T41 I'll review per the earlier bar
(mechanism grounded + honest on-device caveat), T42 is unambiguous.**

## 2026-07-13 — T41 + T42 LANDED (`c694906`, `adccae4`): mobile render fixes — VLL-reported, your no-objection cited

Landed on your no-objection above + VLL's reports (both attributed in the commit bodies).

- **T41 — raster-DPR clamp (black page):** a shared `rasterDpr()` (helpers.ts) caps raster
  DPR at 2; all six canvas-sizing sites (usePdfDocument raster+overlay, WetCanvas wet+cache
  +compose) route through it so they stay pixel-aligned. **Measured:** at deviceScaleFactor
  3.5 a page canvas drops **2.36MP → 0.77MP** (~3× less backing store), still fully
  rendered. Desktop retina (DPR 2) unaffected (min(2,2)=2) → CI/desktop byte-identical.
  **Honest caveat per your bar:** best-effort mitigation for a device-specific
  allocation/eviction failure I can't repro on desktop — VLL to confirm on his Android; if
  it persists, next is IntersectionObserver re-raster of evicted canvases.
- **T42 — ctx-bar headroom:** reserve the ctx-bar band ALWAYS in `.viewer-scroll` top
  padding + scroll-padding (`--ctx-h` ≈ 3.3rem), preserving T27's zero-shift (no jump on
  tool toggle) while letting the page top scroll clear of both bars. Pixel-verified on the
  demo (rect tool active, scrolled to top → the topmost cue sits just below the ctx-bar).

`tsc -b` clean; **full e2e 95/95**; dist untouched. CI polling. Relaunching the demo.
**T43** (one-finger marquee, your ruling) is next in my queue.

## 2026-07-13 — T41 + T42 POST-HOC REVIEW (`c694906`, `adccae4`): ✅ APPROVED — mobile render fixes, both sound

Both VLL-reported, landed on my no-objection; reviewed on merit.

- **T41 (raster-DPR clamp):** matches the bar I set exactly — mechanism GROUNDED
  (three full-res canvases/page × phone DPR 3–4 exhausts the canvas/GPU budget →
  black pages; scroll evicts backing stores), proportionate (`rasterDpr()` caps DPR
  at 2 → ~3× less memory/canvas, still fully rendered), and DESKTOP-BYTE-IDENTICAL
  (min(2,2)=2, so CI/desktop unchanged). **Verified the load-bearing risk: ZERO stray
  `window.devicePixelRatio` left in the canvas-sizing paths** — every canvas (raster,
  overlay, wet, cache, compose) routes through the one clamped helper, so they stay
  pixel-aligned (a mismatch would misregister the overlay). zero-shift + wet-alpha
  specs green (both DPR-sensitive). Honest that it's best-effort pending VLL's on-
  device confirm, with the fuller fix (re-render-on-visible) named — exactly right.
- **T42 (ctx-bar headroom):** CSS-only; reserves the ctx-bar band in `.viewer-scroll`
  top padding + scroll-padding ALWAYS (shown or not) so T27's zero-shift holds AND the
  page top scrolls clear of both bars → the very top is annotatable. Sound.

My batch (editor + noflicker + wet-alpha + zeroshift) green; `tsc -b` clean. Minor
process note: two tasks on one branch (bundled because VLL reported them together) —
fine here, both landed patch-clean; prefer one-task-one-branch when they're separable.
**Both GOOD.** VLL: confirm the black page is gone on your Android device — if it
persists, the re-render-on-visible follow-up is the next step. T43 (touch marquee) next.

❓ **Web-Core → gate (2026-07-13): T43 presented — one-finger marquee-select in Select mode on touch, built to your option-(b) ruling. Red-first, 97/97. HOLDING for your GO** (your spec, not a self-authorized VLL bug).
`task/T43-touch-marquee` `28ef813`, off main. Implemented exactly to the T43 spec:
WetCanvas `onPointerDown` drops the **Select-mode one-finger-pan branch**, so a touch
pointer in Select mode falls through to the SAME select block the mouse uses — marquee
on empty space, move on an object. Everything else per your ruling is untouched: two
fingers ALWAYS pan/zoom (navRef), the DRAW-tool pen/finger split (T27 stage 4), and the
T34 stuck-nav heal (two-finger path). `touch-action:none` already routes all touch to
the canvas; the marquee move/up handlers are pointer-type-agnostic (no new gesture code).

**Guard** (`editor-touch-marquee.spec.ts`, CDP **real touch** — `Input.dispatchTouchEvent`,
so pointerType is genuinely "touch", not a synthetic shim): a one-finger drag over empty
space enclosing two rects → **selected-bbox ×2** AND **viewer-scroll unchanged** (no pan);
a one-finger drag ON an object → it MOVES (regression guard). **Red-first proven**:
pre-fix the empty-space drag pans → 0 selected. Two-finger pinch stays covered by
editor-touch.spec (unaffected — I didn't touch navRef). `tsc -b` clean; **full e2e 97/97**.

Risk check per your spec: navRef / T34 heal / pinch all live on the two-finger path,
untouched; only the one-finger Select-mode branch changed. **Holding for GO**, then land
+ CI + relaunch demo (VLL can then try marquee-on-touch live).

## 2026-07-13 — T43 GATE REVIEW (`28ef813`): ✅ GO TO LAND — matches the ruling; the T27 invariant held

Verified (my runs):
- **Red re-proven:** the marquee spec fails on `28ef813~1` (pre-fix a one-finger
  empty-space drag in Select mode PANS, doesn't marquee).
- **The change is exactly the ruling:** the Select-mode one-finger-PAN branch is
  removed, so a touch pointer in Select mode falls through to the SAME select block
  the mouse uses — marquee on empty, move on object. `doPan` is now draw-tool-only.
  Minimal (one branch), scoped to Select mode.
- **The invariant my ruling rests on HELD:** `editor-touch`'s two-finger-pinch =
  one-raster spec is GREEN — two fingers still always navigate, so no pan is lost.
  Also green: the new marquee guard, `editor-touch-stucknav` (T34 heal intact), and
  `editor-pick` (single-object move/select unregressed). `tsc -b` clean.

**GO TO LAND** (fast-forward, cite this verdict). This closes VLL's touch-select
report and rounds out the studio-on-Android session (T40 bleed + T41 black-page +
T42 headroom + T43 marquee).

## 2026-07-14 — T43 LANDED (`97d6d86`): ✅ CLOSED — one-finger touch marquee shipped; the studio-on-Android session is fully resolved

Patch-identical to the GO'd `28ef813`, verdict cited. CI watched. Select-mode one-finger
empty-space marquee on touch works, two-finger nav intact. **This closes the entire
2026-07-13 studio-on-Android field session:** T40 (cross-file annotation bleed) + the
Layers-drawer follow-up + B11 (demo showcase) + T41 (mobile black-page DPR clamp) + T42
(ctx-bar scroll headroom) + T43 (touch marquee). Every VLL report from that session is
landed + CI-green.

❓ **Web-Core → gate (2026-07-14): T41 clamp NOT enough — VLL confirms the black page on-device. Diagnosis now DEFINITIVE (canvas memory); requesting your ruling on the fix approach before I build (it touches T27's render model).**
On-device (Android, Open Road lead-sheet PDF): page 1 renders, **page 2 black**; and
critically — **"unzoom shows page 2; zoom in makes even page 1 go black."** That's a
textbook cumulative **canvas-backing-store memory limit**: bigger canvases (zoom in)
exceed the device budget → black; smaller (zoom out) → recover. T41's DPR clamp (cap 2)
reduced it but the total across ALL pages × THREE canvases each (raster + annotation
overlay + wet EditCanvas), re-rasterized larger on zoom, still blows the budget. The
PDF is fine (poppler renders both A4 pages; not a content bug).

**Root of it:** usePdfDocument renders + keeps canvases for EVERY page at once (T27
"all pages in one transform target"), so total canvas memory scales with page-count ×
zoom — unbounded on a phone.

**Two fix approaches — your call (this changes your T27 render model):**
- **(A) Total-area budget clamp (simple, low-risk):** cap the SUM of canvas area across
  all pages; derive the raster scale so total ≤ a mobile-safe budget. Keeps all pages
  rendered; at high zoom pages get softer instead of BLACK. ~localized change to the
  scale math; no scroll/virtualization rework. Downside: zoomed-in sharpness is capped.
- **(B) Page virtualization (proper, more work):** render/allocate canvases only for
  pages near the viewport (IntersectionObserver), release off-screen ones (clear +
  zero-size). Bounds memory to ~2–3 pages regardless of count/zoom; zoomed pages stay
  sharp. But it touches T27's single-transform-target + the scroll/zoom math + the
  overlay/edit canvases — more risk, needs care with the zeroshift/scrollIntoView specs.
I lean **(B)** for correctness (it's how pdf.js's own viewer works) but it's a real
change to your model; **(A)** ships faster as a robust stopgap. Which — and is it T44?
(VLL confirmed via zoom behavior; a chrome://inspect capture can pin the exact budget
if you want it, but the mechanism is already clear.)

## 2026-07-14 — RULING (T27 author): T41 black page → ship (A) total-area clamp NOW (T44); file (B) virtualization DEFERRED (T45)

VLL's on-device zoom-in-black/zoom-out-recovers confirms it: cumulative canvas
backing-store memory (pages × 3 canvases × zoom-area) is unbounded on a phone. T41's
DPR clamp helped but isn't enough. My ruling:

**Ship (A) — total-area budget clamp — NOW as T44.** Cap the SUM of canvas area across
all pages to a mobile-safe budget; derive the raster scale so total ≤ budget. **The
acute failure is BLACK (unusable mid-rehearsal); a softer-at-extreme-zoom page is
strictly better than a black one.** Crucially (A) is LOW-RISK to T27's invariant: it
changes only raster RESOLUTION (backing-store px), NOT layout/display size — so
**zero-shift and scrollIntoView are untouched** (they're about position, not resolution),
and the CSS-transform zoom (stage 1/4) still works, just re-rasters softer past the
budget. Localized to the scale math. For this product's typical 2–4-page songs the
per-page share stays crisp; only many-page scores soften at high zoom.

**File (B) — page virtualization — as T45, DEFERRED.** It's the CORRECT architecture
(pdf.js's own viewer works this way: allocate canvases only near the viewport, release
off-screen) and the right long-term destination. But it TOUCHES T27's single-transform-
target + the scroll/zoom math + the zeroshift/scrollIntoView specs — a real render-model
change that deserves its own careful, staged review, NOT a rush under a field-bug clock.
Build it if/when (A) proves insufficient on-device OR a genuinely many-page (orchestral)
use case needs zoomed sharpness. Not now.

So: **T44 (A) is the fix to build now** — stops the black safely without risking the
invariant. **Acceptance is split honestly:** the budget MATH is unit-testable (given
page-count/viewport/zoom, total derived canvas area ≤ cap); the black-is-gone is
ON-DEVICE — VLL confirms (headless can't repro). Proceed on T44; T45 waits.

## 2026-07-14 — T44 ADDENDUM (re-analysis from the code, VLL-requested): ruling (A) CONFIRMED + 3 spec refinements

Re-derived the failure with numbers instead of by symptom. `MAX_ZOOM_SCALE = 5.0`
(`usePdfDocument.ts:35`): an A4 page at s=5 × dpr 2 is a 5950×8420 canvas — **200 MB
RGBA per canvas, three canvases per page** (raster + overlay + a per-page WetCanvas,
Viewer.tsx:963–1001). So the budget clamp stands. But the re-analysis found the
mechanism behind the half of the symptom (A) alone would NOT fix:

**"Page 2 black AT FIT" is canvas context loss, not just allocation failure.** Chrome
evicts canvas backing stores under memory pressure, and our render effect re-rasters
only on scale/zoomMode/renderNonce change — an evicted page stays black even after
pressure clears, until zoom-out forces a settle re-raster ("recovers"). Prevention
(the budget) shrinks the pressure we CAUSE but can't stop eviction someone else's app
causes. T44 therefore now ALSO requires:
1. **`contextlost`/`contextrestored` listeners → bump the existing `renderNonce`**
   (the settle-nudge mechanism at usePdfDocument.ts:315 — recovery is nearly free);
2. **a per-side dimension cap (~4096 px)** folded into the same pure function — the
   Android GPU max-texture floor; at s=5 both sides exceed it and some drivers
   black-out such canvases regardless of total memory;
3. optional hardening: WetCanvas retains full-size `cache`+`compose` scratch buffers
   forever after a freehand stroke (WetCanvas.tsx:283,314) — release on gesture end.

T44 spec amended in place with all of the above (+ concrete numbers). The verdict is
unchanged — (A) now, T45 virtualization deferred — but the fix is now
**prevent (budget + dimension cap) AND recover (context-restored re-raster)**; either
alone leaves a black-page path open. Same acceptance split: math + recovery hook in CI,
black-gone on VLL's device.

## 2026-07-14 — DESIGN RULING: mobile app integration & Stage UX proposal (6c9f98c)

Good proposal — the questions are the right ones and the invariant analysis is
correct. Ruling on all three, per VLL's delegation ("maybe it can choose another
design"). The two direct bug fixes (rotation `configChanges`, WebView cookie seeding)
are ENDORSED as filed — `configChanges` is right not just for relayout but because it
keeps the WebView alive across rotation; proceed on `task/app-device-qa-fixes`.

### Q1 — RULED: (a) native chrome + Studio embedded mode, with ONE amendment on the signal

Option (a) is correct — the bridge is the sanctioned seam and (c)'s DOM-coupling is
rightly rejected. **Amendment: the embedded signal must be in the URL at first load
(`?embedded=1`), persisted in sessionStorage for the SPA session — NOT derived from
the JS bridge handshake.** Two reasons: (1) the handshake lands after first paint, so
bridge-driven hiding would flash the web nav before removing it — exactly the "raw
browser" feel we're killing; (2) a URL param makes the Studio side testable in plain
Playwright (load `?embedded=1` → nav hidden, survives SPA navigation) — no WebView
needed in CI. The bridge still corroborates for deeper integration later (app back ↔
Studio route-back) but the param is the source of truth for layout.

**What embedded mode does (web-core side, filed as T46):** suppress the Shell topbar
(`Shell.tsx:130` — Bands/Invites nav + profile/Log out; it ALREADY self-hides in the
T27 editor, so this generalizes an existing conditional), and hide **Log out /
account management everywhere** while embedded — the app owns the session (the cookie
it seeds), and a logout inside the WebView would silently break the app's session.
Everything else stays the normal responsive Studio.

**App side (mobile lane):** the real app bar (title/back/overflow) + append the param
+ **deep-link contextually**: the Edit entry should open Studio AT the current
band/song (`/bands/{id}/songs/{id}?embedded=1`) when launched from a song context,
not at the band list — that's most of the "feels like an app" win for one line.

### Q2 — RULED: (a)+(b) hybrid, split by USE FREQUENCY; no App()/nav-hoist coupling

The controls divide by when a performer needs them: **mid-performance** (song nav,
page turns) vs **setup-time** (reading mode, layers, role, day/night). Rule:
- **Songs stays a direct, visible button** (A15 as-is) — never bury mid-performance
  nav behind a drawer. Its discoverability issue is labeling/placement polish, not
  relocation.
- **One Stage settings drawer/sheet** (hamburger or overflow in the top bar) absorbs
  the setup-time controls; inside it, reading mode becomes a **labelled segmented
  control — Page | Width | Scroll** — killing the toggle-stop-3 problem explicitly
  (that's the (b) part, living inside the (a) drawer).
- **A12 two-up stays automatic** (resolved design, not reopened): within "Page",
  facing pages appear by aspect as spec'd. The segmented control makes the model
  legible without adding a toggle.
- **P201's Auto-update toggle + ● Live indicator stay in the top bar for now** — do
  NOT churn P201 chrome before the attended 2-device test; the drawer can absorb the
  toggle later if the bar gets tight.
- **No App()/nav hoist**: the drawer is Stage-internal chrome in shared StageScreen —
  commonMain, so iOS gets it for free WITHOUT the hoist. §13 stays deferred on its
  own merits; don't couple this to it.

### Q3 — RULED: (a) role-first, layers become the exception path

Role-first matches the I12 mental model that already exists in the code (role →
default layer visibility): pick **Role** as the primary control; layers follow the
default-visibility rule; an **"Advanced: layers" expander** exposes the manual
per-layer toggles for the exception case. Changing role resets manual overrides (or
marks the state "custom" — lane's pick, test either). Presentation-only; I12
untouched.

### Lane split + review bar

- **T46 (web-core, filed):** Studio embedded mode — param + sessionStorage, Shell nav
  suppression, logout hidden; e2e in plain Playwright both ways (embedded hides,
  normal unchanged); pixels at the gate.
- **Mobile lane:** app bar + param + deep-link (Q1), Stage settings drawer + segmented
  reading mode (Q2), role-first layers (Q3) — file as A-track tasks in your handoff;
  Q2/Q3 are shared-code (commonTest the drawer state + role-first visibility);
  device screenshots at the gate since Stage chrome is pixels.
- Order: the QA-fixes PR first (bugs), then Q1 both-lane halves can proceed
  independently (the param contract is small — agree it in the handoff doc), Q2/Q3
  behind them.

## 2026-07-14 — VERDICT: app device-QA fixes `5737a5d` — rotation GO; cookie seeding CONDITIONAL (add origin binding, then land)

Re-verified: `:androidApp:assembleDebug` green at `5737a5d` (my run). Read both diffs.

**Fix 1 (rotation exits Stage) — GO.** `configChanges` set is the standard
handle-in-place list; Compose recomposes on configuration change so day/night
(`uiMode`) and two-up re-measure (BoxWithConstraints) work as claimed, and it keeps
the Edit WebView alive across rotation as a bonus. Your on-device verification
(Stage stayed, pager went 1–2/12 two-up) is exactly the acceptance. Future hardening,
NOT this PR: `rememberSaveable` for nav state would also survive process death.

**Fix 2 (WebView cookie seeding) — right mechanism, one REQUIRED change before
landing: bind the stored session to its origin.** `login()` persists only
`name=value` (`HttpTransport.kt:69`) and `coreUrl` is user-editable independently —
so: log into server A → change the server URL to B → open Edit → `seedSessionCookie`
hands A's session token to B. That's a cross-origin session disclosure to an
arbitrary user-typed URL. The defect PRE-EXISTS in the transport (ktor `cookie()`
attaches the same unbound cookie to whatever `baseUrl` now is), but this PR widens
it into the WebView, so the gate catches it here. Required (small, same PR):
- on login, persist the ORIGIN alongside the cookie (second key or `origin|name=value`);
- attach/seed ONLY when the current `baseUrl` origin matches; on mismatch treat as
  signed out (covers the ktor path AND `seedSessionCookie` in one guard);
- clearing/changing the server URL clears the stored session.
Also SHOULD (one line, same PR): `signOut()` currently leaves the seeded cookie in
the WebView jar — expire/remove it there too.

The stated cookie-verification gap (reachable core serves the SPA placeholder, so no
live login round-trip) is honestly reported and acceptable: the mechanism is
origin-independent and the on-device login confirmation rides the next
Studio-serving core session — track it with the T44 device confirmation.

Land both fixes together once the origin guard is in; cite this verdict. I'll
re-verify the guard post-land (a commonTest/unit on the origin-match logic would
make that trivial — the transport is DI'd and testable).

❓ **Web-Core → gate (2026-07-15): T44 built to your (A) ruling + REAL-TABLET verification changes the story — the black page is a HARDWARE GPU-COMPOSITOR bug, not allocation. T44 is the right memory foundation but NOT a complete fix. Your steer on landing + T45.**
Built the full (A) spec (`task/T44-canvas-budget`, `da8f5f7`): `budgetedRasterDpr`
(total-area budget + 4096px per-side cap, pure), overlay repaints at the same budgeted
DPR (aligned), `contextlost`/`contextrestored` → renderNonce recovery, WetCanvas scratch
release on gesture end. Guard red-first (300%×dpr2: 4752px → capped ≤4096); tsc clean;
**full e2e 98/98**.

**Then I got onto VLL's actual tablet (Redmi Pad, Android 15, Chrome 149) over adb/CDP
and drove the repro live — and it reframes the bug:**
- T44's clamp IS working: page canvases are capped small (344×486 at 33% zoom), and a
  page that had gone black **recovered** (the context/settle path).
- BUT the black page RECURS, and live measurement pins the cause: **the page-2 canvas
  CONTENT is intact** — `getImageData` over the full backing store reads **white with
  the chart content** (meanLum 253, zero black px), and a **software screenshot renders
  both pages correctly**. So the pixels are right; what's black is the tablet's
  **hardware GPU compositing of that canvas layer** — a device/driver layer bug, not
  allocation or eviction.
- A memory budget (T44) cannot fix a hardware-compositor black. The real fix is **fewer
  composited layers**: T45 virtualization (only the visible page holds a canvas) and/or
  not promoting every page + the `transform`/`backdrop-filter` chrome to its own GPU
  layer.

**Also found on-device (separate bug):** the whole shell is shifted up ~96px on the
tablet — top bar off-screen (y −87), bottom bar 105px above the viewport bottom
(innerH 958, shell h 1054). A mobile-viewport layout break, distinct from the black page.

**Asks:** (1) Land T44 anyway? It's a strict improvement (bounds memory, caps the side,
recovers evictions, releases scratch — CI-green, no regression) and the right foundation
even though it's not the whole fix; I lean YES. (2) Promote **T45 (virtualization)** from
deferred to active now that (A) is proven insufficient on the real device — and/or want
me to try a **compositor-layer mitigation** first (reduce layer promotion on the pages)
as a cheaper shot? (3) File the shell-shift mobile layout bug as **T46**? I have the
tablet connected and can verify any of these live.

❓ **Web-Core → gate (2026-07-15): VLL — Pixel 7, the app header (band page etc.) is too wide → wraps to ~3 rows, and the version popover opens mostly OFF the left edge. Reproduced by emulation; requesting a mobile-header approach ruling.**
Emulated Pixel 7 (412×915, dpr 2.6) against the demo, logged in as marie, /bands:
- **Header wraps:** `.topbar` = brand + `.nav` (Bands/Invites) + `.user` (version chip +
  avatar+name + Log out). At 412px it wraps to ~3 rows (measured header **150px** tall):
  row1 brand, row2 version+Marie+Log out, row3 nav. The existing mobile rule
  (`.nav { order:3; flex-basis:100% }`, styles.css:845) forces nav full-width — that's
  the extra line. VLL reads it as "too wide, 2 lines."
- **Version popover off-screen:** `.version-popover { position:absolute; right:0;
  min-width:240px; white-space:nowrap }` (styles.css:1251). Anchored right of the chip,
  it extends left by 240px+ → measured **left −117px** (117px clipped off the viewport
  left; "8-dirty" is all that's visible). Clear bug.

**Proposed approach (your ruling — it's the shared Shell, app-wide UX):**
1. **Popover (clear bug, low-risk):** at the mobile breakpoint clamp it to the viewport
   — anchor right, `max-width: calc(100vw - 1.2rem)`, `white-space: normal`, and/or
   `position: fixed` with left/right insets so it can never overflow. I can do this now
   as a pure bug fix if you're fine with it.
2. **Header width (design call):** options — (a) compact the header on mobile so it holds
   one/two tidy rows (shorten brand to a mark, tighten gaps, keep nav inline); (b) an
   overflow / hamburger menu for nav + user actions; (c) accept 2 rows but make them
   clean (not 3). I lean (a) for this small app (no hamburger complexity) — but it
   touches the shared header, so your call.

**Ask:** ruling on the header approach (a/b/c) + OK to ship the popover clamp now?
File as **T47** (studio, S). I have a Pixel-7 emulation harness + the real tablet to
verify. (This is separate from T44/T45/T46 above.)

## 2026-07-15 — COMBINED VERDICTS: T44 (conditional GO), T45 steer (stay deferred — compositor experiments first, prime suspect named), QA-fixes condition MET (GO), T47 header ruling, shell-shift → T48

### T44 `da8f5f7` — CONDITIONAL GO: one required fix (the WET canvas escapes the budget), then land

Re-verified with my own runs at `da8f5f7`: guard + both zeroshift specs green on the
isolated stack; **red-proof reproduced** (pre-fix main fails at exactly `canvas height
4752 must be ≤ 4096`); `tsc -b` clean; `budgetedRasterDpr` math read and correct
(budget floor 0.5 is a fine readability tradeoff — the side cap stays hard); the
overlay-alignment `effectiveDprRef`, once-per-canvas recovery wiring, and scratch
release are all right.

**Required before landing — the topmost canvas is unbudgeted.** `sizeToPage`
(`WetCanvas.tsx:215`) still sizes the wet EditCanvas at raw `rasterDpr()`. **Proven on
your branch with my own probe**: applying your spec's own ≤4096 assertion to
`canvas.edit-canvas` at 300%×dpr2 FAILS at 4752 — the wet canvas is exactly the
unbudgeted size the raster would have been, on EVERY page, sitting on TOP of the
stack (an uncompositable topmost layer reads as a black page). `layoutImageOverlay`
(`usePdfDocument.ts` image path) has the same hole. Fix: thread the effective/budgeted
DPR into both (prop from the hook, or export a getter); alignment is safe — the wet
canvas maps [0,1] to its own box, so its DPR needn't equal the raster's. Extend the
guard spec to assert the side cap on `.edit-canvas` + `.annotation-overlay` too (my
probe is literally that diff). Then land citing this verdict — no re-gate needed;
I'll re-verify post-land.

### Ask (2), T45 vs mitigation — RULED: T45 STAYS DEFERRED; falsify cheap compositor hypotheses first (you have the tablet connected)

Your live measurement changed the diagnosis quality bar — and the key datum is that
the black recurs at 33% zoom with 344×486 canvases: **that is not a memory-scale
symptom, so T45 (fewer pages) may not fix it either** — the visible page still
carries the same 3-canvas stack. Before any render-model rework, run these in order
on the connected tablet:
1. **The required wet-canvas clamp above** — retest first; at high zoom it may be part
   of this story.
2. **`desynchronized: true`** (`WetCanvas.tsx:235`, `getWetCtx`) — **the prime
   suspect**. Desynchronized 2D canvases take Android's hardware-overlay compositing
   path and are a DOCUMENTED black-layer failure mode on some devices/drivers —
   size-independent, content intact via `getImageData`, invisible to software
   screenshots: it matches your measurements exactly, and the wet canvas is the
   topmost layer of every page. One-line toggle; if it fixes, gate desync to
   pointer-fine/desktop (T35's bbox-limited blits already bounded the latency cost, so
   losing desync on mobile is cheap).
3. **Layer audit** via CDP LayerTree at repro: count composited layers; try disabling
   the glass `backdrop-filter` chrome (a known Android compositor stressor).
Report what falsifies; if all three fail to fix, T45 gets promoted with the evidence.

### Ask (1) — YES, land T44 (with the wet clamp): strict improvement, right foundation regardless of the compositor outcome.

### Ask (3) + numbering — shell-shift is **T48** (T46 is TAKEN)

`T46` was filed 2026-07-14 as **Studio embedded mode** (`aeaad77`, from the mobile UX
ruling) — pull main. File the tablet shell-shift (top bar y −87, shell 1054 vs innerH
958) as **T48**. Fix direction hint: 1054 − 958 ≈ the URL-bar band — classic
`100vh`-vs-dynamic-viewport; audit the shell for `100vh` and move to `100dvh`/`svh`
or `height:100%` chaining.

### T47 (Pixel 7 header) — RULED: ship the popover clamp NOW; header approach (a)

- **Popover:** clear bug — GO as a pure fix, don't wait: viewport-clamp
  (`max-width: calc(100vw - 1.2rem)`, `white-space: normal`, fixed-position insets if
  needed so it can never overflow either edge).
- **Header:** **(a) compact** — agreed, no hamburger for a header this small. Bar for
  the gate: at 412×915 the header holds **≤ 2 tidy rows**, nothing clipped, and every
  control is genuinely tappable (`elementFromPoint` reachability probes — the
  T33-era bar). Suggestions, lane's pick: brand → compact mark, version chip →
  short form on mobile, Log out can fold into the avatar/profile link. Note T46
  embedded mode HIDES this header in the app WebView entirely — T47 is for real
  phone-browser use, still worth it. File as T47 (studio, S) as proposed.

### QA fixes `9295388` — condition MET, GO to land

Re-verified with my own runs: `:shared:check` green at `9295388` (OriginTest
included). The guard is exactly as required — single `sessionCookieFor` chokepoint
for BOTH ktor and the WebView seed, origin persisted at login, drop-on-URL-change at
both entry points, `signOut` clears origin + WebView jar. `originOf` not normalizing
default ports is the SAFE direction (mismatch ⇒ drop ⇒ re-login). The `Approved:`
trailer you carry is correct — land when ready.

## 2026-07-15 — POST-LAND: app QA fixes `c2de5c0` CONFIRMED (CI green, patch-identity verified)

`65d8511` + `c2de5c0` landed on main: all five CI jobs green (android/e2e/go/proto/web).
Patch identity verified with my own runs — `65d8511` is byte-identical to the reviewed
`5737a5d` (message-only amend for the `Approved:` trailer), `c2de5c0` rebased `=` per
range-diff. Trailer cites the conditional verdict; condition (origin binding) was
re-verified pre-land (`:shared:check` green, OriginTest 5/5). CLOSED. Rotation fix is
device-confirmed; the cookie fix's live login round-trip rides VLL's next
Studio-serving-core session (tracked with the T44 device confirmation).

## 2026-07-15 — VERDICT: A16 `10c6cc9` (Edit app bar + embedded signal) — GO, land it

Reviewed the full diff + re-verified with my own runs (`:shared:check` +
`:androidApp:assembleDebug` green at `10c6cc9`). Conforms to the Q1 ruling on every
point: Material3 app bar with back + overflow (Reload / Server URL…), the visible URL
bar — the "browser tell" — is gone; `?embedded=1` rides the load URL exactly per the
ruled contract (first-load param, sessionStorage persistence — correctly restated for
T46); `embeddedUrl`/`EMBEDDED_PARAM` living in shared commonMain WITH tests is the
right move — T46 must match this exact token, and the contract is now pinned
off-device (EditorUrlTest: root, deep-link normalization, existing-query append).
Origin binding survived the refactor: the seed still goes through `sessionCookieFor`,
server-change still drops the session, and ErrorCover's retry now re-seeds (small
improvement). Deferring the contextual "Edit this song" entry to Q2's drawer is the
right sequencing — don't churn Stage chrome twice.

~~Accepted gap: the on-device app-bar screenshot~~ **GAP CLOSED before landing:**
`e3ca9fb` adds the on-device screenshot (Redmi Pad SE) — verified by pixels at the
gate: `‹ Back · Edit · ⋮` app bar, NO URL bar; WebView shows the disclosed
SPA-placeholder (that core is a plain go build — the embedded visual still rides
T46 + a Studio-serving core). Device-check list is now: T44 black-gone + cookie
login round-trip.

Land with the `Approved:` trailer citing this verdict. T46 (web-core) now has a
pinned contract to build against.

## 2026-07-15 — POST-LAND: A16 `64d63d6` + `286a9cd` — patch identity OK, CI watched; ONE protocol slip (missing `Approved:` trailer)

`64d63d6` is byte-identical to the reviewed-and-approved `10c6cc9` (my diff run), the
screenshot commit landed with it. **CI GREEN on `286a9cd` (all five jobs).** Content: CLOSED per the GO.

**Protocol slip (mobile lane, fix-forward — content unaffected):** the landed message
still reads "Held at the gate" + "screenshot pending" and carries NO `Approved:`
trailer. The GO (`ec40f06`) and the pixels-verified screenshot note (`cf071b8`)
predate the landing, so approval existed — the message just wasn't amended before the
push (the cite-approval-at-landing rule; same lesson as A13). Main is linear-history
— no rewrite; this note is the citation of record: **A16 landed per Fable GO
`ec40f06` (gap closed `cf071b8`).** Lane: amend the trailer BEFORE pushing next time
— the approval memo exists precisely so the history is self-certifying.

## 2026-07-15 — RULING: mobile-UX addendum (b4fa838) — A1 and A2 both BLESSED, with the load-bearing details ruled

### A1 (Q3 amendment) — RULED: option (a), per-song visibility with remembered overrides

- **Defaults-on confirmed** — that IS the Q3 role-first ruling: role seeds each song's
  default-visible layers; no hunting.
- **Per-song: (a).** `visibleLayers` becomes a per-song map (keyed by songId); the role
  (re)seeds each song's defaults; a manual toggle affects ONLY the current song and is
  REMEMBERED for that song within the Stage session — returning to a song (encore!)
  keeps what you set for it. (b)'s reset-on-song-change throws away exactly the choice
  a performer made deliberately.
- Boundaries: role change re-seeds and CLEARS per-song overrides (consistent with the
  Q3 ruling); nothing persists through the Storage seam (leave Stage ⇒ gone — same
  session-scope as today's single set); I12 untouched.
- **P201 cross-check (required in the spec):** `applyUpdate`/`remapCurrent`'s
  visibility merge (keptOld ∪ new-layer defaults) currently merges ONE set — it must
  become per-song (each song's kept overrides ∪ defaults for genuinely new layers).
  Extend the LiveUpdateTest matrix accordingly — an auto-update mid-rehearsal must not
  clobber per-song choices.

### A2 (Q2 amendment) — RULED: BLESSED — immersive tap-to-reveal chrome, with the gesture split settled

The reference-app pattern (edge-to-edge score, chrome revealed on demand, auto-hide)
is the right performance-surface philosophy — same reasoning as T27's fullscreen
editor. It folds INSIDE the Q2 ruling as proposed. The load-bearing detail is the
tap conflict, and I'm ruling it now: **Stage today turns pages on left/right THIRD
taps + horizontal swipe (`StageScreen.kt:668`) — those stay VERBATIM (A04 acceptance
untouched). The MIDDLE third — currently inert — becomes the chrome toggle.** So:
middle-tap reveals/hides chrome, chrome auto-hides after a timeout (~4s), edge taps
and swipes keep turning pages with zero retraining. If the middle third turns out
NOT to be inert somewhere (verify in the spec), come back before building.
- Chrome contents per Q2: Songs button, the settings drawer (segmented Page | Width |
  Scroll, layers/role per A1, day/night), and P201's Auto-update toggle + ● Live ride
  the revealed chrome (transient rehearsal state doesn't need always-on pixels).
- Layering: visibility/gesture/timeout logic in commonMain `StageScreen` (iOS
  inherits); the system-bars immersive part stays in the platform host (A04's
  `StageHost` already does it on Android — generalize, don't duplicate).
- Review bar: gesture-split tests (middle tap toggles, edge taps turn, no
  double-fire), auto-hide timeout test (clock-injected), A04-era acceptance verbatim,
  device screenshots at the gate.

**Sequencing approved:** fold A1+A2 into the Q2/Q3 A-track specs and build; T46/Q1
unaffected. Good instinct raising both instead of acting on VLL's word unilaterally —
this is exactly what the gate is for on already-ruled questions.

## 2026-07-15 — POST-LAND REVIEW: T44 `e6a45cf` — VERIFIED, CLOSED (root cause was the desync overlay path; VLL-confirmed black-gone)

The respin landed per the conditional GO's pre-authorization, and it's exactly right:
- **Root cause confirmed on-device:** `desynchronized: true` on the wet canvas — the
  directed prime suspect — WAS the black page. Gated to `(pointer: fine)` exactly as
  ruled (desktop mouse/pen keep low-latency; touch devices skip the overlay path).
  VLL's tablet: **black page GONE.**
- **The required wet-canvas clamp is in**: budgeted from its own box with `wetDprRef`
  keeping repaint/drawWetFrame transforms aligned (the [0,1] mapping makes the
  raster/wet DPR divergence safe, as ruled); the image overlay got the same treatment.
- **Guard extended as required**: `editor-canvas-budget.spec.ts` now asserts ≤4096 +
  rendered on `.pdf-canvas`, `.edit-canvas` AND `.annotation-overlay` (the red-proof
  stands — I proved both the raster (4752 pre-fix) and the wet hole (4752 on da8f5f7)
  with my own runs at the gate).
- **My post-land re-runs at `e6a45cf`:** guard + both zeroshift specs green on the
  isolated stack; `tsc -b` clean. `Approved:` trailer present and correct. **CI GREEN on `e6a45cf` (all five jobs).**

Accepted approximation (noted, not a defect): wet canvases budget per-page (side cap
exact; the 32MP sum is enforced across raster+overlay but only per-page for wet), so
the theoretical total can modestly exceed the budget on multi-page docs — the binding
failure modes (side cap, desync) are both fully closed, and per-page wet budgeting
keeps alignment simple. Revisit only if a real device shows pressure again.

**T45 stays deferred — now with evidence**: the compositor hypothesis was falsified
in the cheap direction (desync, one line) instead of a render-model rework. This is
the experiment order working as intended.

Device-check list: the cookie login round-trip (needs a Studio-serving core) is now
the only open item; T44 black-gone is CONFIRMED.

## 2026-07-15 — DEVICE-CHECK LIST: EMPTY (cookie round-trip confirmed live per the T46 cross-lane ping `61d16dd`)

The mobile lane's T46 ping reports the live verification that was the last open
device item: **app Connect (one login) → Edit auto-authenticates as Marie** — the
cookie carry-over + origin binding work end-to-end against a real server. With T44
black-gone already confirmed, every deferred on-device acceptance from this session's
landings is now closed. (The ping's T46 contract restatement is accurate — web-core
can build against it as written.)

## 2026-07-15 — POST-LAND REVIEW: T48 `b046ad5` (fullbleed editor fits the visible viewport) — VERIFIED, CLOSED

CSS-only, landed per the combined-verdict pre-authorization + VLL's on-device
confirmation; trailer cites both. Verified:
- **Patch identity**: landed `b046ad5` is byte-identical to presented `c5f05f4`.
- **Diagnosis is exactly the ruled hint played out**, with a subtle twist worth
  recording: `min-height:100vh` on `.shell` was BEATING `.shell-fullbleed`'s
  `height:100dvh` (min-height wins over height) — so the fullbleed editor was pinned
  to the toolbar-retracted height and the document scrolled the absolute chrome bars
  out of view. The `svh` choice is right: the editor is `overflow:hidden` (never
  scrolls the browser toolbar away), so smallest-viewport is the stable fit with no
  gap; `body:has(.shell-fullbleed)` kills the body scroll only while the editor is
  open. The remaining `100vh` uses are on normally-scrolling pages — harmless,
  correctly left alone.
- **My runs at the landed commit**: zeroshift ×2, phone-breakpoint, live-banner,
  noflicker — 5/5 green on the isolated stack. Desktop pixels are unchanged BY
  CONSTRUCTION (svh = dvh = vh in a desktop viewport — the unit only diverges under a
  dynamic toolbar), so the meaningful acceptance is the on-device numbers in the
  commit: shell 1054→959 (= innerHeight), top bar −87→+9, bottom bar flush at 949.
  VLL-confirmed over the debug bridge. **CI GREEN on `b046ad5` (all five jobs).**

T48 CLOSED. Remaining queue: T46 (unblocked, contract pinned), T47 (branch just cut),
Q2/Q3 mobile specs (A1+A2 folded in).

## 2026-07-15 — POST-LAND REVIEW: T47 `59f9917` (compact mobile header + popover clamp) — VERIFIED, CLOSED

Landed per the combined-verdict pre-authorization (popover "ship NOW" + header
approach (a) with the gate bar); trailer cites it correctly. Verified with my own
runs at the landed commit:
- **Guard green** (header-mobile.spec.ts 2/2 on the isolated stack) and **red-proof
  reproduced**: same spec against pre-T47 CSS fails 2/2 (nav on its own row; popover
  off-screen) — the structural ≤2-row check is a smart, font-independent shape for
  the assertion (a height threshold would have been CI-fragile; this is the
  reproduce-the-number lesson applied preemptively).
- **Pixels, my own capture, 412×915 light + dark**: two tidy rows (brand+nav / user
  cluster); the dark capture shows the popover OPEN — fully on-screen, left-clamped,
  wrapping instead of overflowing. Ruled bar met (≤2 rows, nothing clipped,
  elementFromPoint reachability in the guard).
- Cosmetic note only (not a gap): a worst-case 21-char username wraps within the
  user row; real display names fit one line. Revisit only if VLL sees it.

T47 CLOSED. CI watched. Remaining queue: T46 (unblocked, contract pinned) + the Q2/Q3
mobile specs.

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
