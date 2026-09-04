# T135 — Tab blocks in the chart dialect: tablature, or tab on top + chords/lyrics below, in the existing chart file

**Lane:** web-core (core/Go + studio). **Size:** S/M core + S studio, stageable. **Status:** spec, ruled
by VLL. **Not frozen.**
**Asked by VLL, 2026-09-04:** *"je cherche à faire un nouveau type de fichier texte qui serait des
tablatures ou éventuellement des mix 50/50 tablature en haut et texte en bas"* — then, on the feasibility
study below: *"seems fine, land it as a spec."*

**Gate note.** T95 §4 ruled that tab grammar would not be added to the dialect *to share fixture code*, and
§7 reserved it as "a product decision with its own spec" if a user ever asked to write tab in the Studio. A
user has. This is that spec; it lands as a proposal for Fable's review on merit before any implementation,
per the standing rule that new designs are reviewed before they are built.

## Verdict, in four lines

- **Feasible, small.** The renderer already draws monospace rows (chord rows) and `cmd/mkcharts` already
  draws tab in Courier 9. What is missing is only *syntax*. A scratch prototype (a patched copy of
  `core/internal/chartpdf`, ~120 lines incl. comments) renders all of §4's cases.
- **No new file type.** A tab chart is a chart whose source contains a tab block. Same generated PDF, same
  `chart-source` API, same `Generated` flag, same bake, same Stage, same folders and `.tband`.
- **The 50/50 mix is authoring, not a mode.** Blocks flow in order: write the tab first and the lyrics
  after; auto-fit (T76) sizes the page. No layout directive.
- **Existing charts are byte-identical.** The three `docs/demo-charts/*.chart` render to the same bytes
  through the prototype (`cmp`), so nothing a band reads today moves.

## 1. What happens today if you paste a tab

`layout()` classifies each line as title / section / chord row / lyric / marker / text. A tab line such as
`e|---0---|` is one whitespace token that is not a chord, so it falls to **plain text** and is drawn in
Helvetica — proportional, so dashes and digits stop lining up between strings. Worse: the chord names an
author writes above a stave (`G   D   Em`) are a valid chord row, which **pairs with the first string
line only** — that line comes out in Courier, the five below it in Helvetica. See
`docs/screenshots/t135-mixed-today.png` (today) beside `docs/screenshots/t135-mixed-proto.png` (the
prototype, same source).

## 2. The grammar

One fenced block, using ChordPro's names — the same choice T77 made for `{new_page}` and T95 for
`{footnote}`; members who know OnSong/ChordPro already know these.

```
# The Open Road
original demo song · capo 2

## Intro riff — 2x
{start_of_tab}                    ← or {sot}; whole line, case-insensitive
     G                 D                 Em                C
e|-----------------0-------------------0---------------|-----------------0---------|
B|-------0-----------------3-------0-------------3------|-------1-----------1-------|
G|---0-------0---------0-------0-------0-----0----------|---0-------0-----------0---|
A|-2---------------3-----------------2-----------------|-2---------------3---------|
E|-3---------------------------------3-----------------|---------------------------|
                                  ← blank line = next stave
     G                 D
e|-----------------0-------------------0---------------|
…
{end_of_tab}                      ← or {eot}; an unclosed block runs to EOF

## Verse 1
G                 C
Morning on the highway, engine running low
```

| rule | decision |
|---|---|
| **Opener / closer** | `{start_of_tab}` or `{sot}`; `{end_of_tab}` or `{eot}`. T77 discipline: the whole line, case-insensitive, surrounding whitespace ignored. `{tab}`, `{sot} x`, `{{sot}}`, `sot` are **not** markers and render as text. |
| **Inside the block** | Every line is content, drawn verbatim in monospace. `#`, `##`, `{np}`, `{fn}` and `**bold**` are **literal** inside a block; the only thing recognised is the closer. A tab can therefore contain anything a tab website produces. A line that is a chord row (chord names above the strings) is drawn bold in the chord colour at the **same** monospace size, so its columns stay over the frets. |
| **Staves** | A blank line inside the block separates two staves. A stave is one layout unit — never split across a page, exactly as a chord+lyric pair is never split today. Consecutive blanks collapse to one gap. |
| **Unclosed block** | Runs to EOF (ChordPro semantics): predictable, and a file that is only a tab needs no closer. The Studio editor hints about it (§7). |
| **Header block** | An opener directly after `# Title` ends the header block (no subtitle lifted). Title → artist line → `{sot}` still yields the artist subtitle. `size:` keeps working. |
| **Character set** | Unchanged (ASCII + Latin-1). Tab notation (`\| - 0–24 h p b r / \ ~ x ( )`) is pure ASCII; `ErrUnsupportedChar` stays. |
| **Any instrument** | Six strings, four, seven, ukulele GCEA, drop-D, drum tabs (`HH\|x-x-x-\|`): nothing is parsed, so nothing is assumed. The block is the only contract. |

