# P201 — Rehearsal live mode: autobake + transient auto-update (I11/I13, R10)

**Priority:** ✅ IMPLEMENTED 2026-07-13 (core+studio+app); only the attended 2-device test remains ·
**Size:** L (staged: core → studio → app) · **Area:** core, `web/studio`, `app/shared` ·
*Spec refreshed 2026-07-12 against current main (originally written pre-T27/pre-B07;
every building-block claim below re-verified).*

## Context

The constitution carves out exactly one exception to "explicit bake, explicit update":
**rehearsal**. I11: autobake is a special opt-in mode with a **prominent red/orange
banner** in Studio; I13: a presenter may opt into auto-update, but the toggle is
**transient — never persisted, resets to OFF on leaving Stage**. Design R10 adds the
polish that makes it usable: content-hash-based, **viewport-preserving** page swaps so
the score doesn't jump under a musician's eyes.

**Every building block exists on main (re-verified 2026-07-12):**
- `UpdatePolicy.AUTO` is landed and documented "inert in B03 (P201 wires it)"
  (`app/shared/.../distribution/Updates.kt`), alongside the metadata-only manifest
  diff (`Availability`) and the atomic-swap import path.
- `PageRaster.raster_hash` + `Overlay.content_hash` carry the R10 comments in
  `proto/troubastack/v1/bundle.proto` (fields 2/3) and ride every bundle.
- The manifest the app polls is `GET /api/bands/{bandId}/concerts` (authed; the app
  has connect credentials since B03); download is `.../concerts/{id}/bundle`.
- The Baker is autobake-ready: B08's claim loop + B09's two-phase publish make
  concurrent bakes of the same setlist safe BY DESIGN — a debounce race can't corrupt
  or clobber. Per-member parts (B07), encore/bench (T23) and titles (T26) ride along
  with no P201-specific work.
- Retention exists: `troubacore gc` / `bake.keep_revs` (P202) — relevant because a
  2-hour rehearsal at a ~10 s debounce mints dozens of revs (see Changes §1).

## Changes

1. **Core — live mode on a setlist.** An app-layer flag on the runtime setlist
   (REST PATCH, admin-only; the proto `Setlist` message is deliberately not
   involved — the runtime item type already diverges by design). Auto-clears after
   N hours via an injected clock (a forgotten live mode must not survive to the
   gig). While live: debounce annotation commits to the setlist's songs (~5–10 s
   quiet period) → auto-bake via the EXISTING Baker path (claim loop + two-phase —
   no new concurrency reasoning needed). Ship guidance with it: recommend
   `bake.keep_revs` ≥ 1 in the deploy README's rehearsal note (dozens of revs per
   rehearsal; gc reclaims them; `final_locked` is never pruned).
