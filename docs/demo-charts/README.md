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
| `house-rising-sun-tab.pdf` | **Public domain** | *House of the Rising Sun* (traditional folk, no known author). A one-page guitar tab — a generic demo arrangement of the standard Am–C–D–F–E arpeggio; no copyrighted transcription. |
| `house-rising-sun-drums.pdf` | **Public domain** | *House of the Rising Sun* — a 6/8 drum groove box (hi-hat / snare / kick grid). Original demo notation. |
| `greensleeves.pdf` | **CC-BY-SA 4.0** | *Greensleeves* (traditional English, PD music) — a **real published edition** for voice + guitar (melody, 5 verses, guitar tab). Typeset © 2014 David Kastrup (Mutopia). See the attribution below. |
| `blank-chart.pdf` | **Placeholder** | A generic chart — empty staff systems, bar lines, and chord boxes. No song content at all. |

> **Attribution (CC-BY-SA 4.0)** for *Greensleeves*: typeset by David Kastrup,
> Mutopia-2014/03/10-1943, Mutopia Project — https://www.mutopiaproject.org/ — CC-BY-SA 4.0
> (free to distribute/modify/perform with attribution + share-alike).

### Orchestral scores & parts — real, complete editions (Mutopia Project)

The orchestra's two pieces are **real, complete published editions** downloaded from the
[Mutopia Project](https://www.mutopiaproject.org/) (professional LilyPond typesets). Each
ships as a **conductor's full score + separate string parts**, so the conductor marks up the
score and each player opens their own desk's part:

- **Eine kleine Nachtmusik**, Mozart K.525, 1st mvt — `ek-score.pdf` + `ek-violin1.pdf`,
  `ek-violin2.pdf`, `ek-viola.pdf`, `ek-cello.pdf`. **License: Public Domain** (music and
  edition). Source: Mutopia `MozartWA/KV525/eine-kleine-nachtmusik-mvt1`.
- **Canon in D** (Canon per 3 Violini e Basso), Pachelbel 1680 — `canon-score.pdf` +
  `canon-violin1.pdf`, `canon-violin2.pdf`, `canon-violin3.pdf`, `canon-cello.pdf`. The music
  is public domain; the **typeset edition is © 2015 Michael Fischer v. Mollard (Mutopia),
  licensed CC-BY 4.0** — free to distribute/modify/perform **with attribution**. Source:
  Mutopia `PachelbelJ/Canon_per_3_Violini_e_Basso`.

> **Attribution (CC-BY 4.0)** for *Canon in D*: typeset by Michael Fischer v. Mollard,
> Mutopia-2015/09/02-2047, Mutopia Project — https://www.mutopiaproject.org/ — CC-BY 4.0.

These are committed PDFs (not regenerated in build/CI). To refresh, re-download the
`-a4-pdfs.zip` bundles from the Mutopia work pages above.

Chord-dialect text charts (`*.chart`, rendered server-side as text-chart PDFs — T19) sit
alongside: `open-road-lyrics.chart` (original), `house-of-the-rising-sun.chart` (traditional,
public domain) and `amazing-grace.chart` (Newton, 1779 — public domain). The seed
(`core/cmd/seed`) wires all of these into the demo band **The Troubadours** (DEMO-VID Part A —
the demo is entirely original/public-domain, no copyrighted song titles or lyrics).

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

## Why original + public-domain only?

Reproducing a copyrighted song's actual lyrics, tab or sheet music would be a copyright
violation, so the demo ships none. Everything here is either **written for this project**
(*The Open Road*) or **public domain** (*House of the Rising Sun*, *Amazing Grace*, and
the *Canon in D* orchestral parts) — the same realistic look for demos, screenshots and
the walkthrough video, using only material that is free to ship. (Earlier seeds named
real songs as *metadata* over synthetic placeholder PDFs; DEMO-VID Part A retired those
titles for the real charts above.)
