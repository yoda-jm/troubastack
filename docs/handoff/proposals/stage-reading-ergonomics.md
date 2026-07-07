# Proposal — Stage reading ergonomics (legacy-app follow-ups)

- **Status:** PROPOSED — awaiting architect validation. **No code until validated.**
- **Raised by:** Mobile App Agent (A-track), 2026-07-07, at the user's (Vincent's) request.
- **Relates to:** A08 (metadata strip), A09 (hardware page turns), A10 (night mode — the
  client-display-pref precedent), A12 (facing pages, landed `35423a3`).
- **Reference:** the legacy native app `~/AndroidStudioProjects/TroubaShare/` (superseded by
  ADR 0001), which the user is mining for reading features that felt good on stage.

The user is reviewing what the legacy app did well and asking which of it belongs in the current
Stage. Per the standing rule (new designs are validated by the review gate before implementation),
this catalogs the items with feasibility + invariant/bake impact for a verdict + prioritization.
None of it is built yet.

---

## 1. Hardware page turns in landscape two-up — one path is correct, one is a defect

**Question raised:** in landscape two-up, does the BT "next page" jump a full spread (2 pages)?

**Answer — pedals/keyboard: yes ✓ (already correct, landed with A12).** A12 routes all in-screen
navigation through one spread-aware `turnNext`/`turnPrev` (`StageScreen.kt:149`): in two-up,
`turnNext` → `nextSpreadPage` (`spreadFor(current)+2`). A09's `stageKeyAction` (PageDown / arrows /
Space — what BT pedals and keyboards emit) goes through these, so **one press advances one spread.**

**Android VOLUME keys: no ✗ — a real bug A12 introduced.** The volume-key turn is wired separately
in `App()` (`MainActivity.kt:149`) and calls `vm.next()` / `vm.previous()` **directly** (turn-by-1),
bypassing the spread-aware turn. Consequence in two-up:

- Spread `[0,1]` shown. Volume-next → `current=1`; but `spreadFor(1)=0` ⇒ **still `[0,1]`** — a
  visual no-op. A second press → `current=2` ⇒ `[2,3]`.
- So volume keys need **two presses per spread, the first doing nothing** — while a pedal advances
  one spread per press. Inconsistent and confusing on stage.

**Proposed fix (validate the placement):** register the volume-key turn from **inside StageScreen**,
where `twoUp` + `turnNext`/`turnPrev` already live, reusing A09's existing `stageVolumeTurn` hook via
the `findActivity()` accessor; remove the `App()`-level `vm.next/previous` volume wiring. Result: a
single spread-aware turn drives keys, pedals, taps, swipes, buttons **and** volume. No new seam (I15
intact — reuses A09's hook), read-only (I12), iOS has no volume-key turn (unaffected). Smallest slice;
looks like a bug-fix task. Suggested id: **A13**.

## 2. Optional two-up toggle — WITHDRAWN

Earlier I floated making two-up an opt-out toggle. The user prefers it **automatic** — so A12's
resolved "automatic, not a mode" decision **stands unchanged**. Recorded here only so it isn't
re-litigated. (Confirmed not a bake concern regardless: the bundle format is content-only.)

## 3. Continuous ("infinite") scroll vs per-page — NEW

**Today:** Stage has `FIT_PAGE` (whole page, discrete turns) and `FIT_WIDTH` (a **single** page,
vertical scroll). Neither is continuous scrolling across *all* pages.

**Legacy:** a per-file/per-member **"View mode" segmented control — Swipe vs Scroll** (Swipe =
`HorizontalPager`, one page; Scroll = `LazyColumn` of all pages), persisted in SharedPreferences
keyed `fileId_memberId`.

**Proposal:** add a continuous-scroll reading mode to Stage. Questions for the architect:
- **Interaction with A12 two-up:** scroll implies a single continuous column ⇒ scroll mode and
  two-up are mutually exclusive (scroll wins when on). OK?
- **A09 turns in scroll mode:** a pedal/key "next" should scroll ~one page (animate to the next page
  top), not paginate. Confirm.
- **Persistence scope:** A10 set the precedent of a **global** Stage pref (Storage KV, entrypoint DI,
  no seam). Recommend global here too (simpler than the legacy per-file model). Architect to decide
  per-concert vs global.
- **Bake:** none — pure client display.

Suggested id: **A14**.

## 4. Quick song access for encores / special requests — ENHANCEMENT + a bake question

**Today:** a **"Songs" dropdown** (`StageScreen.kt:241`, `vm.goToSong`) already jumps to any song's
first page — quick access *exists*, just as a compact menu, not a drawer.

**Legacy:** Concert Mode had a hamburger → **side drawer** song list.

**Proposal (mobile, small):** promote the dropdown to a proper nav drawer — larger tap targets,
current-song highlight, scannable at a glance for a fast encore jump. Read-only, no bake change.
Suggested id: **A15**.

**Open bake question (cross-lane — core/studio, not mobile):** the user notes "the song should be in
the bake." Quick-jump can only reach songs **in the bundle**, and `baker.Bake(bandID, setlistID, …)`
renders exactly the **setlist's** songs in order. So an encore / special-request song is only
reachable on stage if it's part of the baked setlist. **Should a setlist support "bench" / encore-pool
songs — baked and jumpable, but outside the main performance order?** If yes, that's a
core + studio + bake change (setlist model + bake + the Songs picker distinguishing "in order" vs
"on call"), owned by the B/T lane, not this A-track proposal. Flagging for the architect to route.

---

## Ask

Validate and prioritize. If approved, Fable specs the accepted items as tasks (per the task-pack
workflow) and I execute + hold each at the gate. Independent, separately-landable slices:

| id | item | size | lane | bake change |
|----|------|------|------|-------------|
| A13 | volume-key spread consistency (§1) | XS — bug fix | mobile | no |
| A14 | continuous-scroll reading mode (§3) | M | mobile | no |
| A15 | song-jump nav drawer (§4) | S | mobile | no |
| — | encore/bench songs in the bake (§4) | ? | core/studio + bake | **yes** — needs its own spec |

Recommend taking **A13 first** (it's a defect in landed A12 behavior), then A14/A15 by appetite.
No code until this is validated.
