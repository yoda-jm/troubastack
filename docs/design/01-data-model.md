# Design: Data model

Derives from **I2, I3, I4, I5, I7**. Authoritative shapes live in [`proto/`](../../proto); this
doc explains intent.

## Objects

Everything a user draws is an **object** with a client-generated UUID:

```
Object {
  uuid        // client-generated; the idempotency + addressing key (I2)
  type        // FREEHAND | LINE | RECT | ELLIPSE | TEXT | HIGHLIGHT | ICON
  geometry    // points/handles in PDF-relative [0,1] (I3)
  text        // per-type content: TEXT → the label; ICON → the glyph id
  style       // color, width, opacity, ...
  ownerId     // who authored it
  scope       // PERSONAL | PART | ALL
  layerId     // owning layer
  version     // for LWW (I5)
}
```

- **Freehand** = the immutable point-list case (the only type the native wet layer touches, I9).
- **Icon** (T51) = a tinted glyph stamped in page space. Objects dispatch on `type` first, so the
  `text` field carries the **glyph id** for `type=icon` (from `web/ink/glyphs.json`; an unknown id
  renders the `note` fallback), the same way `text` carries the label for `type=text`. `geometry` is
  the bbox and `style.color` the tint. Rendered by the one `@troubastack/ink` `drawIcon` (I8), so the
  editor, the bake, and the app all draw it identically.
- Creation is an *append*; editing is a *mutation event* (`move`/`resize`/`setStyle`/`setText`/
  `delete`) keyed by `uuid`.

## Conflict resolution (I5)

- Per-object **last-write-wins** by `version`.
- **Delete = terminal tombstone.** Always wins. A mutation targeting a tombstoned uuid is
  **rejected** → client gets `deleted-remotely` → optimistic rollback.
- **Restore** is the *only* op that revives a uuid (explicit, deliberate). Restore-vs-redelete is
  itself LWW on the live/dead flag.

## Songs, revisions, pins (I4)

- A **song** has one **linear, append-only** history `r1 → r2 → … → head`.
- The only writes: **append a revision**, **move a pin**.
- **Revert(N)** appends a new head whose content = N. `git revert`, never `git reset`. History is
  never rewritten.
- A **setlist** holds **pins** (one per song). Its only moves: *update-to-latest* (fast-forward to
  head) or *keep-current*. No merge ever exists, because no branch ever exists.
- `head` is **self-contained** (the full live object set) — it does not need ancestors to
  reconstruct. Intermediate revisions exist *only* as revert/audit anchors.

### Need divergent versions of a song?
There is no branch. **Duplicate as a new song** (explicit copy, own linear history). Confirm this
is acceptable — a song carries *one* shared annotation lineage across all setlists.
**OPEN.**

## Layers (R2/R7)
A file has **ordered layers** stacked above the PDF; objects belong to a layer (`Object.layerId`).
A `Layer` carries: `owner` (member UUID, or `_shared_`), `zone` (CONDUCTOR | SHARED | PERSONAL) +
`order` (ordering **within the owner's** layers in that zone), `access` (**RW** = any band member
edits | **RO** = owner edits, others read), `mandatory` (viewer cannot hide — admin/conductor cues)
vs optional (hideable), and an optional `role_tag` for default visibility.

**Z-order = fixed zones, per-viewer (R7 resolved).** There is no single contested global order. The
stack is computed *per viewer*: **PDF < conductor/admin < shared < personal**, and within *personal*
**your own layers float above other members'** activated layers. Each owner sets `order` only
*within their own zone*; zone precedence + "mine on top" is applied per-viewer. So everyone sees
conductor cues at the bottom and their own work on top, with **no cross-user reorder conflicts** —
and, like visibility, it's a per-viewer presentation concern, not shared state. (Server-gated: only
admin/conductor may create CONDUCTOR-zone layers.) **Delete and "promote" are owner/admin only; hide
is a per-viewer LOCAL toggle (optional layers only).** Layer ops flow through the same per-action
mutation + commit model.

