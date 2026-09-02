# OPS02 — Docker image embeds the ready apps + a download link in Studio

> **Superseded detail (BRAND02, 2026-09-02):** the served artifact was renamed
> with the app — the endpoint is now `/apps/troubastage.apk` and the on-disk file
> `deploy/apps/troubastage.apk`. The paths quoted below are the ones this task
> shipped; the contract is otherwise unchanged.


**Priority:** normal (VLL 2026-07-19: "a docker image that also embeds the apps that
are ready and there is a link to download them") · **Size:** M · **Area:** deploy/
Dockerfile + CI, core (static route + manifest), `web/studio` (the link/QR card).
Web-core lane. Relates to OPS01 (compose stack) and the user-blocked release-keystore
item.

## The idea

A band self-hosts ONE artifact: the troubacore image. It already embeds the Studio
SPA; it should also carry the **ready app binaries** (today: the Android APK; iOS
joins when that track exists) and Studio should show a **download link** — so a new
band member points their phone at the server and installs the app with zero other
infrastructure.

## Design

1. **Serving (core):**
   - `GET /apps/troubashare.apk` — static, unauthenticated (like the SPA assets; the
     APK is not a secret and pre-account members are exactly the audience), correct
     MIME `application/vnd.android.package-archive`, `Content-Disposition` filename
     carrying the version (`troubashare-<version>.apk`).
   - `GET /api/apps` — a tiny manifest: `[{platform:"android", version, size,
     path}]`, empty entries omitted. iOS appears here when it exists; the UI hides
     what the manifest lacks. Serve from an `apps/` dir next to the SPA embed;
     ABSENT dir ⇒ empty manifest, no errors (dev runs unaffected).
2. **Image (deploy/ + CI):**
   - The APK is built in CI (`:androidApp:assembleRelease`, debug-signed until the
     keystore item lands — installable with unknown-sources, fine for band use) and
     COPY'd into the image at `apps/`. The Dockerfile takes it as an optional build
     input: **absent ⇒ the image still builds and runs** (no Android SDK in the
     image build — keep local `docker compose up` light).
   - Version stamping: the manifest's `version` = the build's git describe/commit
     (mirrors the existing version chip).
3. **AMENDED (VLL 2026-07-19):** the card also shows an **iOS row, greyed, "Coming
   soon"** (not tappable) — intent over absence. It's a hardcoded coming-soon row
   that flips to a live download the moment an `ios` entry appears in the manifest.
   No-apps-at-all still means no card.
3b. **Studio (the link):** a "Get the app" card — home: the **band page** (visible to
   every member, NOT the editor; embedded mode unaffected since the WebView user
   already has the app). Contents: the Android download button (from `/api/apps`;
   hidden when the manifest is empty) **and a QR code** of the absolute APK URL —
   the actual flow is bandleader-laptop-screen → member-phone-camera. QR: generate
   client-side or a tiny server-side PNG; NO heavy dependency — lane's pick, flag
   anything beyond a small single-purpose lib at the gate.

## Acceptance

- Image built WITH the APK: `/api/apps` lists android+version, the APK downloads
  with the right MIME/filename, the band page shows the card + QR (pixels
  light+dark).
- **Both install flows verified on device** (VLL 2026-07-19: "both are good"):
  (a) PHONE-BROWSER flow — open the server URL on the phone, log in, tap the
  card's download button, Android offers the install (the card must be
  tap-friendly at phone width — T47-era 412px check); (b) QR flow — scan the
  laptop-screen QR with the phone camera → download. Device check rides a VLL
  session.
- Image built WITHOUT the APK: builds, runs, manifest empty, no card shown, no
  errors (e2e against the dev stack where `apps/` is absent).
- gofmt/vet/tests; e2e for card-present/card-absent; CI job wiring green.

## Out of scope

- Release signing (the keystore item stays user-blocked; debug-signed now, drop-in
  upgrade later); iOS artifacts (manifest-ready, absent); auto-update of installed
  apps (the app's own update flows exist); publishing to any store.

## Device-flow verification (mobile lane, 2026-07-20)

Drove the phone-browser tap-to-download on the QA tablet (Brave) against a live core
with `TROUBA_APPS_DIR` pointing at a dir holding the debug APK. End to end:

- **Topbar chip → popover:** "Get the app" chip renders in the authenticated topbar;
  tapping opens the popover: **QR code** (the camera path — VLL's 30-second moment),
  **"Download for Android"**, the meta line (`dev · 12.9 MB · Android`), and the
  **iOS · COMING SOON** greyed row ("Available in a future release"). Matches the spec
  (incl. the AMENDED intent-over-absence iOS row).
- **`/api/apps`** returns the android entry (`version dev`, size, `/apps/troubashare.apk`,
  `filename troubashare-dev.apk`); **`/apps/troubashare.apk`** serves with
  `Content-Type: application/vnd.android.package-archive` +
  `Content-Disposition: attachment; filename="troubashare-dev.apk"`.
- **Tap-to-download works:** the file lands in Downloads at the exact served size
  (12,935,246 bytes, byte-for-byte). Tapping the APK then routes to the OS package
  installer (the standard sideload step; needs the browser's unknown-sources grant).

**Finding (worth a deploy/README note):** over **plain HTTP** the browser shows TWO
confirmations before saving an APK — (1) *"can't download securely"* (HTTP, not HTTPS)
and (2) *"potentially dangerous file"* (inherent to any web-sourced APK). (1) disappears
once the server is behind **HTTPS** (OPS01's self-signed/local-CA) — recommend the
deploy/README call this out so a self-hosting band expects the two taps (and that
HTTPS removes the first). (2) and the unknown-sources grant are inherent to sideloading
outside a store and can't be avoided.
