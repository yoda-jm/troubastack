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

---

## Addendum (2026-07-15) — more VLL device feedback (two amendments to your Q2/Q3 rulings)

> **RULED 2026-07-15 (reviews.md):** A1 → option (a) per-song visibility, remembered per song, role re-seed clears, P201 merge must go per-song. A2 → BLESSED; gesture split ruled: left/right-third taps + swipes turn pages VERBATIM, the middle third (inert today) toggles the auto-hiding chrome.

After the first-device run, VLL gave two further steers. Both touch already-ruled questions, so
raising them here rather than acting unilaterally. **Still your call — re-rule or fold in.**

### A1 — Q3 amendment: layer visibility should be PER-SONG, and defaults ON

VLL: *"the default layer should be displayed by default, and layer display should be on the current
song not the whole concert."*
- **Defaults-on** is consistent with your role-first Q3 ruling (role → default-visibility rule, I12) —
  just confirming the default state shows the role's layers, no hunting. No conflict.
- **Per-song is the new bit and a model change.** Today `StageState.visibleLayers` is a SINGLE set
  applied across the whole concert (`StageModel.kt` — `aggregateLayers` collapses the bundle; toggling
  a layer changes it everywhere). VLL wants a toggle to affect only the **current song**. That means
  per-song visibility state (keyed by songId), role still seeding each song's default, and deciding
  whether a manual toggle is remembered per song or resets on song change. Presentation-only (I12
  intact — still no writes), but it reshapes the Stage layer model, so: **ruling wanted.** Options:
  (a) per-song `visibleLayers` map, role seeds each song, manual overrides remembered per song;
  (b) per-song but overrides reset on song change (simpler, less state); (c) your alternative.

### A2 — Q2 amendment: adopt the ORIGINAL app's fullscreen + nav feel

VLL pointed at the legacy Android app (`~/AndroidStudioProjects/TroubaShare`,
`ui/screens/concert/ConcertModeScreen.kt`) and said he *"liked it a lot, especially the navigation and
the fullscreen effect."* Its pattern, which our Stage does NOT do (our chrome is always visible):
- **True immersive fullscreen with auto-hiding chrome:** `WindowInsetsControllerCompat` hides status +
  nav bars + cutout with `BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE`; a `showControls` flag drives an
  `AnimatedVisibility` chrome overlay that **auto-hides after a timeout** and **toggles on tap** — so
  the score is edge-to-edge during performance and controls appear on demand.
- **Drawer navigation:** a `ModalNavigationDrawer` opened from a menu button (we already have the A15
  song drawer — this would host the Q2 settings drawer too).

This fits INSIDE your Q2 ruling rather than replacing it: the Songs button + the new settings drawer +
segmented reading mode all live in the **auto-hiding chrome**; tapping the score reveals them, and they
fade back so performance is fullscreen. Suggested amendment: **Q2's chrome becomes tap-to-reveal /
auto-hide immersive** (reference-app style), with the segmented reading-mode control + settings in the
revealed bar/drawer. Shared `StageScreen` (commonMain) so iOS inherits it; still no App()/nav hoist.
Note the existing A04 `StageHost` already does immersive on Android — this generalizes it to
auto-hiding chrome and gives iOS the same via a commonMain gesture/visibility layer.

**Sequencing:** these fold into the Q2/Q3 A-track tasks (not yet started) — no new lane split; T46 and
Q1 (landed) are unaffected. If you bless A1+A2, I'll spec them into the Q2/Q3 tasks and build.

---

## Addendum 2 (2026-07-17) — Stage nav semantics (recommendation wanted) + two rendering reports (investigated)

> **RULED 2026-07-17 (reviews.md):** N3 → (b) drop edge-tap-turn (A2 tap split REVERSED on device evidence; any tap = chrome, nav = swipe/FABs/pedals). N1 → (c) continuous + transient title-card cue at song boundaries. N2 → (b) per-song scroll; song crossings always explicit + cued. B1 → both halves required (failure-aware decode + pin current page), lands FIRST. B2 folds into N3.

VLL feedback after living with A17 (immersive Stage) on the device. Design calls on navigation
semantics are raised for your recommendation; two behaviour reports were investigated in code.

