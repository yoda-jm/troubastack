# TroubaStack — Architecture Constitution

This document is **normative**. It lists the invariants that the whole system must uphold.
They are ordered roughly from "most foundational" to "most local". Each invariant has a
**rule**, a **why**, and **where it is enforced**.

Detailed, derivable design lives in [`design/`](design/) and **must not contradict** anything here.
When in doubt, this file wins.

> How to read enforcement: "spec" = encoded in `proto/`; "review" = checked in code review /
> ADR; "test" = there is an automated check; "structure" = the repo layout makes the violation
> hard. Each invariant is also tagged with its status **today** (the rule always stands — the tag
> describes whether a guardrail actually prevents violation yet):
> **✅ enforced today** = an automated test or the type system prevents violation now;
> **🎯 target** = true only by convention/structure, or still aspirational — guardrail not built.
> `docs/tasks/` is the live queue that closes the 🎯 gaps.

---

## A. The contract

### I1 — One source of truth for types and protocol
**Rule.** Every domain type and every wire message is defined once, in [`proto/`](../proto).
All three clients (`core`, `web`, `app`) **generate** their types from it. Hand-written
duplicates of a wire type are forbidden.
**Why.** Three languages (Go, TS, Kotlin) cannot be kept in sync by discipline. Drift here
is the most expensive bug class.
**Enforced.** 🎯 target — spec only today. Codegen has never run: Go, TS, and Kotlin each
hand-write mirrored wire types, kept in sync by review + "AUTHORITY: object.proto" comments
(see [docs/tasks/T09](tasks/T09-proto-reconciliation.md)). Wiring `buf generate` and adopting
generated types in the clients is the open I1 debt.

**Scope note (2026-07-17, T50/T51/A20).** I1's principle — *cross-lane contracts have ONE
generated, guarded source* — is not proto-specific. It also covers the shared **glyph
geometry**: [`web/ink/glyphs.json`](../web/ink/glyphs.json) (+ its Kotlin twin
`CueGlyphData.kt`), both emitted by `web/ink/gen-glyphs.mjs` from the authoring source and
**CI drift-guarded** (regenerate && `git diff --exit-code`). Consumed identically by studio
(cue picker), the bake (T51 stamps), and the app (A20 cue render). Hand-editing a generated
artifact — under `gen/` or anywhere else — is forbidden; placement is consumer-driven, the
invariant is *generated-and-guarded*.

---

## B. Domain & data model  → details in [design/01-data-model.md](design/01-data-model.md)

