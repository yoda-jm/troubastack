# TroubaStack — Full Project Audit

**Date:** 2026-08-25 · **Audited version:** `origin/main` @ `088cf4f` (2026-08-25, "T105 landing verified").
**Method:** four parallel deep-dive passes (Go core, web, mobile app, docs/CI/deploy) over the checked-out tree (`f27b755`), then every finding re-verified against `origin/main` — 266 commits newer. Line numbers cite the deep-dive tree; each was confirmed still present (or marked fixed) on main.

> **What the 266-commit delta changed:** bake is now asynchronous with per-song progress (T96/T97/T98/T99/T103) and its errors are sanitised at a single choke point (T102); studio's blocking `window.prompt`/`confirm` dialogs were replaced by an in-app dialog (T90/T91/T101); the chart editor was extracted from `SongDetails` onto its own route (T104/T105); all 39 e2e `waitForTimeout` sleeps were removed (T93); a third shared cross-language contract landed (metre vectors, T92); the app gained the visual beat, reading color schemes, download timeout/progress, one-tap update/re-bake, an honest Home landing, and an account chip (A34–A45). **Every critical finding below was re-checked and remains open on main.**

**Sizing legend used throughout:** **XS** ≤ ½ day · **S** ~1 day · **M** 2–5 days · **L** 1–3 weeks · **XL** 1+ month.

**📋 Filed tags (added 2026-08-25, after the audit was written).** A finding tagged 📋 has a spec in
`docs/tasks/` and is queued; untagged findings are recorded but **not** routed to a lane — which is
exactly the gap §7 describes. Tags follow the repo's ✅/🎯 honesty convention rather than colour, so the
audit stays greppable and diffable. First pack: **T106–T113**, filed from VLL's own ordering.

---

## 1. What it is

**TroubaShare**: collaborative sheet-music & lyrics annotation for bands, self-hosted. One product, three layers, one contract:

| Layer | Name | What it does |
|---|---|---|
| Server | **TroubaCore** | Single Go binary. Authoritative truth (LWW engine, append-only song history), WebSocket realtime sync, bakes setlists into offline `.tstage` bundles, serves the embedded SPA. |
| Web | **TroubaStudio** | Canvas-first React/Vite editor. Realtime multi-user annotation over PDF/text charts; `web/ink` is *the* single stroke renderer, reused server-side by the `web/bake` Node worker (pixel-parity tested). |
| Mobile | **TroubaStage** | Kotlin Multiplatform / Compose presenter (Android + iOS-simulator). Offline, "never crash mid-performance": pedal page turns, night mode, count-in, facing pages, per-role layers. |

The contract lives in `proto/` (protobuf as a *schema language*, JSON on the wire) and is mirrored into Go/TS/Kotlin by a bespoke generator (`core/cmd/gen-mirrors`), with CI drift guards on every generated artifact.

**Scale (on main):** ~33k LOC Go, ~31k LOC TS/TSX/CSS (studio + ink + bake + 81 e2e specs), ~11k LOC Kotlin, ~300 doc files. Dependencies are deliberately thin (no web framework, no ORM, no state library, pure-Go static binary).

**How it's built:** `make demo` → Vite build → SPA copied into `core/internal/webassets/dist/` → `//go:embed` → one binary on :8080. Dev mode runs core + Vite HMR side by side. Mobile builds via Gradle (JDK 21 pinned); iOS via XcodeGen linking the shared framework. Deploy is a 3-stage Dockerfile + compose + Caddy (auto-TLS).

---

## 2. How it's organized

```
proto/     the contract (5 .proto files; buf lint + breaking in CI; codegen via gen-mirrors, not buf)
core/      Go server: cmd/{troubacore,seed,mkbundle,mkcharts,gen-mirrors}, internal/{domain,store,engine,sync,app,bake,httpapi,config,…}
web/       ink (renderer lib) · studio (SPA) · bake (Node overlay worker) — npm *without* workspaces, ink aliased from source
app/       KMP: shared (presenter, 3.4k LOC common) · androidApp · iosApp (XcodeGen, not a Gradle module)
docs/      ARCHITECTURE.md (numbered invariants I1–I15) · design/ (10 docs) · adr/ (4) · tasks/ (157 agent-executable specs)
           handoff/ (multi-agent review-gate protocol + 963 KB verdict log) · contracts/ (shared test vectors) · USER-JOURNEY.md · risks.md
deploy/    docker-compose + Caddyfile + backup.sh + ops README
```

