# WIN08 — Studio inside the Windows app (WebView2), starting with the stage VLL checks

Owner: **web-core**. Spec: Fable, 2026-11-03. VLL decided on his decisions page: "Oui, WebView2, en commençant par
l'étape à valider". Size: stage 1 M; the whole task L.

## Where it starts

WIN05 measured three ways to host Studio in the desktop app. Its report and two deltas are in
`docs/handoff/reviews.md`, 2026-11-02, from "web-core: WIN05 embedded-Studio spike — REPORT" through "WIN05
delta 2". The decision: **WebView2 through Eclipse SWT's `Browser(SWT.EDGE)`**
(`org.eclipse.platform:org.eclipse.swt.win32.win32.x86_64`, EPL-2.0; the spike used 3.134.0).

**What is proven, on `windows-latest`, in a bare SWT `Shell` on a scratch page:**
- +2.78 MiB on the MSI;
- about 289 MiB of memory, summed over the process tree;
- the WebView2 environment is created in about 80 ms **only with a writable `EdgeDataDir`**. With SWT's default
  data dir it never finishes, and SWT's 5 s wait fires (run 41443342422; the timeout was never the cause);
- `Browser.setCookie` works once a Browser exists;
- the bridge round-trip works (`BrowserFunction` web→app, `evaluate` app→web);
- `LocationListener.changing` with `doit = false` vetoes a navigation.

**What is NOT proven, and is why stage 1 exists:**
- SWT's Browser **inside the TroubaStage (Compose) window**, with two UI threads;
- the app's own popups over it;
- real Studio loading with the app's session;
- pen, keyboard and focus inside the window.

SWT also has **no load-error event**. A failed top navigation skips `LocationListener.changed`, and nothing else
reports it, so the host has to detect errors itself.

**The code it replaces:**
- `StudioComingSoon` in `app/desktopApp/.../DesktopAppPlatform.kt` (WIN03 D6);
- the loud-failing desktop `actual` of seam 1, `app/shared/src/desktopMain/.../WebViewHost.desktop.kt`.

**The bar to match is Android's `EditScreen`** (`app/androidApp/.../EditScreen.kt`):
- the session is seeded through `sessionCookieFor` (only a session this origin issued);
- it loads `embeddedUrl(server, path)`;
- ⋮ Reload goes through A93's `reloadUrl(current, entry)`;
- it shows the error cover with Retry;
- Back walks Studio's history first;
- changing the server drops the session (`dropSessionIfOriginChanged`).

## Stage 1 — Studio in the window, for VLL's pen check

Stage 1 is built, reviewed at the gate, and then **checked by VLL on his Windows machine before it lands**. He
installs the PR run's MSI (the `troubastage-windows-msi` artifact). Stage 2 starts only after his check.

### S1.1 The host

- **On Windows**, the TroubaStudio screen shows Studio **inside the TroubaStage window**:
  - the app's bar on top: ‹ Back, the title (the band when opened from a band, as A65 does), and ⋮ with
    **Reload**;
  - Studio fills the rest of the window, and follows resize, maximise and F11 full screen.
- **Linux and macOS keep "Coming soon."** exactly as they are. The SWT Windows jar must not reach their builds:
  choose the dependency by the host OS at build time. The Linux and macOS jobs must show that their app image
  holds no SWT jar.
- **Embedding:** your choice. SWT_AWT in a heavyweight canvas inside a `SwingPanel`, or a child HWND, or anything
  else. Say which, and why. State the **threading model**:
  - which thread owns the SWT `Display`;
  - how calls cross between that thread and Compose's;
  - that nothing calls SWT off its thread, and nothing writes Compose state off Compose's.
- **Leaving and re-entering** the Studio screen disposes everything and works again. **Closing the window with
  Studio open ends the process** within the smoke's timeout. WIN04's updater waits for TroubaStage to close, so a
  leftover SWT thread would break updates.

### S1.2 Where it writes

- **`org.eclipse.swt.browser.EdgeDataDir`** points to a folder under the app's per-user data root (WIN03's
  data-root seam). Never the temp dir, never the install dir.
- **SWT's own native libraries** load from the install dir, or are extracted under the data root
  (`swt.library.path`). **Never into `~/.swt`.** This is WIN07's lesson, from Skiko's shared unpack. The same goes
  for test tasks.

### S1.3 Signed in, on the right page

- Seed the session with `Browser.setCookie` for the server's origin, after the Browser exists and before the
  first load. Take the value **only** from `sessionCookieFor`.
- Load `embeddedUrl(server, path)`. Studio opens **signed in**: no Studio login screen.

### S1.4 Locked to the server

