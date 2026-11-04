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

**Amended 2026-11-04, VLL's call:** stage 1 lands without his pre-land check. He checks the released build
instead, and stage 2 is specced now rather than after his check. If his pen check finds Studio worse than Edge,
that finding goes ahead of stage 2.

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

**Amended 2026-11-03 at the design GO.** A BrowserFunction's name must be a bare identifier, so SWT can't present
`window.TroubaStageShell.receive`. `bridge.ts` may therefore also accept a function-shaped shell:
`window.TroubaStageShellReceive`, used when `window.TroubaStageShell` is absent. That additive change is the one
Studio change WIN08 allows. The GO entry in `docs/handoff/reviews.md` has its tests.

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
- F11 toggles full screen from inside Studio too: it is the app's full-screen key (amended at the design GO).
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

### S1.9 VLL's check (after my GO on stage 1; VLL chose on 2026-11-04 to land first and check the released build)

I give him the PR run's MSI link and these steps. He uses his Surface for the pen, and the VM too if it's ready.
1. Install the MSI over the current TroubaStage.
2. Connect to his server as usual, then Home → **TroubaStudio**.
3. Studio opens signed in, inside the window.
4. Open a song and draw with the pen. Then draw on the same song in **Microsoft Edge** on the same machine, and say
   whether the feel is the same.
5. Resize, maximise, press F11. Open ⋮. Open a Studio dialog and press Esc.
6. Say whether anything is cut off, blurry or the wrong size. If the Surface is an ARM model, say so.

If he says the pen is worse than in Edge, we stop and look at why before stage 2.

## Stage 2 — Studio on Windows behaves like Studio on the tablet

Specced 2026-11-04, after stage 1 landed. **The bar is still Android's `EditScreen`.** Each item names what Android
does, or says why Windows differs. Stage 1's smoke, guards and sabotages all stay.

### S2.1 Loading, and a server that can't be reached

SWT has no load-error event, so the host decides for itself. **Studio's own `ready` over the bridge is the signal
that Studio loaded.**
- **While loading:** "Loading…" in the Studio area, as on Android, until `ready` arrives. Today the area is blank.
- **A load that fails:** the server is unreachable, or a navigation completes and no `ready` arrives within 10 s.
  Show Android's cover in its words:
  - **"Can't reach the server"**;
  - the reason, in one line;
  - **Back** and **Retry**.

  Retry goes through `reloadUrl`, so it retries the page that failed (A93).
- **Before the first load,** check that the server can be reached through the app's own `HttpTransport`. A server
  that's down shows the cover at once, not after 10 s.

### S2.2 Back

- **‹ Back in the app bar, and the mouse's back button,** first walk Studio's own history
  (`browser.back()` while `isBackEnabled`). They leave the screen only when there's nothing left, as Android's
  `BackHandler` does.
- **Alt+←** inside Studio does the same.

### S2.3 Changing the server, and signing out

- **⋮ gains "Server URL…"**, with Android's dialog: **"Core server URL"**, Save and Cancel.
- **Saving a different origin** calls `dropSessionIfOriginChanged`. Desktop's `clearCookies()` stops being a no-op:
  it clears the WebView2 profile's cookies, through SWT's API or the data dir. Say which, and measure it.
- **Signing out of the app** clears them too.
- **The proof, in the smoke:** after a server change or a sign-out, reopen Studio. The loopback server's `Cookie`
  header must no longer carry the old session.

### S2.4 Links that open a new window

Stage 1 blocks them all. Stage 2 routes them by origin:
- **The server's origin** (for example, a file-name link in a song's file list) opens in the embedded view, in place.
  Android's WebView does the same with `target=_blank`.
- **Any other origin** opens in the person's default browser, **without** the session.
- **No new window ever opens inside the app.**

The proof: one smoke vector for each origin. The same-origin one ends on the right page. The other one ends with the
default browser invoked (a stub in CI) and no new window in the process.

