# A96 — The tablet's app update: progress where you pressed Update, and the app comes back after it

Owner: **mobile**. Spec: Fable, 2026-11-03, per VLL's note on his decisions page after updating his tablet from the
app. Size: S.

> "currently update is not working so I canot check the refresh. My bad it was just super long, maybe we need a
> download progress bar or something like that, some statust sure the app restart after an update from the app.
> Reload validated BTW"

So the update worked. It looked broken because nothing on screen said it was happening, and VLL wants the app to
come back by itself after it updates. (The "Reload" he validated is A93's ⋮ Reload. Nothing to do there.)

## What is true on main today

- **Home's banner** shows progress: "Downloading the app update… N %", a `LinearProgressIndicator` and Cancel
  (`HomeScreen.kt`, the A88 banner).
- **Parameters → About shows nothing.** Its **Update** button (`SettingsScreen.kt`, the About section) starts the
  same download, but the row keeps saying "Latest: … · built …", and Update stays enabled. A second tap is a silent
  no-op (`AppUpdateController.update`: `if (job != null) return`). Pressed from About, a ~15 MB download is
  invisible from start to finish. This is the most likely reason it looked broken. Check this reading before you
  build on it: ask whether VLL pressed Update in Parameters or on Home, through me, and say what you found.
- **After the download**, nothing says what happens next. Verification and the PackageInstaller session write show
  no status, and then Android's own confirmation appears. On VLL's tablet (Android 14, MIUI), Xiaomi's installer
  can add its own scan before installing. We can't show anything inside that step, but we can say before it that
  Android takes over.
- **"The app restarts on Home"** is claimed by A88 §2 step 4 and by two code comments (`UpdateInstallReceiver`'s
  KDoc and the `install` call in `AppUpdateController.update`). Nothing in the code makes it happen. When an app
  installs an update to itself, Android ends the app's process, and nothing reopens it.

## The feature

### 1. The status shows wherever Update was pressed

- **About:** while an update is in progress, the row shows the same status Home's banner shows. That means the
  same words, a progress bar under the row, and **Cancel** in place of Update. Use one source for the words: lift
  the banner's line into one shared function that both screens call. Don't copy it.
- **Both surfaces** show every phase, with these words:
  1. **"Downloading the app update… N %"** (today's words, unchanged).
  2. **"Checking the download…"** while `verify` runs.
  3. **"Android will ask you to confirm the update."** from the session commit until the process ends, or until
     the session reports failure or abort. No progress bar in this phase. Android owns this step, so we don't
     pretend to know how long it takes.
- **A failure or abort** (the session reports `STATUS_FAILURE*` or `STATUS_ABORTED` after the confirmation, for
  example when the person taps Cancel in Android's dialog) returns the row to Update. One line explains it: **"The
  update was not installed. Nothing was changed."** Today those statuses are ignored, and the row would stay in
  phase 3 forever.

### 2. The app comes back after it updates

**Measure first.** Don't assume Android allows a relaunch. On an emulator at API 34 (the tablet's level), and at
API 29, find out what happens after a confirmed self-update with today's code. Then find out what each mechanism
below actually does. Report it with logcat lines (`ActivityTaskManager` "Background activity start … blocked", or
the activity starting).

Use the **first mechanism that the measurement proves works on API 34**:

1. **Relaunch:** a manifest receiver for `Intent.ACTION_MY_PACKAGE_REPLACED` (or the session's `STATUS_SUCCESS`
   callback, if it reaches the new version) starts the launcher activity, and the app opens on Home.
2. **One tap:** if Android blocks the relaunch, that receiver posts a notification instead: **"TroubaStage was
   updated"** / **"Tap to open."**, which opens Home.
   - On API 33 and up, posting needs `POST_NOTIFICATIONS`. Ask for it only when the person presses Update, with
     one line before Android's prompt: **"Allow a notification so you can reopen TroubaStage when the update is
     done."**
   - If they refuse, the update still goes ahead. Phase 3's line then ends with **" Then open TroubaStage
     again."**
3. If neither works, say so at the gate with the logcat. Don't invent a third mechanism.

MIUI can hold back what stock Android allows (its per-app Autostart setting can stop receivers). The emulator
can't show that. VLL's pass on his tablet is the check, and if it fails there, we follow up from what he sees.

**Once the app opens after an update**, Home shows **"Updated · <the A84 build line>"** one time. Desktop already
uses this line (WIN04: `DesktopAppUpdate.kt`, `HandoffResult.Updated`). Use the same words in the same kind of
place. A banner the person dismisses is fine; it must not come back on the next launch. Detect "after an update"
by comparing the stored last-run build with the running one, so an `adb install -r` also counts.

### 3. Correct the claims

- Fix the two code comments and A88 §2 step 4 so they say what the code now does.
- A88's device acceptance step 3 ("Update → confirm → restart") becomes "→ the app reopens (or the notification
  opens it)".

## Not in this task

- The download speed. GitHub serves the file; we only show progress.
- Silent updates (`USER_ACTION_NOT_REQUIRED`). The app isn't the installer of record on a sideloaded tablet.
- Windows, Linux and macOS. WIN04 and DESK01 already own those flows.
- Any change to Stage. The update is never offered during a concert (A88's gate, unchanged).

## Acceptance

- **Unit (pure, shared):** the phase-to-words function covers each phase plus failure and abort. **Teeth:** map
  phase 3 to the download line, and the test goes red.
- **UI test:** About during a fake download shows the status, the bar and Cancel, and not Update. **Teeth:** remove
  the About branch, and the test goes red. Home's banner keeps its current test.
- **The "Updated ·" line:** shown once after the build changes, and not on the next launch. Not shown on a first
  install, or when the build is unchanged. **Teeth:** drop the stored build write, and "shown once" goes red.
- **Emulator, API 34, with two CI-numbered APKs from the same key** (old installed, new offered through a local
  `latest` stand-in, or whatever A88's tests used; say which):
  1. Press Update **in About**, and screenshot each phase.
  2. Confirm in Android's dialog.
  3. Screenshot what comes back: the app on Home with "Updated · …", or the notification and Home after the tap.
  4. Cancel in Android's dialog on a second run: screenshot the "not installed" line.

  Include the logcat for step 3. Repeat step 3 at API 29.
- **Pixels:** Home and About with no update in progress are identical to main (the A93/A95 method: main's APK vs
  the head's, same journey, fresh state).
- **CI:** the usual set, green, `desktop (windows)` included, since `app/shared` changes.
- **VLL's tablet** (his pass, after landing): update from About, see each phase, and the app comes back (or one
  tap on the notification opens it). The lane never touches the tablet.
