# P205 — Band-wide bundle: view-time identity + bake-time defaults (N11, VLL-approved)

**Priority:** approved 2026-07-17 (VLL sign-off incl. the ⚠ band-wide-bytes tradeoff —
a conscious trusted-band choice superseding B07's download-gating; no sensitive-layer
opt-out for now, later addition if wanted) · **Size:** L, staged · **Areas:** proto,
core (baker + bakeapi + studio bake dialog), `app/shared`+hosts. Ruling + design
decisions: reviews.md 2026-07-17 (N11 → P205).

## The model (one paragraph)

ONE band-wide bundle per concert carries ALL layers (each tagged with its owner) +
every member's cues + the member roster. Identity resolves at VIEW time: logged-in
(B03 Connect session matching a roster member) → automatic; anonymous → a one-tap
"Who are you?" picker, remembered per concert/device (I12 intact: no account needed).
The viewer shows shared/conductor layers + YOUR personal layers; other members'
personal layers are NOT listed. Default visibility is captured at BAKE time,
explicitly confirmed in the bake dialog.

## Stage 1 — proto + baker + bake dialog (web-core)

Additive proto (all mirrors carry `AUTHORITY: bundle.proto`):
- `ConcertBundle`: `repeated BundleMember roster = 8;` with
  `message BundleMember { string member_id = 1; string display_name = 2; string role = 3; }`
- `LayerImage`: `string owner = 8;` ("" = band/shared content; member-id = that
  member's personal layer) · `optional bool default_on = 9;` — **proto3 `optional`
  for presence** (absent ⇒ compute as today: `mandatory || role_tag rules`; Go mirror
  `*bool`/omitempty, Kotlin nullable).
- `BakedSong`: `repeated MemberCues member_cues = 11;` with
  `message MemberCues { string member_id = 1; repeated SongCue cues = 2; }`.
  Field 10 (`cues`) KEEPS its per-member-bake semantics while scope=mine exists and
  is left EMPTY in band-wide bakes (old apps degrade to no-cues, never wrong-cues).
- Baker: band-wide bake includes every layer (owner-tagged), all members' cues,
  the roster, and per-layer `default_on` captured from the bake dialog.
- **Bake dialog (studio):** shows the capture explicitly — "Baking with: Cues ✓ ·
  Form ✓ · My notes ✗ — edit?" (WYSIWYG seeded from the baker's current visibility,
  editable, remembered per setlist). No silent capture (the ruled footgun guard).
- Tests: proto additive round-trips (old bundle → new reader; new bundle → old
  reader ignores), baker owner/roster/default_on injection, dialog e2e.

## Stage 2 — band-wide becomes THE bake (web-core)

- The band-wide bake replaces the shared bake output; `?scope=mine` (bakeapi) retires
  AFTER stage 3 ships in the app (one release of overlap; then remove + tests).
- Demo returns to ONE bundle (`demo-concert.tstage`), regen per B05; README updated;
  `-mine` variant deleted.
- Rehearsal/live (P201) path re-verified: autobake produces band-wide bundles; R10
  remap unaffected (page identity unchanged).

## Stage 3 — app identity + filtering + defaults (mobile)

- Identity: roster read; Connect-session match → auto; else one-tap picker
  (remembered per concert/device via Storage KV — a VIEW preference, not an account;
  I12 held). Changing identity re-seeds like a role change (A18 semantics).
- Filtering: personal layers with `owner != me` are dropped at load (not listed
  anywhere); `owner == me` join the per-song model as today; cues = my `member_cues`
  entry (field 10 fallback for old bundles).
- Defaults precedence (ruled): mandatory (I12) > manual per-song toggles (A1,
  session) > identity (my personal layers on for me) > `default_on` ∧ role_tag rule;
  `default_on` absent ⇒ legacy compute. Test matrix mirrors A18/LiveUpdate.
- Role picker remains (role still drives role_tag scoping within defaults); identity
  supplies the default role from the roster.

## Acceptance (program)

- One bundle serves the whole band: two identities on two devices see DIFFERENT
  layer lists + cues from the SAME file (device check, both identities).
- Old app + new bundle: renders shared content, no cues, no crash. New app + old
  bundle: behaves exactly as today (field-absence paths tested).
- Bake dialog never silently captures (e2e: toggle a layer off → dialog shows it ✗ →
  bundle `default_on=false` for it; red-first).
- P201 live flow + A18 per-song + N-series behaviors unregressed (existing suites).

## Out of scope

- Sensitive-layer opt-out (later, VLL-triggered); server auth changes; per-song
  granular apply; P204 retention; iOS host (rides the iOS track as usual).
