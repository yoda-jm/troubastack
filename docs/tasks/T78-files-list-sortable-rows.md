# T78 — The Files section as a sortable list with a per-row menu (shared with the setlist)

**Priority:** normal · **Size:** M · **Area:** `web/studio` only — no server change.
Agreed in discussion with VLL 2026-08-19/20.

## Why

A song's file pool is presented as a grid (`files-list`), while the setlist — the other
"ordered list of things you drag around" in this app — has a grip handle and drag-to-reorder
(`SetlistDetail.tsx`). Two different interaction models for the same idea. VLL wants the
Files section to feel like the setlist, with a `…` menu per row for the per-file actions.

## Design (decided)

1. **Extract, don't copy.** The setlist's row + drag behaviour becomes a **shared component**
   (e.g. `components/SortableList`), consumed by both the setlist and the Files list. VLL's
   steer on touch is explicit: *"homogeneity first, we'll fix it later in one single place if
   it is the issue (think components)"* — so the point of the extraction is that a future
   touch/pointer-events fix lands **once**. Do not fork behaviour between the two call sites.
   - Extract the **row/drag primitive**, not the setlist's main/bench grouping — the Files
     list is a single group.
   - The setlist's existing `data-testid`s must stay attached to the equivalent elements
     (repo ground rule 5); its e2e must pass **unchanged**, which is the real proof the
     extraction was behaviour-preserving.
2. **Reorder writes the pool order**: dragging a row PATCHes `displayOrder`
   (`api.updateFile(..., { displayOrder })`, already exists). This is the *shared pool* order —
   distinct from each member's personal `my-files` order, which this task does not touch.
3. **Per-row `…` menu** replacing the current inline controls:
   - **Rename** — meaningful now that T72 made names stick.
   - **Delete** — destructive, so keep whatever confirmation exists today (do not quietly
     drop it in the redesign).
   - **View source** — **only for text charts** (`generated`); a menu that offers it on an
     uploaded PDF is a bug, not a no-op.
   - **Move up / Move down** — see below.
4. **Move up/down in the menu, alongside drag.** Cheap, and it buys three things at once: a
   working reorder path if HTML5 drag turns out to be mouse-only on the tablet (the open
   question we deliberately deferred), a keyboard-reachable path, and a way to reorder on a
   phone. It makes the touch question non-blocking rather than merely postponed.
5. **Permissions unchanged.** Whatever gating the file actions have today (rename/delete/
   reorder) carries over verbatim. This task changes presentation, not who may do what —
   state in the handoff what that gating actually is, so it is recorded rather than assumed.

## Acceptance criteria

- One shared sortable-list component used by **both** the setlist and the Files list; no
  duplicated drag logic remains.
- The setlist's own e2e suite passes **unchanged** (no testid churn, no behaviour change).
- Files list: drag reorders and persists (`displayOrder` PATCHed; order survives reload);
  `…` menu offers rename / delete / move up / move down, plus view source **only** on text
  charts.
- Move up/down produce the same persisted order as the equivalent drag.
- Testids for the new affordances (`file-row`, `file-menu`, `file-menu-rename`, …) and e2e
  covering: reorder by menu, rename, and that view-source is absent on an uploaded PDF.
- `tsc -b studio` clean; `make e2e` green.
- Before/after screenshots of the Files section in the handoff.

## Out of scope

- The **matrix** view (parked by VLL — it needs a new server capability for admin-writes-
  others, and `my-files` is an ordered list that a checkbox grid can't express; both are
  product decisions, not drawing work).
- Per-member `my-files` (the personal selection/order stays in its own panel).
- The add-file flow (**T79**).
- Fixing touch drag itself — if it proves broken, that is a follow-up in the shared
  component, which is exactly why it is being extracted.
