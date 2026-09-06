# T163 — the printed running order gives too much page to its header

**Lane:** core (the `setlistpdf` document). **Kind:** polish. **Number claimed** in the same push.
VLL, 2026-09-06: *"le header est un peu trop épais, y'a possibilité de centrer des choses et mettre des
choses à droite ou gauche ?"*

## The answer to his question is yes, and it needs no new machinery

`setlistpdf` already draws through `MultiCell(w, h, txt, border, align, fill)` and **already uses `"C"`** for
the intermission row. `"L"`, `"C"`, `"R"` are available on every line, and two items can share one row via
`CellFormat` with the first cell not breaking the line. So this is a layout decision, not a capability
problem.

## Why it reads as thick

Four **stacked, left-aligned** rows before a single song appears:

| line | font | row height |
|---|---|---|
| band name | 20 pt bold | 9 mm |
| setlist name | 14 pt bold | 7 mm |
| venue | 11 pt | 6 mm |
| date | 11 pt | 6 mm |

Plus `Ln(3)` and `Ln(4)` around the rule: **≈ 35 mm**, about **12 % of an A4 page**, spent before the
running order starts. On a sheet whose whole job is "what comes next", that is the wrong allocation.

## The shape to aim for

**Three rows instead of four**, by putting the two short facts on one line:

1. **band name** — left, the identity, keep it large but not 20 pt.
2. **setlist name** — left, smaller.
3. **venue left · date right, on the SAME row** — the conventional gig-sheet arrangement, and it uses the
   alignment VLL asked about.

Then tighten the row heights and the two `Ln` gaps. Target **≈ 20 mm**, which buys back roughly four song
lines on the first page.

**Do not centre the band name.** A centred title looks like a programme, not a working sheet: the running
order below is left-aligned and numbered, and a centred header would fight the reading line the eye follows
down the page. Centring is right for the intermission row precisely *because* it is an interruption.

## Constraints that already exist and must not break

- **Absent venue/date still omit the line entirely** — never a bare label, never a zero date (T158). With
  both on one row, absent-both must drop the whole row, and absent-one must not leave a dangling separator.
- The **numbering rule is untouched** — this is layout only.
- Determinism: the document is byte-compared in tests elsewhere; a layout change is fine, a
  non-deterministic one is not.

## ⟨R1⟩ Red first

- **Measure, do not eyeball:** assert the Y position at which the first song row is drawn is below a
  threshold that today's header exceeds. That is the whole point of the task, and "it looks thinner" is not
  a test.
- Venue present + date present → **one** row, venue left, date right.
- Venue absent, date present → still one row, the date still right-aligned, no empty gap on the left that
  reads as a missing value.
- Both absent → the row is not drawn at all, and the first song moves up accordingly.
- The existing T158 assertions (numbering, the intermission row, the on-call section) stay green
  unchanged — if any needs editing, the layout change has reached further than it should.

## Done means

VLL's printed sheet starts its running order noticeably higher on the page, with the venue and date reading
as one line rather than two, and nothing about the numbering or the omission rules has moved.
