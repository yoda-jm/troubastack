# Proposal — mobile Home IA + nav affordance + live connection status (for arch decision)

**Status:** RULED 2026-07-19 (reviews.md) — Concerts-under-Studio ratified; ONE list two intents (perform=lean+offline, manage=affordances); back/Home affordance everywhere; live-verified connection status + no persisted connecting flag. One task: A31. · **Raised by:** Mobile lane (2026-07-19,
relaying VLL device feedback) · **Area:** `app/androidApp` (`MainActivity` nav host,
`HomeScreen`, `ConcertsScreen`), `app/shared` (`HomeScreen` state/tiles) · **Relates
to:** A27 (landing page), the P205 acceptance (just passed on-device).

VLL, driving the tablet after the P205 two-identity acceptance, raised two nav
frictions and one IA question. All three are UX/structure changes to the A27 landing
page, and the mobile lane is resting post-P205 — so they're gated here before any code.

## The feedback (verbatim)

1. *"it is complicated to go from concerts to the home page again on the app, it is
   probably the same to navigate between other things, isn't concert part of
   troubastudio?"*
2. *"I am shown as connected on the app but clicking on TroubaStage set me on the
   connection page, strange (maybe timeout or something like that)."*

## The gaps (as they stand in `MainActivity.kt`, main @ 6737104)

- **No on-screen back/Home affordance.** `ConcertsScreen` and the embedded Studio rely
  on `BackHandler { atHome = true }` — system-back only. There is no visible Home/back
  control, so returning to the landing page is non-obvious. (Stage has its ✕ exit; the
  other sub-screens do not.)
- **IA: three top-level peers.** A27 Home exposes **TroubaStage · TroubaStudio ·
  Concerts** as sibling tiles. VLL reads Concerts as *belonging under* TroubaStudio
  (you author/manage concerts in the Studio; TroubaStage is purely the perform surface).
- **Stale connection status.** Home renders `Identity.Connected` from
  `transport.isConnected` — a cached flag, never re-tested. It can read "Connected ✓"
  while the server is unreachable. (This, plus a `rememberSaveable connecting` flag that
  survives process death, is the likely cause of #2: a reinstall/kill mid-Connect
  cold-started onto the Connect screen while Home still showed "Connected". There is no
  code edge from the TroubaStage tile to Connect — `onPerform` only does `atHome=false`.)

## Proposed direction (VLL's steer, 2026-07-19 — pending arch sign-off)

1. **IA — Concerts under Studio.** Home shows **two** primary surfaces:
   - **TroubaStage** → perform (→ concerts list → stage). Unchanged; still fully offline
     (I12 preserved — Home never gates Stage).
   - **TroubaStudio** → author/manage, with **Concerts living inside it** (import,
     update, edit). Concerts stops being a Home peer.
   ```
   Home
    ├─ TroubaStage    → perform  (concerts list → stage)
    └─ TroubaStudio   → author/manage
          └─ Concerts (import, update, edit)
   ```
   Open question for arch: TroubaStage's own concerts-list is the *same* list as
   Studio→Concerts — do we keep one list reached from two entry points (perform-intent
   vs manage-intent), or does Stage get a lean "pick a concert to perform" view while
   full management lives only under Studio? (Mobile lane leans: one list, two intents,
   with manage actions shown only when entered via Studio.)

2. **Back/Home affordance.** A visible Home/back control on every sub-screen (Concerts,
   Studio), not just system-back. Keep the `BackHandler` as well.

3. **Live connection status.** On Home (when online), actively refresh/test the
   connection rather than trusting the cached `isConnected` flag, so "Connected ✓"
   reflects reality. Fold in: don't persist `connecting` across process death (removes
   the cold-start-onto-Connect glitch).

## Ask for Fable

- Ratify (or adjust) the **Concerts-under-Studio** IA and rule on the one-list-vs-two
  open question above.
- Confirm scope 2 + 3 ride with the IA change as one mobile task, or split.
- Then the mobile lane picks it up as a specced task (A-track).
