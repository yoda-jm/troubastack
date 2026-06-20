# Design: Distribution & the presenter

Derives from **I12, I13**. Covers the baked bundle format and how devices learn about updates.

## Bundle format — flattened images (I12)
Per page, two layers:
- a **rasterized PDF page image** (baked at ~2–3× display DPI for modest crisp zoom),
- one or more **transparent annotation-overlay images** (one per *layer-group* if performance-time
  toggles are wanted; otherwise a single overlay).

The presenter is a **pure image compositor + pager** (scroll / swipe / goto). No `PdfRenderer`, no
stroke drawing, **no annotation-model or access-control logic** — it is fully decoupled from the
editing data model. This is what makes it bulletproof on stage and trivially cross-platform.

- **Size:** transparent overlays compress to ~nothing (mostly empty); page rasters dominate → use
  **WebP** + sane DPI.
- **Parity:** the server overlay renderer reuses `web/ink` via `web/bake` (I8).

> Flattening is right *here* because the presenter only reads/performs. The **editor** stays
> vector/object-model because it needs precise zoom + live editing. Two jobs, two representations.

## Revisions & "what's available to me"
Each bundle carries a **monotonic revision** (for stale comparison) + **timestamp/author** (for
display). Revisions nest:

```
concert { rev, songs: [ { songId, rev } ] }     // concert rev = f(song revs + structure)
```

Two chip types:
- **song-content change** → "Song A changed — apply?" → re-bake+swap just that song's pages
  (small download).
- **structure change** (reorder / add / remove / key·tempo override) → concert-level.

The device runs a cheap **metadata-only** manifest call (a few KB), diffs vs its
**per-song downloaded revisions**, and surfaces:
- downloaded & `server rev > local` → "New version of Concert A available"
- not downloaded but shared to the band → "Concert B is available for your band"

Applying songs individually yields a **mixed-revision bundle**; "fully up to date" = all songs match
server.

## Presenter modes & update policy (I11/I13)
The presenter runs in **LIVE** or **REHEARSAL** mode:
- **LIVE** — pinned/stable; updates are **never** automatic, never mid-show.
- **REHEARSAL** — may **auto-update** (a "new version" chip, or auto), swapping only changed images
  (by `content_hash`) while **preserving the viewport**.

**Default is explicit/manual update.** The auto-update opt-in is **transient — NOT persisted; it
resets to OFF every time you leave TroubaStage**, so explicit is always the real default (I13).
Likewise **autobake** (producer side) is a rehearsal-only special; the default bake is a **manual
admin/band bake** (I11), and TroubaStudio shows a **red/orange banner** while live + autobake are on.

- **Atomic swap** on re-download: fetch to temp, verify, then replace; keep the old until verified.

### Baked bundles are a regenerable cache → auto-GC
Unlike the source history (manual GC, keep-all default — I7/R12), **baked bundles are derived and
regenerable, so old/superseded ones are pruned AUTOMATICALLY** — keep the latest + any a LIVE
presenter is pinned to; the rest serve no purpose and are collected without a manual trigger.

### Frozen, two flavors
- **Local pin** — a performer freezes their copy; no chips; explicit unfreeze.
- **Admin lock** — leader marks a revision `final` band-wide; no new baked revisions are emitted.

Frozen is *safe* precisely because bundles are self-contained images (depend on nothing server-side
staying available). Show a lock/pin icon + "frozen at rev N · date".
