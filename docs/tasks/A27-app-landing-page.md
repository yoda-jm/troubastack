# A27 — App landing page: products + identity, not the bake list (VLL, designed by Fable)

**Priority:** high (VLL 2026-07-18: "a landing page that is not the bake list, so we
clearly identify the products inside the app (studio, …) and we can login from
here") · **Size:** M · **Area:** `app/shared` (nav + screens) + hosts. **Mobile
lane.** Numbering note: the settings-sheet sweep files as **A26** (the A25 branch
name collides with the landed layer-number fallback); this is **A27**.

## Design (ruled)

**Cold start lands on HOME, never the concerts list.** Home presents the app's
products as large stage-friendly tiles + one identity card:

```
┌────────────────────────────────┐
│  TroubaShare      [The Troubadours]   ← band name once connected
│
│  ┌──────────────────────────┐
│  │ ▶ PERFORM                │  ← primary, biggest tile
│  │   Sat @ The Anchor · 4 songs │  subtitle: last-opened concert
│  │   3 concerts on device    │
│  └──────────────────────────┘
│  ┌───────────┐ ┌───────────┐
│  │ ✎ EDIT    │ │ ⇩ CONCERTS │   Edit = the embedded Studio (A16/T46)
│  │  Studio   │ │  get/update│   Concerts = download/update offers (B03 chips)
│  └───────────┘ └───────────┘
│
│  ┌──────────────────────────┐
│  │ 👤 Marie · demo-server ✓  │  ← identity card (see below)
│  └──────────────────────────┘
└────────────────────────────────┘
```

1. **Perform** → the existing concerts-on-device list → Stage (unchanged from
   there). Bonus action on the tile: "Resume «last concert»" one-tap straight to
   Stage.
2. **Edit** → the embedded Studio (A16 app bar, T46 embedded mode) — unchanged,
   just entered from Home.
3. **Concerts** → available/downloadable bundles + update offers (the B03 surface,
   promoted from wherever it hides today). If it's currently fused with the
   device list, splitting is OPTIONAL v1 — the tile may open the same list
   scrolled to offers; don't over-engineer.
4. **Identity card** — the login VLL asked for, and the natural home of P205's
   identity: disconnected → "Connect to your band" (server + login, the B03
   Connect flow); connected → name · server · sync state; tap → account/server
   management. When P205 Stage 3 lands, the resolved identity ("performing as
   Marie") lives HERE. Must render OFFLINE (state: "offline · last synced …").

## Rulings that ride this task

- **The §13 App()/nav hoist happens NOW, as part of A27.** The earlier deferral
  ("don't couple the hoist to the A17 drawer") was correct then; a landing page IS
  the nav restructure the hoist was waiting for. Nav (home → product → screens)
  goes commonMain so iOS inherits; while in there, nav state becomes
  `rememberSaveable` (the A21-era process-death hardening note — pays for itself
  here).
- **I12 intact:** the identity card is OPTIONAL — Perform works fully
  offline/sideloaded with no login, no roster, no server. The landing must never
  gate Stage.
- Back semantics: product screens back-navigate to Home; Home is the task root
  (back exits). Stage exit (✕) returns to the concerts list as today.

## Acceptance

- Cold start → Home (test: nav-state default); Perform/Edit/Concerts reachable in
  one tap; resume-last works; rotation + process-death land back where you were
  (rememberSaveable test).
- Offline: Home renders with no server (identity card shows the offline state;
  Perform fully functional).
- Device screenshots at the gate: Home connected + disconnected, light is fine
  (app is day/night inside Stage only).
- `:shared:check` + `:androidApp:assembleDebug`; iOS klibs compile (commonMain nav).

## Out of scope

- P205 Stage 3's picker itself (separate item; the card just HOSTS it later); any
  Stage/Editor behavior change; tile theming/branding beyond clean defaults.
