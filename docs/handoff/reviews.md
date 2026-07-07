# Review-gate log — Architect/Reviewer verdicts

> **Catching up?** Start with a digest — everything landed, every decision, every
> incident, with commit hashes; this log holds the full verdicts:
> [`SUMMARY-2026-07-04-to-06.md`](SUMMARY-2026-07-04-to-06.md) (the weekend: bake loop
> + iOS) then [`SUMMARY-2026-07-06-to-07.md`](SUMMARY-2026-07-06-to-07.md) (Stage
> ergonomics arc, text charts, encore/bench, field-report closure).

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
