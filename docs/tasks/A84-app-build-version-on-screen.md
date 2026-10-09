# A84 — The app says which build it is

**Lane:** mobile · **Status:** specced 2026-10-09, not started · **Origin:** VLL, 2026-10-09: *"il faut un
moyen d'avoir un hash ou version dans un écran de l'app"*.

## 1. Why

Today nobody can tell which build is on a tablet. `versionName` is `0.1.0` and `versionCode` is `1` for every
build (`app/androidApp/build.gradle.kts`). On 2026-10-09 Fable could only date the tablet's install
(`lastUpdateTime` 10-02 17:49) and had to *guess* which of A78, A79 and the stroke fix it had. The server
already answers this question (`GET /api/version` → `version` + `builtAt`, T29). The app must answer it too,
in the app, without adb.

## 2. What to show

**⟨D1⟩ A "Version" line at the bottom of the Parameters screen** (`ui/SettingsScreen.kt`), in its own last
`Section("About")`:

- `TroubaStage <hash> · built <date>`. `<hash>` is the git short hash of the commit the APK was built from,
  the same form the server reports (`git describe --always --dirty`). `<date>` is the build time in UTC,
  minute precision (`2026-10-09 15:07 UTC`).
- If the build came from a dirty tree, keep the `-dirty` suffix. It is the honest answer for a local build,
  and a red flag for CI.
- A long-press (or a small copy icon) copies the line to the clipboard, for pasting into a bug report.
  Optional; skip it if it costs more than a few lines.

**⟨D2⟩ The servers the app is joined to, with their version.** In the same section, one line per joined
server: its address, plus the `version` from `GET /api/version` (the probe `JoinFlow` already knows how to
make). Fetch it when the screen opens. An offline or unreachable server shows *unreachable*, not an error
dialog. This answers "is the server current?" from the tablet, and puts both versions side by side when they
are compared.

**⟨D3⟩ `versionName` carries the hash too**, e.g. `0.1.0+73dee974`, so `adb shell dumpsys package
com.troubastack.app | grep versionName` answers without opening the app. Leave `versionCode` alone (any change
to it is a separate decision about upgrades).

## 3. Where the hash comes from — properties, not a prescribed mechanism

- **It is computed at build time from git**, for the commit being built, with no hand-maintained constant. A
  version string someone has to remember to bump will rot.
- **It reaches common code**, so the screen in `commonMain` reads it. iOS should report the same thing, or a
  clearly marked `unknown` until the iOS build is wired: it must never invent a value.
- **It survives the Gradle configuration cache** (the build uses it). Read git through a provider
  (`providers.exec { … }`), not a bare `"git …".execute()` at configuration time. A cached configuration must
  not serve a stale hash. Prove it: build, make an empty commit, build again with the cache on, and show the two
  hashes differ.
- **It works in CI**, where `actions/checkout` is a shallow clone (`fetch-depth: 1`). `git describe --always`
  still yields the short hash there. Check it in the CI APK, not only locally.
- **No git → a clear fallback** (`unknown`), never a build failure. Someone may build from a source tarball.

## 4. Not in this task

- Changing `versionCode`, release signing, or any store listing.
- An update checker ("a newer build exists"). That is a follow-up, and it needs a decision on where "newer" is
  read from.
- Showing the version anywhere other than Parameters.

## 5. Acceptance

- **Unit (common):** the formatter turns `(hash, builtAt)` into the displayed line, and `(null, null)` into
  `unknown`. No test asserts a literal hash.
- **Build:** after a build, the line's hash equals `git rev-parse --short HEAD` (or `describe` with `-dirty`),
  and `dumpsys`'s `versionName` ends in the same hash. Paste both in the gate entry.
- **Configuration cache:** the two-builds-across-a-commit proof from §3.
- **CI:** download the `troubastage-debug-apk` artifact from the PR's run and show its `versionName` hash
  equals the run's `head_sha` prefix (`aapt dump badging` or install and `dumpsys`).
- **Screen:** a screenshot of Parameters → About on a tablet or emulator, with at least one joined server line
  showing its version and one *unreachable* case (stop a scratch server). Synthetic data only.
- **Regression:** `:shared:testDebugUnitTest`, `:androidApp:assembleDebug`, and the iOS compile are green.
