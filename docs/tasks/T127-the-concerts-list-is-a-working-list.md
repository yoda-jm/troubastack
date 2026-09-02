# T127 — make the concerts list a working list: create small, find, clone, and see what is past

**Lane:** web-core (Studio; one deliberate *non*-change in core). **Size:** M.
**Status:** spec, not started.
**Asked by:** VLL, 2026-09-02 — five things in one breath: a concert can be cloned from a "3 dot"
menu; creating a concert should be a small popup because *"on mobile it is 80% of what we see and
the concerts are lost"*; the same search-with-view-more the songs list has; a way to **see** which
concerts are past; and an order with the next/most recent ones first.

They are one task because four of the five live in the same 120-line file
(`web/studio/src/pages/Setlists.tsx`) and would collide as separate branches.

## Survey — most of the parts already exist; this is assembly, not invention

Checked rather than assumed:

1. **Cloning already works, end to end.** `POST /api/bands/{bandId}/setlists/{setlistId}/duplicate`
   (`core/internal/httpapi/webapi.go:97`) → `Service.DuplicateSetlist`
   (`core/internal/app/service.go:2138`): a new concert named `"<original> (copy)"` carrying the same
   metadata and **every item's song, position and overrides, including `onCall`**, with a fresh id and
   **no bake history**, so baking the copy mints rev 1 by construction. The Studio client has
   `api.duplicateSetlist` (`web/studio/src/api.ts:696`, T20) and the **detail page already renders it**
   — as a standalone panel with a full-width button and a sentence of explanation
   (`SetlistDetail.tsx:115`, component at `:941`). **No server work in this task.** What VLL is asking
   for is placement and discoverability.
2. **The "3 dot" menu exists**: `web/studio/src/components/RowMenu.tsx` (T78, hardened in T87 — the
   panel is portalled to `document.body` at `position:fixed` precisely so an `overflow:hidden`
   ancestor cannot clip it into a dead control), with `RowMenuItem`. It has **exactly one call site
   today** (`pages/song-editor/SongDetails.tsx`). This task is its second.
3. **The small-popup creation pattern exists**: `components/NewItem.tsx` (T04) — renders `+ <label>`,
   swaps to the inline form on click, autofocuses the first field, collapses on Escape or Cancel.
   The songs panel already uses it ("Add song"). The concerts page does not.
4. **The search + view-more pattern exists**: `pages/BandDetail.tsx` — `foldText` (`:332`) strips
   accents so `tete` matches `Tété`, the filter (`:382`) is shown only once the list passes
   `SONGS_PAGE = 12`, and `songs-view-more` grows `limit` by one page. **Reuse this shape; do not
   invent a second one.**
5. **The order is the one genuine defect.** `app.SortSetlists`
   (`core/internal/app/ordering.go:35`) sorts concerts **alphabetically by name — the date is not in
   the comparison at all**. Both repos (`filerepo.go:852`, `memrepo.go:689`) call it. So today a band's
   next gig sits wherever its title falls in the alphabet.

## What the page looks like now

`Setlists.tsx` renders, in order: a back link, the title, the section tabs, then an
**always-open `<section class="card">` holding a three-input form** (name, date, venue) plus a
Create button — and only *then* the list. On a phone the list starts below the fold. That is
exactly the "80%" VLL is describing.

## Work

### 1. Creating a concert becomes progressive disclosure

Wrap the existing form in `NewItem`, the way the songs panel does. **Keep every existing
`data-testid` on the revealed form** (`setlist-name`, `setlist-eventDate`, `setlist-venue`,
`create-setlist`, `setlist-create-form`) — `NewItem` only gates *when* the form is shown. Give the
trigger its own id (`new-setlist-btn`).

**⚠ Count the call sites before you start — this is the expensive half of the task.** The create
form is driven by **34 occurrences across 15 e2e spec files**
(`bake.spec.ts`, `bake-pdf.spec.ts`, `bake-progress.spec.ts`, `bake-insecure-origin.spec.ts`,
`editor-live-banner.spec.ts`, `editor-song-cues.spec.ts`, `encore-bench.spec.ts`, `flows.spec.ts`,
`in-app-dialogs.spec.ts`, `setlist-bake-dialog.spec.ts`, `setlist-dnd.spec.ts`,
`setlist-duplicate.spec.ts`, `setlist-live-mode.spec.ts`, `setlist-song-link.spec.ts`,
`setlist-transpose.spec.ts`). Every one of them will start failing on a hidden field the moment the
form is gated. Fix them by **routing them all through one helper** in the existing e2e helpers
module — a second copy of "click the trigger, then fill" pasted fifteen times is how the next such
change costs fifteen edits again.

