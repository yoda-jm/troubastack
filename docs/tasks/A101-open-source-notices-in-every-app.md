# A101 — The open-source notices of every library the apps ship

**Owner:** mobile (it owns `app/`). **Reviewer:** Fable. **Size:** medium. **Design first:** present the design in
`docs/handoff/reviews.md` before writing code.

## Why

WIN08 S2.8 (479cc8fc) ships SWT's licence and notices in the Windows app, under ⚙ Parameters → About → Licences. The
other libraries the apps ship have no such page, on any platform.

Many of those licences require their notice to travel with the binary:
- **Apache-2.0** (Kotlin, kotlinx, Compose, AndroidX, ktor, OkHttp/Okio, ZXing, JNA's Apache option): a copy of the
  licence, plus the component's NOTICE file where it has one (§4).
- **BSD and MIT** (inside Skia, which Skiko carries natively): the copyright line and the permission notice.
- **GPLv2 with the Classpath Exception** (the Java runtime the desktop packages bundle): the runtime's own legal texts.

The APK, the MSI, the AppImage and the DMG are all published on the `latest` release, so each of them counts as a
distribution.

S2.8 built the Licences screen as a list of entries so that this task only has to add entries.

## What to build

### 1. Measure what ships today (in the design)

For each artifact (APK, MSI image, AppImage, DMG), list what the build actually resolves and packages:
- the runtime dependencies, as `group:artifact:version` with their licence;
- the native libraries, such as Skiko's, any of AndroidX's, and JNA's;
- the Java runtime in the desktop images;
- which licence or NOTICE texts are **already** inside the artifact (inside jars, the APK's `META-INF/`, or the
  runtime's `legal/` folder) and which are missing.

Take this from the resolved build, not from memory or the version catalog. Transitive dependencies are most of the list.

### 2. A generated list, never a hand-written one

Each app's list of entries is generated at build time **from the dependencies that build resolved**. A dependency added
or bumped then appears with no one editing a list.

The mechanism is your choice: a Gradle plugin (AboutLibraries, say, which is Kotlin Multiplatform and Compose), or an
in-repo task in the style of `swtLicences`. Say why in the design. Whichever it is:
- **No network at build time for licence texts.** The texts come from the artifacts, their POMs, or committed SPDX texts.
  A build must not depend on GitHub's API, which is rate-limited.
- **Native components get entries too.** A JVM dependency list doesn't show Skia's third-party code, so name where
  those texts come from.
- **The Java runtime:** say whether jpackage's image keeps the runtime's `legal/` folder and whether that is enough, or
  whether the app needs an entry pointing to it.
- **SWT stays its own entry,** on Windows only, as S2.8 built it.

### 3. The Licences screen on every app

- **Android:** ⚙ Parameters → About gets a Licences button, and so do the Linux and macOS apps.
- **Windows:** the SWT line stays, and Licences lists SWT and the other entries.
- **Words:** propose them in the design. A UI string is a product word (British spelling, as S2.8: "Licences").
- **Grouping:** if a long list of components needs grouping or search to be usable on the tablet, propose it.

### 4. Two follow-ups from S2.8's review (GO 4c6ba5e5)

- **`SWT-about.txt`** (about.html as plain text) is a seventh shipped file. The MSI step neither lists nor checks it.
  - List it in the MSI step.
  - When the conversion turns `<a href="X">Y</a>` into text and Y ≠ X, render it as "Y (X)". Today the WebView2 NuGet link
    (https://www.nuget.org/packages/Microsoft.Web.WebView2/1.0.721-prerelease) is lost.
- **SWT-NOTICE.txt's third-party sentence is written by hand** (IJG JPEG, PuTTY, the WebView2 loader). It's right for
  3.134.0, but it can go stale on an SWT bump while about.html stays right.
  - Make `swtLicences` fail when the about.html it copies no longer names each component that sentence names.

## Not in A101

- The server, and the Studio web app it serves (its Go modules and npm packages). That is web-core's, and a separate
  question.
- iOS. No iOS app ships; the klibs only compile.
- Any change to S2.8's SWT files, its CI check, or its About line.

## Proof

- **An independent CI coverage check per artifact.** The check computes the runtime dependencies from the build and
  compares them with the list shipped **inside** the artifact. It must not compare the generator with itself. Any
  dependency without an entry reds the check. Each check prints `A101-CHECK name: PASS|FAIL`.
  - Sabotage `licencedropone` drops one entry from the generated list and reds only the coverage check.
- **NOTICE files:** every shipped component whose artifact carries a NOTICE file has that text in the list.
  - Sabotage `licencenonotice` drops one, and reds.
- **Tests:**
  - a uiTest for the Licences screen with several entries;
  - on Android, a uiTest or the emulator showing Parameters → About → Licences → one entry's text.
- **The Windows About smoke** (`--about-smoke`) still passes. Extend it to open one non-SWT entry.
- **Size:** state the APK and MSI size difference, and how long the generation step adds to the build.
- **Screenshots:** the Android and Windows Licences screens, uploaded by CI or left in your scratchpad for review.
