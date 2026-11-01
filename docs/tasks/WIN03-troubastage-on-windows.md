# WIN03 — TroubaStage on Windows: Stage in a window or full screen, driven by mouse and keyboard; Studio says "Coming soon"

Owner: **mobile**. Starts when **WIN01 and WIN02 have both landed** (they have: 6e486bba, 2b9ffed6), after A94.
Spec: Fable, 2026-11-01. Size: L. **Read "Additions from the gate" at the end first**: it supersedes D1 where they differ.

## Why (VLL, 2026-11-01)

> "refactor en premier puis juste stage avec studio qui fait un 'bientôt' […] Pense au full screen et windowed,
> pense à l'interaction à la souris dans stage et ce genre de choses."

## Scope in one paragraph

The Windows app does what the Android app does, apart from Studio. It shows Home, connects and signs in, joins
with a pasted invite link, downloads and updates concerts, imports a `.tstage`, and lists, sends and clears
rehearsal notes. It performs a concert in Stage (all reading modes, drawer, note mode, clock and chrono, count-in,
identity), and it has Settings. **TroubaStudio shows "Coming soon".** It runs in a normal window and in full
screen, and every Stage interaction works with a mouse and a keyboard. It runs offline exactly as on Android
(I12).

## Design (decided — changes go through the gate)

### D1 — Platform capabilities on Windows (WIN01's interface)

