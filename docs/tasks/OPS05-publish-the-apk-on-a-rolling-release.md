# OPS05 — publish the Android APK on a rolling Release (tap → install)

**Lane:** web-core (CI/ops + `web/site`). **Status:** spec drafted by Web-Core, **for Fable to validate before implementation** (per VLL: "spec OPS05, make it validate by Fable, then take it").
**Asked by:** VLL — reviewing the live OPS03 page on a tablet: *"I would have hoped to directly land on the latest apk to download, so you click it installs."*

## The problem (verified on the device)

The page's Android QR + "Get the app" button point at `APK_URL` in `web/site/build.sh:31`, which is the **CI successful-runs list** (`…/actions/workflows/ci.yml?query=branch%3Amain+is%3Asuccess`). Opening it on the tablet lands on that list — and to actually get an APK from there a person must:

1. **sign in to GitHub** — Actions artifacts are not downloadable anonymously (the page label already admits this);
2. pick the latest green run → Artifacts;
3. download a **`.zip`** (`troubastage-debug-apk`) that *wraps* the APK — Android cannot install a zip.

So the current flow is three navigations, a login, and an un-installable archive. VLL wants **one tap → the installer**.

## What already exists — surveyed, not assumed

- The **`android` job** (`.github/workflows/ci.yml:217`) runs `./gradlew :shared:check :androidApp:test :androidApp:assembleDebug` (`ci.yml:237`) and uploads the result as an **artifact** named `troubastage-debug-apk` from `app/androidApp/build/outputs/apk/debug/androidApp-debug.apk` (`ci.yml:246-250`). It runs on **every push and PR**.
- `web/site/build.sh` already parameterises the QR/button target as `APK_URL` (`:31`) and generates `qr-apk.svg` from it (`:87`); `index.html` carries the "ANDROID · GET THE APP" block and the "GitHub asks you to sign in" caveat.
- **No git tags exist** (confirmed in OPS04) and **no release workflow exists.** Nothing publishes a Release today.

The APK build is not touched by this task; **only CI wiring + `web/site` change.** (The APK is the A-track's output; distribution — CI, the Release, the public page — is web-core/OPS. Flagging the boundary: this task must not edit anything under `app/`.)

## The mechanism: one rolling Release, one stable asset

Publish the debug APK as an asset on a **single rolling Release** so a phone can fetch the raw `.apk` with no login:

```
https://github.com/yoda-jm/troubastack/releases/download/latest/troubastage-debug.apk
```

- **Public, no sign-in** (Release assets are anonymous on a public repo — unlike Actions artifacts).
- **The raw APK**, not a zip → on Android the browser downloads it and taps into the installer.
- **Stable URL**: a fixed tag `latest` + a fixed asset name, so the page never needs re-pointing as builds roll.

### A footgun to name (finding 1)

`…/releases/**latest**/download/NAME` (the "latest" *keyword* shortcut) resolves to the newest **non-prerelease** release and **skips prereleases**. So if the rolling release is marked *prerelease*, that shortcut 404s. Two clean ways out, pick one (a decision for Fable):

- **(recommended) explicit-tag URL** `…/releases/download/latest/troubastage-debug.apk` — depends only on the tag name `latest` and the asset name being constant, independent of prerelease flag. Robust and legible.
- the `…/releases/latest/download/…` keyword shortcut — then the release must be a **full** release (not prerelease).

Recommending the explicit-tag form; it survives us later marking the release "prerelease" to signal debug-signing.

## The work

1. **Publish step in CI.** After `assembleDebug`, on **push to `main` only** (never PRs) and only when the build passed, copy the APK to a stable name and upload/replace it on the `latest` release:
   - rename `androidApp-debug.apk` → **`troubastage-debug.apk`** (the URL embeds the name; `androidApp-debug` is an internal gradle name — finding 2);
   - create-or-update the `latest` release and **replace** the asset (`--clobber`), so it rolls rather than accumulates;
   - `gh release upload` (the `gh` CLI is on the runner) or `softprops/action-gh-release`. Either is fine; Fable's call.
   - **Gate:** `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`. The `android` job runs on PRs too; a release must never be cut from unreviewed PR code (finding 3).
   - **Permissions:** the step needs `permissions: contents: write` on the job (the default `GITHUB_TOKEN` is read-only for releases, so this 403s without it — finding 4).
2. **Repoint the page.** In `build.sh`, set `APK_URL` to the release download URL (keep it a single var, like `SITE_URL`, so a later rename is one edit). The QR + button follow automatically. Update the label: drop "sign in to download" (no longer true), keep the honest debug-signing note (see below).

## The one honest caveat (keep it on the page)

The APK is **debug-signed** — there is no release keystore in the repo. Android still shows a one-time **"install from an unknown source"** prompt; true one-tap install is not possible off the Play Store. This is as close as it gets, and the label should say so plainly rather than imply a store install. **Release-signing (a keystore secret) is out of scope** — a later task if VLL wants the warning gone.

## Decisions — settled by Fable, 2026-09-02

Citations checked first, all four exact: `APK_URL` is `build.sh:31`, the `android` job is `ci.yml:217`,
the gradlew line `:237`, the upload `:246-250`, and there is indeed no release workflow.

**a) Explicit-tag URL** — `…/releases/download/latest/troubastage-debug.apk`. Agreed, for the reason
given: it depends only on the tag name and the asset name, so marking the release prerelease later
cannot break it.