**Visibility (R2 — the band is the privacy boundary).** All layers sync to all band members; **no
per-recipient server filtering.** Presentation defaults:
- **shared / mandatory** → visible to everyone (mandatory can't be hidden);
- **non-shared (owned) optional** → **active by default only for the owner**; others still receive
  it, see it **off by default**, and may toggle it on (reading each other's layers is a feature).
  Only the owner edits (RO) / deletes.

Each viewer keeps a **local visibility set** (per file) — a presentation preference, **not** a
mutation (no commit, no audit, affects only your screen). Defaults come from the flags above + your
role (`role_tag`); you may toggle any **optional** layer on/off, but **mandatory layers always stay
on**. Handy preset: **"show only mine + shared"** → really *mine + shared + mandatory* (conductor/
admin mandatory layers always remain; it only hides other members' non-activated **optional** layers).
The **same per-viewer model applies in the presenter** (TroubaStage) — the bundle ships all layers +
flags; role picks defaults; optionals stay toggleable.

> True "owner-only-*visible*" private layers would reintroduce per-recipient filtering + per-recipient
> bundles (the cost R2 removed). If real privacy is ever needed, use **client-side per-layer
> encryption** (data still broadcasts; only the owner decrypts) — not server filtering.

## History, revert & reuse (R3) — no undo, only revert
Every action carries a **human-readable summary** (`Mutation.summary`) — auto-generated ("Marie: +4
strokes · Chords") or user-set for a milestone. With the git backend the summary **is the commit
message**, so `git log` *is* the history-browser feed. History is browsable at **two granularities**:
whole **song**, or one **layer** (filter the action stream by `layerId`).

**There is no undo/redo.** Undo gives a false sense of a clean inverse, and redo (undo-of-undo) breaks
in an append-only, multi-user world. The only ways back are explicit and honest:
- **view** — read-only render of any revision.
- **revert** — **song-revert** (all layers) or **layer-revert** (inverse mutations scoped to one
  `layerId`), each behind a **validation/confirm window** (preview + confirm), never instant.
- **import-onto-HEAD** — export a revision's annotations/files and re-apply them as **new additive
  actions on HEAD** (new UUIDs). Append-only, conflict-free — a safe "cherry-pick from the past"
  that never rewinds; generalizes "duplicate as a new song".

## Retention / GC (I7) — a policy ladder, least→most aggressive

1. **Keep all** *(default)* — every revision; revert to anything; full audit timeline. Cheap:
   history grows with *strokes* (KB), not files (PDFs/images are content-addressed, stored once).
2. **Reachability prune** — keep only `{pins} ∪ {head}`; loses intermediate anchors.
3. **Smart squash** — keep `{pins} ∪ {head} ∪ {one auto-milestone per (song, author, session)}`;
   drop keystroke-level noise. Cleaner timeline, not just deletion.

**Invariant over all tiers (I7):** never break a reference; any pinned/head/anchor revision must
remain reconstructable. Live objects are never destroyed — only redundant intermediate checkpoints
and fully-superseded tombstones.

**Shared & cross-layer (I7).** History lives in several layers — song commits + asset blobs (git),
baked TroubaStage bundle revisions, the audit log. GC is therefore **one coordinated reachability
pass over a global root set**, not per-layer sweeps. Unifying mechanism: represent every cross-layer
reference as a **git ref/tag** — song head = branch, setlist pin = tag, a bake's source revision =
tag *iff* bakes pin sources, milestones = tags — so plain **`git gc` is the shared GC** for all
git-resident layers (commits + blobs pruned together). A layer stored outside git (self-contained
image bundles) keeps its own retention but **must register its inbound references** so git never
prunes under it. One global retention *policy* applies (the ladder above), but a layer may set a
longer **retention floor** where it must outlive the others (e.g. the audit log, for compliance).

**GC lives on top of *history-aware* storage.** GC is layered as a capability —
**`Store`** (current state) ⊂ **`HistoryAware`** (retains revisions) ⊂ **`Collector`** (GC). **Every
current backend is a full `Collector`, including the in-memory `mem`** — which makes `mem` the fast
substrate for unit-testing the whole history / revert / pin / GC contract without disk or services.
The narrow `Store` / `HistoryAware` interfaces remain so *consumers* depend on the minimum they need
(interface segregation); a hypothetical snapshot-only backend could still be `Store`-only. Same
uniform policy + root set; each backend runs the sweep natively (`git gc` / log compaction / row
deletes / in-RAM). Compile-time `var _ store.Collector = (*Mem)(nil)` assertions enforce
capabilities. See `core/internal/store`.

### Resolved (via the shared-GC mechanism)
- **Are bakes GC roots?** Expressed through git refs: if old concerts must be re-openable/re-bakeable
  in the editor, the bake **registers its source revisions as git refs** (→ retained by the shared
  `git gc`); if bundles are self-contained images only, it registers no ref (→ the song revision is
  collectible, the bundle persists independently as images). **Default: self-contained, no ref.**
