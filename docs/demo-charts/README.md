# Demo charts — lead sheets, tab, and a placeholder (copyright-safe)

Sheet-music PDFs used as **test/demo artifacts** and for **screenshots**. Everything
here is either **original** (written for this project) or **public domain**; no
copyrighted lyrics, tab, or transcriptions are included — deliberately.

Regenerate them with the dev tool (deterministic, pure Go / `go-pdf/fpdf`):

```sh
cd core && go run ./cmd/mkcharts -out ../docs/demo-charts
```

| File | Kind | Content |
|---|---|---|
| `open-road-leadsheet.pdf` | **Original** | *"The Open Road"* — an original song: page 1 is a chords-over-lyrics lead sheet, page 2 is the intro riff as guitar tab (standard EADGBe). Lyrics, chords and the riff are all written for this repo. |
| `amazing-grace.pdf` | **Public domain** | *Amazing Grace* (words: John Newton, 1779 — long out of copyright). A one-page lead sheet with a simple demo chord accompaniment. |
| `blank-chart.pdf` | **Placeholder** | A generic chart — empty staff systems, bar lines, and chord boxes. No song content at all. |

## Meaningful annotations (the showcase)

`open-road.annotations.json` layers three *purposeful* annotation sets over the lead
sheet (page 0), in the exact shape `POST /api/bands/{b}/songs/{s}/annotations/import`
accepts (page-relative `[0,1]` coords). Substitute `FILE_ID` with the uploaded PDF's id
and `OWNER_ID` with your user id before importing:

- **Form / sections** (`mandatory`) — a highlight band + label flagging the **chorus**,
  so performers can find the form at a glance.
- **Conductor cues** (`mandatory`, `roleTag: conductor`) — *"rit. — watch me"* on the
  last chorus line, a circle round the turnaround, and a pointer. Shown by default only
  to the conductor role; can't be hidden.
- **My notes** (personal) — *"capo 2 ✓"*, a circled tricky chord change, a *"breathe"*
  mark before verse 2, and a freehand flourish.

Rendered in the Studio editor (all layers on) — see
[`../screenshots/demo-chart-annotated.png`](../screenshots/demo-chart-annotated.png).
This demonstrates the layer model: mandatory vs. optional, role-targeted visibility, and
a per-member personal layer, over a realistic chart.

## Why not the "real" songs?

The seeded band's set names real songs (Wonderwall, Hallelujah, …) as *metadata*, but
their PDFs are synthetic placeholders — reproducing those songs' actual lyrics/tab/sheet
would be a copyright violation. These charts give the same realistic look for demos and
screenshots using only original + public-domain material that is free to ship.