### N1 — What does "next" mean: page, song, or both? (+ the song-boundary feel)
Today `›`, edge-tap, swipe, and pedals all advance one **page** through the shared `turnNext`, and pages
flow **continuously across song boundaries** (Wonderwall p3/3 → next song p1/4); the title card's
"Song X/N · P/T" flips song mid-advance with **no seam**. VLL finds the cross-song chaining
disorienting. Options: **(a)** page-only, no seam (today); **(b)** page-within-song + an explicit
next-song affordance (or stop-at-song-end); **(c)** keep continuous but add a **song-boundary cue**
(title-card flash / interstitial / divider) so crossing a song reads as intentional. Recommendation?

### N2 — Scroll scope: whole concert vs per-song
A14 scroll is one continuous column of the **entire concert** (all songs). VLL asks: whole concert or
"just multipage PDFs" (within a song)? Options: **(a)** whole-concert (today); **(b)** scroll within the
current song only, song change = discrete jump; **(c)** continuous with section breaks between songs.
Couples to N1 (what "next" does while scrolling). Recommendation?

### N3 — Tap-to-turn vs swipe (reopens the A2 tap ruling — flagging explicitly)
VLL: tapping the bottom-right turns the page and "feels not expected"; *"a swipe in page mode should
navigate to next page — feels natural."* Current model (A04 + the A2 ruling): left/right-third **tap**
turns, middle-third tap toggles chrome, **and** horizontal swipe turns. So an edge tap turns the page,
which competes with the "tap to reveal chrome" mental model and causes accidental turns. Options:
**(a)** keep edge-tap-turn (A04 verbatim, as A2 ruled); **(b)** **drop edge-tap-turn** → page nav becomes
**swipe + ‹ › FABs + pedals**, and ANY tap toggles chrome (matches VLL's instinct, removes accidental
turns) — this **reopens the A2 tap split you just ruled**; **(c)** keep edge-tap but shrink the turn
zones / require a firmer press. Recommendation? (This one changes A2, hence the explicit flag.)

### B1 — BUG (investigated): "same page, fewer annotations after a tap"
Root cause found in the read-only compositor (`StageScreen` `PageView`/`ScrollPage`): overlays are
decoded via `overlayRefs.mapNotNull { decodeCached(...) }` — a **failed overlay decode is silently
dropped** (no retry; a failed decode is also **not cached**), and `PageImageCache` holds only
**12 entries** (raster + several overlays per page ⇒ ~2–4 pages before the LRU evicts). Navigating
evicts a page's overlays; a transient re-decode failure then re-renders the **same page with fewer
annotation layers**. This violates I12 (a page must never silently lose annotations). Fix direction for
your ⟶ ok: don't silently drop a failed overlay (retry / visible-degrade, and be failure-aware so a
miss isn't mistaken for "no layer"), and/or raise the cache budget so a page's own overlays aren't
evicted while it's on screen. Mobile lane will fix on your confirmation.

### B2 — "bottom-right tap renders something different"
Investigated: this is N3's edge-tap-turn firing (right third → next page ⇒ a different page's
annotations), i.e. by-design tap-to-turn, **not** a separate glitch. Folds into N3.

**Sequencing:** mobile lane will fix **B1** once you bless the direction, and implement whatever you rule
for **N1/N2/N3**. **A1 (per-song layers)** is code-complete and PAUSED — orthogonal to all of the above
(it touches neither nav nor the compositor decode); land it when convenient or after these.

---

## Addendum 3 (2026-07-17) — ❓ DESIGN REVIEW REQUEST: page-turn animation on swipe (N4?)

Context: B1/A19, the N1/N2/N3 nav rework, A1/A18, and A21 (the stale-swipe-closure fix)
are all landed and device-verified. A page turn today (swipe / ‹ › FABs / pedals / keys /
volume) swaps the page **instantly** — no motion. VLL asks for an **optional "nice page-turn
animation on swipe."** VLL explicitly deferred the appetite decision to you ("ask Fable for
recommendation"). Requesting a ruling on **whether** to add it and, if so, **which flavor** —
before mobile implements (new-design-first-gate).

Options, scoped against the current invariants (single nav path; I12 read-only; A12 two-up;
N1 boundary cue; N2 per-song scroll):

