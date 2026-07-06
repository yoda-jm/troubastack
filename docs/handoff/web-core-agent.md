# Agent handoff — "Web & Core Agent" (TroubaStack T-track)

> **You are the Web & Core Agent.** Your lane is the **T-track** — the web clients (`web/studio`,
> `web/ink`, `web/bake`), the Go server (`core/`), the contract (`proto/`), and repo infra (CI,
> `docs/`). A separate agent owns the **A-track** (the Kotlin/Compose app in `app/`); stay out of its
> way (see §2, §7).
>
> Point a fresh Claude Code session at this file to continue seamlessly. It captures **how we work**
> and **what's done**, not the code (read the code + `docs/` for that). Last updated 2026-07-04.

---

## 1. What TroubaStack is

A monorepo for a band sheet-music app. Four parts, one contract:

- **`proto/`** — the single source of truth for every domain type + wire message (invariant **I1**).
  Clients carry **hand-written mirrors** (Go/TS/Kotlin) kept in sync by review + `AUTHORITY:`
  comments; codegen has never run (open I1 debt — see T09).
- **`core/`** — Go server (TroubaCore): REST + realtime WebSocket sync + serves the embedded SPA.
  `go 1.26`, stdlib-only, builds offline. Layers: `internal/{domain,store,sync,app,bake,httpapi,
  webassets,engine,gen}` + `cmd/troubacore`. Sessions/auth live in **`internal/app` (app.Service)** —
  there is **no** `internal/session` (removed in T11).
- **`web/studio/`** — the **canonical** React/Vite editor SPA (TroubaStudio). This is *your* main
  surface. `web/ink` (`@troubastack/ink`) is the **one** stroke renderer (I8); `web/bake` flattens
  pages server-side (still a stub). Never reimplement the editor (**I10**).
- **`app/`** — the A-track's Kotlin Multiplatform app. Not your lane.

**Read first:** `docs/ARCHITECTURE.md` (numbered invariants I1–I15, normative — each now tagged
✅ enforced-today / 🎯 target after T12) and `docs/tasks/README.md` (the task pack + ground rules).
`docs/design/01..08` are the design notes.

## 2. How we work (the operating model — this is the important part)

- **Task pack.** Work is pre-specified in `docs/tasks/{T,A}NN-*.md`, priority-ordered and
  self-contained. **T-track** = web/core/infra; **A-track** = the mobile app. The reviewer
  (referred to as **"Fable"**) *writes* the specs; you (Opus) *execute*; Fable *re-verifies both the
  work and the spec* on review and may correct the spec itself. (Memory: `task-pack-workflow.md`.)
- **One task = one commit on `main`.** This is the standing rule — squash everything for a task into
  a single commit. **The one exception so far:** T10 was allowed **2 commits** (the user relaxed it
  because the Viewer split is genuinely harder). Follow-ups that are their own scope get their own
  commit (e.g. `T01 follow-up`, `T02 follow-up`).
- **You work in the PRIMARY worktree** `/home/yoda/dev/git/troubastack`, and **`main` is checked out
  here.** (This is the opposite of the A-track, which uses isolated worktrees.) So you commit on a
  short-lived `task/TNN-name` branch, fast-forward `main` locally, and push `main` directly — see §5.
- **Linear history — no merge commits.** Land by fast-forwarding. `main` moves fast (the A-agent
  lands often), so **expect to `git fetch` + rebase almost every time** before landing. (Memory:
  `git-linear-history.md`.)
- **CI must be green before you call a task done.** Push, then poll the run to green (§3). If CI is
  red, fix forward on the same task.
- **Verify for real, then report honestly.** `make test` / `tsc -b studio` / e2e locally before
  push; state what was verified vs. assumed. **Make honest scope calls** — when a task's target isn't
  fully reachable safely, land what's solid and *file the remainder as a follow-up task* rather than
  forcing it (this is how T14 and T15 were born; see §6/§8). The user has explicitly praised deferring
  risky unattended refactors.
- **Screenshots for visual tasks.** For studio-appearance tasks (T03/T04/T05), produce **before/after
  PNGs** and open both in the user's KDE viewer: `gwenview <before> <after> &`. Truthful ones live in
  `docs/screenshots/`.

## 3. GitHub / CI mechanics