### 2. Filter + view more, same shape as songs

Reuse `foldText` (lift it into a shared module rather than copying it — one definition, two
callers). Haystack: **name, venue and `eventDate`** — the date string is in it deliberately, so
typing `2026-09` finds a month. Same threshold (12) and the same "view more (N more)" control, with
`setlists-filter` / `setlists-view-more` / `setlists-no-match` ids mirroring the songs ones.

### 3. A "…" menu on each concert row

`RowMenu` per row, containing **Duplicate** (all members — duplicating is member-level, see
`DuplicateSetlist`'s doc comment) and **Delete** for admins only. The page already learns the
caller's role: it fetches the band to decide whether to show the Settings tab (`Setlists.tsx:35-41`)
— reuse that `myRole`, do not add a second fetch.

**The trap:** the row today is a `<Link>` wrapping the whole content. A menu button rendered
*inside* that link makes every tap on "…" navigate. Restructure the row so the trigger is a
**sibling** of the link, not a descendant, and assert it: a test that clicks "…" must still be on
the list page afterwards.

After Duplicate **from the list**, stay on the list and reload it — do not navigate to the copy.
The detail page navigates because you were already inside that concert; from an index the useful
outcome is seeing the copy appear. (The detail page's standalone Duplicate panel may fold into a
header "…" menu in the same pass — it is the same de-cluttering VLL is asking for — but that is
optional here; if you skip it, say so rather than leaving it looking done.)

### 4. Past vs upcoming, and the order

VLL asked the question rather than answering it (*"color ? different list ? something else ?"*).
**Recommendation, to be built unless he says otherwise: one list, upcoming first, with a `Past`
section heading and a muted treatment — not a colour.** Colour alone carries nothing for a
colour-blind reader and every accent in the palette is already spoken for; a separate list or tab
hides history behind a control nobody opens; one list keeps a single search box over everything.

Order within the list:

1. **Upcoming, ascending** — the *next* gig at the very top. That is the reading of "latest/future
   first" that a band actually wants; note it explicitly so a reviewer can disagree on the record.
2. **Undated**, by name (they are drafts; they have no place on a timeline).
3. **Past, descending** — most recent first.

**⚠ Do the ordering in the Studio, not in `SortSetlists`.** That comparator is shared by both repos
and every consumer of a band's concerts — the `.tband` export, anything the mobile bundle inherits,
and the tests that assert today's alphabetical order. Changing it is a separate decision with a
much wider blast radius; this task must leave `core/internal/app/ordering.go` untouched.

**⚠ The date bug you will write if you are not careful.** `eventDate` is a date-only
`"YYYY-MM-DD"` with no timezone. `new Date("2026-09-05")` parses as **UTC midnight**, so in UTC+2 a
`date < now` test calls that concert "past" from 02:00 local **on the morning of the gig** — while
the band is loading the van. Compare **date-only strings** against today formatted in the *local*
zone, and treat the day of the gig as **upcoming**: past means `eventDate < todayLocal`, strictly.

Extract the partition + ordering as a **pure exported function** (`partitionSetlists(setlists,
todayLocal)` or similar) so it can be unit-tested without a browser — the studio already has vitest
(`web/studio/test/`).

## Done when

- On a 390 px viewport the first concert row is visible **without scrolling** — check it at that
  viewport, do not reason about it from the CSS.
- Tapping "…" opens the menu and **does not navigate**; Duplicate from a row leaves you on the list
  with the copy present.
- The filter appears only past the threshold, folds accents, and "view more" reveals the rest.
- Past concerts are visibly separated and still searchable in the same box.
- **Unit vectors that discriminate** — a concert dated **exactly today must be UPCOMING**, and one
  dated yesterday must be PAST. A vector where the correct and the naive-wrong answers agree guards
  nothing, and "today" is precisely where the UTC bug shows.
- **Teeth-check:** replacing the comparison with `new Date(eventDate) < new Date()` makes the
  today-vector **fail**. Prove the guard by reintroducing the regression, not by editing the
  assertion.
- `e2e` is green and the **run reports the same count as before this task, plus the tests you
  added** — a green run with fewer tests than before is not a fix.
- `core/internal/app/ordering.go` is unchanged, and `git diff --stat` shows no `core/` file touched.

## Out of scope

- The mobile app's own concert list (A-track).
- Renaming the copy, or letting the user choose the copy's name/date at clone time — `"(copy)"` is
  the server's contract today and changing it is a server task.
- Any change to bake, live mode, or the `.tband` export ordering.
