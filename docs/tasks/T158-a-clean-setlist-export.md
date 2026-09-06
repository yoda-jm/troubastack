# T158 — a clean export of the running order, and one numbering rule for all three surfaces

**Surfaces:** Studio (the export + the setlist list), TroubaCore (the document), TroubaStage (the drawer).
**Lane:** web-core for the document + Studio; mobile only if the drawer fails the shared vectors.
**Kind:** feature. **Number claimed** in the same push as this file.

**Status (web-core):** THE RULE, unified across the web-core surfaces 2026-09-06 — at the gate. Mobile
landed the Kotlin rule + the shared vectors (`4e2dee03`); Fable's GO flagged "nothing reads the shared
vectors." Now they do on the web-core side: a Go `runningorder` package + a TS `runningOrder.ts`, EACH
running the canonical `docs/contracts/running-order-numbering.vectors.json` as a test (Go reads it directly;
TS via vitest) — teeth-checked (a count-everything impl reddens the mid-list intermission/on-call cases).
`SetlistDetail.tsx` now DERIVES its display number from the shared rule instead of the filtered index, so an
intermission (T153) entering the running order can't miscount it (byte-identical today; encore-bench green).
**THE DOCUMENT + UI — landed.** `internal/setlistpdf.Render(Doc)` draws the A4 sheet (header
band·setlist·venue·date with absent optional lines omitted; numbered running order; inline unnumbered
intermission; unnumbered "On call" section) with deterministic bytes (fixed date + catalog sort, like
chartpdf); it is a document — no bake, blob, or bundle. `Service.ExportSetlistPDF` (mirrors `ExportBand`:
membership-gated, reuses `sanitizeFilename`) numbers via the shared `runningorder` rule and maps to the
renderer. `GET /api/bands/{bandId}/setlists/{setlistId}/export` serves it as an attachment; Studio's
`SetlistDetail` has an "Export PDF" button (`api.exportSetlist`, download via a blob URL). Tests: renderer
unit tests (accents, intermission, bench, omitted lines); the httpapi endpoint test (member gets a real PDF
attachment, non-member 403/404); encore-bench + 79 studio unit tests green; Go build/vet/test green, no
mirror drift. **Web-core half COMPLETE.** (Mobile's `RunningOrderNumberingVectorsTest.kt` reader is the
third leg of the shared-contract discipline.)

VLL, 2026-09-06: he wants a **clean export of the running order** — band name, setlist name, venue, date,
and the numbered list of songs; the **intermission** appears in it **without a number** when T153 lands;
and the **on-call bench** likewise. He asked explicitly that the same numbering hold **in Stage and in
Studio**, not only in the export.

## The real work is the rule, not the document

Read that last sentence again: he is not asking for three lists that happen to look alike. Today the rule
is implemented **once in Kotlin** (`StageScreen.kt:1095` — *"NO numbers on the bench"*, A60 P2) and
**separately in TypeScript** (`SetlistDetail.tsx:501` partitions main/bench). An export would be a
**third** implementation, in Go. Three implementations of one rule in three languages diverge — that is
not a prediction, it is what already happened to the concert subtitle and to the chart renderer.

**State the rule once:**

> **A number belongs to a song in the running order.** Nothing else carries one. The numbering counts
> **only main-order songs**, so an on-call song or an intermission never shifts the number of the song
> after it.

That is what makes his "7/12" mean the same thing on the printed sheet, in the drawer, and in the editor.

**Keep the three honest with shared vectors, not with shared code** — the three cannot share a
implementation across Go/Kotlin/TypeScript. Add a small fixture (repo `fixtures/`, the existing
Go↔Kotlin fixture convention) holding a table of setlists and their expected rows, and have **each of the
three surfaces run it as a test**. A rule with one statement and three tests is maintainable; a rule with
three statements is not.

## The document

**A PDF, A4, served by core.** Printable is the point: this is the sheet that gets taped to the floor or
handed to a sound engineer, and the repo already renders deterministic A4 PDFs. It is a **document, not a
bundle** — it must not touch the bake, the blobs, or any bundle field.

Header, in this order: **band name · setlist name · venue · date**. `venue` and `eventDate` are
**optional** on the model (`api.ts:157`) — when absent, **omit the line entirely**. Never print a label
with an empty value, and never let a missing date reach the page as a zero date. That is the whole of the
header's difficulty and it is where this will break.

Body: the numbered running order; then, only if the setlist has any, an **"On call"** section under its
own heading, unnumbered. An intermission (T153) renders as its own unnumbered row inline at its position
in the running order — not moved to the end, since its position is what it means.

## RED FIRST, with teeth

One golden vector, in the shared fixture, and it must contain **all three kinds at once**: three main
songs, an **intermission between the second and the third**, and two on-call songs. Expected numbers:
`1`, `2`, *(intermission — none)*, `3`, then the bench with none.

**The teeth-check:** if an implementation counts the intermission, the third song reads `4` and the vector
reddens. A vector with the intermission at the **end** of the list would pass under both the correct and
the wrong rule — it would guard nothing. Put it in the middle.

Second vector, deliberately degenerate: a setlist with **no** bench and **no** intermission must produce a
plain `1..n` — so the rule cannot be "fixed" by special-casing.

## Ordering against T153

T158 does **not** depend on T153 landing. The rule is *"number only main-order songs"*, which is already
satisfiable today — the intermission simply has no instances yet. Write the intermission row into the
vectors now and let it be the thing T153 turns on.

## Out of scope

The bake, the bundle format, and any change to how positions are stored. `SetlistItem.position` keeps its
current meaning; the **display number** is derived, never persisted — deriving it is precisely what makes
the three surfaces able to agree.
