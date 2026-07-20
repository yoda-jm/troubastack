# T58 — Topbar account dropdown (consolidate the top-right; VLL GO 2026-07-20)

**Priority:** normal · **Size:** S/M (mostly relocating landed pieces) · **Area:**
`web/studio` Shell. Web-core lane; slot around the T09 stages as convenient.
Design: the 2026-07-20 IA read (reviews.md), VLL-nodded.

## The shape (ruled)

ONE account trigger, top-right: **avatar + display name** (avatar-only at phone
width). Opens a dropdown:
1. **My account** → `/me`
2. **Get the app** → opens the EXISTING QR/download popover panel (reuse, don't
   inline — the QR needs room). The standalone `GetAppChip` retires. Hidden when
   `/api/apps` is empty (unchanged semantics).
3. Footer: **version/build line** (Studio + server + the mismatch warning text) —
   the `VersionChip` retires.
4. **Log out**.

**Stays OUT:** Invites (navigation with a badge — left-side nav, untouched);
Bands likewise.

## Pins

- **The mismatch dot:** when the version-mismatch warning (or any future urgent
  state inside the menu) is active, a warning dot renders ON the avatar trigger —
  the glanceable signal must not be buried. e2e: mismatch mocked → dot present →
  open menu → warning text shown.
- **T47 clamp inherited once:** the dropdown (and the reused app panel) use the
  clamped-popover behavior; 412px pixels required (trigger in the compact header,
  menu fully on-screen, items tappable).
- Embedded mode (T46) unaffected — the topbar is suppressed there; assert no
  regression in the shell-embedded spec.
- Testids for every menu item; existing get-app + version e2e re-pointed,
  assertions preserved.

## Acceptance

e2e: menu opens/closes, all four entries work, mismatch dot, app-panel
present/absent + iOS coming-soon unchanged, embedded suppression; `tsc -b`;
pixels light+dark desktop + 412px. Gate as usual.