- **Navigation:** a navigation to any other origin is vetoed.
- **The bridge** acts only when the current page is on the server's origin.
- **New windows:** say what a new-window request does (`target=_blank`, `window.open`; the file-name link in a
  song's file list is one). Do the same for a download: T208's "Download PDF…" saves a Blob through an anchor.
  Stage 1 only has to report both. Stage 2 makes them right.

### S1.5 The bridge

Studio's handshake (`web/studio/src/bridge.ts`) completes:
- Studio finds `window.TroubaStageShell.receive` **at startup**. `initShellBridge` checks it once, so the shim must
  exist before Studio's script runs, not after the load. Say how you made sure;
- the app logs Studio's `ready`;
- the app's `hello`, sent through `window.__troubastageShell.deliver`, gets `ready` back.

### S1.6 No WebView2

If `Browser(SWT.EDGE)` throws, or `getBrowserType()` isn't `"edge"`, the screen shows:
- **"Studio can't open here: this copy of Windows doesn't have Microsoft Edge WebView2."**
- **‹ Back**.

It never falls back to Internet Explorer and never crashes. A CI case forces this path (for example a system
property that makes the host treat the engine as missing), so the words are tested.

### S1.7 Keyboard, focus and popups

- Typing goes into Studio's fields.
- **Esc pressed inside Studio reaches Studio.** For example it closes T208's Download PDF dialog. It doesn't
  leave the screen, and it doesn't leave full screen.
- Say what F11 does while focus is inside Studio, whichever way it goes.
- **The app's ⋮ menu opens over Studio and is fully visible.** A heavyweight native window paints over Compose's
  in-window popups, so this needs an explicit answer.
- **High DPI:** say the DPI-awareness mode, and how Studio's bounds follow the window at a scale other than 100 %.
  The runner is at 100 %; VLL's Surface is not.

### S1.8 Proof

1. **Main's CI, `desktop (windows)`:** a new step launches the packaged app into the Studio screen against a
   loopback test page (no core needed) and checks each of these, with its own marker line:
   1. the engine is Edge;
   2. the Browser's top-level window is the TroubaStage window: `GetAncestor(…, GA_ROOT)` equals the frame's HWND;
   3. the seeded cookie arrives in the loopback server's `Cookie` request header;
   4. the bridge round-trips;
   5. a navigation to another origin is vetoed;
   6. the data dir exists under the data root, and `~/.swt` doesn't exist;
   7. the process exits after the window closes.

   **Sabotage each check red at least once** and paste the step conclusions (job logs are admin-only for me), as
   for WIN07. Include the S1.6 case.
2. **A throwaway workflow on `windows-latest`, with real Studio.** Build the core with Studio's SPA, run it on a
   scratch data dir with **demo data only**, connect the app to it, and open a demo song's editor in the
   TroubaStage window. Take full-window screenshots of:
   - Studio signed in, inside the window, with the app's bar above it;
   - the ⋮ menu open over Studio;
   - a Studio dialog open, then gone after Esc;
   - text typed into a Studio field;
   - a stroke drawn by a scripted pointer drag in the editor (`SendInput`), before and after.

   Also measure memory with Studio open (process-tree sum) and the MSI delta. Delete the branch afterwards.
3. **Linux and macOS:** their jobs stay green, "Coming soon." is still tested, and their app image has no SWT jar.
4. **The rest of CI** is green, `desktop (windows)` included.

### S1.9 VLL's check (after my GO on stage 1, before it lands)

I give him the PR run's MSI link and these steps. He uses his Surface for the pen, and the VM too if it's ready.
1. Install the MSI over the current TroubaStage.
2. Connect to his server as usual, then Home → **TroubaStudio**.
3. Studio opens signed in, inside the window.
4. Open a song and draw with the pen. Then draw on the same song in **Microsoft Edge** on the same machine, and say
   whether the feel is the same.
5. Resize, maximise, press F11. Open ⋮. Open a Studio dialog and press Esc.
6. Say whether anything is cut off, blurry or the wrong size. If the Surface is an ARM model, say so.

If he says the pen is worse than in Edge, we stop and look at why before stage 2.

## Stage 2 — the rest (specced in full after VLL's check)

The outline, so the size is honest:
- **Errors:** the error cover with **Retry**, detected by the host, since SWT has no load-error event. A93's
  words; Retry goes through `reloadUrl`.
- **Navigation:** Back walks Studio's history first, as on Android. Mouse back button too.
- **Server changes:** "Server URL…" and dropping the session when the origin changes. The WebView2 profile's
  cookies are cleared through the data dir or SWT's API.
- **New windows and downloads**, from S1.4's report: downloads land where a person expects, with a visible result.
- **Licences:** EPL-2.0 for SWT. Ship the licence text and the source-availability notice with the MSI, and say
  where the app shows them.
- **Words:** USER-JOURNEY, the site's Windows paragraph and the README lose "Coming soon" for Windows. Linux and
  macOS keep it. Every quoted label is grepped from the code.
- **CI:** the stage-1 smoke stays, and gains the error cover.

## Not in this task

- Studio inside the Linux or macOS app.
- JCEF.
- The native ink overlay (A07).
- Changing Studio itself. If the bridge needs a Studio change, report it; the change gets its own task.
- Code signing. That still waits for WIN04's install screenshots.