**b) Mark it prerelease — yes.** The build is debug-signed; a badge that says so costs nothing and
stops the release page implying a store-grade artefact. This is exactly why (a) matters.

**c) A SEPARATE `release-apk` job — overriding the in-job recommendation.**
`permissions:` is granted at **job** level, so putting `contents: write` on `android` gives write
capability to *every step of that job* — and that job runs on pull requests and executes a full
Gradle build with third-party plugins. The `if:` guards the *step*, not the *token*. A separate job
with `needs: [android]`, the same `if:`, and `permissions: contents: write` confines release-writing
to a job that only downloads an artefact and uploads it. The artifact round-trip costs seconds;
least privilege is worth more than one saved download.

**d) Every green `main` push** — ~~agreed~~ **amended by VLL, 2026-09-02 after seeing it run: restrict
to pushes that touch `app/`.** Observed cost of the simple version: the release re-fired on a
*documentation* commit, republishing a byte-identical APK, and because publishing is
`delete --cleanup-tag` + `create`, the download URL 404s for a few seconds each time.

A workflow-level `paths:` filter cannot express this — it would gate the whole of CI, not this job —
so the job decides for itself via the compare API (no checkout needed, the token is already present).
It **fails open**: unknown base (branch creation, force push) or a failed compare call publishes
anyway, because a missed publish is worse than a redundant one.

Side benefit worth naming: when the job is skipped the release keeps the previous APK *and* its body
keeps naming the commit that APK was actually built from — which is more honest than re-stamping it
with a docs commit that changed nothing.

## Two findings from the validation

**1. The rolling tag goes stale, and the release page will lie about provenance.**
`gh release create latest` creates the tag at that commit; a later `gh release upload latest
--clobber` replaces the **asset** and leaves the **tag** where it was. Within a week the page will
say `latest` → a commit from days ago while serving today's build. And the APK cannot settle it
either: `versionName` is a static `0.1.0`, so nothing in the artefact identifies which build it is.

**Required:** on every publish, rewrite the release body with the **commit sha and build time**, and
either re-point the tag at the published commit or state in the body that the tag is fixed and the
body is authoritative. Pick one and make it explicit — a release whose stated commit is wrong is
worse than one with no commit at all. This is the same traceability hole OPS04 has with
`VERSION`/`BUILT_AT`, in the other artefact.

**2. The sweep is larger than "update the label", and it collides with OPS04.**
`web/site/index.html`'s honesty panel and `README.md:116` both read *"no published registry image,
**no GitHub Releases binary**, and no store/F-Droid listing"*. OPS05 makes the middle clause false;
**OPS04 makes the first clause false**, in the same two sentences. Whichever lands second must
reconcile the panel rather than re-edit it blind. Rewrite it once, coherently, naming exactly what
exists at that moment.

**Verdict: GO to implement**, with (a)–(d) as settled above and both findings addressed. The spec's
own boundary — nothing under `app/` — is right and I am holding the lane to it.

## Done when

- `curl -L https://github.com/yoda-jm/troubastack/releases/download/latest/troubastage-debug.apk` **unauthenticated** → 200, an APK (zip `PK\x03\x04` magic), non-trivial size.
- The page's QR + button resolve there; **opening it on the tablet downloads the APK and Android offers to install it** (verify on the device, not just the status code — the OPS03 discipline).
- A subsequent green `main` push **replaces** the asset (release does not accumulate duplicates).
- A **PR** does **not** create or modify the release (check a PR run: no release write).

## Out of scope

Release-signing / Play Store · iOS distribution · versioned/semver releases (latest-only, per OPS04) · changing anything the `android` job builds.
