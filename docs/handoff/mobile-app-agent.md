# Agent handoff — "Mobile App Agent" (TroubaStack A-track)

> **You are the Mobile App Agent.** Your lane is the **A-track** — the Kotlin/Compose Multiplatform
> mobile app in `app/` (plus the thin studio/core touchpoints an app task needs). A separate agent
> owns the **T-track** (web/core/infra); stay out of its way (see §2, §7).
>
> Point a fresh Claude Code session at this file to continue seamlessly. It captures **how we work**
> and **what's done**, not the code (read the code + `docs/` for that). Last updated 2026-07-17.
>
> **To resume:** open this repo in a new session and say — *"Read `docs/handoff/mobile-app-agent.md`;
> you are the Mobile App Agent — let's continue."* Then read §2 (how we work) and §5 (landing) before
> touching anything, and `git log main --oneline -15` for current state. **Immediate next action:**
> since 2026-07-11 a full device-QA + Stage-UX arc landed (all Fable-verified): the immersive chrome
> (A17/A2/Q2), the reference-app faithful look, **B1/A19** ("same page, fewer annotations" — failure-
> aware overlay decode + per-owner cache pins), the **N1/N2/N3 nav rework** (any-tap-toggles-chrome /
> dropped edge-tap-turn; continuous advance with a song-boundary cue; per-song scroll), **A1/A18**
> (per-song layer visibility, mandatory forced-visible at read), **A21** (a stale-swipe-closure fix —
> swipe now reads the current page via `rememberUpdatedState`), and **N4** (direction-aware page-turn
> slide). Portrait+landscape device-verified (two-up, rotation preserves position, text charts, cross-
> song cue). **In flight:** **N5** — VLL's "black navigation on black" chrome-contrast fix — is out to
> Fable (proposal Addendum 4; recommended: lighter translucent FAB disc + hairline outline); implement
> + gate + device screenshot pair once ruled. **Next queued:** **A20** (the app half of **T50** personal
> song cues) — ✅ **UNBLOCKED 2026-07-17** (web-core lane): T50 slice 1 (proto+core+bake) and slice 2
> (studio+shared glyph asset) are landed on `main`, all CI green — see the cross-lane handoff note under
> §8. The app section is `docs/tasks/T50-song-cues.md` §5, ready to lift. **Newer cross-lane unblock
> (2026-07-19, web-core): P205 Stage 3a** (app view-time identity + cues + defaults) is now the top
> unblocked item — the whole P205 web-core side (Stage 1 bake dialog + Stage 2 band-wide bake) **and**
> T57's shared **view-resolution vectors** (`core/internal/bake/testdata/view-resolution.vectors.json`,
> run them in commonTest → print == screen) are landed on `main`, all CI green. Full handoff under §8;
> spec `docs/tasks/P205-band-wide-bundle.md` Stage 3. Still user-blocked: **A07**
> (stylus spike), **IOS03 impl** (Mac + Apple creds), **B07** device screenshots, **OPS01** release-APK.
> ⚠️ **Rotate the git-remote PAT** — the embedded token echoes in tool output when the GitHub API is hit
> without `gh` (re-flagged in reviews.md; it leaked once this session via `curl -u` — use the
> `Authorization: Bearer`/`token` header, never `-u`).

---

## 1. What TroubaStack is

A monorepo for a band sheet-music app. Four parts, one contract:

- **`proto/`** — the single source of truth for every domain type + wire message (invariant **I1**).
  Clients carry **hand-written mirrors** until Kotlin/TS codegen is adopted (see T09).
- **`core/`** — Go server (TroubaCore): REST + realtime sync + serves the embedded SPA. `go 1.26`.
- **`web/studio/`** — the **canonical** React/Vite editor SPA (TroubaStudio). `web/ink` is the one
  stroke renderer; `web/bake` flattens pages server-side. Never reimplement the editor (**I10**).