### I2 — Annotations are objects with client-generated UUIDs; apply is idempotent
**Rule.** Every annotation (freehand, line, shape, text…) is an object identified by a
**client-generated UUID**. Applying an object/mutation is idempotent — re-receiving it (your own
echo or a peer's) by UUID is a no-op or in-place replace, never a duplicate.
**Why.** This single property makes optimistic local rendering, server echo, and multi-user
collaboration collapse into *one* code path; it also kills flicker on commit.
**Enforced.** ✅ enforced today — spec + review + test (engine tests + the backend-parametrized `storetest` suite).

### I3 — Coordinates are PDF-relative `[0,1]`, never pixels
**Rule.** All geometry is stored normalized to the page (`0.0`–`1.0`). Pixels never enter
persistence or the wire.
**Why.** Zoom, rotation, screen size, and the native↔web handoff all align *for free*. Pixel
storage would break every one of them.
**Enforced.** 🎯 target — spec + review; the types encode `[0,1]`, but no automated check guards against a pixel leaking into persistence or the wire.

### I4 — Song history is append-only and linear (no branching)
**Rule.** A song has one linear revision history. The only writes are **append a revision** and
**move a pin**. "Revert to revision N" is implemented as a *new appended head* equal to N
(`git revert`, never `git reset`). There is no fork/branch/merge.
**Why.** Removes all merge complexity; a setlist is just a pin on the line, and revert reuses the
ordinary fast-forward path.
**Enforced.** ✅ enforced today — spec + review + test (engine + `storetest`). *(Need divergent versions? Duplicate as a new song — see design doc.)*

### I5 — LWW per object; delete is a terminal tombstone
**Rule.** Concurrent edits resolve by **per-object last-write-wins** (version/timestamp). **Delete
writes a tombstone that always wins**; a mutation arriving for a dead object is rejected
(`deleted-remotely`) and the client rolls back. Only an explicit **restore** revives a UUID.
**Why.** No OT/CRDT needed for this domain; "an edit silently resurrects a deleted object" is
never what anyone wants.
**Enforced.** ✅ enforced today — spec + review + test (engine LWW/tombstone tests).

### I6 — The server is authoritative; clients are optimistic
**Rule.** TroubaCore holds the truth. Clients render optimistically and **reconcile to server
echoes**. No client is ever a source of truth; an unconfirmed local change lives in an outbox
until the server accepts it.
**Why.** Eliminates the file-sync/conflict-resolver class of failure that motivated the rewrite.
**Enforced.** ✅ enforced today — review + test on the server side (`httpapi/ws_test.go`); the client outbox/reconcile half is exercised by the studio e2e but not unit-tested.

### I7 — GC never breaks a reference (and GC is shared across layers)
**Rule.** Anything *referenced — by ANY layer* (a setlist pin, song `head`, a bake's source
revision, a retained milestone/audit anchor) must always be reconstructable. GC is a single,
**shared, cross-layer reachability pass** over a global root set — never independent per-layer
sweeps — and an **optional retention policy**, never part of correctness. Default: keep full history.
**Why.** Reconciles append-only with compaction: "append-only" guarantees *referenced* history is
immortal, not that every byte is. Reachability spans layers, so a per-layer sweep could prune what
another layer still references. Mechanism: represent cross-layer refs as git refs/tags so `git gc`
*is* the shared GC (see design/01).
**Enforced.** 🎯 target — only the default keep-all tier exists and is tested; the cross-layer reachability sweep (the part the rule is really about) is unexercised.

---

## C. Rendering & ink  → details in [design/03-rendering-and-ink.md](design/03-rendering-and-ink.md)

### I8 — One authoritative renderer (dry); the bake reuses it; the wet overlay is transient
**Rule.** There is **one authoritative rendering path — studio's *dry* renderer**
(`@troubastack/ink`). The **bake runs that same code** (the `web/bake` Node worker; a
different Skia backend, so identity is **held by the golden pixel-parity test**, within
a small anti-aliasing tolerance — never a second implementation, never geometry-parity
alone). The **native *wet* overlay** is the only other renderer; it is **transient and
freehand-only**, is replaced by the authoritative dry render on commit, and so needs
only **visual closeness** (share `web/ink` geometry), **not** pixel-identity.
*(Amended 2026-07-06: the original said "headless studio / pixel-identical by
construction"; B01 built it as same-code-different-backend guarded by the parity test —
design/03 was corrected then, the constitution follows now. The intent is unchanged.)*
**Why.** Sharing *geometry* alone never guarantees identical *pixels* (anti-aliasing, text, sub-pixel
differ per canvas backend). So we don't rely on it where it must match (editor vs bake → *same*
renderer), and we don't require it where a sub-second pop at pen-up is harmless (wet → dry).
**Enforced.** ✅ for the web bake / 🎯 for native — studio uses the one `@troubastack/ink` renderer; `web/bake` renders baked **overlays** through that same renderer, guarded by the **golden pixel-parity test** (`web/bake/test/parity.test.mjs`, in CI: bake vs. the studio dry path in headless Chromium, within a small AA tolerance — B01). The **full bake is real** (B02): core shells out to poppler for page rasters and to the B01 worker for overlays — Go draws nothing (grep-verified at review). Still 🎯: the **native** wet-overlay parity test (A07, blocked on the stylus spike). See [design/03](design/03-rendering-and-ink.md).

### I9 — Native renders only the wet (in-progress freehand) layer
**Rule.** The native overlay renders **only the in-progress freehand stroke**. Everything
committed, and every other tool (lines, shapes, text, move, select) renders in the web layer.
On commit, the stroke migrates native→web and the native layer is cleared.
**Why.** Native exists solely to win input-to-photon latency on stylus ink — the one place it
matters. Keep the native surface as small as physically possible.
**Enforced.** 🎯 target — review + structure; the native wet overlay is an app-side scaffold (the seam exists, no implementation — blocked on the stylus spike, see A07).

### I10 — The web editor is canonical; native is an optional accelerator
**Rule.** `web/studio` is the **complete** editor and runs standalone in any browser. The mobile
app embeds it in a webview. The editor is **never reimplemented** natively; the native overlay is a
feature-detected enhancement with a full in-browser fallback.
**Why.** "Web reach" + one editing codebase + the pure-web client is always the baseline.
**Enforced.** 🎯 target — studio *is* the complete standalone editor (real), but the property is held by convention: no arch check prevents a native reimplementation, and the accelerator path is app-scaffold.

---

## D. Publish & distribution  → details in [design/04-publish-pipeline.md](design/04-publish-pipeline.md), [design/05-distribution.md](design/05-distribution.md)

