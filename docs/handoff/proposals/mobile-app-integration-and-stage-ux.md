# Proposal — mobile app integration & Stage UX (for arch decision)

**Status:** RULED 2026-07-14 (see docs/handoff/reviews.md — Q1: (a) with URL-param signal, T46 filed; Q2: hybrid drawer + segmented reading mode, Songs stays direct; Q3: role-first) · **Raised by:** Mobile App Agent (2026-07-14) ·
**Area:** `app/androidApp` (Edit/Connect/Stage chrome), `app/shared` (StageScreen), the A06
`bridge.ts` handshake + `web/studio` (an "embedded" mode) — **cross-lane** · **Relates to:**
I10 (never reimplement the editor), I12 (Stage is read-only), A06 (WebView host), A14/A15
(reading mode + song drawer), the App()/nav hoist deferral (handoff §13).

VLL did the first **real-device** run (Redmi, Android 15) this session and gave hands-on
UX feedback. Two items are plain bugs the mobile lane is fixing directly (separate PR — see
bottom). The rest are **design** questions that touch invariants and the web-core lane, so
per the workflow they come here before anyone builds. **These are options, not a spec —
please rule, or propose a different design; the design is yours to own.**

## What VLL reported (device QA, verbatim intent)

1. **Edit "feels like you launched a browser."** The Edit screen is a thin native bar
   (`Back · <url> · Server`) over the full TroubaStudio web page, which carries its **own**
   `Bands / Invites / Marie / Log out` nav — so the app- and web-chrome visibly duplicate and
   it reads as an embedded browser, not an app screen. VLL's suggestion: *"a mobile webview
   that looks more like an app,"* or *"native stuff around the webview with some [web] stuff
   not displayed when viewed in the webview."*
2. **Discoverability.** VLL couldn't find **continuous-scroll** (A14 — it's the 3rd stop of the
   `Fit: page` toggle, reached by tapping it 3×) or the **song list** (A15 — the bottom-right
   `Songs` button). *"I don't see where I can choose infinite scrolling and list of songs."*
3. **Layers/Role feels unnatural.** *"printing all layers to choose is also strange and not
   natural"* — the Layers panel dumps every layer as toggles; Role is a separate control.
4. (Bug — being fixed, listed for completeness) rotation exits Stage; the app login isn't
   carried into the Edit WebView.

## The design questions

### Q1 — How should the embedded editor (Edit) present?

The editor MUST stay the web TroubaStudio (I10 — no native editor). The question is only how the
app *frames* it. Options:

- **(a) Native chrome + Studio "embedded mode" via the bridge.** The app already has a
  feature-detected `bridge.ts` handshake (A06). Extend it so the app announces `embedded: true`;
  Studio then **hides its own top nav** (Bands/Invites/account/logout — the app owns those) and
  uses its mobile/responsive layout. App supplies a real app bar (title, back, overflow).
  *Cross-lane:* app = the bar + handshake flag; web-core = Studio's embedded layout/CSS. Cleanest
  integration; keeps one editor; needs the web-core lane.
- **(b) Native chrome only, Studio unchanged.** App wraps a nicer bar/drawer around the WebView
  but Studio still shows its own nav → the duplication VLL saw remains. Cheap, app-only, but only
  half-solves it.
- **(c) App-injected CSS to hide Studio chrome.** App injects CSS into the WebView to hide
  Studio's nav. App-only and no Studio change — but couples the app to Studio's DOM/class names
  (brittle; breaks silently when Studio's markup changes). Not recommended.
- **(d) Something else** — e.g. Studio grows a first-class responsive "app skin" independent of the
  bridge, or the editor is scoped to specific routes only. **Your call if you see a better shape.**

*Mobile lean:* (a) — the bridge is the sanctioned seam and it keeps I10 intact; but it's a Studio
design + a web-core commitment, so it's genuinely the architect's + web-core's decision.

### Q2 — Surfacing reading mode + song list (and the nav shape)

A14 (scroll) and A15 (songs) exist but are buried in a toggle-cycle and a corner button. A
**hamburger / navigation drawer** could surface: reading mode (page / width / scroll), the song
list (already a drawer — A15), layers, role, day/night. This also intersects the **deferred
App()/nav hoist** (§13): if we're adding a real nav, that may be the trigger to hoist the shared
`App()` (and it would give iOS the same nav for free). Options:

- **(a)** A Stage overflow/drawer that groups the existing controls (reading mode, songs, layers,
  role, day/night) into one discoverable menu; scroll becomes an explicit choice, not toggle-stop-3.
- **(b)** Keep the top-bar controls but make scroll a labelled segmented control (Page | Width |
  Scroll) instead of a cycling button — smallest change, fixes the "where is scroll" specifically.
- **(c)** Defer until the App()/nav hoist is planned, and do nav discoverability as part of that.
- **(d)** Your alternative.

*Note:* two-up (A12) was deliberately spec'd **automatic, not a toggle** — VLL's "where do I choose
scrolling" partly reflects that resolved decision; flag if you want to revisit it.

### Q3 — Layers/Role model

Options: (a) role-first — pick a role, layers follow the default-visibility rule (I12), with an
"advanced" expander for manual per-layer toggles; (b) keep manual toggles but group/label them; (c)
your call. This is a Stage-read-only (I12) presentation choice.

## Invariant / lane impact

- **I10** — all options keep the editor web-only; (a)/(c) change *how* it's chromed, not that it's Studio.
- **I12** — Q2/Q3 are Stage presentation only; no model/write changes.
- **Cross-lane** — Q1(a) needs the **web-core lane** (Studio embedded layout + the bridge flag); the
  app side (bar, handshake, drawer) is the mobile lane. Q2/Q3 are mobile-lane, but Q2 may pull in the
  App()/nav hoist (§13).

## Not in this proposal (mobile lane is fixing directly — separate PR)

- **Rotation exits Stage** — `MainActivity` has no `android:configChanges` and `App()`'s nav is
  `remember` (not `rememberSaveable`), so rotation recreates the Activity → back to Concerts. Fix:
  add `configChanges` (Compose relayouts in place; A12 two-up then works on rotate). Bug, not design.
- **App login not carried into Edit** — the Connect session is an app-side ktor cookie; the WebView
  has a separate jar and nothing seeds it, so Edit re-prompts. Fix: seed `CookieManager` with the
  persisted session before load. Bug, not design.

These two ship as a mobile-lane defect PR; the questions above wait for your ruling.