| Capability | Windows |
|---|---|
| `storage`, `unpackBundle` | WIN02's actuals |
| `imageDecoder`, `rehearsalNotes` | Skia (decode/encode PNG), the same contracts as the Android implementations, with their tests run in `desktopTest` |
| import a `.tstage` | the system file dialog, filtered to `.tstage` with an "All files" fallback |
| `studio` | **"Coming soon"** (D6) |
| `qrScanner` | `null` (hidden: no camera flow; pasting the invite link in Connect still works) |
| `discovery` | `null` (no LAN discovery in this task; typing the address works) |
| `pedal` (BLE-MIDI) | `null` (Settings hides the MIDI section). **Keyboard-type pedals and learned keys work** (A72) |
| `appUpdate` | `null` here; WIN04 adds it. Settings still shows the build line |
| `pendingInviteLink` | `null` (no `/join/` deep links on Windows) |
| `volumeTurnRegistrar` | `null` |
| `resumeTicks` | the window gaining focus (A31's Home re-probe) |
| clocks | wall clock: `System.currentTimeMillis`; the clock face: the default locale's short time format; monotonic: the chrono contract (T147) says it counts deep sleep. **Check** whether your source does on Windows and state it; do not assume |

### D2 — Window modes

- **Windowed** is the default at launch. Minimum size 800×600.
- Size, position and maximized state are remembered across launches. If the saved position is no longer on a
  connected screen (laptop unplugged from its monitor), the window opens centred on the primary screen.
- **Full screen** is borderless and covers the taskbar, on the screen the window is on.
  - **F11** toggles it on every screen of the app.
  - In Stage, a **full-screen button** in the chrome toggles it too.
- **Entering a concert** goes full screen when the new Settings switch **"Full screen when performing"** is on
  (default **on**, the analog of Android's immersive Stage).
- **Leaving the concert** (✕) restores the mode the window had when the concert was entered.
- Toggling inside a concert does not change the Settings default.
- **Resizing in Stage** re-lays out live. Two-up follows the window's shape (A12 keys off width > height), and
  the reading position is kept when the layout flips.
- While a concert is open, the display does not sleep and the screen saver does not start
  (`SetThreadExecutionState` with display-required, scoped to Stage like `StageHost`; a no-op off Windows).
- **One instance.** A second launch does not open a second app writing the same files. It brings the
  running window to the front, or, if that is not feasible, says "TroubaStage is already open" and exits.
- **Closing the window while a concert is open asks first** (an accidental close mid-show is the A79 failure
  again). Nothing committed is lost on close: notes, the reading position, the chrono.

### D3 — Mouse in Stage

- **Click** on the page toggles the chrome, exactly as a tap (N3). Moving the mouse does **not** reveal the
  chrome.
- **Drag** horizontally with the left button turns the page like a swipe, where a swipe turns today (page and
  width modes).
- **Wheel**:
  - **Page mode, and two-up:** one wheel gesture turns **at most one page** (or one spread). Down/right is next,
    up/left is previous. A gesture is a burst of wheel events: a trackpad or a free-spinning wheel sends dozens,
    and must still turn exactly one page. The burst ends after ~250 ms without wheel events. Put the rule in a
    **pure, shared function with unit tests** (burst → one turn; two bursts → two turns; a reversal inside a burst).
  - **Width mode:** the wheel scrolls the page, as it does natively. At the bottom (top) edge, a **new** gesture
    downward (upward) turns to the next (previous) page. The tail of the burst that reached the edge never turns.
  - **Scroll mode:** the wheel scrolls natively. It never turns.
  - **Note mode:** the plain wheel does nothing; **Ctrl+wheel zooms** with A86/A87's zoom steps. Outside note mode,
    Ctrl+wheel does nothing.
  - **"Lock swipe"** also locks drag and wheel turns. Keys, the pedal and ‹ › still turn. The label stays "Lock
    swipe" (one word for one concept).
  - These rules live in shared `StageScreen`. An Android tablet with a mouse gets them too, which is intended;
    touch is untouched (wheel events come only from a mouse).
- **The cursor hides** over the page after **2 s** without movement, in both window modes, and shows again on any
  movement. It never hides in note mode, over the chrome, or while a dialog or the drawer is open. In note mode it
  is a crosshair.
- **Note mode:** the left button draws, and the eraser tool erases with the left button. No other buttons do
  anything.
- **Right, middle and side buttons**, and double-click: no action in Stage. Side buttons never leave a concert
  (A79).
- Every glyph-only chrome button shows its label (its existing content description) as a tooltip on hover, on
  Windows. Android does not change.

### D4 — Keyboard

- The page-turn keys are A09's, unchanged (`stageKeyAction`).
- **After any mouse click in Stage** (the page, a chrome button, the drawer, a dialog), the keyboard and a
  keyboard pedal **still turn pages**. Focus must come back to the Stage. This is the most likely desktop
  regression, so it gets a test.
- **Esc in Stage:** it closes the drawer if one is open; otherwise it leaves full screen; otherwise it does
  nothing. Esc never leaves a concert; ✕ does (A79). Extend the shared `stageBackAction` decision, with tests,
  rather than adding a second rule.
- **Esc elsewhere:** it closes the open dialog, otherwise it does what Android Back does on that screen. Alt+Left
  and the mouse Back button do the same, outside Stage only.
- **F11** and **Esc** are window controls on Windows. They cannot be learned as pedal keys (extend A72's
  `isLearnableKey` with a per-platform reserved set; Android's set is unchanged).
- Text fields (Connect, sign in, invite link): typing, Ctrl+V and Enter-to-submit all work.

### D5 — Wording

A string changes on Windows **only where it is false there**, and each variant is listed at the gate (a label is
a product word). Known so far:
- `StageScreen` note-mode dialog: "Touch will draw, not turn pages. Pages still turn with a pedal or the volume
  keys." On Windows it becomes **"The mouse will draw, not turn pages. Pages still turn with a pedal or the
  keyboard."**
- `SettingsScreen` "For a MIDI pedal, tap Connect above first." disappears with the MIDI section.

"Tap" elsewhere is understood with a mouse; leave it. Sweep the shared strings for other false ones
(`grep -rnoE '"[^"]*\b([Tt]ap|[Ss]wipe|[Vv]olume|[Tt]ouch|[Pp]inch)\b[^"]*"' app/shared/src`) and list the result.
New strings: "Full screen when performing", the close confirmation, the "already open" notice and D6. Give
their exact text at the gate.

### D6 — Studio: "Coming soon"

Every way into Studio goes through WIN01's one `studio` capability: the Home TroubaStudio card, and any row or
menu that opens the WebView or the room QR. On Windows it shows one screen with the TroubaStudio heading and
**"Coming soon."**, and Back returns. No browser link: VLL asked for "bientôt". If he wants an "Open in browser"
button, that is a one-line follow-up.

### D7 — Layout on a big window

Shared screens were designed for phones and tablets. On a 1920×1080 window nothing may become hard to use (for
example a list row 1 900 px wide with its action at the far edge). A Windows-only maximum content width on list
screens is fine. Any change that is visible on Android goes to the gate first.

### D8 — Distribution

The MSI goes on the rolling `latest` release next to the APK, in the same `release-apk` publish (one release,
two assets, recreated together). The asset name is stable, e.g. `troubastage-windows.msi`. **A88 must keep finding
its APK**: add a case to `AppUpdaterParseTest` with a two-asset release JSON. The site's download section is not
part of this task.

## Acceptance criteria

1. CI green on the landed SHA, including WIN02's `windows` job. Its smoke now renders **Stage with the demo
   concert** (`docs/demo/demo-concert.tstage`, imported through the real import path) at 1920×1080 and at 800×600,
   uploaded as artifacts and attached to the gate entry.
2. Unit tests for every pure rule above: the wheel burst, Esc in Stage (in `stageBackAction`), the reserved
   keys, the window-placement restore (off-screen → centred), the cursor-hide conditions. Each one gets a sabotage
   that is shown to land.
3. On Linux (`:desktopApp:run` with `TROUBASTAGE_HOME` pointing at a scratch dir), against a scratch core: connect,
   sign in, download a concert, perform it, draw a note, send it. Screenshots of Home, the concert list, Stage
   (page, two-up, width, scroll), note mode, Settings and the Studio "Coming soon" screen, at 1280×800 windowed
   and in full screen.
4. A Compose desktop UI test that drives Stage with **mouse input**:
   - click toggles the chrome;
   - one long wheel burst turns exactly one page;
   - after clicking a chrome button, a `PageDown` key still turns.
5. Android: WIN01's screenshot set at your head is still pixel-identical to `main`'s, except for the intended
   changes (the wheel rules, which touch cannot exercise). `:androidApp:assembleDebug` green.
6. **Windows pass by VLL** (or by whoever has the Windows machine), from the MSI on the `latest` release. The
   checklist goes in the gate entry:
   - install with no admin prompt;
   - window and full screen;
   - F11, Esc;
   - the wheel in each mode;
   - the cursor hides;
   - display sleep held during a concert;
   - close asks;
   - a second launch;
   - reinstall over it with a newer build.

   **VLL's machines (2026-11-01): a Proxmox VM and a Surface.** Run the checklist on both and report it per
   machine.
   - **The VM** usually has no GPU. Expect the smoke and the app to log a software render API; that is fine. It
     is the place for install, upgrade, the keyboard and the mouse through the console.
   - **The Surface** adds three things to report:
     - its precision touchpad sends a two-finger swipe as a stream of wheel events with inertia, so the
       one-gesture-one-page rule must hold there (tune the burst end on this device, not on a mouse);
     - touch and the pen reach the app as mouse events through Windows: a tap is a click, a finger drag is a drag
       turn, and the pen draws in note mode;
     - there is no pinch or other multi-touch. Touch stays out of scope (see Out of scope). Report what the Surface
       does, so VLL can decide whether touch on Windows becomes its own task.

## Additions from the gate (2026-11-01, after WIN01 and WIN02 landed)

These come from the WIN01 and WIN02 reviews. Where they differ from D1 to D8, they win.

- **The interface as it landed** is `AppPlatform` in jvmSharedMain (read it; its names refine D1's: `bundlePicker`,
  `StageWindow`, `BackHandler`, `rehearsalNotes` → `RehearsalNotesStore`, and `appUpdate`/`pedal` as factories).
  Windows implements it in `:desktopApp`.
- **Studio (D6) is NOT a null capability.** A null `studio` removes the TroubaStudio tile entirely, but D6 wants
  the tile to open "Coming soon". So Windows provides a `StudioHost`:
  - `Browse` is the "Coming soon" screen.
  - `Edit` and `PresentQr` cannot be reached from it; if one is ever composed, it fails loudly and names WIN05.
  - `clearCookies` does nothing.
- **Every null capability, shown hidden.** On Windows these are null: `qrScanner`, `discovery`, `pedal`,
  `appUpdate` (until WIN04), `pendingInviteLink` and `volumeTurnRegistrar`. Their null branches first run on
  Windows. Acceptance 3's screenshots must show each one hidden, and say which shot shows it:
  - no "Scan a QR" on Home or in Connect;
  - no MIDI line, and no "Connect first" hint, in Parameters;
  - no update banner and no About update lines;
  - Connect with no discovered servers.
- **Back.** The desktop `BackHandler` must keep Android's rule: **the handler composed last wins**, and disposing it
  hands Back to the one before. A79 and every screen's Back rely on it. Esc, Alt+Left and the mouse Back button
  (D4, outside Stage) go through it. Write a unit test with nested handlers (the inner one wins; after it is
  disposed, the outer one does), with a sabotage.
- **Storage (WIN02).**
  - The data root on Windows is **`%LOCALAPPDATA%\TroubaStageData`**. The install folder is
    `%LOCALAPPDATA%\TroubaStage`, and they must stay separate.
  - Every `Storage` over one root is one store, so the entrypoint may open two, as Android does.
  - **web-core's F2 must land before WIN03 lands.** It makes a read error never wipe the store, and it measures
    get/put p50/p95 with DPAPI on the windows runner. If the put p95 is over 16 ms, WIN03 writes the Stage position
    off the UI thread, still in order and still with nothing lost on close (D2). State which applies at the gate.
- **The smoke** (acceptance 1) keeps WIN02's B5 rule: it exits 0 only after the real window has drawn a frame. Log
  the render API.

## Out of scope

- Embedded Studio (WIN05).
- Self-update (WIN04).
- BLE-MIDI pedals on Windows: needs a Windows MIDI stack decision, so file it if VLL needs it.
- LAN discovery; QR scanning; `/join/` deep links.
- Touch and pen on Windows tablets: not a target. Taps and drags arrive as a mouse, so it should work roughly; it
  is not tested.
- Code signing.
- iOS.