## 3. Rendering rules

| rule | decision |
|---|---|
| **Face and size** | Courier, at **one size for all tab blocks in the chart**: `tabPt = 9` at the default 11 pt body, scaling with the body like every other row (T74). It is the size the demo tab PDFs already use and the band already reads on stage. |
| **Never clip** | Tab lines never wrap and must never be cut, so width is a **hard constraint**. The tab size is the *smaller* of the proportional size and the size at which the **longest stave line in the chart** fits the 186 mm column (Courier advance = 0.6 em). Floor **7 pt** (≈125 characters). Longer than that, the save is **refused** with an error naming the line and the limit — the `ErrUnsupportedChar` pattern. Silent clipping of frets is the one failure a stage cannot tolerate. |
| **Leading** | `leadTab = 4.0` mm per string line at 9 pt (1.25× the type — tighter than lyrics so six strings read as one grid); `tabGap = 2.5` between staves; `tabTopGap = 2.0` above a block that follows other content. All scaled with the tab size actually used. |
| **Pagination** | A stave is a unit. Orphan control (T77) keeps a `##` header with its first stave (`firstUnitLead` returns the stave's height). An explicit `{new_page}` between blocks is obeyed as written. A single stave taller than a page is split by line as a last resort, never clipped. |
| **Auto-fit (T76)** | Unchanged in meaning: the largest body size at which no segment overflows its page. The block contributes its height at the size the width rule allows, so a chart with a wide riff may auto-fit to a smaller body than the same lyrics alone. With a manual `size:` the width cap **still applies** to the tab — a manual size never makes a tab overflow. |
| **Transposition (T60)** | **Lines inside a tab block are never transposed**, chord names above the strings included. Frets do not move when the chords do; transposing the names over an untouched stave would print a lie. The transpose form says so in one line. The line-count invariant that keeps annotations anchored holds by construction (the block scan is shared with the renderer — same predicate, not a copy). |
| **Anchors (T95)** | Every stave line records an anchor with its full text, from the same layout walk that draws it, so the demo seed can place a highlight on "bar 1 of the riff" on a dialect chart exactly as it does today on the hand-drawn tab PDF. |
| **Tab-free charts** | No change in bytes — guarded by the existing sha goldens over `docs/demo-charts/*.chart`. |

## 4. What the prototype rendered (real output, not mock-ups)

- **Tab only** — title, artist line, two sections each holding one stave, a `{footnote}` for chord
  shapes. Auto-fit picked the 16 pt ceiling; the tab sits at 13 pt and the 84-character riff still fits.
  This is the "new file type" the request asked about, written in the existing one.
- **Mixed — riff on top, lyrics below** (`t135-mixed-proto.png`) — the exact "tab above, text below"
  layout, produced only by writing the block first. Auto-fit settled at the 11 pt body, tab at 9 pt, one
  page. Sections, chord rows and lyrics come from the untouched code paths.
- **Width rule, deliberately abused** — `size: 16` plus a 111-character pattern: the body is 16 pt as
  asked, the tab is capped by width at 6.7 pt so no fret is lost. The spec's 7 pt floor is one notch
  above this, because 6.7 pt is the lower bound of readable on a 10-inch tablet at the bake's 150 dpi.
- **Transpose +2** on the mixed source: every body chord row moved (`G C / D G` → `A D / E A`); the
  riff's `G D Em C` and every string line stayed byte-identical; line count preserved.

## 5. Width, size and the stage

Courier advances 0.6 em per character; the column is 186 mm.

| body size | tab size | chars per line | typical |
|---|---:|---:|---|
| 16 pt (auto-fit ceiling) | 13.1 pt | 67 | a short riff, one or two bars |
| **11 pt (default)** | **9.0 pt** | **97** | most web tabs (60–90 chars) |
| 8 pt (auto-fit floor) | 6.5 pt | 134 | never reached — the tab floor applies first |
| any | **7.0 pt floor** | **125** | longer lines are refused, naming the line |

Height at the default size: a six-string stave is 24 mm (28 mm with a chord row above); the usable page
holds ~249 mm below the header, i.e. ~8 staves per page for a tab-only sheet, or two staves plus four
sections of lyrics for the mixed sheet. Stage legibility: at 150 dpi a 9 pt Courier digit is ~10 px tall
on the baked page — the same as the demo tab PDFs; at the 7 pt floor ~8 px. The floor is a readability
decision, not a technical one, and is the first number to revisit with a tablet in hand.

## 6. What changes, what does not

| piece | change |
|---|---|
| `core/internal/chartpdf/chart.go` | **edit** — `reTabStart`/`reTabEnd`, `tabBlocks()`/`inTab()`, `tabPointSize()` (width fit + `ErrTabTooWide`), a `tabLine` primitive, one `layout()` case (stave gathering, unit pagination, `note("tab")`), `firstUnitLead`, `parseHeader` (opener ends the header block), anchors via the existing `rec` |
| `core/internal/chartpdf/transpose.go` | **edit** — skip lines in `inTab(lines)`; three lines |
| `chart-source` API, `SongFile`, `Generated`, storage, proto | **none** |
| bake, bundle, Stage, Android/iOS app | **none** — they consume rasterised pages |
| annotations, layers, anchors | **none** (anchors gain stave lines for free) |
| local band folders (B14/B15), `.tband` (T62/T134) | **none** — a `*.txt` with a block is a chart like any other |
| `web/studio/src/pages/song-editor/chartHighlight.ts` | **edit** — block state across lines; classes `hl-marker` (also fixes `{np}`/`{fn}`, plain today) and `hl-tab`; the text-preserving overlay invariant untouched |
| Studio editor (help popover, "New tab" template, lint) | **add** — §7 |
| lyrics paste import (T37: `lyricsimport.go` + `lyrics.ts`) | **optional** — stage 3 wraps runs of ≥3 tab-looking lines automatically (server authority, client mirror) |
| `cmd/mkcharts` | **none now** — the demo tab PDFs *could* later become `.chart` sources and inherit auto-fit + anchors; that is T95 territory, not this task |

## 7. Studio

- **Highlighting.** The tokenizer gains state across lines (in-block or not) and two classes, `hl-marker`
  for the brace markers and `hl-tab` for block content; chord rows inside a block keep `hl-chord`. No
  character added or removed, so the overlay stays under the caret.
- **Entry point.** Beside "New Lyrics & chords", a second choice **"New tab"** creates the same kind of
  chart pre-filled with a template: title line, an open block with `e| B| G| D| A| E|`, the closer. A
  template, not a type; the pool badge stays what it is. T39's naming ruling stands; the pane hint reads
  "Title, sections, chords, lyrics and tab".
- **Lint (hints only, nothing rewritten without a click).** A line matching
  `^[A-Ga-g][b#]?\|[-0-9hpbrx/\\~^()|.\s]+$` outside a block shows a one-line hint under the editor —
  "Looks like tablature. Wrap it in {sot} … {eot}" — with a **Wrap as tab** action on the selection. An
  unclosed block gets a quieter hint.
- **Transpose form.** One sentence when the source contains a block: "Tab blocks are left as written."
- **Preview.** Unchanged — on-demand server render returned as a PDF, so the preview shows the tab exactly
  as it will bake; no client-side rendering to keep in sync.

## 8. Acceptance criteria

- **Marker table:** `{start_of_tab}`, `{sot}`, `{SOT}`, padded, and the four closer forms open/close;
  `{tab}`, `{sot} x`, `{{sot}}`, `sot` render as text.
- **Verbatim:** inside a block `## x`, `{np}`, `{fn}`, `**x**` appear literally in `pdftotext` output; a
  chord row inside the block is drawn Courier-Bold at the tab size (assert via the anchor box height).
- **Byte identity:** every `docs/demo-charts/*.chart` keeps its sha; teeth-check by adding a block to one
  and confirming the sha moves.
- **Width:** a 97-character line at the default body keeps 9 pt; a 111-character line drops the tab size to
  fit; a 126-character line is refused with an error naming the line. Assert `X1 ≤ right` for every stave
  line's anchor.
- **One size:** two blocks with different longest lines draw at one shared size.
- **Pagination:** over a scan of filler lengths pushing a stave across the boundary, no page ends with a
  partial stave, and a `##` header is never a page's last element before a block (extend the T77 trace
  test with kind `"tab"`).
