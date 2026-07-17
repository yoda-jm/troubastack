# T50 — Personal song cues: icons + colors per song per member (core + studio; app half = A20)

**Priority:** normal (VLL feature request 2026-07-17; queue after the in-flight nav/
correctness work) · **Size:** core S/M + studio M (+ app M as **A20**) · **Area:**
`proto`, `core`, `web/studio`, `app/shared`+`androidApp` — cross-lane, contract pinned
here.

## What VLL asked for (verbatim intent)

Per song, per person: a set of **icons + colors** shown in the app's concert/setlist
view so a musician knows what to prepare — "mic + red guitar" = this song the
guitarist sings (set up the mic) and takes the red electric guitar. Also **flash the
icons in the center when you enter the song**. Instruments: kinds of guitars,
ukulele, cajon, bongo, djembe, egg shakers, shakers, autoharp, melodica, bass, …
Follow-up: **each member sets this for themselves**, and it **rides the bake**.

## Design (analyzed + decided)

1. **Model — a cue is `{icon, color}`; a member's cues for a song are a short
   ordered list.**
   - Core: `SongCue { Icon string; Color string }` stored per `(songID, memberID)`
     (own entity, both repo backends, deterministic order preserved as stored).
   - `Color` is an optional `#rrggbb` string ("" = neutral/untinted). The UI offers a
     fixed 8-swatch stage palette; the model deliberately accepts any hex (future
     custom colors cost nothing).
   - **Ownership: SELF-ONLY in v1** (per VLL: "each band member sets this for
     himself"). `PUT /api/bands/{b}/songs/{s}/my-cues` replaces the caller's list
     (member-gated like other song reads); cues ride `GET` song payloads as
     `myCues`. Admin-edits-others is a later, separate ask.
   - Soft cap: UI limits to **4 cues per song** (glanceability); model unbounded.

2. **Icon contract — stable string IDs + a curated MONOCHROME, TINTABLE glyph set.**
   Emoji rejected: a "red electric guitar" needs tinting; emoji can't. v1 set (18):
   `guitar-electric · guitar-acoustic · guitar-classical · bass · ukulele ·
   autoharp · melodica · keys · cajon · bongo · djembe · guiro · cuica ·
   shaker · egg-shaker · tambourine · mic · note`
   (güiro + cuíca added per VLL 2026-07-17 — ASCII IDs `guiro`/`cuica`.)
   `note` doubles as the **fallback: an unknown icon ID renders as `note`+color,
   never an error** — same additive-compatibility argument as the proto fields; new
   icons can ship server/studio-side before the app knows them.
   Assets: one set of minimal single-path SVGs (studio inlines them; the app converts
   to Compose `ImageVector`s — mechanical). Keep every glyph readable at 20dp and
   legible tinted on BOTH day and night surfaces.

3. **Distribution — cues ride the per-member bake (B07 fit, VLL-confirmed).**
   `proto/troubastack/v1/bundle.proto`: add to `BakedSong`
   `message SongCue { string icon = 1; string color = 2; }` +
   `repeated SongCue cues = 10;`
   — additive metadata exactly like fields 5–9 (B02 decision: manifest metadata, not
   burned into pixels; old loaders ignore unknowns; absent = none). The per-member
   bake injects THAT member's cues; the shared bake carries none (cues are personal).
   **No app-side filtering needed — a member's bundle already IS their view.**

4. **Studio (this task) —**
   - Song page: a "My cues" editor — icon picker grid (the 18 glyphs, labelled),
     color swatches, reorder, remove; ≤4 enforced in UI. Testids for e2e.
   - Setlist page rows show each song's own-cues inline (small, tinted) — secondary
     but nearly free once the assets exist.

5. **App (A20, mobile lane — lift this section into the A-track handoff):**
   - **A15 song drawer rows**: right-aligned row of the member's tinted cue icons
     per song — the glanceable "what's coming" list VLL described.
   - **Center flash on song entry**: entering a song (nav or drawer jump) shows the
     cues LARGE in a center overlay ~2.5s then fades — **compose it with the N1
     song-boundary title-card cue: ONE overlay (title/position + big cue icons),
     one clock-injected timeout** (the A17 auto-hide pattern). No cues → the N1 card
     alone, unchanged.
   - Render from the bundle's `BakedSong.cues` (loader: additive field, default
     empty). Unknown icon → `note` fallback, tinted.
   - Tests: loader default/roundtrip, fallback mapping, flash timeout
     (clock-injected), drawer-row presence (state-level).

## Acceptance criteria

- Core: CRUD round-trip both backends (mem+file, deterministic order); member A
  cannot write B's cues; bake injects exactly the baked-for member's cues (per-member
  bake test extends B07's); shared bake has none; `gofmt`/vet/test green.
- Proto: field 10 additive — an OLD bundle loads in the new app (no cues) and a NEW
  bundle loads in an old loader (ignored) — the standard B02 compat argument, tested
  loader-side.
- Studio: e2e — set 2 cues (mic + red guitar-electric) → reload → they persist and
  render tinted; setlist row shows them; unknown-icon fallback unit test; pixels
  light+dark at the gate.
- App (A20): drawer rows show cues; entering the song flashes the combined card
  (device screenshot); timeout clock-injected test; A04/A17/N-rulings acceptance
  untouched.

## Out of scope (v1)

- Admin editing another member's cues; per-SETLIST cue overrides; free-form icon
  upload; cue text labels; web-side flash. All future asks if wanted.

## Sequencing

After the in-flight queue (A19 condition + nav rework N1/N2/N3 + A18; T46 web-core).
The SVG glyph set is the one genuinely new asset job — 18 minimal monochrome glyphs;
studio and app share the geometry.
