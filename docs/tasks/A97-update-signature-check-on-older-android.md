# A97 — The app update's signature check on older Android

Owner: **mobile**. Spec: Fable, 2026-11-03, from mobile's measurement while working on A96. Size: S.

## What was seen

The setup: an API 29 emulator, main's build, updating against the real `latest`, with **Update** pressed in
Parameters → About. The download completes, then `AppUpdater.verify` refuses it: "This update is signed with a
different key than the TroubaStage on this tablet, so Android would not install it over this one…".

That is not true. `apksigner` shows the same certificate on both APKs, and the same pair verifies and installs at API
34. So on that device the app can never update itself, and it gives a false reason.

VLL's tablet (Android 14) is not affected. A band member's Android 9–12 tablet may be.

## What the code does today

- `verify` compares `certDigests(installed)` with `certDigests(archive)` (`AppUpdater.kt`).
- From API 28, both read `PackageInfo.signingInfo` (`GET_SIGNING_CERTIFICATES`). The archive's comes from
  `getPackageArchiveInfo`.
- If either set is empty, the update is refused. That is right: the check fails closed. But the refusal says
  "different key", which is a guess.
- **Mobile's suspicion, not yet proven:** at some API levels, `getPackageArchiveInfo` returns no `signingInfo` for
  an archive, so "theirs" is empty.

## The work

1. **Measure the cause.** A diagnostic build logs these at API 29:
   - both digest sets;
   - whether the archive's `signingInfo` is null;
   - what `signatures` holds for the archive with `GET_SIGNATURES`.

   Then run the same at API 28, 31 or 32, and 33, to find the range. Report the logcat lines verbatim. Fix only what
   the measurement shows.
2. **Read the file's real certificate** at the affected levels. Whatever you use (for example `GET_SIGNATURES` when
   `signingInfo` is null), it must read **the downloaded file**, never a copy of the installed app's signers.
3. **Never fail open.** If a certificate set still can't be read, the update is refused with these words:
   **"TroubaStage could not check who signed this update, so it did not install it. Your notes are safe; nothing was
   changed."** The "different key" sentence is shown only when both sets were read and they differ.
4. **The rotation rule stays:** only the current signer matches (`certDigests`' KDoc).

## Acceptance

- **Unit (pure):** lift the decision into a pure function. Its inputs are our set and theirs, where either can be
  unreadable. Its outputs are install, different key, or could not check. Cases:
  - equal → install;
  - different → different key;
  - theirs unreadable → could not check;
  - ours unreadable → could not check.

  **Teeth:**
  - count "unreadable" as a match → red;
  - report an unreadable set as "different key" → red.
- **Emulator, with two CI-numbered APKs from the same key** (A96's rig):
  - **API 29:** Update from About reaches Android's confirmation. Confirm it, and the new versionCode is installed.
  - **API 34:** the same, unchanged from today.
  - **Positive control, on both levels:** an APK signed with a **different, throwaway key** is still refused with
    the "different key" words. Generate that key for the test, and never commit it. This proves the fix reads the
    file.
  - Include the logcat for the measurement and for each run.
- **CI:** the usual set, green.
- **After it lands,** A96's "repeat at API 29" step runs on top of it.

## Not in this task

- Anything A96 does (progress, the notification, "Updated ·").
- Key rotation.
