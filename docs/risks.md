# Risks & interactions — resolved directions

Adversarial read of how the paths interact, with the **resolved direction** for each. WIP.
Status: `RESOLVED` · `DEFERRED` · `RECOMMENDATION`.

---

## Tier 1

### R1 — Editing while viewing a pinned/old revision `RESOLVED`
**Problem:** editing only advances `head` (no branching), but you may view a song through an older
setlist pin → drawing on an old view is incoherent.
**Resolution:** **you can only edit when LIVE on HEAD** (websocket-current). Off-HEAD / stale →
**read-only**. A small delay is tolerated, but an edit whose base is stale *and conflicting* is
**refused server-side** (optimistic concurrency on the target's base version) — the client catches up
and retries. No branching; divergent versions = *duplicate as a new song*.

### R2 — Scope/privacy vs. broadcast sync `RESOLVED` (simplifies sync)
**Problem:** per-object PERSONAL/PART scope can't be enforced by a broadcast-to-all model without
leaking or per-recipient filtering.
**Resolution:** **the BAND is the privacy boundary, not the annotation.** Every band member
receives and may **read *and* amend ALL band resources** — shared files, shared layers, and each
other's layers. That's a *feature*: e.g. song A's "lyrics" file has a shared base layer (section
marks + lyric fixes) that singer & guitarist both edit, plus the singer's extra *cues* layer and the
guitarist's extra *chords* layer. ⇒ **the broadcast sync model stands — no per-recipient filtering.**
"Scope/role" is about **presentation** (which role you present as) and the **Layer model** (R7), not
access control. (Privacy is between bands; you only ever receive your bands' data.) **Non-shared
(owned) layers** are visible to the band but **default-active only for the owner** (others off-by-
default, may toggle on); owner-only edit (RO) + delete. True owner-only-*visibility* is rejected (it
re-adds per-recipient filtering); use client-side encryption if real privacy is ever needed.

### R3 — Revert, not undo `RESOLVED`
**Resolution:** **no undo/redo** — it gives a false sense of a clean inverse, and redo breaks in an
append-only, multi-user world. The only ways back: **view** (read-only any revision), **revert**
(song-revert = all layers, or layer-revert = scoped to one `layerId`) **behind a validation/confirm
window**, and **import-onto-HEAD** (re-apply an old revision's content as new additive actions —
safe, append-only). History is browsable per-**song** and per-**layer**; each commit carries a
**human-readable summary** (= the git commit message). See `docs/design/01`.

---

## Tier 2

### R4 — Renderer parity / baking `RESOLVED`
**Resolution:** geometry-sharing alone does **not** guarantee identical pixels (AA/text/sub-pixel
differ per backend), so parity is handled by *what must match*. **One authoritative renderer
(studio's dry); the bake reuses it** (headless studio) → editor == bake **pixel-identical by
construction**, no separate renderer. The native **wet** overlay is transient + freehand-only,
replaced by dry on commit → needs only *visual closeness* (share `web/ink`), not pixel-identity.
Output = **multiple ordered transparent images, one per layer** (z-order preserved).

### R5 — Scaling / restart `DEFERRED`
**Resolution:** **single-node for now.** If we ever scale, **shard by band** — a band's data is
self-contained, so affinity is clean. Not a current concern.

### R6 — Durability `RESOLVED` (simple model)
**Resolution:** server durability = **WAL-before-ack** (an acked change survives a crash; replay on
restart). Client resilience = **on websocket reconnect, re-sync** (request a fresh snapshot / catch
up from last `seq`). The websocket delivers in order; the server assigns the authoritative `seq`.
Get the WAL-before-ack ordering right at implementation time; the surface is bounded.

---

## Tier 3

### R7 — Layer model (was: layers/promotion gap) `RESOLVED` → now a core model
**Resolution:** a file has **ordered layers** above the PDF. A layer is **shared** (any band member
may amend) or **personal** (owned), and **mandatory** (viewer cannot deactivate — admin/conductor
cues & changes) or **optional** (toggleable — e.g. rehearsal cues). Admin/conductor layers sit
**above the PDF, below your personal layer**. "Promote" = make a layer shared + mandatory. Layers are
created/reordered/changed through the **same per-action mutation + commit** model. Added `Layer` to
the contract.

### R8 — Auth / membership `RESOLVED` (design later)
**Resolution:** pluggable/optional auth. **Anyone can create a band** and **add registered users by
explicit identifier** — UUID, QR code (app onboarding), email, or username. Users are **not
discoverable** (privacy): you add by identifier, you can't browse others.

### R9 — Native↔webview bridge `RECOMMENDATION` (optimization; debug later)
**Recommendation:** correlation-id + ack with a timeout; on timeout studio renders in dry anyway and
native clears after a max wait; reconcile on the next echo; on webview reload, re-request the wet
handoff state.

---

## Tier 4

### R10 — "Why isn't my change on stage?" `RESOLVED` → near-zero manual gates
**Resolution:** **default bake is MANUAL** (admin *or* band member); **autobake** is a rehearsal /
live-modification special. Presenter has **rehearsal vs live** modes: rehearsal may auto-update (chip
/ auto, viewport preserved via `content_hash`); live is pinned/stable. The auto-update opt-in is
**transient — resets to OFF on leaving TroubaStage** (explicit stays the real default). TroubaStudio
shows a **red/orange banner** while live + autobake are active. Gate maze → a mode switch.

### R11 — Bake cost `DEFERRED`
**Resolution:** fine for now; later optimization — server-side / Node / CPU-optimized Go bake
workers. Not a current problem.

### R12 — GC lifecycle `RESOLVED` (impl deferred)
**Resolution:** for the **source history**, GC is **manual** (explicitly launched), **refs/roots are
stored in the storage**, and GC takes a **lock** while running (impl optional now, design mustn't
preclude it). **Baked bundles are different** — a regenerable *cache*, so old/superseded ones are
**auto-GC'd** (keep the latest + any a LIVE presenter is pinned to).

---

## Holds up well
Contract-first layering (I1/I14), the engine⟂store border (in-memory HEAD + per-action commits +
linear history, no git merges), and wet/dry + web-canonical rendering. R2 made the sync model
*simpler* (band boundary ⇒ broadcast stands); R7 (layers) and R10 (autobake + presenter modes) are
the main model additions that fall out of these answers.