### I11 — Performable revisions come from a bake; the default bake is manual
**Rule.** Edits flow into songs; a **bake** turns them into performable image bundles. **Default: an
explicit manual bake** (by an admin *or* a band member). **Autobake** is a special, opt-in mode for
**rehearsal / live-modification** only. When live mode + autobake are active, **TroubaStudio shows a
prominent red/orange banner** so editors know their edits are auto-publishing.
**Why.** Performers get stable, deliberately-cut releases by default; autobake is a rehearsal
convenience, never a silent default.
**Enforced.** ✅ manual bake / 🎯 autobake — the pipeline is real (B01+B02): an explicit
`POST …/bake` (Studio "Bake" button) flattens a setlist into a downloadable `.tstage`,
covered by Go orchestration/endpoint tests + the bake e2e spec, and live-verified end to
end at review (real poppler + real web/bake worker → the Kotlin loader accepts the
output). v1 gates manual bake to **admins** (deliberately stricter than the rule's
"admin *or* member" allowance — widen later if the product wants it). Still 🎯: autobake
as an opt-in rehearsal mode + the red/orange live banner (P201).

### I12 — The presenter is offline, dumb, and self-contained
**Rule.** A baked concert is **flattened images** (per page: PDF raster + transparent annotation
overlay). The presenter is a pure image compositor + pager; at performance time it depends on
**nothing** server-side and contains **no** annotation-model or access-control logic.
**Why.** Stage reliability. The smartness happened at bake time, on the server.
**Enforced.** ✅ largely — the TroubaStage presenter is real and *proven* offline/dumb:
it performs locally-imported `.tstage` bundles with no session or server dependency on
**both Android (A04/A05, tested) and iOS (IOS02: simulator-proven with real Stage
pixels; IOS04 keeps the screen awake)**; the loader's never-crash contract is tested
(A02 + torture fixtures), and the bake producing the bundles is real (B02). Residual 🎯:
no *automated* check forbids a server dependency creeping into the presenter — held by
structure/review.

### I13 — Updates are explicit by default; auto-update is transient and never mid-show
**Rule.** New versions are surfaced as offers and **applied only by user action by default**, never
mid-performance. A presenter may opt into **automatic update** (rehearsal), but that opt-in is
**transient — NOT persisted; it resets to OFF every time you leave TroubaStage**, so explicit is
always the real default. Freeze/lock honored at setlist, bake, and device tiers.
**Why.** Nothing shifts under a performer's eyes during a show; a forgotten auto-update toggle can't
carry into a live performance.
**Enforced.** ✅ explicit tier / 🎯 auto tier — B03 landed the full explicit path:
manifest diff → offer chips, applied **only on user tap** through A05's atomic import
(a failed download leaves the installed bundle untouched — tested), FROZEN/LocalPin/
server `final_locked` all suppress offers (10-case unit matrix), the session cookie
lives in EncryptedSharedPreferences, and `stage/` keeps its no-network gate (offers
never appear mid-performance). Still 🎯: the transient AUTO tier (P201 — the enum
exists and is inert; I13's "resets to OFF on leaving Stage" semantics land there).

---

## E. Boundaries & separation of concerns  → details in [design/07-boundaries-and-no-duplication.md](design/07-boundaries-and-no-duplication.md)

### I14 — Layered dependencies point only toward the contract
**Rule.** `proto` ⟂ `core` ⟂ `web` ⟂ `app`. Each depends on `proto` and nothing else cross-layer.
No client imports another client; `core` contains no UI; `proto` imports nothing of ours.
**Why.** Clean seams; any layer is replaceable behind the contract.
**Enforced.** 🎯 target — true by layout/convention (and holds today); no dependency-architecture lint prevents a cross-layer import.

### I15 — Mobile is a thin shell; platform-specific code = exactly three seams
**Rule.** The app is shared Kotlin (Compose Multiplatform). Platform-specific code is limited to
**(1) the WebView host, (2) the low-latency ink overlay, (3) storage** — expressed as
`expect/actual`. Everything else (presenter rendering, downloader, sync client, navigation,
revision logic) is shared. iOS = fill in the three `actual`s.
**Why.** "Keep native to the strict minimum." Anything beyond these three seams is a smell.
**Enforced.** 🎯 target — true by layout/convention and *exercised on both platforms*:
Android and iOS each fill the seams as `actual`s (iOS: Storage + WebViewHost real,
InkOverlay `TODO` pending A07; the iOS entrypoint `MainViewController.kt` and the thin
`iosApp` Xcode shell are entrypoint glue, the analog of `androidApp`). No lint enforces
the three-seam limit — that residual check is the 🎯.

---

## Quick map: invariant → where it lives

| Invariant | Primary home |
|---|---|
| I1 | `proto/`; `web/ink/glyphs.json` + `CueGlyphData.kt` (generated glyph geometry, `gen-glyphs.mjs`) |
| I2 I3 I4 I5 I7 | `proto/`, `core/internal/{domain,store}` |
| I6 | `core/internal/sync`, `core/internal/app` (sessions/auth), client outboxes |
| I8 | `web/ink`, `web/bake` (bake↔dry parity test); native overlay parity test (not yet written) |
| I9 I10 | `web/studio`, `app/.../ink-overlay` |
| I11 | `core/internal/bake` |
| I12 I13 | `core/internal/bake`, `app/.../stage` |
| I14 I15 | repo structure |