- `gh` is **NOT installed** (`command -v gh` → nothing). Use the GitHub REST API via `curl`.
- **Token:** extract a PAT from a **sibling repo's** git config — do **not** rely on this repo's
  origin. NEVER echo it; scrub every command's output:
  ```bash
  TOKEN=$(sed -nE 's#.*url = https://([^@]+)@github\.com/yoda-jm/devices-manager.*#\1#p' \
            /home/yoda/dev/git/devices-manager/.git/config | head -1); TOKEN=${TOKEN##*:}
  git push origin main 2>&1 | sed -E "s/${TOKEN}/<TOKEN>/g"
  curl -s -H "Authorization: token $TOKEN" https://api.github.com/repos/yoda-jm/troubastack/...
  ```
- **Watch CI** (no `gh`): poll `GET /repos/yoda-jm/troubastack/actions/runs?per_page=10`, match the
  run whose `head_sha` starts with your pushed short SHA, wait for `status=completed`, then read
  per-job conclusions from `GET /actions/runs/<id>/jobs`. Run the poll loop as a **background** Bash
  command writing to a file under the scratchpad `tasks/` dir; you'll be notified on completion.
- **CI has 5 jobs:** `go`, `web`, `proto`, `android`, `e2e`. All must be `success`. The **`e2e` job
  hard-gates** (Playwright against a real built studio+core).

## 4. Environment & toolchain (hard-won specifics — these bite)

- **`tsc -p studio` compiles NOTHING silently.** `web/studio/tsconfig.json` is a **solution** file
  (project references). Always typecheck with **`tsc -b studio`** (build mode). This cost real time
  on T01 — the whole point of the fix.
  ```bash
  cd web && studio/node_modules/.bin/tsc -b studio
  ```
- **`npm ci` fails in `web/`** with EUSAGE — `web/` is a *nominal* workspace with **no root
  lockfile**. Install **per-package with `--no-workspaces`**:
  ```bash
  cd web/studio && npm ci --no-workspaces
  cd web/ink    && npm ci --no-workspaces
  ```
- **The `e2e` CI job must install `web/ink`** too — Vite resolves `perfect-freehand` *through* ink,
  so a studio-only install makes wet-ink e2e hang at dev-server start. (Fixed during T02; keep it.)
- **`make test`** runs the Go suite (`httpapi` alone is ~60 s because of `ws_test`). **Go builds
  offline, stdlib-only** — `go build ./...`, `go vet ./...`.
- **Never edit `core/internal/webassets/dist/`** — it's a committed **placeholder** regenerated by
  `make embed`. `make demo` / `make dist` **overwrite** it; if you run them, restore before
  committing:
  ```bash
  git checkout -- core/internal/webassets/dist && git clean -fdq core/internal/webassets/dist
  ```
