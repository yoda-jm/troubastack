# A53 — Scan the invite: a camera in the app

**Lane:** Mobile · **Kind:** new feature · **Verified against `b6d23b7`**
**Depends on:** A51 (grammar + decision) and A52 (the join flow this feeds). **Requires T123.**
**Files:** `app/gradle/libs.versions.toml`, `app/androidApp/build.gradle.kts`, `AndroidManifest.xml`,
a new scanner composable next to `ConnectScreen.kt`.

## Goal

Point the tablet at the QR the studio already renders, and land in the band. The scanner's only job is to
produce a **string**; everything after that is A51 and A52, already built and already tested.

**This task therefore contains almost no logic — and that is the design.** Resist putting any join
handling in the camera path.

## Why T123 is a hard dependency here

A52's paste path has a human reading a URL before pasting it. The scanner deliberately removes that human.
`ConfirmServer` showing a hostname to someone who just pointed a tablet at a sticker is weak protection;
proving the host is a TroubaStack server before a password field appears is the real one.

## Deliverable

### 1. Dependencies — CameraX + ZXing, via the catalog

Every dependency in this project goes through `libs.*`; add them there, not inline. Repositories are locked
(`FAIL_ON_PROJECT_REPOS`, google + mavenCentral — `app/settings.gradle.kts:27-33`), and both live in those.

- **CameraX** (`camera-core`, `camera-camera2`, `camera-lifecycle`, `camera-view`) for preview and frames.
- **`com.google.zxing:core`** for decoding, in an `ImageAnalysis.Analyzer`.

**Why not ML Kit:** the unbundled barcode scanner needs Google Play Services, and this APK is sideloaded
onto whatever tablet is on the music stand; the bundled model is several MB for one barcode format. ZXing
core is Apache-2.0 — the project's own licence (`7763839`) — works fully offline, and is small. If CameraX
+ ZXing turns out to be more analyzer plumbing than it is worth, say so at the gate rather than
substituting quietly; `zxing-android-embedded` wrapped in `AndroidView` is the accepted fallback.

`minSdk 26` clears all of these.

### 2. Permission and the refusal path

```xml
<uses-permission android:name="android.permission.CAMERA" />
<uses-feature android:name="android.hardware.camera.any" android:required="false" />
```

`required="false"` matters — a camera must never become an install requirement for a music stand.

Request at the point of use, not at launch. **A denial is a normal outcome, not an error state**: fall
back to A52's paste field, which already works. Never dead-end.

### 3. The scanner

- `ImageAnalysis` with `STRATEGY_KEEP_ONLY_LATEST`; decode the luminance plane.
- **`imageProxy.close()` in a `finally`.** Miss this and the pipeline stalls after two frames — the preview
  keeps rendering and nothing ever decodes, which reads as "the scanner just doesn't work".
- **Stop on dispose.** Unbind in a `DisposableEffect`; the camera indicator must go out when the screen
  leaves. A camera left running on a device on a stage is not acceptable.
- **First decode wins**, then stop analysing — otherwise a held-up code fires the join repeatedly.
- Handle **landscape**: the gig device is a tablet on a stand, and it is probably not in portrait.
- Hand the decoded string straight to `parseTroubaLink`. The scanner does not inspect it.

## Teeth-check

The camera path cannot be unit-tested, so the check is adversarial and manual — run it and report it:

**Print or display a `/join/<token>` URL pointing at a host that is not your server, and scan it.** The app
must name that foreign host and must not offer a password field (T123 having refused it). If it sails
through to a login form, the feature is not shippable.

Also confirm by hand: permission denied still reaches the paste field; leaving the screen turns the camera
off; scanning the same code twice does not double-redeem.

## Verification

Device only, and say so plainly — **there is no automated coverage of the camera path and this task does
not add any.** Report what you actually did on hardware, including the adversarial scan above. A A51/A52
suite total unchanged from their landings is the expected result here; report it rather than leaving it
implied.

## Out of scope

iOS (AVFoundation would be a separate task; nothing here may break the iOS build — keep the scanner in
`androidApp`, where `ConnectScreen` already lives, so `commonMain` stays platform-free) · scanning anything
other than an invite link · generating QR codes in the app.
