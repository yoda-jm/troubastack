# Handoff — Architect / Reviewer

*Last updated: 2026-07-04. If you are a fresh session picking up this role: read this
file top to bottom, then `docs/tasks/README.md`, then act. You do not implement tasks —
you specify, review, steer, and keep the docs truthful.*

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

## State as of this writing

**Landed on `main`** (linear, all CI-green): T01–T12 + T10-part-1, A01–A06 (+ NRGBA
fixture fix), CI with 5 hard-gating jobs + APK artifact + Node-24 action pins, README
quick-start/screenshots, real-music demo bundle (`docs/demo/`, hand-baked — see its
README), the full task pack including the B/IOS/P2 tracks.

**The product today:** collaborative realtime annotation editor (web, fast wet ink,
pluggable annotation types), Go core with tested sync invariants, Android app that
performs `.tstage` bundles offline (resilient/read-only/login-free), imports them
atomically, and hosts the live editor in a WebView.

**Open queue:** T13 (CI e2e quarantine), T14 (chrome ≤220px), T15 (Viewer hook split —
**attended, quiet machine**), T16 (seed em-dash) · B01→B02→B03, OPS01 · IOS01→IOS02 ·
P201–P203 (P203 starts with a cheap decision stage). Priorities: **B-track is the
critical path** (closes compose→bake→distribute→perform); T15 next time the machine is
calm; T16/T13/T14 fillers.

**Blocked on Vincent, not on agents:**
1. **The tablet stylus spike** — decides A07 (native ink) build-or-close. Everything
   needed is on `main`; the web wet path measured ~3 ms event→paint on desktop.
2. **Rotate the credential embedded in the git remote URL** (flagged at session start;
   use `gh auth` or a credential helper instead). Related: IOS03 needs a Mac + Apple ID
   decision eventually.

## Style expectations for new specs (keep the bar)

Verify every file/line claim against the tree before writing it down; resolve design
decisions in the spec (named explicitly, with rationale) rather than leaving them to
executors; give acceptance criteria that are *commands and observable outcomes*, not
vibes; state out-of-scope explicitly; when a criterion turns out impossible, expect the
executor to say so rather than approximate — and honor it (the T05 "372 vs 220px"
close-out is the precedent: honest gap + follow-up task beats forced compliance).