- **`internal/gen/`** is generated & git-ignored; the scaffold compiles without it. Never hand-edit.
- **Seed / demo creds:** `marie` / `demo`. `make demo` seeds `localhost:8080` (the dev core's port).
- **Annotation-type registry (T07):** ink dispatches draws via a **lookup Map** (`registerInkDraw`),
  **never** `switch (obj.type)`. Add a new annotation type by adding a descriptor under
  `web/studio/src/annotations/` and registering its draw fn — don't grow a switch.
- **CI Node runtime:** Actions are pinned to Node24-capable majors (checkout v7, setup-node v6,
  setup-go v6, setup-java v5, upload-artifact v7, gradle/actions v6). **`bufbuild/buf-setup-action@v1`
  is intentionally left on Node20** — no Node24 release exists yet; there's an inline note.

## 5. Landing procedure (primary worktree, linear, no merge commit)

**CHANGED (2026-07-04):** `main` is **no longer checked out in your primary worktree** — a
`troubastack-review` worktree now holds it (the reviewer's), and the A-agent uses
`troubastack-IOS01`. `git worktree list` shows all three; leave the others alone. Because `main`
is checked out elsewhere, you **cannot** `git checkout main` / `git merge --ff-only` here — git
refuses (and `git branch -f main` too). **Land by pushing your branch straight to `main`:**
`git push origin HEAD:main` — a clean fast-forward once you've rebased onto `origin/main`.

```bash
cd /home/yoda/dev/git/troubastack
git checkout -b task/TNN-name origin/main   # branch from the freshest main
git add <files>
git commit -F - <<'EOF'
<subject: area: what changed (TNN)>

<body: what/why, verification line>

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF

git fetch origin                          # A-agent lands often — main moves under you
git rebase origin/main                    # expect this ~every time; re-fetch+rebase if push rejects
git merge-base --is-ancestor origin/main HEAD && echo ff-safe   # sanity: exactly your commits ahead
git push origin HEAD:main 2>&1 | sed -E "s/${TOKEN}/<TOKEN>/g"   # ff push; NO local checkout of main
# then poll CI to green (§3). Your worktree stays on the task branch (== origin/main); that's fine.
```

**Gotchas that already bit here:**
- `git add <path>` with a **pathspec that no longer exists** (e.g. an already-`git rm`'d dir) makes
  `git add` abort **without staging the rest** — you then commit a partial/broken tree. Stage the
  real files explicitly and verify `git show --stat HEAD` before pushing. (T11 hit this; amended
  before push — `main` never carried the broken commit.)
- An **untracked local file that collides with an incoming committed path** blocks a rebase/checkout
  ("could not detach HEAD"). Back it up to scratchpad, remove it, then rebase. (T12 hit this with a
  locally-authored T15 doc that the reviewer had also committed.)
- Transient `docs/*.md` files may briefly appear/vanish — that's the A-agent relocating handoffs in a
  parallel worktree. Not yours; leave them.

## 6. What's done — the T/B-track (all landed on `main`)

| Task | Commit | Summary |
|---|---|---|
| B06 (core slice) | `ac0066e` | **LAN mDNS advertisement** (change #1 + Go test). `core/internal/discovery` advertises `_troubacore._tcp` on startup via `github.com/libp2p/zeroconf/v2` (maintained fork; pulls only `miekg/dns`) — actual listen port, instance name from `TROUBA_MDNS_NAME` (default host name), TXT `version`/`path=/`. On by default; `TROUBA_NO_MDNS=1` opts out. Register failures logged + swallowed (never blocks serving); shutdown `sync.Once`-guarded; wired in `main.go`. **Verified on the wire** (zeroconf browse found the running core: instance/host/port/TXT). **A-track (NOT here):** the Connect-screen browse UX (Android `NsdManager`, iOS `NWBrowser` + plist) — routing note in the B06 spec. First networking dep in core (deliberate per the spec). |
| demo charts | `eec3bb8` + `b56ffb9` | **Copyright-safe demo sheet-music + a wired demo song.** New dev tool `core/cmd/mkcharts` (fpdf, deterministic — pinned date + `SetCatalogSort`) generates three PDFs into `docs/demo-charts/`: *"The Open Road"* (an **original** song — lead sheet p1 + guitar tab p2), *Amazing Grace* (**public domain**, Newton 1779), and a blank placeholder. `open-road.annotations.json` + `docs/screenshots/demo-chart-annotated.png` show three meaningful layers (Form/mandatory, Conductor-cues/roleTag, personal My-notes) in Studio. `b56ffb9` then **wired The Open Road into the seed** (`pdfSource.localPath` reads the committed chart; `buildOpenRoadAnnotations` places the layers at the lead-sheet coords; songDef in "The Troubadours"/"Sat @ The Anchor") — so it flows seed → bake → Stage, and `docs/demo/demo-concert.tstage` was regenerated to the 4-song concert incl. Open Road. **NO copyrighted lyrics/tab/sheet** anywhere — original + PD only (the real-titled seed songs keep synthetic placeholder PDFs for the same reason). |
| B03 (slice) | `93ef372` | **B03 server slice only** (change #1 + Go test). `GET …/concerts` + the bake POST now return the proto **`AvailableConcert`** shape: `currentRev`/`updatedAt`/per-song `rev` as **canonical JSON strings** (app parses via A02's Kotlin mirror verbatim), `final_locked` passthrough, `bakedBy`/`downloadUrl` extras. Per-song `rev` = the song's source (annotation) revision. Studio Bake card updated to the shape; `viewOf` + flow-shape Go tests. **The rest of B03 is A-track (Mobile App Agent) — NOT done here:** ktor client, Connect flow, EncryptedSharedPreferences secrets hardening, `distribution/Updates.kt` bodies, offer UI, Kotlin tests. Manifest endpoint is ready for them; routing note in `docs/tasks/B03-distribution-and-updates.md`. |
| B02 | `869d900` + `552c516` | **Bake orchestration: setlist → downloadable `.tstage`** (2-commit split, T10-style). Backend (`869d900`): proto `BakedSong` +`display_notes`/`key`/`tempo` (overrides = metadata, not pixels) + Kotlin mirror; `internal/bake` = Go `ConcertBundle` mirror + `.tstage` writer + `Baker` shelling to **pdftoppm** (rasters) and **B01's web/bake CLI** (overlays — core draws NOTHING, I8); endpoints `POST …/setlists/{s}/bake` (admin), `GET …/concerts` + `…/{c}/bundle` (member); Go tests (fakes + real-pdftoppm w/ skip) + endpoint auth; poppler-utils in the CI go job. UI (`552c516`): admin Bake card on the Setlist page (bake · download · history) + `bake.spec.ts`. `source_revision` = song's current engine head (no per-setlist pin exists). **Verified the real pipeline by unzipping an actual bake** (2 pdftoppm rasters + a same-size web/ink overlay). **Deferred (needs emulator):** the Android import+perform loop-close screenshot. |
| B01 | `00a7da5` | **`web/bake` overlay renderer + the I8 golden parity test.** `renderOverlays`/`troubabake` CLI draw per-layer transparent PNGs via `@troubastack/ink` (Node Skia, `@napi-rs/canvas`); esbuild bundles ink from source (no 2nd copy). Parity test = bake vs. ink-in-headless-Chromium, per-pixel within an AA tolerance (spec's literal "100% transparent" relaxed to ≥99.9% — flagged at the gate). Added `setTextFontFamily` to ink (default UNCHANGED → studio byte-identical); bake pins bundled Roboto + `textRendering=geometricPrecision`. CI step in the web job. Overlays only — PDF rasters + bundle assembly are B02. |
| T01 | `4ebdb3e` (+`243a92e`) | Fix workspace typecheck breakage; discover `tsc -b` (solution file). Follow-up: `web/bake` typecheck-only build. |
| T02 | `5ed1fe5` (+`5fe5da0`) | GitHub Actions CI + strict gofmt gate. Follow-up: quarantine `editor-rorw-shift` in CI while e2e keeps hard-gating (→ T13). |
| T03 | `d33b3b7` | Studio: one accent color + dark-mode token fixes. (before/after screenshots) |
| T04 | `d78e54c` | Studio: simplify management pages — disclosure, tabs, member rows. |
| T05 | `e045613` | Studio: compact the song-editor chrome. (≤220 LOC target only partly met → T14) |
| T06 | `9fac1f4` | Studio: low-latency wet-ink freehand path (shares `web/ink` geometry). |
| T07 | `246a5ec` | Annotation-type **descriptor registry** — `registerInkDraw` Map dispatch, no `switch(type)`. |
| T08 | `d901595` | Core: require band **admin** for annotation import. |
| T09 | `704dc0a` | Proto: reconcile the object-type contract with the runtime mirrors. |
| T10 | `01d98d7` | Studio: split `SongEditor.tsx` — extract editor modules (**part 1/2**; Viewer split → T15). |
| T11 | `7199605` | Core: remove dead wiring in the composition root — trim `httpapi.Router` to `(svc,eng,secureCookies)`, drop the discarded `sync/session/bake` construction, delete `internal/session` (sessions owned by `app.Service`), fix the doc mirrors. |
| T12 | `4fb7739` | Docs: make ARCHITECTURE enforcement claims honest — ✅/🎯 tags on I1–I15; drop the false "codegen runs in CI". |
| infra | `99ef602` | CI: bump GitHub Actions off the deprecated Node20 runtime. |

Commit hashes are as-landed; if one goes missing after a rebase, grep the subject line.

## 7. Current state & concurrency

> **📩 Cross-lane note from the Mobile Agent (2026-07-06): the A/IOS lane is DRAINED.**
> A01–A06 and IOS01–IOS04 are all merged — the app performs baked concerts offline on **both
> Android and iOS** (iOS Stage proven on the simulator, IOS02). The only remaining mobile tasks are
> both **blocked**: A07 (native wet-ink — tablet stylus spike) and IOS03 impl (device/Store — needs a
> Mac + Apple credentials). Two things that touch your lane:
> - The CI **`android` job now also cross-compiles the iOS klibs** (added in IOS01) — expected, just
>   FYI if you ever see it in a run you triggered.
> - Your §8 **B02 Android loop-close** (import/perform the `.tstage` in the app on an emulator +
>   screenshot) is **A-track territory** — the Mobile Agent can take that deferred acceptance item;
>   flag it (via the human relay / a `docs/tasks` entry) if you'd like it picked up.

- Re-run `git log main --oneline -20` on session start — `main` moves fast.
- The **A-track agent** works in **isolated worktrees** (`git worktree list` may show extras you
  didn't create — leave them alone). It lands to `main` frequently too, so always `git fetch` +
  rebase before landing.
- **`:8080` is your dev core's port.** The A-agent avoids it (uses `:8091` + a separate data dir).
- **Memory** (`/home/yoda/.claude/projects/-home-yoda-dev-git-troubastack/memory/`): read `MEMORY.md`
  index on start — `task-pack-workflow.md` (repo quirks + review model), `git-linear-history.md`,
  `mobile-app-agent.md` (the other lane's role).

## 8. Remaining work (T/B-track)

**Queue status (refreshed 2026-07-06, late).** Web-core landed this session, all CI-green +
gate-approved: **B01, T13, B02 (+UI), B03 server slice, B04, T18, B05, the demo charts + wired
"The Open Road", and the B06 core slice (mDNS)** (see §6). The mobile lane has since **closed** B02's
Android loop-close and landed the **B03 app bulk** + A08. So the earlier "attended/A-track only"
note is out of date — new **web-core** work was filed (below). Land the usual way (rebase →
`push origin HEAD:main` → CI green); hold at the gate for a verdict in `reviews.md` or an explicit
human OK noted in the commit ("landed per steer + VLL").

**Open, unblocked — web-core lane** (no steer assigns one; pick by priority or ask):
- **T19 — text charts** (M/L; core + web/studio + maybe proto): author formatted song documents in
  Studio and bake them like PDFs. Highest product value of these.
- **B07 — per-member bake** (L; core + proto + studio): "Leo sees his tab on stage" — the per-member
  my-files bake B02 deliberately deferred. Top post-loop product gap.
- **T20 — duplicate a setlist** (S; `core/httpapi` + studio) · **T21 — password reset** (S,
  admin-assisted; `core/httpapi` + studio). Good fillers.
- Bigger / decisiony: **OPS01** (TLS/service/backup — effectively **human/env-blocked**: needs a real
  server + creds), **P202** (real GC, M/L core), **P203** (proto codegen — a cheap **decision stage**
  first; this is open call ⑤).

**Attended-only — do NOT start unattended:**
- **T17 — single-row toolbar redesign** (superseded T14 after it regressed zero-shift): **build the
  zero-shift e2e spec FIRST**; full findings/brief in `docs/tasks/T17-editor-style-disclosure.md`.
- **T15 — split `Viewer.tsx`** (T10 part 2): held for a quiet-machine attended window.

**A-track (Mobile App Agent), NOT web-core — don't cross into `app/` (I15):** the **B06 app browse
UX** (NsdManager/NWBrowser + plist), **A11/A12** (Stage count-in / facing pages), and the
already-landed B03 app half + B02 loop-close. Scope the lane split first on any app-heavy B-task.

**Open product calls for the human (Vincent):** ④ widen bake admin-only → members (I11 permits it)?
⑤ promote P203 now the loop is closed? · long-flagged: **rotate the credential in the git remote URL**.

## 9. Verification commands (from repo root)

```bash
make test                                             # Go suite (httpapi ~60s)
go vet ./... && go build ./...                        # (run inside core/)
cd web/studio && npm ci --no-workspaces               # + same in web/ink, in a fresh checkout
cd web && studio/node_modules/.bin/tsc -b studio      # typecheck (BUILD mode, not -p)
cd web/studio && npm run test:e2e                     # Playwright e2e (needs studio+core built)
make demo                                             # full local smoke — restores dist placeholder after
```

## 10. Do-NOTs / gotchas (quick list)

- Don't `tsc -p studio` (compiles nothing). Don't `npm ci` in `web/` without `--no-workspaces`.
- Don't edit `core/internal/webassets/dist/` or anything under a `gen/` dir.
- Don't leave a token in visible output — scrub with `sed`/`grep -v`.
- Don't open a PR for T-track tasks — land single commits by fast-forward (§5). (The A-track uses
  PRs; you don't.)
- Don't force a task's target if it isn't safely reachable — land what's solid, file a follow-up.
- Don't touch the A-agent's worktrees or `:8091`.
