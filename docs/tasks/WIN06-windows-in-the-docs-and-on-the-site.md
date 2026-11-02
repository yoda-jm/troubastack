# WIN06 — Windows in the docs and on the public site

Owner: **web-core**. Starts when **WIN03 has landed**, because the MSI is then on the `latest` release. Spec: Fable,
2026-11-01, per VLL ("à la fin n'oublie pas la documentation et les pages web"). Size: S.

## Goal

When the Windows app ships, a reader of the site or the docs learns that it exists, how to install it, what it
does and does not do, and how to use Stage with a mouse and a keyboard. Every statement is true at the moment it
lands.

## What is there (main, 2026-11-01)

- `web/site/index.html`, section "2 · The tablets":
  - "Each player installs the Android app…", with a QR code and a `{{APK_URL}}` link that `web/site/build.sh`
    resolves from the `latest` release;
  - the "Where packaging stands" paragraph ("The Android app is not in a store yet: it is a test build…").
- `docs/USER-JOURNEY.md`: the install step and the Stage steps, written for a tablet.
- `README.md`: packaging status.
- `docs/glossary.md`: the product words.

## Changes

1. **Site.**
   - Next to the Android download, add a Windows download: a plain link (no QR), through `build.sh` like
     `{{APK_URL}}`, to the stable asset name WIN03 chose (for example `troubastage-windows.msi`).
   - A link to an asset that is missing must fail the site build, not ship a dead link.
   - The section title must not say tablets only.
   - In "Where packaging stands", one sentence: the Windows installer is not signed yet, so the browser or
     Windows may warn before running it (keep it, or run it anyway); it installs for the current user without an
     admin prompt. **Amended at the WIN06 gate (Fable, 2026-11-02):** no screenshot of the site download exists, so
     no button names are quoted. They come in a one-line follow-up from VLL's Windows machine pass, which also
     confirms that no administrator prompt appears (CI can't prove that, because its runner is an administrator).
   - Studio on Windows: the app shows "Coming soon" there, and Studio is used in the browser. Write exactly what
     WIN05's outcome makes true at landing time.
2. **User docs** (`docs/USER-JOURNEY.md`):
   - A "On Windows" install paragraph.
   - A short "Stage with a mouse and a keyboard" section, from WIN03's D2–D4 as they landed:
     - click shows and hides the controls;
     - drag or wheel turns pages, with the one-gesture-one-page rule;
     - width and scroll modes;
     - F11 and the full-screen button, with the "Full screen when performing" setting;
     - Esc, the ✕ that leaves a concert, and that the cursor hides;
     - note mode with the mouse;
     - learned pedal keys, and the keys that cannot be learned.
3. **README**: packaging status adds the Windows MSI and where CI builds it.
4. **Glossary**: any new product word WIN03 introduced, in the glossary's format.

## Rules

- **Labels are product words.** Every UI label quoted in the site or the docs is copied from the shared strings,
  and the submission shows each one being found by a grep in `app/shared/src`. No paraphrased button names.
- Screenshots, if any: the demo concert only (the repo is public).
- Later steps of the track carry their own doc line in their own task: WIN04 (the app updates itself), a WIN05
  implementation, Linux/Mac if they come. WIN06 does not pre-announce them.

## Acceptance criteria

1. `web/site/build.sh` builds the site, and both download links resolve against the real `latest` release.
   Sabotage: a wrong asset name fails the build.
2. The label grep from Rules, with every hit listed.
3. The rendered site section and the edited docs, read at the gate. The site in light and dark, at phone width.
4. No statement about something that has not landed.
