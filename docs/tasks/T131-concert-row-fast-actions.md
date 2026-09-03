# T131 — fast actions on the concert row: re-bake, and what else is genuinely worth putting there

**Lane:** web-core. **Size:** M. **Status:** spec, not started. **Not frozen** (Studio).
**Raised by:** VLL, 2026-09-03: *"in the setlist view, the … we can probably add a rebake? see if there
is other opportunities of fast links there."*

## What is on the row today

`Setlists.tsx:192` — a `RowMenu` labelled "Concert actions" with exactly two items: **Duplicate** (any
role) and **Delete** (`myRole === "admin"`, `danger`). Everything else costs a navigation into the
concert.

Baking today lives on the detail page as a **section** (`SetlistDetail.tsx:268`), behind a button that
opens a confirm dialog and is **disabled when the setlist has no songs**, with a title explaining why.
So baking is deliberately guarded, not a one-click — and a row action must not quietly drop that.

## ⚠ The constraint that shapes this whole task

`GET /api/bands/{bandId}/setlists` returns a `Setlist` with **no bake state and no song count**
(`api.ts:157-169` — id, bandId, name, eventDate, venue, notes, createdAt, liveUntil). So from the list,
the UI **cannot currently know**:

- whether a concert has any songs → it cannot reproduce the empty-setlist guard;
- whether a bake exists → it cannot offer a PDF or a bundle link;
- when it was last baked → it cannot tell you the re-bake is worth doing.

**One small server addition unlocks all three:** add **`songCount`** and **`lastBakedAt`** (and the
bake's `downloadUrl` if it is cheap at that point) to the list payload. Do that first; everything below
depends on it. **Do not** work around it by fetching each setlist's detail from the list — that turns
one request into N.

## Work

### 1. Re-bake in the row menu — mirror A42②, do not invent a contract

The app already solved exactly this: *"A42②: one-tap re-bake. `canReBake` is true only for a connected
**ADMIN** of the resume concert's band (the row is hidden otherwise); `bake` is the live re-bake status
driven by the progress poll"* (`HomeScreen.kt:102-104`). Mirror it:

- **Admin only, hidden otherwise** — consistent with `Delete` in this same menu.
- **Disabled with the existing explanation when `songCount === 0`.** Reuse the detail page's wording
  rather than writing a second sentence for the same rule.
- **T103's kick-and-poll is the contract**: `POST` kicks and returns 202, **the poll is the source of
  truth**. So the row must show live status and a terminal result, not fire-and-forget. A re-bake that
  silently does nothing visible is worse than the navigation it replaces.
- **Confirm or not?** The detail page confirms. On a row, the risk is mis-clicking the wrong concert —
  which the dialog exists to catch. Keep a confirmation that **names the concert**; that is the part
  that matters, not the dialog itself.

### 2. The other fast links worth having — and the ones that are not

Judged by one test: *does it save a navigation for something people actually do from a list?*

- **Concert PDF** — worth it. `api.concertPdfUrl(bandConcert)` already exists and is used on the detail
  page; printing the running order is a real from-the-list act. Needs `lastBakedAt` to know it exists.
- **Download the baked bundle** — worth it for the same reason, same precondition.
- **Rename / edit date** — **not** worth it. It is a form, not an action; the detail page is the right
  place and the row would need an inline editor for no gain.
- **Set "live"** (`liveUntil`, P201 rehearsal auto-bake) — **worth considering, and VLL should decide.**
  It is a genuine one-click state with real consequences (edits auto-bake for a window), and it is
  currently invisible from the list even though `liveUntil` **is already in the payload**. A concert
  that is silently live is exactly the thing you want visible on the row. **Show the live state on the
  row regardless**; whether toggling it belongs there is the open question.

**Both PDF and bundle must be absent, not broken, when nothing is baked.** A menu item that leads to a
404 is worse than no menu item.

## Do not

- Do not drop the empty-setlist guard, or reword it into a second version of the same sentence.
- Do not fetch per-row detail to fill the gaps — extend the list payload once.
- Do not add row actions that are forms.
- Do not make re-bake fire-and-forget; T103's poll is the truth.

## Done when

- The list payload carries `songCount` and `lastBakedAt`, with a Go test covering a setlist that has
  never been baked (the null case is the one that breaks UIs).
- **Re-bake** is on the row for admins only, disabled with the existing explanation at zero songs,
  confirms while naming the concert, and shows live progress through to a terminal state — verified by
  actually re-baking a real concert, not by unit-testing the click handler.
- **PDF** and **bundle** appear only when a bake exists, and are absent otherwise.
- The row shows whether a concert is **live**.
- A non-admin sees Duplicate only — checked with a non-admin account.
- `tsc -b` clean, e2e green, `gofmt -l core` empty.
