# Handoff — Architect / Reviewer

*Last updated: 2026-07-10. If you are a fresh session picking up this role: read this
file top to bottom, then the **digests** (`SUMMARY-2026-07-04-to-06.md` →
`…-06-to-07.md` → `…-08-to-10.md` — reviews.md is ~2.5k lines of verdicts; the digests
are the entry points), then `docs/tasks/README.md` § Queue state, then act. You do not
implement tasks by default — you specify, review, steer, and keep the docs truthful.
(Exception used in practice: VLL sometimes directs the architect to implement an
XS/S task directly — T28/T29/T30 were done that way, each with attached evidence in
lieu of independent review; record the role note in the verdict when it happens.)*

## The working model (three agents + Vincent)

- **Architect/Reviewer (this role).** Writes self-contained task specs into
  `docs/tasks/` (context · exact changes with *verified* file/line claims · acceptance
  criteria with runnable commands · out-of-scope). Reviews every task the other agents
  present: **re-verify the acceptance criteria with tools — never approve from the
  report alone.** Makes design decisions *inside* specs so executors don't improvise.
  Maintains README/screenshots/demo assets and this file. Lands its own docs-only
  commits directly (fast-forward).
- **Mobile-app developer.** Executes A/IOS-track tasks. Works in an isolated git
  worktree (`../troubastack-<task>`), one task = one branch (`task/<id>-name`) = one
  PR, holds at the review gate, lands by rebase + fast-forward push (never a merge
  commit), verifies the push before deleting the branch.
- **Core/webservice developer.** Same protocol for T/B/OPS-track tasks (Go core,
  studio/ink/bake, CI).
- **Vincent (the human)** launches agents, relays "Txx committed, review" to this role,
  makes product calls, and owns the two credential-shaped blockers (below).

The loop that has worked for ~25 task landings: spec → execute (agent may deviate from
a wrong spec — twice the executors caught real spec errors; when they do, they update
the task file too) → present at review gate → this role verifies → land linear → CI
(5 hard-gating jobs) has the final word. Discovered follow-ups become new task files
(T13/T14/T15/T16 all arose this way) instead of scope creep.

## Review protocol (what "verify" means here)

- Walk the task's acceptance criteria one by one, executing each check yourself:
  greps, `make test`, typechecks, fresh `--rerun-tasks` Gradle test runs, `buf lint`
  via `npx @bufbuild/buf`.
- UI claims ⇒ look at pixels: `make dist` + run the binary with the seeded
  `core/troubadata` (login `marie`/`demo`), drive it with Playwright from
  `web/studio/node_modules` (scripts must live inside `web/studio/` for module
  resolution), crop/zoom screenshots with PIL before trusting your eyes — full-page
  impressions have been wrong twice.
- App claims ⇒ the Pixel_7 AVD at `~/Android/Sdk` (headless:
  `-no-window -gpu swiftshader_indirect`). New worktrees need
  `app/local.properties` with `sdk.dir`. To skip the SAF picker, inject bundles via
  `adb shell run-as com.troubashare.app`. Under heavy host load the emulator ANR-storms
  — schedule attended/e2e-sensitive work (like T15) for a quiet machine.
- CI ⇒ query the GitHub API for the head SHA's run + per-job conclusions (auth: `gh`
  or the credential already configured in the git remote — do not copy it anywhere).
- Restore `core/internal/webassets/dist/` (committed placeholder) if a local build
  dirties it; never commit rebuilds of it.

## Repo quirks that keep biting (verified, not folklore)

- `web/` npm workspace is nominal: **no root lockfile**; install per-package with
  `npm ci --no-workspaces`; typecheck with `web/studio/node_modules/.bin/tsc`
  (`-p ink`, **`-b studio`** — its tsconfig is a solution file, `-p` checks nothing —
  `-p bake`).
- Parallel servers: `TROUBACORE_ADDR=:8091` etc. — port 8080 may belong to another
  agent's e2e run.
- The task pack's `README.md` is the collision hotspot between agents — rebase early.
- Demo data lives in `core/troubadata` (file store) — my T06 review drew scribbles on
  the *Score* file of Wonderwall; the Vocals part is clean (the demo bundle came from
  there). `rm -rf core/troubadata && make demo` reseeds.

## State as of this writing (2026-07-06)

**Landed on `main`** (linear, all CI-green): T01–T13 + T16, A01–A06, **B01 + B02 (+
B03's server slice)**, **IOS01–IOS04** (+ the IOS03 prep runbook), the LRU portability
fix, CI with 5 hard-gating jobs (+ iOS klib cross-compile in the android job, + the
manual `ios.yml` simulator proof — Wonderwall pixels verified). T14 closed honestly
without landing (~10px, superseded by T17). Full verdict history:
`docs/handoff/reviews.md` — ~15 reviews on 2026-07-04/05/06 alone, every landing
independently re-verified (including a live seed→bake→download→Kotlin-loader run for
B02 and artifact-pixel checks for IOS02).

**The product today:** collaborative realtime annotation editor (web), Go core with
tested sync invariants and a **real bake pipeline** (Studio "Bake" button →
downloadable `.tstage`, admin-gated), and an app that imports + performs those bundles
offline on **Android and iOS** (simulator-proven; screen stays awake on stage). The
compose → bake → download → perform loop is closed; in-app distribution (B03) is the
last product gap.

**Open queue** (see `docs/tasks/README.md` "Queue state" for the always-current list):
**B03 app half** is the critical path · B04 (bake atomicity) · B05 (regen demo bundle)
· T18 (mirror dedup) · OPS01 · P201–P203. **Attended-only:** T15, T17 (read its attempt
log first), and the B02 Android loop-close screenshot (assigned to the mobile lane).

**Blocked on Vincent, not on agents:**
1. **The tablet stylus spike** — decides A07 (native ink) build-or-close. Everything
   needed is on `main`; the web wet path measured ~3 ms event→paint on desktop.
2. **Rotate the credential embedded in the git remote URL** (long-flagged; use a
   credential helper). Related: IOS03 impl needs a Mac + Apple ID decision.

## Style expectations for new specs (keep the bar)

Verify every file/line claim against the tree before writing it down; resolve design
decisions in the spec (named explicitly, with rationale) rather than leaving them to
executors; give acceptance criteria that are *commands and observable outcomes*, not
vibes; state out-of-scope explicitly; when a criterion turns out impossible, expect the
executor to say so rather than approximate — and honor it (the T05 "372 vs 220px"
close-out is the precedent: honest gap + follow-up task beats forced compliance).
