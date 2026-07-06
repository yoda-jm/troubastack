# B07 — Per-member bake: "Leo sees his tab on stage"

**Priority:** B-track, the top post-loop product gap (USER-JOURNEY #4) · **Size:** L ·
**Area:** `core/internal/bake`, `httpapi`, `app`, `web/studio`

## Context

`my-files` already lets each member keep a personal ordered view of a song's file pool
— but the Baker (B02 decision 2, deliberate) bakes only the **default** shared-pool
file, so every stand shows the same part. The whole point of parts is that Leo performs
from his tab while Marie performs from the vocals chart.

**Design decisions (resolved — the contract questions are the task):**
1. **A personal bake is a separate concert variant, not a fork of the band bake.**
   Keyed `<setlistId>~<userId>` under `bakes/`; `concertId` in its manifest =
   `<setlistId>~<userId>` (the loader doesn't care; ids are opaque). Rev numbering is
   per-variant, same atomic claim/publish (B04 mechanics reused verbatim).
2. **Who bakes it: the member themself** — `POST …/setlists/{s}/bake?scope=mine`
   (member-allowed). This does NOT touch the I11 admin-only question: a member baking
   *their own view* of an admin-curated setlist is reading, not publishing to the band.
   The band-wide default bake stays admin-only until Vincent widens it.
3. **File resolution per song:** the member's `MyFileSelection` first viewable PDF;
   fall back to the default-pool choice when they have no selection (so a personal
   bake always succeeds and equals the band bake in the degenerate case).
4. **Distribution:** `GET …/concerts` returns the band concerts PLUS the caller's own
   variants (never other members'); the app needs no change beyond what the manifest
   already carries — offers/pins/freeze work per-variant for free (they key on
   concertId). Studio's Bake card gains a member-visible "Bake my parts" secondary
   action.
5. **Annotations are NOT per-member-filtered** (same snapshot as the band bake);
   personal LAYER visibility remains Stage's Role/Layers job. (Per-part annotation
   coords may not match a different part's layout — surface the honest caveat in the
   UI: "annotations were made on the default part". A follow-up may bake per-file
   annotations; out of scope here.)

## Changes

1. Baker: accept a file-resolver strategy (default vs. member); variant dir naming +
   listing; `bakedBy`/naming carries "(Leo's parts)".
2. httpapi: `scope=mine` (member), list-merge of own variants, download scoping (a
   member can fetch only band concerts + their own variants — test the negative).
3. Studio: the secondary bake action + variant rows in history.
4. Tests: resolver fallback matrix; authz (member A cannot see/fetch B's variant);
   e2e: member bakes mine → download link appears.

## Acceptance criteria

- Leo (member, with a my-files selection putting the tab first) bakes `scope=mine`,
  downloads on his device, and Stage shows the TAB for that song while Marie's band
  bake still shows the score (emulator screenshot pair).
- Authz negatives green; B04 concurrency tests still green including variants;
  `make test` + e2e green.

## Out of scope

- Widening the band-wide bake to members (I11 — Vincent's call); per-part annotation
  re-projection; auto-baking variants when the band bake happens (nice later — note it).