Governance is unusual and real: `ARCHITECTURE.md` is normative ("if code and an invariant disagree, the code is wrong"), each invariant tagged **✅ enforced today** vs **🎯 target**, and `docs/tasks/` is the named queue that closes the 🎯 gaps. Work flows spec → execute → review gate (independent re-verification) → linear fast-forward landing.

---

## 3. What's good

### Architecture & contracts ★ the standout
- **The invariant system is auditable, not aspirational.** ✅/🎯 honesty tags, enforcement mechanism named per rule, and a documented amendment history (I8's "pixel-identical" wording corrected in place with provenance).
- **Four flavors of contract teeth in CI:** regenerate-and-diff on 5 generated mirrors across 3 languages; glyph geometry drift guard (`glyphs.json` ↔ `CueGlyphData.kt`); shared hand-derived test vectors executed across languages (`view-resolution.vectors.json`, `beat-phase.vectors.json`, and since T92 the metre-parser vectors — with the derivation and the original bug written into the file); a golden **pixel-parity** test between the browser renderer and the server-side bake. Plus `buf breaking` wired correctly for a push-based repo.
- **Layering is held in code.** Every Go package states its import boundary in its `doc.go` and honors it (`sync` never imports `app`; `livebake` injects a closure to avoid a cycle). Studio's single-renderer invariant verified: zero stroke-geometry duplication outside the one dev-only demo type.
- **Elegant config design:** one ordered knobs table drives loading *and* `--print-default-config`, with a byte-equality test pinning the committed example INI.

### Testing (where it exists)
- Go: 61 test files vs 59 source files, 238 tests; a **backend conformance suite** run against mem/file/git stores; the full WS authorization matrix tested; deterministic race-window test seams in the baker.
- App: 169 JVM tests with systematic pure-function extraction (page-turn math, facing pages, LRU pinning, zip parsing, update policy); torture fixtures for the never-crash contract.
- Web: 81 Playwright specs (1174+ assertions), many pinned to specific past bugs (canvas memory blackout, insecure-context UUIDs, touch stuck-nav); 346+ stable `data-testid`s; zero skips; all timing sleeps removed as of T93.

### Security wins (the ones that are present are genuinely good)
- Textbook SSRF guard on lyrics import (dial-time resolver check defeats DNS rebinding); zip-bomb + zip-slip defenses on both import paths (incl. negative-size overflow); bcrypt with dummy-compare timing flattening; hashed single-use reset tokens that kill all sessions; content sniffing on uploads; origin-bound app session cookies with WebView cookie-jar clearing; Docker runs non-root; no secrets committed anywhere.

### Product & UX care
- Failure-as-value everywhere in the presenter: corrupt bundles degrade to per-page issues, never crash; missing layers get a badge, not a blank stage.
- Studio a11y/mobile: 120 aria attributes, `prefers-reduced-motion`, pinch-zoom preserved outside the editor route (with the WCAG reasoning inline), first-class Pointer Events touch pipeline, "no silent ink" when the socket is down.
- Effectively **zero `any`** in 15k lines of strict TS; **3 TODO markers** in 30k lines of Go — both remarkable.

### Process & docs
- 157 task specs with runnable acceptance criteria and "Out of scope" sections; a landed-task **archive mapping each task to its implementing commit** (added on main); **negative results recorded** (tasks closed-without-landing, decisions "closed-not-adopted"); a ~600-verdict review log with independent re-verification evidence ("never approve from the report alone" — and it's honored).
- Honesty is institutionalized: USER-JOURNEY's classified gap register, README's "Packaging status (honest)", the deploy README volunteering that registration is open.
- CI/Dockerfile/deploy files read as teaching documents — nearly every non-obvious line explains the failure it prevents.
- Copyright hygiene fits a music product: per-file provenance for demo charts, `bands/` gitignored so real repertoire can't be committed, CC attribution pre-committed in the video plan.

---

## 4. What's bad

### 4.1 Critical — all eight re-verified still present on `origin/main`

| # | Finding | Where |
|---|---|---|
| C1 📋 **T113** | **No LICENSE file** — `NOTICE` references one that doesn't exist; the code is legally "all rights reserved" while the repo is public with badges. | repo root |
| C2 ⚠️ *VLL only* | **Credential embedded in the `origin` remote URL**, with one documented leak into tool output. Self-flagged for weeks, still unrotated. | git config; `docs/handoff/mobile-app-agent.md` |
| C3 📋 **T107** *(file-mode half)* | **Sessions never expire and are stored in plaintext**, in a world-readable `app.json` (`0o644`) that also holds every bcrypt hash. Cookie's 30-day cap is client-side only. | `core/internal/app/app.go:137`, `service.go:141`, `filerepo.go:187` |
| C4 📋 **T106** — *see note below* | **Confirmed data race** on `conn.dropped` (written under lock, read without; guards a `close(chan)` → double-close panic path) — and **CI never runs `-race`**. | `core/internal/sync/sync.go:157` vs `conn.go:221`; `ci.yml` |
| C5 | **Page-image cache used concurrently despite documented single-thread invariant** — `decodeCached`/`pin` called from `Dispatchers.Default` in three places; two-up + prefetch can corrupt the LRU mid-gig. | `app/shared/.../stage/StageScreen.kt:309,887,947`, `LruCache.kt:9` |
| C6 *(deferred — LAN-only)* | **No rate limiting anywhere**, open registration with no off switch, and `minPasswordLen = 1` (not even enforced on register). | `core/internal/app/service.go:161` |
| C7 *(deferred — LAN-only)* | **Cross-site WebSocket hijacking surface:** `CheckOrigin` returns `true` unconditionally; no CSRF tokens; zero security headers (no CSP/nosniff/HSTS); `image/svg+xml` accepted + served `inline` → stored-XSS path on the cookie's origin. | `sync/conn.go:138`, `webapi.go:169,919`, `service.go:1024–1056` |
| C8 📋 **T111** | **The production Docker image is never built by CI** — the least-tested artifact in an otherwise five-way-gated repo. | `ci.yml`, `Dockerfile` |

> **📋 Correction to C4 (added 2026-08-25, on filing T106).** Re-reading main before writing the spec:
> `readPump`'s teardown calls `c.hub.unregister(c)` *before* reading `c.dropped`, and `unregister`
> (`sync.go:121`) acquires the same `r.mu` that `broadcast` holds when it writes the flag — and deletes
> the conn from `r.conns` there. That is a happens-before edge, so the write is either visible to the
> read or unreachable. `sendTo` already guards with `recover()`. **The audit overstates this as a
> confirmed race.** What remains true, and is what T106 fixes: the safety is *emergent* — it depends on
> three separate facts holding together, is stated nowhere at the read site, and is pinned by no test.
> T106 installs `-race` as the arbiter first and lets the detector, not an argument, decide.
>
> **⚠️ C2 is VLL's to act on** — no lane can rotate his credential.
> **C6/C7 deferred:** the instance is LAN-only for now (VLL, 2026-08-25). They re-enter the queue the
> day it is exposed, and that is the trigger to write down, not a date.

### 4.2 High

**Performance / scalability (core):**
- Default `filestore` is **O(n²) per song**: every accepted stroke re-parses the entire song history twice (`filestore.go:134–167`). `gitstore` rewrites the whole file per mutation and never repacks. `sync/apply.go:274` deep-clones the **entire song HEAD per stroke** to look up one version.
- Unbounded in-memory growth (every touched song's HEAD lives in RAM forever); `filerepo` rewrites the whole relational dataset on every mutation — hence three CLI tools that require the server stopped.
- **No graceful shutdown, no Write/Idle/Read timeouts** — SIGTERM kills mid-write; a slow client holds a connection forever (`cmd/troubacore/main.go:104–134`).

**Data loss / stage-critical (app + studio):**
- `SyncClient.sendMutation` **silently drops writes** when the socket isn't open — no outbound queue; T30's read-only mode is a UI mitigation, not a data guarantee (`web/studio/src/sync.ts`).
- **Pedal focus is requested exactly once** — after any dialog/sheet takes focus, a Bluetooth pedal silently stops turning pages. The exact failure this product cannot have; untested because there are zero Compose UI tests (`StageScreen.kt:262`).
- **No `rememberSaveable`/persisted position:** process death mid-gig resumes the concert at page 0.
- Bundle I/O, download, and 512 MB unzip run **on the Android main thread** (`MainActivity.kt:136,228,262–351`).
- Cache budget is entry-count (64), not bytes — ≈1 GB potential on a tablet; iOS decodes **full-resolution with no downsampling at all**.

**Error handling:**
- 500s leak raw internal errors (paths, git internals) to clients — `writeErr` still returns `err.Error()` verbatim on the generic 500 branch on main (`webapi.go:1173`). The bake channel specifically was fixed by T102 (errors sanitised at one choke point, raw stderr logged server-side only); the general edge remains.
- `auth.tsx` collapses network failure into "logged out" — an offline blip evicts a valid session to `/login`.
- App-side `catch (e: Exception)` swallows causes with no logging in the loader/unpack/update paths — field diagnosis of a bad bundle is impossible.

### 4.3 Medium

**Structure — god files (line counts on main):** `core/internal/app/service.go` (2233 lines, ~110 methods), `webapi.go` (1188), `cmd/seed/main.go` (1494); `Viewer.tsx` (1581 — still growing, 18 useState/10 useEffect), `WetCanvas.tsx` (1091, a 25-prop component), `styles.css` (1747); `StageScreen.kt` (1248, with a ~380-line composable). One genuine improvement: T104/T105 extracted the chart editor out of `SongDetails.tsx` (1039 → 822, with `ChartEditor.tsx` at 400 on its own route).

**Dead / stub code shipping:**
- A large, contract-tested **history/revert/pins/GC subsystem has zero production callers** — no route, no CLI. Built, tested, unreachable.
- `pgstore` is selectable (`store = pg`) but every method returns `ErrUnimplemented` — the server starts happily broken and fails on first write.
- App `sync/SyncClient` is 3 `TODO()`-throwing stubs in `commonMain`; the InkOverlay seam is `TODO()` on **both** platforms; the promised golden ink-parity test for native is "not yet written".

**Testing gaps (the asymmetry):**
- 📋 **T110** — **Studio and ink have zero unit tests** — 713 lines of hit-testing geometry and the single authoritative renderer are validated only through a long serial browser suite (`workers:1`, `retries:0` — one flake reds the push; ~20 min on CI now). The 39 `waitForTimeout` sleeps flagged in the deep-dive were removed on main (T93 de-flake) — the flake vector is gone, the single-point-of-failure config remains.
- 📋 **T108** — e2e copy-paste debt got *worse* on main: 77 of 81 specs define their own `register()` helper; a registration-flow change is a 77-file edit.
- 📋 **T109** — `internal/sync` (the WS hub) has **zero in-package tests**; `:androidApp` has no test source set (pure functions like `sessionCookieFor`, `safeSegment` untested); iOS tests compile but never execute; **no visual regression** anywhere; no coverage measurement.

**iOS parity:** an iOS user has **no way to get a bundle onto the device** (no import, no Connect/login/download); no Home IA; identity re-prompts every entry; `ConcertEntry`/`listConcerts` duplicated between Android and iOS entry points with divergent shapes.

**Frontend delivery:** 📋 **T112** — one **755 KB JS chunk + 1.34 MB pdf.js worker** downloaded by every visitor including `/login` — still zero code splitting on main (no `React.lazy`, no manualChunks) on a product meant to run off a garage laptop over Wi-Fi. Text annotation entry moved from `window.prompt()` to a proper in-app dialog on main (T90/T91/T101 — a real fix), but the dialog uses a single-line `<input>`, so the latent multi-line divergence remains: `textBBox` counts `\n` lines, ink's `drawText` still renders one `fillText` line. Editor is pointer-only — no arrow-nudge/Delete/Escape keyboard path.

**Tooling / release:** no ESLint/Prettier/biome, no ktlint/detekt (only Go gets format gating); no Dependabot/Renovate, no CodeQL/secret scanning; buf unpinned in CI; no `concurrency: cancel-in-progress`, no `timeout-minutes` on 4 of 5 jobs; Playwright browsers re-downloaded every run. **No tags, no releases, no changelog, no registry image, no signed APK** — `git describe` degrades to a bare SHA.

**Observability:** 38 unstructured `log.` calls, no request IDs, no `/metrics`, no pprof; the entire ops surface is `/healthz`. No backup scheduling, no restore drill, no upgrade/rollback runbook, no uptime alerting — for a product whose stated failure mode is "the garage laptop died".

**Hygiene & doc drift:** two git worktrees nested *inside* the repo (`gate-push/` — 88 commits ahead, `t93-wt/`) that are not gitignored and would be destroyed by `git clean -fd`; stray `a34f2/` (empty) and `core/gvo-8080.log`; an uncommitted `.gitignore` edit guarding real band data; `reviews.md` at 963 KB is the largest text file in the repo; `proto/README.md` still describes the buf-codegen mechanism that P203 explicitly closed; `risks.md` R4 keeps wording ARCHITECTURE amended; USER-JOURNEY lists P201/P205-S3 as open while README says landed; WebView leaks per Edit entry and exposes its JS bridge to any user-typed URL; `androidx.security-crypto` pinned to an alpha; `CORE_URL` default declared three times.

---

## 5. Improvements (priced)

### Security & correctness — do these first

| Improvement | Size | Notes |
|---|---|---|
| Add `LICENSE`, fix the NOTICE reference | **XS** | Ten minutes; unblocks everything external. |
| Rotate the origin credential; switch to SSH/credential helper; add a gitleaks CI job | **XS** | Known leak already on record. |
| Add `-race` to `make test` + CI, then fix `conn.dropped` (atomic or lock-scoped close) | **S** | Highest value/effort ratio in the whole audit. |
| Session TTL + hashed tokens + sweep; invalidate other sessions on password change | **S** | Mirror the discipline the reset path already has. |
| File modes `0o600` for `app.json` / blobs / stores | **XS** | |
| Real `CheckOrigin` + security-headers middleware (CSP, nosniff, X-Frame-Options, HSTS); drop SVG from upload types; `attachment` disposition | **S** | |
| Login/register rate limiting (in-memory token bucket) + real `minPasswordLen` + an "invite-only / registration off" config knob | **M** | Closes the open-registration gap the deploy README apologizes for. |
| Stop leaking 500 error strings; log server-side, generic to client | **XS** | |
| Fix the app page-cache concurrency (Mutex or single-confinement) + a hammer test; update the now-false comments | **S** | Stage-critical. |
| Re-request pedal key focus when overlays close (`LaunchedEffect(overlayOpen)`) | **XS** | Highest severity/effort ratio in the app. |
| Graceful shutdown + full `http.Server` timeouts; wire autobaker to the signal context | **S** | |
| Harden the WebView host: `DisposableEffect { destroy() }`, attach the JS bridge only on the configured origin | **S** | |

### Performance & robustness

| Improvement | Size | Notes |
|---|---|---|
| Fix `filestore` O(n²) (cache last-seen seq per song); append-don't-rewrite in `gitstore` | **M** | Guarded by the existing conformance suite. |
| Add `Engine.ObjectVersion()` — kill the full-HEAD clone per stroke | **S** | |
| Route-level code splitting (`React.lazy` the editor; vendor chunk) | **S** | Biggest single frontend win: login page drops from ~2.1 MB to a fraction. |
| Byte-budgeted page cache + Android `sampleSize` fix + iOS Skia subsampling + RGB_565 rasters | **M** | |
| Move all app bundle I/O off the main thread (`Dispatchers.IO` + loading states) | **S** | |
| Outbound mutation queue in studio's `SyncClient` (buffer while closed, replay after snapshot) | **M** | Turns T30's UI mitigation into a data guarantee. |
| Bake concurrency semaphore + deadline | **S** | T103 rightly detached bakes from the request (202 + poll, server context) — but nothing now bounds how many bake goroutines run or for how long. |
| Distinguish network-failure from 401 in `auth.tsx` | **XS** | |

### Testing

| Improvement | Size | Notes |
|---|---|---|
| Vitest for studio + ink (geometry, strokeWidth, DPR budget, SyncClient vs fake WS); move the beat-vector test out of e2e | **M** | Millisecond feedback for code that currently costs 30 s/assertion. |
| Shared e2e helper module (`register`, `createBandAndOpen`, API-based setup); then `retries: 1` + a smoke/full split | **M** | 77 of 81 specs duplicate setup; API-driven setup cuts suite runtime and the single-point-of-failure config. (The 39 sleeps were already removed by T93.) |
| Table-driven unit tests for `sync.authorizeWrite` (the 100-line policy matrix) | **S** | |
| Compose UI tests for Stage (pedal-after-sheet, blocked-turn cue, two-up, badge) + an `:androidApp` test source set | **M** | |
| Visual regression via `toHaveScreenshot` on the ~20 views already screenshotted for debug | **M** | Reuse the bake parity tolerance philosophy. |
| Make iOS CI screenshots real assertions (golden compare or non-blank check) | **S** | |

### DX, CI & release engineering

| Improvement | Size | Notes |
|---|---|---|
| ESLint (react-hooks, jsx-a11y) + Prettier + ktlint/detekt, wired into CI | **S** | The obvious asymmetry vs Go's gates. |
| Build-only `docker build` job + compose config validation in CI | **S** | Closes the largest verification gap. |
| CI polish: `concurrency: cancel-in-progress`, `timeout-minutes` everywhere, cache Playwright, pin buf | **XS** | |
| Dependabot/Renovate + CodeQL + secret scanning | **S** | |
| Tags + CHANGELOG (generate from the task queue-state, which is already 90% of one) + GitHub Releases with the binary + registry image + signed APK | **M** | Gives `git describe` something to stamp; needed before any app/bundle compat matrix. |
| `CONTRIBUTING.md` + `SECURITY.md` (point at the existing ground rules; reconcile "one task = one PR" with the actual push-landing model) | **XS** | |

### Structure & hygiene

| Improvement | Size | Notes |
|---|---|---|
| Split `service.go` / `webapi.go` along their existing section comments | **M** | Mechanical, no behavior change. |
| Editor context/reducer to kill the 25/23-prop interfaces; further split `Viewer.tsx` / `SongDetails.tsx`; split `styles.css` at its banner comments | **M** | Also unlocks memoization of the canvas. |
| Split `StageScreen.kt` (chrome state / navigation / pager); merge `PageView`+`ScrollPage` | **M** | |
| Multi-line text annotations: swap the T91 dialog's `<input>` for a textarea and make ink's `drawText` `\n`-aware; call `getTextFontFamily()` instead of the hardcoded copy | **S** | The prompt→dialog half already landed (T91); this closes the latent bbox-vs-render divergence it left behind. |
| Make `store = pg` fail at startup, not first write | **XS** | |
| Decide the dead history/revert surface: expose it or gate it (see feature F1) | — | Product call. |
| Structured logging (`slog` + request IDs) | **S** | |
| Move worktrees out of the repo, delete strays, commit the pending `.gitignore` line; defensive ignore patterns | **XS** | |
| Fix doc drift (proto README, risks R4, USER-JOURNEY status); README for `docs/contracts/`; rotate `reviews.md` into monthly archives | **S** | |
| Hoist `CORE_URL` constants and `ConcertEntry`/`listConcerts` into `commonMain` | **S** | Also a prerequisite for iOS parity. |
| Promote I14/I15 from 🎯 to ✅ with cheap lints (import-graph check, seam file-list assertion) | **S** | The repo's own philosophy says these should exist. |

---

## 6. New features (priced)

Grounded in the gap register, dead code, and what the product story implies — roughly in the order I'd do them:

| # | Feature | Size | Why / what it builds on |
|---|---|---|---|
| F1 | **History & revert UI** — expose the already-built, contract-tested revisions/revert/pins ladder as routes + a Studio timeline panel ("what did Leo change last night?") | **L** | The backend is done and dead (§4.3); this is mostly HTTP + UI. Turns I4's append-only history from plumbing into a headline feature. |
| F2 | **Instance administration** — bootstrap admin, registration modes (open/invite-only/closed), user list, per-instance settings page | **M** | Deploy README currently apologizes for the absence; pairs with the rate-limiting work. |
| F3 | **Offline-resilient editing** — outbound queue (above) grown into a real outbox with reconnect reconciliation UX ("3 strokes pending") | **M** | Design doc 02 already specifies the outbox; the client half was never built. |
| F4 | **iOS parity: import + Connect + download + identity persistence** | **L** | The shared code is ready (`ManifestTransport` is an interface; Home IA is common); today an iOS user literally cannot load a concert. |
| F5 | **iOS on-device + App Store / F-Droid + published binaries** | **XL** | Signing, provisioning, store review, release pipeline — mostly process, but long. |
| F6 | **Observability & band-ops dashboard** — `/metrics` (bake durations, WS rooms, apply latency), a tiny admin status page, documented backup cron + uptime alert | **M** | The "garage laptop died" failure mode deserves a first-class answer. |
| F7 | **Presence** — who's viewing/editing which song, live cursors optional | **M** | Listed as accepted-later in USER-JOURNEY; the WS hub and room model already exist. |
| F8 | **Bulk upload** — multi-file drag-drop of a song folder / whole repertoire import | **S** | Accepted-later gap; API already handles per-file upload. |
| F9 | **Push/update notifications** — "new bake available" push to the app instead of poll-on-open | **L** | Needs a push transport decision (self-hosted constraint rules out plain FCM-only). |
| F10 | **Keyboard-first editor** — arrow-nudge, Delete, Escape, focus-visible selection model | **M** | Closes the pointer-only a11y gap; prerequisite groundwork for power users. |
| F11 | **Postgres backend** (`pgstore` + `pgrepo` + `WithTx`) — multi-band scale, concurrent CLI ops without stopping the server, fixes the no-transactions class of bugs | **XL** | ADR 0002 planned for it; only worth it when a deployment actually needs it. |
| F12 | **Stage rehearsal extras** — persisted bookmark (survive process death), partial-damage warning chip at soundcheck, per-song tempo tap | **S–M** | The bookmark and issues surfacing are half-built already. |
| F13 | **Native ink overlay (A07/I9)** — low-latency wet ink on the device, with the sanctioned golden parity test | **XL** | The one allowed duplication; high risk, high payoff — flagged as the riskiest subsystem by the design docs themselves. |

---

## 7. Bottom line

This is an unusually disciplined codebase for its size — the invariant governance, cross-language contract guards, and review-gate process are better than most professional teams manage, and the domain design (append-only history, LWW objects, bake-as-publisher, offline-dumb presenter) is coherent and honestly documented.

The weaknesses cluster in exactly the places the process didn't point its guns: **operational security** (sessions, rate limiting, headers — the threat model assumed a friendly LAN), **concurrency under load** (two confirmed race conditions, no `-race` in CI), **the unlit corners of the test pyramid** (frontend unit tests, Compose UI tests, the Docker image), and **release engineering** (no license, no versions, no published artifacts). None of these are architectural flaws; nearly all are S/M-sized fixes on a sound foundation.

The 266-commit delta between the deep-dive tree and main is itself evidence: it fixed real product-level issues (async bake, sanitised bake errors, in-app dialogs, e2e de-flaking) — and touched **none** of the critical security/correctness findings above. The task queue optimizes what users feel; nothing currently routes this class of issue into it. These items need to be filed as tasks explicitly or they will stay open.

If only five things get done: **LICENSE + credential rotation (XS)**, **`-race` + the two race fixes (S)**, **session TTL + rate limiting + headers (M)**, **pedal focus + cache fix in Stage (S)**, and **a CI docker build (S)**. That set retires every critical finding for roughly two weeks of work.