- **Measure drift:** `TestT75_MeasureMatchesRender` extended with a mixed chart; `contentHeight` includes
  blocks.
- **Auto-fit:** the mixed fixture fits one page at 11 pt; with an extra 30-line stave it breaks or shrinks,
  never clips.
- **Transpose:** the mixed fixture at +2 changes every body chord row and no line inside a block; line
  count preserved.
- **Anchors:** every stave line has an anchor whose box contains its text at the rendered size; golden
  pinned.
- **Studio e2e:** create from "New tab" → preview shows a PDF → highlight classes present → lint hint on an
  unwrapped pasted tab, gone after "Wrap as tab".
- `gofmt -l core` clean; `go vet`; `make test` green; dialect docs (the `chart.go` header + the tasks
  README's dialect section) gain the block, its aliases, the stave rule and the width limit.

## 9. Staging

1. **Stage 1 — core (S/M).** Markers, block scan, width fit, `tabLine`, pagination unit, transpose skip,
   anchors, tests, dialect docs. Ships alone: a member can already write a tab and preview it.
2. **Stage 2 — studio (S).** Stateful highlighter, "New tab" template, help popover, transpose note, lint +
   "Wrap as tab".
3. **Stage 3 — paste (S, optional).** T37 import wraps runs of ≥3 tab-looking lines. Demo: one riff moved
   from `mkcharts` to a `.chart` if VLL wants the walkthrough to show it.

## 10. Rejected alternatives

- **A separate "tab" file type with its own renderer.** Doubles the chart-source machinery (endpoints, LWW
  save, transpose eligibility, badge, folder discovery, export) for a document that differs by one block,
  and makes the mixed sheet impossible without composing two files.
- **Auto-detect tab lines, no markers.** Fragile by construction (drum tabs, four-string bass, alternate
  tunings, a lyric beginning `A| …`), and a mis-detection silently changes a page the band reads on stage.
  Detection is kept where a wrong guess costs nothing: the editor lint and the paste importer, which only
  *suggest* the block.
- **A "split page" layout mode.** Sequential flow already yields tab-above/lyrics-below when the tab is
  written first; a hard 50/50 split wastes space on short riffs and clips long ones.
- **Wrap long tab lines.** A wrapped stave is unreadable and a bar broken across rows is wrong music.
  Shrink to fit, refuse below the floor.
- **Transpose the chord names above the strings.** Would print names that no longer match the frets. If the
  band transposes, the tab is a capo question the author writes in the subtitle or a footnote.
- **A width cap per block.** Two staves at two sizes on one page looks like a mistake. One size per chart.

## 11. Decisions to confirm (each has a default; implementation can start without an answer)

1. **Tab size ratio** — 9 pt under an 11 pt body (default, matches the demo tab PDFs), or the same size as
   the lyrics (more legible on a tablet, ~20 % more height per stave, so mixed sheets auto-fit smaller)?
2. **Readability floor** — 7 pt (default), or lower to admit 130-plus-character lines? Decide with a
   tablet on a stand; it is one constant.
3. **Entry point wording** — "New tab" beside "New Lyrics & chords" (default), or one button with a
   template picker?

## Out of scope

- Chord-box rows, drum-groove grids (T95 §2's other residue) — not asked for.
- Any parsing of the tab content (tuning detection, bar counting, playback).
- A second column or a facing layout.