- **`app/`** — Kotlin Multiplatform / Compose Multiplatform mobile app (**Android + iOS**, both real
  as of IOS01–IOS02). A thin shell (**I15**): all shared logic in `:shared` commonMain; platform code
  confined to the seam `actual`s (3 `expect` in commonMain + 3 Android + 3 iOS `actual`s — iOS
  `Storage`/`WebViewHost` are implemented, `InkOverlay` stays `TODO` behind A07) **plus** the thin
  `:androidApp` and the `:iosApp` **Xcode** entrypoint (xcodegen; links the `Shared` framework —
  it's not a Gradle module). The iOS Concerts/Stage entrypoint lives in `shared/iosMain`
  (`MainViewController.kt`), the analog of `:androidApp`'s `MainActivity`.

**Read first:** `docs/ARCHITECTURE.md` (numbered invariants I1–I15, normative) and
`docs/tasks/README.md` (the task pack + ground rules). `docs/design/01..08` are the design notes.

## 2. How we work (the operating model — this is the important part)

- **Task pack.** Work is pre-specified in `docs/tasks/{A,T}NN-*.md`, priority-ordered. **A-track** =
  the mobile app; **T-track** = web/core/infra. Each file is self-contained and executable.
- **One task = one branch = one PR**, branched from `main`, named `task/ANN-short-name`.
- **A concurrent agent runs the T-track** in the primary worktree. To avoid collisions, **do every
  task in an isolated git worktree**, not the shared checkout:
  ```
  git worktree add -b task/A0X-name ../troubastack-A0X main
  ```
  All edits/builds happen there. This has kept the two agents from ever stepping on each other.
- **Review gate.** After implementing + verifying, open a PR and **stop for review**. A reviewer
  (referred to as "Fable") re-verifies independently and says "approve — land it." **Do not
  self-merge before that.**
- **Linear history — no merge commits.** Land by fast-forwarding `main` to the branch commit.
  See the exact procedure in §5. (Memory: `git-linear-history.md`.)
- **CI must be green before landing.** GitHub Actions runs jobs: `go`, `web`, `proto`, `android`,
  `e2e`. `main` moves fast (the T-agent lands often), so **expect to rebase almost every time**.
- **Verify for real.** App tasks whose contract has runtime behavior get **emulator verification**
  (screenshots + logcat), not just build+tests. "Assemble-only" was acceptable for A01; A04/A05
  required driving the app; A06 required a running core.
- **Report honestly.** State what was verified vs. assumed; flag gaps and follow-ups rather than
  smuggling them. File follow-ups as their own PRs (e.g. the mkbundle color fix).

## 3. GitHub / PR mechanics

- `gh` is **not installed**. Use the GitHub REST API via `curl` with a token.
- **The `origin` remote URL has an embedded PAT.** NEVER print it. Extract into a var without
  echoing, e.g.:
  ```bash
  TOKEN=$(git remote get-url origin | sed -nE 's#https://([^@]+)@.*#\1#p')
  curl -s -H "Authorization: token $TOKEN" https://api.github.com/repos/yoda-jm/troubastack/...
  ```
  When pushing, pipe through `grep -v "ghp_"` so the token never lands in visible output.
- Open a PR: `POST /repos/yoda-jm/troubastack/pulls` with `{title,head,base:"main",body}`.
- Watch CI: `GET /repos/yoda-jm/troubastack/commits/<sha>/check-runs`.

## 4. Environment & toolchain (hard-won specifics)

- **JDK 25** is the only installed JDK (Temurin, at `/opt/openjdk-bin-*`). A **JDK 21** is also
  present (`/opt/openjdk-bin-21.0.7*`) and Gradle auto-detects it.
- **Gradle wrapper is pinned to 9.5.1** — NOT 9.6.x. AGP 8.13.2 uses a Gradle internal API removed
  in 9.6.0; the build fails on 9.6. 9.5.1 runs fine on JDK 25.
- **Compile toolchain is JDK 21**, not the installed 25: Kotlin 2.2.20 caps its JVM target at 24, so
  a JDK-25 toolchain trips AGP's Java/Kotlin target-consistency check. The Gradle daemon still runs
  on JDK 25. (`jvmToolchain(21)`; the catalog header documents it.)
- Versions (in `app/gradle/libs.versions.toml`): AGP 8.13.2, Kotlin 2.2.20, Compose MP 1.9.0,
  compileSdk 36, minSdk 26. `androidx.core-ktx` pinned to **1.15.0** (newer needs compileSdk 37 / AGP 9.1).
- **Android SDK** at `~/Android/Sdk` (android-36 platform, build-tools 36). Each worktree needs a
  **`app/local.properties`** with `sdk.dir=/home/yoda/Android/Sdk` — it's **gitignored**, so create
  it in every new worktree. CI uses the runner's preinstalled SDK (`android` job, Temurin 21).
- **Emulator:** AVD **`Pixel_7`** (android-36 x86_64). `/dev/kvm` is usable (user in `kvm` group).
  Boot headless:
  ```
  ANDROID_HOME=/home/yoda/Android/Sdk HOME=/home/yoda \
    ~/Android/Sdk/emulator/emulator -avd Pixel_7 -no-window -no-audio -no-snapshot -no-boot-anim \
    -gpu swiftshader_indirect [-read-only]
  ```
  Quirks: on cold boot the emulator's own SystemUI/Files processes throw **"… isn't responding"
  ANRs** under software GL — tap **Wait** (≈ `input tap 320 1368`) and let it settle ~20 s. The app
  also shows a splash for ~15–20 s on first launch. If boot fails with "multiple emulators same AVD",
  remove `~/.android/avd/Pixel_7.avd/*.lock`. Drive via `adb shell input tap/text`, read UI with
  `uiautomator dump`, capture with `adb exec-out screencap -p > file.png`.

## 5. Landing procedure (linear, no merge commit, verify-before-delete)

`main` is **not** checked out in a worktree, so push to remote `main` directly. Order matters
(mistakes here left PR #6 and PR #7 cosmetically "closed-not-merged" — the commits landed fine, but
the PR didn't associate). Correct sequence:

```bash
cd <worktree>
git fetch -q origin main
git rebase origin/main                              # expect this most times
git push --force-with-lease origin <branch>         # 1) update the PR head to the rebased commit
git fetch -q origin main                            # 2) re-check for concurrent movement
git push origin <branch>:main                       # 3) fast-forward main
git fetch -q origin main
[ "$(git rev-parse origin/main)" = "$(git rev-parse HEAD)" ] && echo LANDED   # 4) VERIFY before deleting
# only now:
cd /home/yoda/dev/git/troubastack
git worktree remove --force <worktree>
git branch -D <branch>
git push origin --delete <branch>                   # PR auto-closes as merged
```
Skipping step 1 (force-push branch) when you rebased → PR shows closed-not-merged. Deleting before
step 4 → risk losing the branch if the push was rejected.

**Cite the approval IN THE COMMIT before landing (hard rule — cost us on the A-track this session).**
The gate accepts either a Fable verdict in `reviews.md` OR explicit human approval — but the commit
message must record it, not just the chat. A13 landed on VLL's "go for it" without a citation and was
logged as the mobile lane's first **gate breach** (approved on merit, not reverted, but a black mark).
The trap: you commit *before* the approval exists. Fix: once the GO lands, **amend an `Approved: <verdict
ref>` trailer** (message-only amend → the diff stays identical to the reviewed SHA, so it's still
patch-identical) *then* fast-forward. When there's an open scope/completeness question, **hold at the
gate** for the verdict instead of land-then-fix (B06 did this; the verdict ratified the iOS deferral).
See memory `cite-approval-at-landing.md`.

## 6. What's done — the A-track (all merged to `main`)

| Task | Commit | Summary |
|---|---|---|
| A01 | `db9bf8e` | Wire the KMP/CMP Gradle build (wrapper, version catalog, `:shared`+`:androidApp`, `make app`, CI `android` job). |
| A02 | `a3aff02` | Concert-bundle model (`bundle/BundleModel.kt`, proto3-canonical-JSON mirror) + resilient **never-throw** `BundleLoader` + `docs/design/08-bundle-container.md`. |
| A03 | `5b2f755` | `core/cmd/mkbundle` deterministic fixture generator + committed demo/torture fixtures + `make fixtures`. |
| fix | `be7c2c7` | mkbundle → `color.NRGBA` (Go `color.RGBA` is alpha-premultiplied → hue-shifted overlays). |
| A04 | `1e18aa4` | **TroubaStage presenter** — `stage/` (StageModel/ViewModel/Screen): resilient, read-only, offline compositor + pager. Emulator-verified incl. torture fixtures, keep-screen-on, immersive. |
| A05 | `21a991d` | Android **Storage seam** actual + atomic `.tstage` **import** (zip-slip + zip-bomb guards, atomic swap, `BundleImporter`). Emulator-verified import + bad-bundle rejection. |
| A06 | `6927793` | Host **TroubaStudio in a WebView** (WebViewHost actual) + feature-detected `bridge.ts` handshake + Edit screen/server config. Emulator-verified login as marie/demo + handshake in logcat. |
| IOS01 | `8e53e42` | Enable `iosArm64`/`iosSimulatorArm64`; fill **iOS Storage** (NSFileManager + Keychain + `unpackBundle`) & **WebViewHost** (WKWebView) actuals; portable commonMain **zip reader** (`ZipReader.kt`: EOCD→central dir, `exceedsSizeCap`/`isSafeZipEntryName`, `expect rawInflate`) with JVM tests; CI `android` job now cross-compiles iOS klibs on Linux. (LRU fix `b146356` landed first.) |
| IOS02 | `e786418` (+`3bb2777`) | **`app/iosApp`** Xcode entrypoint (xcodegen `project.yml` + Swift), dynamic `Shared` framework export, `MainViewController.kt` (Concerts + Stage), **`.github/workflows/ios.yml`** (manual macOS: link → build unsigned → simulator → inject demo → screenshot + honest smoke). **Proven green with real Wonderwall Stage pixels.** |
| IOS04 | `5946874` | Keep the iOS screen awake during Stage (`KeepScreenAwake()` DisposableEffect in `App()`'s Stage branch; iOS analog of Android `StageHost`'s `FLAG_KEEP_SCREEN_ON`). |
| IOS03 | `d953e51` | **Prep-plan runbook only** (§11) — the impl is BLOCKED on a Mac + Apple credentials. |
| B02/B03 | (see reviews.md) | Distribution loop: bake→offer→download→import→perform in-app (Android), emulator-proven. |

**Stage reading-ergonomics arc (all in the shared `StageScreen`, not a shared nav — §13):**

| Task | Commit | Summary |
|---|---|---|
| A08 | `07e1e5d` | Setlist **metadata strip** (notes/key/tempo on each song's first page; footprint-stable overlay). |
| A09 | `02808fb` | **Hardware page turns** — BT pedals/keyboards (`onPreviewKeyEvent`) + Android volume keys (`onKeyDown`); pure `stageKeyAction` map. |
| A10 | `a34d4fc` | **Night mode** — invert `ColorFilter` at draw time (no re-encode); Day/Night toggle persisted via Storage KV, both entrypoints. |
| A11 | `b537d5f` | Visual **count-in** — tap the tempo chip for a silent beat pulse at the song's tempo (`CountIn.kt`, pure timing). |
| A12 | `57b2c9b` | **Facing pages (two-up)** in landscape FIT_PAGE — `FacingPages.kt` spread math + a two-up `Row`; turn by spread. |

**This session (2026-07-11 and the days before it) — landed + Fable-approved:**

| Task | Commit | Summary |
|---|---|---|
| A13 | `66e872f` | **Volume keys turn by spread in two-up** (A12 defect). StageScreen publishes its spread-aware turn via a commonMain CompositionLocal `LocalVolumeTurnRegistrar`; androidApp provides it; pure `turnTarget()` unifies every input. No new seam; iOS = default no-op. |
| A15 | `6619496` | **Song-jump navigation drawer** — `ModalNavigationDrawer`, A08 meta lines, current-song highlight; tap → `goToSong` (spread-aligned) + close. Pure `songMetaLine()`. |
| A14 | `06bfb4d` | **Continuous-scroll reading mode** — third Fit toggle (page→width→Scroll), a `LazyColumn` of all pages reusing the LRU cache; persisted via VM `initialFit` (A10 pattern); turns animate to page top; A08 strip inline. |
| T26 (app half) | `309f06f` | Consume the baked **song title** (kills "Song N") + **T23 bench grouping** — `BakedSong` mirror gains `title`/`onCall`; drawer groups "On call" below the main order. Verified against a **real bake** (built the web/bake CLI + core, benched a song, baked, injected). Proto/bake half was web-core's (already landed). |
| B06 (app half) | `4e0c024` | **LAN server discovery** on the Connect screen — commonMain `ServerDiscovery`/`DiscoveredServer` + pure dedup; Android `NsdServerDiscovery` (NsdManager, multicast lock, lifecycle-scoped); tap a "🎵 name — host:port" row to **prefill** the URL (no auto-connect). iOS: Bonjour plist keys only (NWBrowser wiring deferred to the App()/nav hoist — Fable-ratified). |

Net: the app **performs baked concerts offline (Stage)** with full reading ergonomics (metadata, night
mode, count-in, facing pages, hardware/volume turns, a song drawer, continuous scroll, real song titles
+ bench grouping), **imports `.tstage` bundles**, **hosts the live web editor (Edit)**, **downloads
band-baked concerts** (B03), and **finds the band server on the LAN without typing an IP** (B06) — on
Android (iOS: Stage proven on the simulator, IOS02; distribution/Connect UI awaits the App()/nav hoist).
I15 held throughout (platform code only in the seam actuals + the two thin entrypoints; discovery is DI
glue, not a seam). Commit hashes are as-landed; they may have been rebased since — grep the subject line
if a hash goes missing.

## 7. Current state & concurrency

- Re-run `git log main --oneline -15` on session start — `main` moves fast.
- The **T-track agent** works in the primary worktree `/home/yoda/dev/git/troubastack` and lands
  frequently. Don't edit there; use your own worktree.
- There may be extra worktrees you didn't create (`git worktree list`) — leave them alone.
- **Memory** (`/home/yoda/.claude/projects/-home-yoda-dev-git-troubastack/memory/`): `MEMORY.md`
  index, `mobile-app-agent.md`, `task-pack-workflow.md`, `git-linear-history.md`. Read on start.

## 8. Remaining work

**Unblocked, ready to pick up (authoritative queue: `docs/tasks/README.md` § Queue-state):**

- **P205 Stage 3a — app view-time identity + cues + defaults: ✅ UNBLOCKED 2026-07-19 by the web-core lane.**
  The whole P205 web-core side is landed on `main` (Stage 1 bake dialog + Stage 2 band-wide bake),
  all CI green. The bundle NOW carries everything Stage 3a needs — nothing more is owed from web-core.
  Spec: `docs/tasks/P205-band-wide-bundle.md` Stage 3 = your work.
  - **Bundle contract (landed, additive — mirror in `BundleModel.kt`):** `ConcertBundle.roster`
    (field 8, `repeated BundleMember{member_id, display_name, role}`) · `LayerImage.owner` (field 8,
    `""` = shared/band, a member-id = that member's personal layer) · `LayerImage.default_on`
    (field 9, **proto3 `optional` → Kotlin nullable**: absent ⇒ compute as today) ·
    `BakedSong.member_cues` (field 11, `repeated MemberCues{member_id, repeated SongCue cues}`).
    Field 10 (`cues`) is **empty in band-wide bundles** — read cues from `member_cues[myId]`, with
    field-10 as the fallback for OLD (pre-P205) bundles so nothing regresses.
  - **THE shared rule (landed, run it verbatim):** `core/internal/bake/testdata/view-resolution.vectors.json`
    — **12 cases + schema**, documented in `core/internal/bake/testdata/README.md`. Implement the
    Kotlin `layerVisible(layer, viewerRole, viewerMemberId)` to the precedence: **mandatory** (always on)
    > **personal** (`owner != "" → owner == viewerMemberId`; identity outranks default_on) >
    **shared** (`roleOK && defaultOK`, `roleOK = roleTag=="" || roleTag==role`,
    `defaultOK = defaultOn==null ? true : defaultOn`). **Never** a live session's toggles (a printed/
    offline view must be reproducible). Copy the vectors JSON into
    `app/shared/src/commonTest/resources/` and add a commonTest that runs EVERY case (mirror the Go
    `TestLayerVisible_Vectors`) — **print == screen is then a tested invariant**, not a hope. Add a **CI
    drift-guard** so the two copies stay byte-identical (mirror the `glyphs.json`/`CueGlyphData.kt`
    check in `.github/workflows/ci.yml`). Reference impl: Go `bake.LayerVisible` + `bake.ConcertPDF`
    (T57) already do exactly this server-side.
  - **Identity (Stage 3a):** resolve who's holding the tablet — a B03 Connect session matching a roster
    member → auto; else a one-tap **"Who are you?"** picker, remembered per concert/device (Storage KV;
    a VIEW preference, **not** an account — I12 held). Changing identity re-seeds like a role change
    (A18 semantics). Role picker stays (role still drives `role_tag` scoping); identity supplies the
    default role from the roster. **Defaults precedence** (P205 §Stage-3): mandatory > manual per-song
    toggles (A1/A18 session) > identity (my personal layers on) > `default_on ∧ role_tag`; absent ⇒ legacy.
  - **On your landing checklist (Fable, at Stage 3a land):** (1) delete the temporary
    `docs/demo/demo-concert-mine.tstage` bridge (Stage 3a makes the band-wide bundle enough — the app
    then reads `member_cues` + filters by identity); (2) wire the vectors CI drift-guard above. The
    web-core `scope=mine` bakeapi retirement is BLOCKED on Stage 3a landing (one release of overlap).
  - **Landed commits to diff:** `git log a254f5f..origin/main -- proto/ core/internal/bake/ docs/tasks/P205-band-wide-bundle.md`
    — P205 Stage 1 (`df0f3be`) + Stage 2 (`ed1966c`) + T57 (`0ebb346`, the reference rule); verdicts in
    `reviews.md` (2026-07-18 "P205 … GATE REVIEW/GATE ANSWER", "T57 GATE REVIEW … GO").

- **A20 — app half of T50 (personal song cues): ✅ UNBLOCKED 2026-07-17 by the web-core lane.**
  Handoff from the T-track (spec: `docs/tasks/T50-song-cues.md` §5 = your work; §2/§3 = the contract):
  - **Bundle contract (landed):** `BakedSong` gained field 10 `repeated SongCue cues`, with
    `message SongCue { string icon = 1; string color = 2; }` — additive metadata exactly like fields
    5–9 (B02). Mirror it in `BundleModel.kt` as a new **optional** field (default empty). The
    **per-member bake injects THAT member's cues**; the shared bake carries none — so a member's bundle
    already IS their view: **no app-side filtering.** `color` is `""` (neutral/untinted) or `#rrggbb`.
  - **Glyph asset (shared, landed):** `web/ink/glyphs.json` — the **18 glyphs** (ids listed in T50 §2)
    as pre-tessellated **normalized polylines**: `{version:1, glyphs:{id:{strokes:[[[x,y]…]…],
    fills:[[[x,y]…]…], strokeWidth}}}`, coords in a **1×1 box, y-down**, stroke round caps/joins,
    fills non-zero, ONE tint color for both. Convert to Compose `ImageVector`s (mechanical — stroke the
    `strokes`, fill the `fills`, scale by render size). **DO NOT hand-author glyph geometry** or parse
    SVG at runtime — that file is generated from `web/ink/glyphs.authoring.mjs` by `gen-glyphs.mjs`
    (CI drift-guards it). **Unknown icon id → render `note`** (the pinned fallback — required so new
    ids can ship server-side before the app knows them). This same asset feeds the T51 stamp tool later.
  - **Your build (T50 §5):** A15 drawer rows = right-aligned tinted cue icons per song; **center flash
    on song entry** = compose with the **N1 song-boundary title card** (ONE overlay, one clock-injected
    timeout — the A17 auto-hide pattern); no cues → the N1 card alone, unchanged. Tests: loader default/
    roundtrip, unknown→`note`, flash timeout (clock-injected), drawer-row presence.
  - **Landed commits to diff:** `git log 06bfb4d..origin/main -- proto/ web/ink/ core/internal/bake/`
    — T50 slice 1 (proto+core+bake) and slice 2 (studio + `web/ink/glyphs.json`), all CI green; verdicts
    in `reviews.md` (2026-07-17 "T50 SLICE 2 GATE REVIEW … GO", "T50 slice 2 … CI GREEN").

- Otherwise: the rest of the unattended A-track queue is drained (§6). New unblocked work appears
  when the Architect files a fresh `docs/tasks/*` **or** the web-core lane lands a proto/bake half
  that queues an app consumption piece (as A20, **T26**, **B06** all did) — on a "web made changes"
  prompt, `git log 06bfb4d..origin/main -- proto/ app/` and grep reviews.md for the app-half handoff.

**Attended — need the user's device/hardware for a live check (code is landed/verified otherwise):**
- **B06 live two-host mDNS check** — the Android emulator's NAT drops host multicast, so live
  `NsdManager` discovery can't be exercised on it; the browse UI + prefill are proven with an injected
  fake source (`docs/screenshots/b06-connect-discovered.png`) and the pure logic is unit-tested. A real
  device + a real core on the same LAN confirms live discovery.
- **B07 device screenshot pair** and **OPS01 release-APK** — ride the next attended device/emulator session.

**Deferred by decision (not a gap):**
- **iOS distribution/Connect UI** (offers, Connect screen, and the B06 NWBrowser discovery impl) rides
  the **shared App()/nav hoist** (§13) — trigger = when an iOS `ManifestTransport`/Connect surface lands.
  B06 shipped the iOS Bonjour plist keys so the capability is ready; the NWBrowser provider is intentionally
  NOT built yet (would be dead code — Fable-ratified 2026-07-11).

**Blocked — do not start without the user unblocking hardware/credentials:**
- **A07 — native wet-ink overlay: BLOCKED.** Needs a **real-tablet stylus latency spike** (input→
  photon latency + pen parity vs `web/ink`) that decides whether the optimized web path suffices. The
  emulator can't measure this. Everything A07 needs is on `main`; `InkOverlay` actuals (Android + iOS)
  stay `TODO`. May close **unbuilt**.
- **IOS03 impl — BLOCKED** on a **Mac + Apple credentials** (own-device free team / TestFlight $99 /
  App Store). The full execution runbook is **§11** — follow it the day the Mac + credentials exist;
  don't improvise a signing pipeline before then, and never commit certs/profiles/keys.
- **T-track** is the other agent's lane; don't duplicate. Check `docs/tasks/` + `git log` for status.
- **`ios.yml` is manual** (`workflow_dispatch` + weekly) — never make it per-push (macOS bills 10×).
  To re-prove iOS, dispatch it and verify `stage.png` shows Wonderwall (see §11 / the IOS02 history).
- Follow-ups noted in-code / PRs: none outstanding for the A-track.

## 9. Do-NOTs / gotchas

- **Never edit** `core/internal/webassets/dist/` (build artifact — a committed placeholder
  regenerated by `make embed`) or anything under a `gen/` dir. `make demo`/`make dist` overwrite the
  dist placeholder — if you run them, `git checkout -- core/internal/webassets/dist && git clean -fdq
  core/internal/webassets/dist` before committing.
- **`:8080` is used by the T-track agent's dev core.** Don't bind it and don't `make demo` (its seed
  step targets `localhost:8080` and would seed *their* server). To run a core for A-track WebView
  testing, use an **isolated port + data dir**:
  ```
  TROUBACORE_ADDR=:8091 TROUBA_APP_STORE=file TROUBA_STORE=file TROUBA_DATA_DIR=./troubadata-a06 \
    core/bin/troubacore     # then: go run ./cmd/seed -addr http://localhost:8091 -password demo
  ```
  Kill it with `fuser -k 8091/tcp`. The emulator reaches the host at `10.0.2.2:<port>`.
- **Airplane mode:** Stage (A04/A05) is offline — verify in airplane mode. Edit (A06) needs the
  network — airplane OFF.
- Studio deps aren't installed in a fresh worktree; `cd web/studio && npm ci --no-workspaces` (and
  same in `web/ink`) if you need to build the SPA.

## 10. Verification commands (from repo root)

```
make test                                   # Go suite
cd app && ./gradlew :shared:check :androidApp:assembleDebug   # app build + KMP tests (needs local.properties)
cd app && ./gradlew :shared:compileKotlinIosArm64 :shared:compileKotlinIosSimulatorArm64  # iOS klibs (Linux cross-compile; framework LINK is macOS-only → SKIPPED here)
make fixtures                               # regenerate committed TroubaStage fixtures (deterministic)
# web typecheck: cd web && studio/node_modules/.bin/tsc -b studio  (after per-pkg npm ci --no-workspaces)
```

## 11. IOS03 prep plan — device / TestFlight / App Store (⛔ BLOCKED runbook)

`docs/tasks/IOS03-*.md` is a **decision stub, blocked** on things this loop doesn't have: a
**Mac** (or rented mac-cloud) and **Apple credentials** (Apple ID, later a paid Developer Program).
Nothing here is buildable/verifiable in CI the way IOS01/02 were — a signing/export pipeline can't
even run without those. So this section is a **runbook to execute the day someone has the Mac +
credentials**, not code to land now. Do **not** improvise a signing pipeline before then (the stub
exists precisely to stop that), and **never commit secrets** (signing certs, provisioning profiles,
App Store Connect keys, `Apple ID`/app-specific passwords). Rewriting the task spec itself is the
Architect/Reviewer's lane — this is the mobile-dev execution notes.

**What IOS02 already gives us (the foundation):** a working, unsigned **simulator** build proven in
`.github/workflows/ios.yml` (framework link → `xcodebuild` → simulator → Wonderwall Stage pixels).
IOS03 only adds **signing + a real destination**; the app itself is done.

### The three tiers (pick by goal)

| Goal | Needs | Cost | Notes |
|---|---|---|---|
| Run on **your own** iPhone/iPad | Mac + Xcode + a **free** Apple ID (personal team) | free | App re-signs every **7 days**; **3-app** limit; simplest for a quick real-device check. |
| **TestFlight** for the band | **Apple Developer Program** + App Store Connect | **$99/yr** | 90-day builds, up to 100 internal / 10k external testers; needs an App Store Connect app record. |
| **App Store** release | Developer Program + App Review | $99/yr | Full review; privacy nutrition labels, screenshots, etc. Probably overkill for a band tool. |

**Recommendation:** for a band, **TestFlight** is the sweet spot; own-device (free team) is the
cheapest way to first prove it on real hardware / do the Stage QA below.

### Signing config — kept OUT of the repo

- Device/TestFlight builds must be **signed**; unlike IOS02's simulator build (`CODE_SIGNING_ALLOWED=NO`)
  this needs a real identity + provisioning profile. Keep all of it out of git:
  - **Local Mac:** let Xcode "Automatically manage signing" with your Apple ID (Signing & Capabilities
    → Team). `project.yml` can set `DEVELOPMENT_TEAM` via a **`settings` override read from an env var
    or a gitignored `app/iosApp/Signing.local.xcconfig`** — never hardcode the team id.
  - **CI (only with a paid program):** store the **distribution cert (.p12)**, **provisioning profile**,
    and an **App Store Connect API key (.p8)** as GitHub **Secrets**; import with `fastlane match`
    (private cert repo) or `apple-actions/import-codesign-certs`. The macOS job stays
    `workflow_dispatch` (10× minutes). Add `app/iosApp/Signing.local.xcconfig` and `*.p12/*.p8/*.mobileprovision`
    to `.gitignore` **before** anyone generates them.

### Build/export mechanics

- Reuse IOS02's flow but archive+export instead of a simulator build:
  ```sh
  cd app && ./gradlew :shared:linkReleaseFrameworkIosArm64   # device = iosArm64, Release
  cd iosApp && xcodegen generate
  xcodebuild -project iosApp.xcodeproj -scheme iosApp -sdk iphoneos -configuration Release \
    -archivePath build/iosApp.xcarchive archive
  xcodebuild -exportArchive -archivePath build/iosApp.xcarchive \
    -exportOptionsPlist ExportOptions.plist -exportPath build/export
  ```
  - `ExportOptions.plist` (gitignored) sets `method` = `development` (own device) / `app-store` /
    `release-testing` and the team id.
  - `project.yml` needs a **device** framework-search path too (`iosArm64/releaseFramework`) — IOS02
    only wired `iosSimulatorArm64/debugFramework`. Parameterize by SDK or add a Release/device variant.
  - **fastlane** (`gym` + `pilot`/`deliver`) is the higher-level alternative to raw `xcodebuild` for
    TestFlight/Store uploads — worth it once there's a paid program; overkill for own-device.

### PencilKit / ink

- Only relevant **if A07 is ever built** (native wet-ink overlay — currently blocked on the tablet
  stylus spike; iosMain `InkOverlay` is all-`TODO`). On device, PencilKit + Apple Pencil is the iOS
  path; the golden-image parity vs `web/ink` (I8) would need real-device capture. Ignore until A07.

### Stage-specific device QA (the point of going to hardware)

Run the same Wonderwall demo but validate the **performance ergonomics** the simulator can't:
- **Stand-mounted iPad**, landscape + portrait, real-tablet screen size and brightness.
- **Guided Access** (triple-click) during a performance so a stray touch can't exit Stage.
- **Screen stays awake** mid-performance (iOS: `UIApplication.isIdleTimerDisabled` — currently NOT set;
  add it to the iOS Stage host, mirroring Android's `FLAG_KEEP_SCREEN_ON` in `StageHost`).
- **Apple Pencil / finger** page turns (tap/swipe), fit modes, layer + role toggles — offline (airplane).
- Battery/thermals over a full set; large real scans (decode/memory — IOS02 used the small demo).

### When unblocked

Turn the above into a real task, land signing config out-of-repo, add a **manual** `ios-release.yml`
(archive → export → optional TestFlight upload) alongside `ios.yml`, and do the device QA. Until then:
**Android is the shipped device story; iOS lives in CI simulators (IOS02).**

## 12. Follow-ups — ✅ GO'd 2026-07-06 (reviews.md); implementing

Four small cleanups (mostly the reviewer's own parked notes), approved on all four with three
riders (below). Batched as proposed; each lands the usual way (rebase → ff → verify-before-delete).

1. **iOS `unpackBundle`: size-check before read** — `NSData.dataWithContentsOfFile` reads the
   whole file *before* the 512 MB cap (jetsam risk on a multi-GB pick). Add an
   `attributesOfItemAtPath` size gate first. *(Parked IOS01 note #1.)* — XS, Linux-verifiable.
2. **Tighten iOS `rawInflate`** — it accepts `Z_OK` with an exactly-filled buffer, tolerating a
   stream longer than declared (cap-contained but lenient); fail closed. *(Parked IOS01 note #3.)* — XS.
3. **Extract `jsQuote` (WebViewHost) → commonMain + unit test** — the bridge JS-string escaper is
   iOS-only and untested; hoisting makes it JVM-testable. — S.
4. **`PageImageCache` unit test** — extract a generic `LruCache<K,V>` (its `ImageBitmap` value type
   blocked direct testing) and test eviction/access-order. *(Reviewer flagged twice.)* — S.

Batching: **#1+#2+#3 as one "iOS seam hardening" PR**, **#4 as its own small PR**.

Reviewer's riders (fold into implementation):
- **#2 zlib trap:** when the output buffer is exactly filled, `inflate(Z_FINISH)` can return `Z_OK`
  and only report `Z_STREAM_END` on a **second call** (`avail_out = 0`). Use the double-call pattern
  (require `Z_STREAM_END` after the follow-up) so valid bundles that exactly fill `expectedSize`
  aren't rejected; keep it consistent with Android `Inflater.finished()`.
- **#4:** `LruCache<K,V>` goes to commonMain; `PageImageCache` becomes a thin typed wrapper (Stage
  behaviour byte-identical); commit the eviction/access-order matrix (the 2026-07-04 LRU scratch test).

Withheld for their own proposals (NOT here): B03 expired-session 401 handling (parked B03 note #2);
hoisting the shared App()/nav into commonMain (a design decision).

## 13. Shared App()/nav hoist — DECISION: (c) DEFER (reviews.md 2026-07-06)

**Decided: defer.** Building the shared `App()` with optional slots before a second real consumer
exists is speculative generality. **Concrete trigger:** this becomes its own spec'd task the moment
an **iOS `ManifestTransport`** lands — plan the transport + the hoist together (the transport is what
validates the slots). Option (b) is explicitly NOT banked now (half a shared nav still encodes
today's Android shape). Until then the duplication is stable; new A-track work (A08/A09/A10…) lands
in the already-shared `StageScreen`, not the nav. The original proposal + options are kept below.

**Problem.** The Concerts↔Stage navigation + concert listing is duplicated:
`androidApp/MainActivity.kt` (`App`/`ConcertsScreen`/`listConcerts` + B03 offer chips, per-concert
overflow, Connect, Edit, Import) and `shared/iosMain/MainViewController.kt` (a trimmed `App`/
`ConcertsScreen`/`listConcerts` — Stage-only; no offers/Connect/Edit/Import). Only `StageScreen` is
truly shared today. So B03's distribution UI lives on Android only; iOS would re-implement it.

**Sketch.** A commonMain `@Composable fun App(deps)` owning the nav + offer/overflow rendering,
with platform bits injected (keeps I15 — no new seam):
- `ImageDecoder` factory (Android BitmapFactory / iOS Skia) — already an interface.
- bundle listing: inject `listInstalled: () -> List<ConcertEntry>` (Android `File.listFiles` / iOS
  `NSFileManager`) — a DI lambda, NOT a new Storage-seam method.
- optional `UpdatesManager` (null ⇒ offline-only, no offers) — iOS passes null until it has a ktor
  transport (IOS-track); Android passes the real one.
- optional platform slots: `stageWrapper` (Android StageHost keep-awake+immersive / iOS
  KeepScreenAwake), and `onEdit`/`onImport`/`onConnect` (Android-only today; absent on iOS).

**Trade-offs.** Pro: DRY; iOS gets offers/overflow for free once it has a transport; one tested nav.
Con: the two entrypoints genuinely diverge (Android feature-rich; iOS Stage-only), so the shared
`App()` needs several optional slots — risk of a leaky abstraction built before a second real
consumer exists. Touches both entrypoints → re-verify Android on emulator + iOS klib. Medium/large.

**Options (requesting a pick):**
- **(a) Full hoist now** — shared `App()` with all the injected slots above.
- **(b) Partial** — hoist only `listConcerts` + Concerts↔Stage nav + offer-chip *rendering*; leave
  Edit/Import/Connect as platform screens. Incremental, smaller blast radius.
- **(c) Defer** — hoist when IOS actually needs the distribution UI (i.e., an iOS-transport task
  lands), so the abstraction is validated by a concrete second consumer instead of built speculatively.

**Mobile Agent's lean: (c)**, or **(b)** if you want to bank the DRY now — a full (a) before iOS has
a transport risks speculative generality. Your call.

---

## 2026-08-07 — Fable dispatch: incoming shared-glyph change (⚠ warning) — heads-up, non-blocking

The web/core lane's DEMO-VID work adds a **`warning` (⚠) glyph** to the shared cue/stamp set (commit `ccddc7d`, landing with DEMOVID tip `50def5b`). Because `gen-glyphs.mjs` is the single source for both platforms, the run **also regenerated your `CueGlyphData.kt`** (18→19 glyphs) and bumped the guard **`CueTest.kt` 18→19 (+"warning")**.

**Why it's safe (I verified):** your renderer draws glyphs generically from `CueGlyph.strokes/fills` — no per-id switch — so the new entry is pure data the app renders automatically. The only build-breaker (the count-guard test) is already bumped, so `./gradlew test` stays green. I confirmed the regen is deterministic (re-running gen-glyphs reproduces both `glyphs.json` and `CueGlyphData.kt` byte-for-byte).

**Two small asks (non-blocking — this lands regardless):**
1. **Claim/own** the one-line `CueTest.kt` bump so it's on your radar (it's your test file).
2. **Eyeball the ⚠ on-device** — the triangle is `strokes=[(0.5,0.15)→(0.89,0.83)→(0.11,0.83)→apex, exclamation (0.5,0.39)→(0.5,0.62)] + dot fill`. If it renders poorly on the Stage canvas, a tweak just re-runs gen-glyphs (updates both platforms) — ping me and I'll route it.

No proto/server change; the glyph id rides in the icon object's text field like every other glyph. — Fable

---

## 2026-08-21 — Fable dispatch: A34 is **GO** — two fixes required before you ff-push

Full verdict in `docs/handoff/reviews.md` (commit `bc61dd2`). The engineering is verified and I have
no objection to the code: the Kotlin port matches T85's TS definition for definition, the mirrored
vectors are byte-identical to `docs/contracts/beat-phase.vectors.json`, and I teeth-checked the suite
by breaking the implementation three ways — each break went red in the right test. `:shared:check`
green here, 9 tests genuinely executed (JUnit XML read, not just an exit code).

**VLL has asked for both of the following before the push.** Neither is a code change.

### 1. Four comments describe behaviour you removed

Two of these are actively misleading, and A35 is about to be written against this file:

| file:line | says | reality |
|---|---|---|
| `StageBeat.kt:100` | *"scoped to [resetKey] (**the current page**) so a page turn makes a fresh instance and **cancels any in-progress count-in**"* | `resetKey = state.currentSong`. A page turn must **not** stop the beat — that was VLL's explicit request in iteration 5. |
| `StageBeat.kt:48–49` | *"Tap → an 8-beat count-in; **long-press** → continuous until a second tap or **a page turn**"* | The long-press is gone; it's the ∞ segment now. That hidden gesture is the exact thing VLL never discovered. |
| `CountIn.kt:25` | *"Tap = count-in; **long-press** = continuous"* | Same. |
| `StageScreen.kt:1028–1029` | documents a `[resetKey]` parameter and *"in two-up it's the per-side page index"* | `MetaStrip` no longer takes `resetKey`. |

### 2. Both committed screenshots are three commits stale

`docs/screenshots/a34-beat-downbeat.png` and `a34-beat-offbeat.png` were added in `3c68371` and never
refreshed across `abe22f0`, `4a2625d`, `a81169f`. They show the `○` auto-update FAB that `a81169f`
removed and the `♩=88` meta-strip chip that `abe22f0` removed — and they show **neither the centre
beat number nor the ∞ capsule**, i.e. none of the three iterations VLL actually drove.

I am not disputing your device verification. The border pulse reads exactly as intended in them
(amber downbeat, aqua off-beat, all four sides, clear of the music), so the pulse is verified. The
problem is that the repo's evidence for the *final* UI does not exist, and a year from now the
screenshot is what someone will believe.

**Please re-shoot both on the current build**, showing the centre beat number and the `[metronome|∞]`
capsule — one on a downbeat (amber `1`), one off-beat (aqua). Same tablet/song is fine.

### Then push

Once those two land, ff-push on VLL's word — the T82/T83 landing grant expired with T83, so I am
verdict-only here and cannot hand you the LAND IT myself.

### One non-blocking note, and one heads-up

- `LaunchedEffect(beat.runToken)` keys on an `Int`, not the instance. It is correct today only
  because a started beat always has `runToken >= 1`, so a fresh instance's `0` can never collide with
  a running predecessor's token. That is a subtle invariant propping up a behaviour by accident —
  adding `beat` to the key makes the reasoning unnecessary.
- **A35 supersedes `BEATS_PER_BAR = 4` and the two-tier colouring.** T86/A35 were revised today
  (`5b1f12d`) after VLL's proposal: a metre resolves to **group lengths** and each unit gets a tier
  (bar amber / felt pulse aqua / free subdivision grey), with additive metres (`3+4/8`, `3+3+1/4`) in
  scope. Don't build anything further on the `% 4` assumption.
- Build tip for a fresh worktree: `:shared:*` needs `ANDROID_HOME=~/Android/Sdk` — there is no
  `local.properties`, and without it Gradle can report **exit 0 having built nothing**.

— Fable
