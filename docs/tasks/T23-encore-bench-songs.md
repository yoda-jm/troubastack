# T23 — Encore / bench songs: baked and jumpable, outside the running order

**Priority:** VLL-raised ("the song should be in the bake") · **Size:** M/L ·
**Area:** `core` (app model + bake) + `proto` + `web/studio`; small A-track follow-up

## Context

Routed from the Stage reading-ergonomics proposal §4: on-stage quick-jump (the Songs
picker / A15 drawer) can only reach songs in the baked bundle, and the baker renders
exactly the setlist's songs. An encore or a likely special request that isn't in the
main set is unreachable on stage. Bands want a **bench**: songs baked into the concert
and jumpable, but not part of the performance order.

**Design decisions (resolved, arch 2026-07-07):**
1. **Model: an item-level flag, not a second list.** Setlist items gain
   `onCall: bool` (absent/false = normal). One ordered list;
   bench items keep positions but sort after the main order for display/bake.
   Rationale: no new entity, duplication (T20), overrides, and B07 variants all keep
   working on items unchanged.
2. **Bake:** the baker renders main-order songs first (as today), then on-call songs;
   the bundle's per-song entry carries `onCall` so clients can separate "In order"
   from "On call". **Additive, default-false** — old bundles stay valid, old readers
   ignore the field (verify the app's bundle parser tolerates unknown fields; if it
   doesn't, fix that first and note it).
3. **Studio:** the setlist page gets a "Bench (on call)" section below the main order;
   move items in/out (sets the flag); bench items excluded from the printed/main
   numbering but included in the bake. Member-editable wherever setlist editing
   already is.
4. **Stage (A-track follow-up, ride A15):** the song drawer groups "On call" below
   the main order, visually distinct. Page numbering/pager unaffected (bench pages are
   just the trailing pages of the bundle).
5. **T20 interplay:** duplicate must copy the flag — extend the copy-fidelity test.

## Changes

1. proto + canonical JSON — **as ruled 2026-07-07 (reviews.md), option A:** proto
   gains ONLY the bundle field, `bool on_call = 8;` on `BakedSong` (field 8 is
   T23's; T26's `title` takes 9). The ITEM flag lives on `app.SetlistItem` (Go) +
   the TS mirror exactly like `Notes` — there is no proto setlist-item message
   (pre-existing I1 divergence, deliberately deferred to P203; add a comment on
   `message Setlist` documenting that). Regenerate mirrors per the repo's current
   (pre-P203) process.
2. core: model/repo/service plumbing; baker ordering (main then bench); bundle
   emission; tests — ordering in the bake, flag round-trip on both repo backends,
   duplicate copies the flag, B07 variant bake includes bench with the member's files.
3. studio: bench section UI + move in/out; e2e — add a bench song, bake, bundle
   contains it flagged, main numbering unaffected.

## Acceptance criteria

- A setlist with 3 main + 1 bench song bakes to a bundle whose songs are main-order
  then bench, with `onCall` set on the bench entry; the demo's Stage songs picker
  reaches it (grouping lands with the A-track follow-up — reaching it suffices here).
- Old bundles (no flag) still import/perform; duplicating a setlist preserves bench;
  `make test` + e2e green on both backends.

## Out of scope

- Reordering within the bench; per-member benches; auto-promoting a bench song into
  the order mid-show; the drawer grouping itself (A-track, with A15).
