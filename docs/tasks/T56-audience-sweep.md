# T56 — Band/Mine vocabulary sweep (studio surfaces)

**Priority:** normal (scheme A part 3b) · **Size:** studio S · **Area:** `web/studio`.
Presentation only — no API/model change.

## Ruling (Fable, docs/design/09-global-vs-personal-ia.md §3 sweep)

Scheme A: the `AudienceTag` (T55) is the ONE vocabulary component; sweep it onto the
remaining personal/band surfaces so the whole app speaks 👥 Band / 👤 Mine consistently.

## Design (web-core / studio slice)

1. **Layer drawer chip (SidePanels):** replace the ad-hoc `"personal · mine"` / raw-zone
   text with `AudienceTag` — 👤 Mine for my personal layer, 👥 Band for shared/conductor
   (conductor labelled). Another member's personal layer (locked, not mine) keeps a
   neutral zone label (it is neither Band nor my Mine).
2. **Bake card (SetlistDetail):** the two bake options declare audience — "Bake setlist"
   👥 Band, "Bake my parts" 👤 Mine; the bake-history rows use `AudienceTag` instead of
   the "Band"/"My parts" text.

The app-side sweep (Auto-update toggle + layer toggles "just for you" in the settings
sheet) is the MOBILE lane — noted for A-track, not built here.

## Acceptance

- Layer drawer: my personal layer shows 👤 Mine; shared/conductor show 👥 Band
  (conductor labelled) — the same component as the "Drawing on" indicator.
- Bake card: both options + the history rows carry the Band/Mine tag.
- `tsc -b` clean; existing editor/setlist suites unaffected (testids preserved); pixels
  light+dark at the gate.

## Out of scope

App settings-sheet chips (mobile lane); concert/bundle filename wording.