- **(a) Lightweight direction-aware slide (mobile-lane recommendation).** Wrap the page
  content in `AnimatedContent` (or a manual `Animatable` offset) keyed on `state.current`,
  sliding the outgoing page out and the incoming page in (~200–250ms), direction inferred
  from prev/next. **Every** turn path animates identically (swipe, FABs, pedals, keys,
  volume) because they all funnel through `goToPage` — no per-input special-casing. Contained,
  low-risk; two-up animates the spread as a unit; scroll mode keeps its own vertical motion
  (this is page/width only); the N1 cue is unaffected (it keys off `currentSong`, fires after
  the turn). Cost: small. Doesn't track the finger — it snaps then animates.
- **(b) Follow-the-finger (HorizontalPager).** Replace the discrete turn model with a
  `HorizontalPager`: the page tracks the drag 1:1, rubber-bands at ends, snaps on release —
  the "premium" e-reader feel. Cost: a real rework. The single-nav-path invariant now has two
  masters (the pager's own gesture **and** goToPage from FABs/pedals/keys/volume, which must
  `animateScrollToPage` the pager); two-up (spread paging), per-song scroll (N2 disables the
  horizontal pager in scroll mode), and the boundary-cue timing all need re-derivation and a
  full device re-test. Higher risk to freshly-blessed behavior.
- **(c) Skip.** Keep instant swaps; keep the queue on A20/T50 cues. Zero cost/risk.

**Mobile-lane recommendation: (a)** — most of the perceived polish for a fraction of (b)'s
risk, and it animates every turn path, not just swipe. Would gate the concrete diff as usual.
If you prefer (b), I'd want an explicit re-verify checklist for two-up/scroll/pedals/cue.
**Ruling?** (Tagging this N4 for reference.)

> **RULED 2026-07-17 (reviews.md):** (a) — direction-aware slide, all turn paths, interruptible under rapid pedal fire; (b) rejected FOR NOW on stability (two masters on the nav path, re-opens freshly-stabilized derivations) — revisit only if VLL still wants finger-tracking after living with (a).

---

## Addendum 4 (2026-07-17) — ❓ DESIGN REVIEW REQUEST: chrome contrast, "black navigation on black" (N5?)

Device QA (VLL, Redmi Pad SE, this session): *"black navigation on black has challenge."*
Confirmed from screenshots. The Stage chrome FABs (`StageFab`) use a translucent-DARK
disc — `container = Color(0xC0000000)` with a white glyph — from the A17/A2 reference-app
look. On the immersive BLACK canvas (and its letterbox margins around the page), the disc
itself effectively disappears; only the white ‹ › / ☰ / ⚙ glyph floats, so the tap targets
are hard to locate and the chrome reads unfinished. (The ✕ is fine — it's red; and over the
WHITE page a dark disc is fine — the problem is dark-disc-on-black.) Same issue affects the
top bar's ☰ / ● / ⚙ discs. This is a contrast/visibility regression against the blessed A17
look, so raising it here rather than restyling blessed chrome unilaterally.

Options:
- **(a) Lighter translucent disc + hairline border (mobile-lane recommendation).** Swap the
  FAB container to a translucent light/neutral (e.g. `~0x66FFFFFF` frost, or a mid `0xB0303030`)
  **plus a thin `~0x40FFFFFF` outline** so the disc reads on BOTH the black canvas and the
  white page, glyph stays high-contrast. Smallest change; one `StageFab` styling constant;
  keeps the round-FAB shape VLL liked.
- **(b) Keep dark discs, add a soft scrim/gradient behind the top+bottom bars** so the whole
  control strip separates from the canvas. Slightly more chrome; closer to a "toolbar" look.
- **(c) Elevation/shadow only** — rely on a drop shadow to separate the disc from black.
  Weakest on a pure-black background (shadow barely reads).
- **(d) Your alternative / leave as-is** if you judge it acceptable.

Mobile-lane recommendation: **(a)** — a one-constant restyle that fixes contrast on both
backgrounds without changing layout or the reference-app silhouette. Would gate the diff (and
a device screenshot pair, black-canvas + white-page) as usual. **Ruling?** (Tagging N5.)
