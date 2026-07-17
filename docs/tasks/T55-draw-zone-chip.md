# T55 — Zone-at-draw-time chip: the "Drawing on:" indicator declares its audience

**Priority:** normal (scheme A part 3a) · **Size:** studio S · **Area:** `web/studio`.
Presentation only — no API/model change.

## Ruling (Fable, docs/design/09-global-vs-personal-ia.md §3 + 2026-07-17)

Global-vs-personal scheme A: the ONLY place the personal/band boundary is decided
invisibly today is at draw time — a stroke/stamp silently inherits the active layer's
zone. Fix: the always-visible **"Drawing on: `<layer>`"** indicator gains the audience
chip, so every stroke/stamp declares its audience at the moment it matters.

## Design

- New reusable **`AudienceTag`** component (the scheme's one vocabulary component,
  reused by the later sweep): **👤 Mine** vs **👥 Band**, from `audienceForZone(zone)`
  — classify by who sees the EFFECT (Fable's rule): `personal` → Mine; `shared` +
  `conductor` → Band. Conductor adds a "(conductor)" note.
- The `active-layer-indicator` renders the tag beside the layer name: e.g. "Drawing on:
  My notes 👤 Mine" / "Drawing on: Cues 👥 Band (conductor)". No editable layer → no tag.

## Acceptance

- Drawing on a personal layer shows the 👤 Mine tag; a shared/conductor layer shows 👥
  Band (conductor labelled). The tag updates when the active layer changes.
- `tsc -b` clean; existing editor suites unaffected (testid preserved); pixels
  light+dark at the gate.

## Out of scope

The full Band/Mine chip sweep (scheme A part 3b: layer drawer chips, "Bake my parts",
app settings) — separate.
