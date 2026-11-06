# WIN09 — The band QR in the Windows app

**Owner:** mobile (it owns A65's QR screen). **Reviewer:** Fable. **Size:** small to medium.
**Decided by VLL, 2026-11-06:** "Oui, sur Windows aussi". He wants the band's join QR in the Windows app too.

## Why

On the tablet, a band admin can show the band's QR on screen, and a musician scans it with their phone to join (A65 §4).
The Windows app can't show it:
- WIN08 opens Studio directly, so the native band list that carries the QR button is never shown.
- `DesktopAppPlatform.PresentQr` is a placeholder that throws "not on Windows yet".

## What to build

### 1. One QR screen for both apps

Move `QrPresentScreen` out of `androidApp` into shared JVM code (`app/shared/src/jvmSharedMain`).
- **Unchanged:** its words, its layout and A65's rules.
  - Find a suitable standing link first and reuse it.
  - Create a link only on the "Create standing QR" button, never when the screen opens.
  - State the link's terms on the same screen.
  - No revoke here; "Manage / revoke in Studio" goes to Studio.
- **QR drawing:** draw ZXing's `BitMatrix` in Compose instead of building an Android `Bitmap`. Use the same `zxing-core` from the version catalog; the desktop app adds it as a dependency.
- **Keep the screen awake while the QR is shown,** behind one small platform hook. Release it when the person leaves the screen.
  - Android: `FLAG_KEEP_SCREEN_ON`, as today.
  - Windows: `SetThreadExecutionState(ES_CONTINUOUS | ES_DISPLAY_REQUIRED)`, which the app already calls for Stage (WIN03). Reuse that code.

### 2. `PresentQr` on Windows

Replace the placeholder in `DesktopAppPlatform` with the shared screen. "Manage / revoke in Studio" opens the embedded Studio at `/bands/<id>/settings`, which is what Android does. Back returns to Studio where it was.

### 3. How a Windows user reaches it

- **The menu item:** the embedded Studio screen's ⋮ menu gets **"Show band QR…"**, shown only if the person is an admin of at least one band (`fetchStudioBands()`, `isAdmin`).
  - One admin band: the item opens its QR directly.
  - Several: a short list of those bands, with nothing else on it; choosing one opens its QR.
  - No admin band: the item isn't in the menu.
- **Coordination with web-core:** the ⋮ menu lives in `SwtEmbeddedStudioProvider.kt`, where web-core is building WIN08 stage 2.
  - Do parts 1 and 2 first.
  - Before part 3, fetch main, read web-core's latest changes to that file, and tell web-core that you are adding one menu item.
  - Keep the change to that file to the one item and its picker.
- **Doubts:** if you think the entry belongs somewhere else (the Home screen, say), say so in your presentation before you build it.

## Not in WIN09

- Scanning a QR on Windows. There is no camera flow; desktop's `qrScanner` stays null.
- Any change to how invite links work on the server, or to their terms.
- macOS and Linux. Their Studio screens aren't embedded yet.

## Proof

- **Android is unchanged.**
  - The existing tests pass.
  - Run the emulator pixel-identity check on the QR screen before and after the move: same script, fresh state for each APK.
  - Its QR decodes to the same link.
- **Windows: a CI smoke, `--win09-qr`,** in the style of the WIN08 smokes. It runs against a loopback fake server that serves the bands list and the invite links, and checks four things:
  1. The ⋮ item exists for an admin and is absent for a non-admin.
  2. The QR screen opens. A capture of the window decodes, with ZXing in the smoke, to the link the fake server served.
  3. Opening the screen sends no create request. Only the button creates.
  4. Keep-awake is on while the QR is shown and off after Back. Measure it if the runner allows (`powercfg /requests` lists display requests, and runners are admin); if it can't be measured, say so instead of asserting a seam.
- **Sabotages, each red on its own check:**
  - Draw the QR from the wrong text, or leave it blank (check 2).
  - Create a link when the screen opens (check 3).
  - Skip keep-awake (check 4, if measured).
  - Show the ⋮ item to a non-admin (check 1).
- **This touches `app/shared`,** so present with a CI run that includes `desktop (windows)` and Android.
- **For VLL, after it lands:** Windows app → TroubaStudio → ⋮ → "Show band QR…", then scan it with a phone.