2. **Studio — I11's banner, in the T27 fullscreen reality.** The editor is
   full-bleed now (no Shell header). Placement RULED: a persistent, non-dismissible
   red/orange strip rendered with the editor chrome (the T30 read-only chip / T32
   banner are the visual precedents — but this one is its own element, always
   visible while the open song belongs to a live setlist: "● LIVE — edits are
   publishing to performers"). It must also show on the NON-editor song page
   states and the setlist page. The live-mode toggle itself lives on the setlist
   page (admin-only), next to the existing frozen control.
> **STATUS 2026-07-12 (architect-implemented per VLL):** changes 1 + 2 are DONE
> and landed — **stage 1a** (`eaa393f`) live-mode state/toggle/expiry, **stage 1b**
> (`95db8e8`) the debounced autobaker, **stage 2a** (`3952840`) setlist toggle+banner,
> **stage 2b** (`b49694e`) the in-editor banner. Web+core rehearsal live mode works
> end to end. **What remains is change 3 + 4 below — the APP (stage 3), for the mobile
> lane** + an attended two-device test. Concrete handoff notes are inlined ⤵.

3. **App — I13's transient toggle.** In Stage (not the concerts list): an
   "Auto-update (rehearsal)" control in the Stage top bar (with Scroll/Layers/
   Role/Day — the A08–A15 chrome). **In-memory only — never written through the
   Storage seam** (deliberate contrast: A10's Day/Night toggle IS persisted;
   this one must not be), reset when Stage is left for ANY reason. While on:
   poll the manifest at a gentle interval (~15 s) and on a new rev of the
   CURRENT concert, download + apply via the existing atomic-swap importer.
   Stage stays network-free (I12): the poller lives in `distribution/` (which
   already owns ktor via `ManifestTransport`) and is driven by a callback the
   Stage HOST registers — same layering as B03/B06; document the seam comment.
   > **STAGE 3a DONE (`3d17c53`, architect-implemented):** the SHARED, off-device-testable
> core — `StageState.autoUpdate` (transient/I13), `StageViewModel.setAutoUpdate` +
> `applyUpdate`/`remapCurrent` (the R10 viewport-preserving swap: hash → same-page →
> nearest → clamp; fit/layers/role preserved; A12/A14 follow from `current`), 6 tests.
> **3b DONE (`0c85e63`):** `UpdatesManager.autoUpdateTick` (6 tests). **3c DONE (`97b24bf`):**
> the Android host wiring — StageScreen 'Auto-update'/'● Live' toggle (gated on a
> server-backed concert) + a MainActivity poll loop (LaunchedEffect keyed on the
> transient toggle → autoUpdateTick every ~15s → reload → applyUpdate; cancels on
> toggle-off/Stage-exit). `:shared:check` + `:androidApp:assembleDebug` green.
> **ALL that remains: the ATTENDED 2-device rehearsal test** (editor draws → Stage
> device auto-updates without moving the page) — real hardware + a server, not
> verifiable off-device. iOS host wiring rides the iOS device track.
> **Mobile-lane handoff (server side is ready):** the app already lists concerts
   > via `GET /api/bands/{bandId}/concerts` (B03) and downloads the bundle — the
   > SAME manifest the poller should diff for a new `currentRev` of the open concert.
   > No new server endpoint is needed for stage 3: `currentRev` bumps on each
   > autobake (stage 1b), so the transient poller is "if serverRev > localRev,
   > atomic-swap import" — reuse `Updates.kt`'s `Availability.UpdateOffered` +
   > `BundleImporter`, just triggered automatically instead of on a tap, ONLY while
   > the in-memory toggle is on. `UpdatePolicy.AUTO` (already in `Updates.kt`,
   > inert) is the natural flag. The transiency test mirrors A10's persistence test
   > but asserts the OPPOSITE (leave Stage → the toggle is OFF; nothing written to
   > the Storage seam).
4. **Viewport-preserving swap (R10) — now with the A12/A14 modes.** After an
   auto-apply, map old→new per page by `raster_hash`/`content_hash`:
   - unchanged current page ⇒ stay exactly (same page index, same state);
   - changed current page ⇒ re-render in place, position kept;
   - structural change (page count) ⇒ keep current song, nearest page index.
   The original spec predates A12/A14 — the mapping MUST also cover:
   **facing pages** (A12: preserve the SPREAD — map to the spread containing the
   nearest page) and **scroll mode** (A14: preserve the scroll FRACTION within
   the song when pages resize/renumber). Unit-test the full matrix in commonTest
   (fit × facing × scroll × {unchanged, content-changed, count-changed}).
5. **Tests.** Go: debounce (N rapid commits ⇒ one bake, clock-injected) + live
   expiry; app: transiency (leave Stage ⇒ OFF — instrumented state test), the
   R10 matrix; studio: banner presence gated by an e2e (live setlist ⇒ strip
   visible in the fullscreen editor — panel/pill-safe placement, pixels at the
   gate).

## Acceptance criteria

- Two-device rehearsal demo (or emulator + browser): editor draws → within ~15 s
  the Stage device shows it **without moving the page/spread/scroll position the
  performer is on**; Studio shows the banner the whole time; killing Stage and
  reopening shows auto-update OFF.
- Live mode expires on its own (clock-injected test); the banner disappears.
- Stage stays network-free (the poller is in `distribution/`, host-registered —
  same review bar as B06's discovery glue); the A04-era Stage acceptance
  behaviors still pass verbatim.
- Explicit-by-default untouched: with live mode off, nothing auto-applies
  (regression test on B03's offer flow); `UpdatePolicy.AUTO` remains inert
  unless BOTH the setlist is live AND the device toggle is on.

## Out of scope

- WebSocket push to the app (polling is fine at rehearsal scale); multi-band
  live sessions; any change to the default explicit flows; persisting the
  device toggle in any form; auto-update outside Stage.