### S2.5 Downloads

Keep WebView2's default: its own Downloads panel, saving to the person's Downloads folder. That's what a person
expects from Edge, and it already works. Prove it with **T208's real "Download PDF…"**, reached from the song's file
list, not with a probe:
- the panel appears;
- the file in Downloads is the PDF, with its name and size.

### S2.6 The right-click menu and browser keys

- **Measure WebView2's default right-click menu** in Studio, on the chart and in a text field, and screenshot both.
- **Keep text editing** (cut, copy, paste, select all). **Drop** page items that duplicate the app or leave it:
  Back, Reload, Save as, Print, Inspect. Use SWT's menu hooks (`MenuDetect`, or the Browser's menu), and say which.
- **Stage 1's key results stand:** F12 and Ctrl+Shift+I open nothing, and Ctrl+P prints nothing. Keep them as smoke
  checks.

### S2.7 The ⋮ menu looks like the app's

Stage 1's menu is a plain Swing window. Give it the app's look, for example a `ComposePanel` inside the same owned
window. It must stay **non-activating and never topmost**: L1's dismissal and the `menu-no-activation-steal` check,
with its sabotage, stay as they are.

### S2.8 Licences

- SWT is **EPL-2.0**. Ship its licence text and a source-availability notice with the MSI.
- Parameters → About gets one line pointing to them. The words are yours; I'll check them.

### S2.9 Words

- **USER-JOURNEY, the site's Windows paragraph and the README** lose "Coming soon" for Windows, and say what's there:
  Studio inside the window, plus S2.4's rule for links.
- **Linux and macOS keep "Coming soon."**
- Every quoted label is grepped from the code.

### S2.10 Report only

- **A session that expires inside Studio:** what Studio shows, and whether signing in there leaves the app's stored
  session stale.
- **Whether functions injected with `AddScriptToExecuteOnDocumentCreated` run in cross-origin iframes.** Stage 2
  adds no bridge actions, so this stays a report, needed before any task that does.

### S2.11 What it costs

Stage 1 measured **614 MiB** with real Studio open: TroubaStage plus its `msedgewebview2` children, found by parent
pid. WIN05's 289 MiB was a bare test Shell on a scratch page, so the two don't compare. Measure, on the same runner,
in the same job:
- **TroubaStage on Home**, with Studio never opened;
- **TroubaStage with the demo song open in Studio** (stage 1's number);
- **Microsoft Edge on the same demo song**, signed in (Edge's own process tree);
- **after leaving and re-entering Studio 5 times:** the tree must come back near its first-open size. A climb is a
  leak.

Report the four numbers, and the MSI size against main (stage 1: +2.86 MiB).

### Not in stage 2

- **The band's room QR on Windows** (A65's present-QR screen is Android-only code). It's a question on VLL's page,
  not part of this stage.
- **F11's title bar:** depends on VLL's check. If it stays on Home too, that's a DESK01 follow-up for all of
  Windows.

### Proof

- **Each item above gets a smoke check with its own sabotage,** in `desktop (windows)`: S2.1's cover, S2.2's history
  walk, S2.3's dropped cookie, S2.4's two vectors, S2.6's menu.
- **Screenshots, from the real-Studio workflow:**
  - Loading…;
  - the cover;
  - Back walking history;
  - the server dialog;
  - the download panel;
  - both right-click menus;
  - the restyled ⋮ menu with the title bar active.
- **Linux and macOS:** unchanged, still "Coming soon.", and still no SWT jar.
- **The rest of CI** is green, `desktop (windows)` included.

## Not in this task

- Studio inside the Linux or macOS app.
- JCEF.
- The native ink overlay (A07).
- Changing Studio itself, except S1.5's additive change in `bridge.ts`. Any other Studio change the bridge needs
  gets its own task.
- Code signing. That still waits for WIN04's install screenshots.
