# T20 — Duplicate a setlist ("same as last month, swap two songs")

**Priority:** filler, touring-real (USER-JOURNEY #7) · **Size:** S · **Area:** `core/httpapi`, `web/studio`

## Changes

1. `POST /api/bands/{b}/setlists/{s}/duplicate` (member — creating setlists is already
   member-level): server-side deep copy — name `"<original> (copy)"`, all items with
   their overrides (key/tempo/notes) and order. Returns the new setlist. The copy has
   NO bake history (concertId = new setlist id — clean by construction).
2. Studio: a "Duplicate" action on the setlist page (next to Delete, member-visible),
   navigating to the copy.
3. Tests: Go — copy fidelity (items, overrides, order), source untouched, authz
   (outsider 403); e2e — duplicate → edit the copy's name → both listed.

## Acceptance criteria

- Duplicating the seeded "Sat @ The Anchor" yields an independent, fully-editable copy
  with identical items/overrides; baking the copy mints concert rev 1 (no inherited
  history). `make test` + e2e green.

## Out of scope

- Cross-band copy; templates; copying bakes.
