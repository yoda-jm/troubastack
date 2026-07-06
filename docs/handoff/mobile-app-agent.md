# Agent handoff — "Mobile App Agent" (TroubaStack A-track)

> **You are the Mobile App Agent.** Your lane is the **A-track** — the Kotlin/Compose Multiplatform
> mobile app in `app/` (plus the thin studio/core touchpoints an app task needs). A separate agent
> owns the **T-track** (web/core/infra); stay out of its way (see §2, §7).
>
> Point a fresh Claude Code session at this file to continue seamlessly. It captures **how we work**
> and **what's done**, not the code (read the code + `docs/` for that). Last updated 2026-07-05.
>
> **To resume:** open this repo in a new session and say — *"Read `docs/handoff/mobile-app-agent.md`;
> you are the Mobile App Agent — let's continue."* Then read §2 (how we work) and §5 (landing) before
> touching anything, and `git log main --oneline -15` for current state. **Immediate next action:**
> the actionable iOS + Android queue is **drained** (A01–A06, IOS01–IOS04 all merged — §6). The only
> remaining app tasks are **both blocked**: **A07** (native wet-ink — real-tablet stylus spike) and
> **IOS03 impl** (device/TestFlight/Store — needs a Mac + Apple credentials; the runbook is §11).
> Don't start either without the user unblocking hardware/credentials. Otherwise there's nothing to
> pick up unless a new `docs/tasks/*` is filed — check `git log` + `docs/tasks/` + the review-gate log
> (`docs/handoff/reviews.md`) on start.

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

Net: the app **performs baked concerts offline (Stage)**, **imports `.tstage` bundles**, and **hosts
the live web editor (Edit)** — on **both Android and iOS** (iOS Stage proven on the simulator, IOS02).
I15 held throughout (platform code only in the seam actuals + the two thin entrypoints). Commit hashes
are the as-landed values; they may have been rebased since — grep the subject line if a hash goes missing.

## 7. Current state & concurrency

- Re-run `git log main --oneline -15` on session start — `main` moves fast.
- The **T-track agent** works in the primary worktree `/home/yoda/dev/git/troubastack` and lands
  frequently. Don't edit there; use your own worktree.
- There may be extra worktrees you didn't create (`git worktree list`) — leave them alone.
- **Memory** (`/home/yoda/.claude/projects/-home-yoda-dev-git-troubastack/memory/`): `MEMORY.md`
  index, `mobile-app-agent.md`, `task-pack-workflow.md`, `git-linear-history.md`. Read on start.

## 8. Remaining work

The actionable A/IOS queue is **drained** (§6). Both remaining app tasks are **BLOCKED** — do not
start either without the user unblocking hardware/credentials:

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
