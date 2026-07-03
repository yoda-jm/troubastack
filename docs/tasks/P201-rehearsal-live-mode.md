# P201 — Rehearsal live mode: autobake + transient auto-update (I11/I13, R10)

**Priority:** phase 2 (after B03) · **Size:** L · **Area:** core, `web/studio`, `app/shared`

## Context

The constitution carves out exactly one exception to "explicit bake, explicit update":
**rehearsal**. I11: autobake is a special opt-in mode with a **prominent red/orange
banner** in Studio; I13: a presenter may opt into auto-update, but the toggle is
**transient — never persisted, resets to OFF on leaving Stage**. Design R10 adds the
polish that makes it usable: content-hash-based, **viewport-preserving** page swaps so
the score doesn't jump under a musician's eyes.

Everything this builds on exists: bake (B02), distribution (B03, including the inert
`AUTO` policy enum), `rasterHash`/`contentHash` in the container, and the app's
manifest diff.

## Changes

1. **Core**: a per-setlist "live mode" flag (admin toggles it; auto-clears after N hours
   — a forgotten live mode must not survive to the gig). While live: debounce annotation
   commits (~5–10 s quiet period) → auto-bake that setlist (reuse B02's Baker; bump
   `concert_rev` as usual). Endpoint + state in the setlist API.
2. **Studio (I11's banner)**: while the open song belongs to a live-mode setlist, show
   the unmissable red/orange banner ("LIVE — edits are publishing to performers");
   toggling live mode is on the setlist page, admin-only.
3. **App (I13's transient toggle)**: in Stage (not the list), an "Auto-update
   (rehearsal)" toggle — in-memory only, **never** written through the Storage seam,
   reset when the Stage screen is left for any reason. While on: poll the manifest at a
   gentle interval (~15 s), and on a new rev apply it via the importer automatically.
   This is the single sanctioned network touch connected to Stage — implement it in
   `distribution/` driven by a callback the Stage host registers, so the `stage/`
   package's no-network grep gate **still passes**; document the seam.
4. **Viewport-preserving swap (R10)**: after an auto-apply, compare per-page
   `rasterHash`/`contentHash` old→new: unchanged page + position ⇒ stay exactly where
   you are (same page index, same scroll fraction); changed current page ⇒ re-render it
   in place without changing position; structural change (page count) ⇒ keep the
   current song + nearest page index. Unit-test the mapping.
5. **Tests**: debounce (Go — N rapid commits ⇒ one bake); transiency (leave Stage ⇒
   toggle off — instrumented state test); the R10 mapping matrix (commonTest).

## Acceptance criteria

- Two-device rehearsal demo (or emulator + browser): editor draws → within ~15 s the
  Stage device shows it, **without moving the page the performer is on**; Studio shows
  the banner the whole time; killing Stage and reopening shows auto-update OFF.
- Live mode expires on its own (clock-injected test).
- The `stage/` no-network gate and all A04 acceptance gates still pass verbatim.
- Explicit-by-default untouched: with live mode off, nothing auto-applies (regression
  test on B03's offer flow).

## Out of scope

- WebSocket push to the app (polling is fine at rehearsal scale); multi-band live
  sessions; any change to the default explicit flows.
