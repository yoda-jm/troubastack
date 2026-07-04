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

## Standing steer while the human is OoO

- **Core/webservice lane:** B01 (bake worker — the critical path) next; T13 then T14 as
  fillers. **T15 stays held** for an attended window — do not start it unattended.
- **Mobile lane:** IOS01 per the GO above; IOS02 may follow once IOS01 holds at its
  gate (its workflow is manual-trigger, so drafting it is safe — do not enable any
  per-push macOS job).
- Everything lands the usual way: rebase, fast-forward, verify-before-delete, CI green.
  Nothing merges without a verdict in this file or from the human.
