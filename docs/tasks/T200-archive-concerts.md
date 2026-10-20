# T200 — Archive a concert: visible only in Studio, no longer offered on the tablets

Owner: web-core. Spec: Fable, 2026-10-20. Pairs with A91 on the mobile side.

## Why (VLL)

> "je me demande si on ne veut pas aussi « archiver » des concerts, ça les mettrait dans une zone juste visible dans
> Studio et plus proposé dans Stage"

Past concerts pile up. They clutter the tablet list (A91) and Studio's setlist list.

## Model

- **A setlist gets `archived` (bool) and `archivedAt`.** Both are additive, `omitempty`. Existing data reads as not
  archived.
- **Archive / Unarchive is an admin action**, the same bar as bake and live. It is reversible, and nothing is deleted:
  the setlist, its bakes and its notes all stay.
- **Archiving also turns OFF rehearsal live mode** for that concert, if it was on.
- **An archived concert can still be opened, edited, exported and baked in Studio.** Archiving only changes where it
  is listed and whether tablets are offered it.

## Server

- `PATCH /api/bands/{b}/setlists/{s}` accepts `{"archived": true|false}`, admin only.
- **The manifest** (`GET /api/bands/{b}/concerts`) **still includes archived concerts, flagged `archived: true`.**
  - The tablet must be able to tell "archived" from "gone", or A91 would show a local copy as "on this tablet only".
  - The list endpoints (`GET …/setlists`) also carry the flag.
- **The manifest grows a little** (for A91; ruling on A91 Q3). Each concert entry gains `eventDate`, `venue`, and
  per-song `title`. These are additive fields.
  - Server-only rows on the tablet must show WHEN a concert is: two concerts with the same band otherwise read alike.
  - "What changed" can then name the added songs too.
- **Export / import** (`.tband` v2 `setlists.json`) carries `archived`. Add it to the round-trip test.

## Studio (web)

- On the band's setlist list:
  - archived setlists move to a collapsed **"Archives (N)"** section at the bottom;
  - each row has an **Archive** action in its menu, and an **Unarchive** action inside the section.
- Opening an archived setlist shows a discreet "Archived" tag, and a one-click Unarchive.
- **Labels** go on the docs-stage label list.

## Tests

- **PATCH:** admin can archive, a non-admin gets 403, and the live mode is switched off on archive.
- **The manifest** flags archived concerts, and does not drop them.
- **The `.tband` round-trip** keeps the flag.
- **Studio e2e:** archive, then the row sits in Archives; unarchive, then it is back.

## Out of scope

- Automatic archiving after the event date. Possible later: "archive concerts older than N days?" would be a
  suggestion, never automatic.
