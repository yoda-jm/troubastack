# T137 — Each member reads their own files, in their own order, on Stage

**Owned by: the MOBILE lane, all three stages** (VLL, 2026-09-04 — a deliberate, task-scoped lane
crossing while core is in the seeding rework). **Size:** M/L, staged.
**Status:** gap analysis + design, asked for by VLL 2026-09-04 — *"vérifie ce qu'il manque pour que
chaque personne puisse avoir les fichiers qu'il veut dans le bake et que Stage le présente conformément
à ce qu'on a configuré."*

## Answer to the question that prompted it

**Yes, several people can already have different orders.** `SetMyFileSelection(caller, bandID, songID,
fileIDs)` replaces *the caller's* ordered list, stored as `FileSelection{UserID, SongID, FileIDs []string}`
— an ordered list per (member, song), independent per member. Studio's viewer honours it
(*"getMyFiles returns my order; honour it — don't re-sort by displayOrder"*).

**It does not reach the stage.** The baker picks **one file per song, band-wide** — *"the lowest
DisplayOrder that is a viewable PDF"* — and never reads `FileSelection`; grepping `Selection` in
`core/internal/bake/` returns nothing.

## Gap analysis, layer by layer

| Layer | State | What is missing |
|---|---|---|
| Server model | ✅ | nothing — ordered, per-member, with a replace API |
| `.tband` | ✅ | nothing — travels in `cues.json` as `{member, song, files[]}`, order preserved; an unresolved ref **refuses** the import rather than dropping it |
| Studio viewer | ✅ | nothing |
| **Bundle (proto)** | ❌ | `BakedSong.pages` is one band-wide `repeated PageImages`. No per-identity page dimension |
| **Baker** | ❌ | bakes one file per song; never consults `FileSelection` |
| **Stage** | ⚠️ | identity resolution **already exists** (`IdentityPicker`, the bundle roster, a stored member id — P205 Stage 3a); the page sequence is a flat index it would have to derive per identity |

**The bundle already has two per-member constructs to copy**: `LayerImage.owner` (field 8) and
`MemberCues { member_id, cues[] }` (field 11). This is not a new concept in the format — it is the same
concept applied to pages instead of annotations and cues. Today the bundle personalises **what you
write**; it does not personalise **what you read**.

## Design: a shared page POOL, a per-identity SEQUENCE

Do not bake a bundle per member. P205 deliberately chose one band-wide bundle filtered at view time, and
per-member bundles multiply the artefact the whole band downloads.

1. **`BakedSong.pages` becomes the pool** — the union of the pages of every file any member selected for
   that song, deduplicated by the raster hash the bundle already computes.
2. **Add `repeated MemberPages { string member_id = 1; repeated int32 page = 2; }`** to `BakedSong`,
   mirroring `MemberCues`: the viewer's ordered indices into that pool.
3. **Absent means today's behaviour.** A member with no selection, and a bundle with no `MemberPages`,
   resolve to the current rule (lowest DisplayOrder viewable PDF). The change is additive and an old
   bundle plays unchanged.
4. **Stage** resolves its identity (it already does), maps its linear position through that member's list,
   and derives `songStarts` from the resolved sequence rather than from the pool.

### What this does NOT break, verified

- **Annotation overlays ride the page, not the sequence.** They hang off `PageImages.overlays`, i.e. off
  the pool entry, so a page carries its overlays no matter who sequences it or where.
- **The persisted reading position is already logical.** A46 resolves a saved position as
  `(songId, pageInSong)`, not a flat index — so it survives a different sequence far better than an index
  would. **Still required:** invalidate it when the *resolved identity* changes, because `pageInSong`
  counts within that identity's sequence.
- **Facing pages** compute spreads per song from `songStarts`; they work on any sequence.

### The one thing to measure before building

**The pool is the union, so the bundle grows for everyone.** Today it is one file per song. Measure the
real growth on a band where members genuinely differ before committing — if it is large, the trade is
"one bundle everybody downloads" against "a bundle per identity", and that is a decision to take on
numbers rather than on taste. State the measured factor in the gate submission.

## Staging

1. **Stage 1 — proto + mirrors (core).** `MemberPages` on `BakedSong`; generated mirrors; nothing reads it
   yet. Ships alone.
2. **Stage 2 — baker (core).** Bake the union per song, dedup by raster hash, emit `MemberPages` from
   `FileSelection`. Absent selection ⇒ today's single file. **Report the bundle-size delta.**
3. **Stage 3 — Stage (mobile).** Resolve the sequence for the viewer's identity, derive `songStarts`,
   invalidate a persisted position on identity change.

**RESTAGED 2026-09-04 — VLL: mobile takes ALL THREE stages**, a deliberate lane crossing scoped to this
task while core is deep in the seeding rework. Route-by-first-stage is not violated: the rule exists so a
lane is never handed work it cannot start, and here one lane owns the whole chain, so it can.

Mobile can also decouple from the baker entirely if it wants Stage first: **A03 already built the way
out** — *"the real bundle producer (the server-side bake) doesn't exist yet, and the presenter track must
not wait for it"* — so `core/cmd/mkbundle` plus committed fixtures let Stage 3 be complete and tested
against a fixture bundle carrying `MemberPages`, before Stage 2 emits a real one. Use it or not; the
option is there.

**Two things the crossing costs, worth knowing before starting:**

- **The baker's tests need the bake toolchain** (`pdftoppm`, the Node overlay renderer). Confirm they run
  in this lane's environment *before* committing to Stage 2 — discovering it at submission time is the
  expensive way.
- **Two lanes landing Go on `main` concurrently.** Land Stage 1 (proto) on its own and quickly: a proto
  change regenerates every lane's mirrors, and a slow one collides with whatever core lands next. Expect
  `reviews.md` rebase conflicts and remember main's CI cancels *pending* runs — do not stack pushes while
  a code run is queued.

## Acceptance

- Two members with different selections on one song get different page sequences from **one** bundle.
- A member with no selection reads exactly what they read today, from a bundle built by the new baker.
- A bundle without `MemberPages` plays unchanged on a new Stage (old bundles keep working).
- The same raster is stored **once** when two members select the same file (assert on the pool, not on
  the sequences).
- A persisted position taken under one identity does not silently apply under another.
- Bundle-size delta measured and stated.
