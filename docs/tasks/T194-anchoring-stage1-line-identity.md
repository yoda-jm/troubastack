# T194 — Anchoring stage 1: line identity

**Lane:** web-core (core) · **Status:** implemented on `fable/anchoring-stage1`, at the gate. **Size:** L.
**Origin:** `docs/design/anchoring/critique.md` §7 "Stage 1", VLL's choice of option G (README). It serves the
identity half of P14, P15, P19 and P20 (`anchoring-spec.md` Part B).

## 0. Scope

Stage 1 only adds data and functions. The T193 freeze (`chartpdf.anchoringFrozen`) stays **on**; nothing is
projected and nothing a user sees changes. Stage 2 will switch projection over to what this builds.

## 1. Runs carry their source location

Every run in the anchor manifest (`chartpdf.Anchor`) gains four fields, all additive JSON (`srcLine`,
`srcSeg`, `srcChar`, `srcWrap`). The mkcharts shape keeps every old field, so readers are unaffected:

- `SrcLine`: the 0-based index in `chartLines(source)`. A chord+lyric pair is two source lines, so its
  chord row and its lyric carry different lines. A chart with no `# ` line draws "Chart", which comes from
  no line: `-1`.
- `SrcSeg`: the run's ordinal among the runs of its source line, in draw order. One line is several runs
  when it wraps, has `**bold**` splits, or is a chord row with its `(x2)` note.
- `SrcChar`: the **rune** offset, in that line, of the run's first character. That is after the `# ` or
  `## `, after `**` markers, and at the continuation point of a wrapped line.
- `SrcWrap`: the visual-line index within the source line. A continuation can land in the other column or
  on the next page.

The offsets are computed structurally, never by searching for the text:
- `wrapPair` tracks the consumed runes;
- `wrapProseLoc` keeps each word's offset through the re-join;
- footnote lines map back by counting words;
- tab staves carry their line indices (`gatherStaveLines`).

The PDF bytes are unchanged; the goldens and a Render = RenderWithAnchors check prove it.

## 2. Stable line ids (`chartpdf/lineid_t194.go`, `app/chartlines_t194.go`)

**Data model.** There is one `chartpdf.LineIdentity` per generated file, stored beside the source. It is
`Repo.{Set,Get,Delete}ChartLineIdentity`, and in filerepo it is `app.json` → `chartLines`, next to
`chartSources`. The record holds:
- `sourceHash`: the sha256 of the source it numbers;
- `ids`: one per source line, blank lines and directives included;
- `rederived`: an optional flag, with a reason and a render;
- `history`: the last 32 saves, each `{fromRender, toRender, maps}`. The renders are PDF blob hashes, so
  they are the same key as a mark's `PointsRenderHash`. `maps` holds a `CharMap` only for the lines that
  step **modified**.

Ids are 64 random bits in base32 (13 characters). Ids only need to be unique within a file, but a
per-file counter could be re-minted after a re-derivation, onto ids an old link still names.

**Pairing rule:**
1. Lines are aligned by an LCS on their exact text. On a tie, the **later** old line wins. So:
   - a copy inserted directly above a line gets the new id (P14);
   - deleting one of two identical adjacent lines retires the **first**;
   - the two rules are inverses, so insert-then-delete restores every id.
2. Inside each hunk, a deleted and an inserted line are paired as **modified** when their rune similarity
   2·LCS/(la+lb) ≥ **0.6**. The pairs are chosen by a monotone alignment that maximises total similarity, so
   a typo fix pairs even in a hunk that also inserts a line.
   - Measured: a typo is ≥ 0.9; an inserted word is about 0.8; a rewritten lyric is about 0.3–0.45.
   - A rewrite below 0.6 gets a **new id**, and the old id is retired. A mark on it becomes lost (P20).
     That is visible, never a silent re-attachment.
3. Every other new line gets a fresh id, and every other old id is retired. A line moved elsewhere is
   "deleted + inserted": it gets a new id.

**Char map (P19).** Words are aligned first (an LCS on blank/non-blank tokens), then characters inside the
words left unaligned. With a character-only LCS, "the road" → "the red road" moves the r of *road* onto
*red*; the test pins that this does not happen. `CharMap.Map(off)` returns `(new offset, exact)`. A deleted
character maps just after the last kept one, with `exact=false`, which stage 2 turns into "to check".

**Transposition** is carried **by index** (`TransposeChartSource` → `saveChartSource(..., byIndex)`). A
transposed dense chord row (`G C D` → `A D E`) is a 0.4-similarity "rewrite", so a plain diff would retire
every chord-row id. If the line count changes, the diff runs instead.

**Writes that bypass `SaveChartSource`.** Each has a defined behaviour; the Rederived flag tells stage 2 to
mark the file's linked marks "to check", and only stage 2 clears it. A later save **keeps** the flag.

| writer | behaviour |
|---|---|
| `CreateTextChart` | fresh ids, no flag |
| band import (v1/v2), ids in the archive describe the source | kept verbatim, no flag |
| band import, no ids or a mismatch (a pre-T194 or hand-edited folder) | fresh, `import` |
| band re-import over a stable file id, source changed | carried by diff, `import-changed` |
| band re-import, source unchanged | untouched |
| a chart with no ids (it predates T194) | backfilled on first use, `backfill` |
| stored ids whose `sourceHash` ≠ the source | re-derived, `mismatch` (never trusted) |
| `cmd/rerender-charts` (renderer change, same source) | ids unchanged; an empty history step keeps the render chain |
| `DeleteSongFile` | ids deleted with the file |

The seed goes through the HTTP band import, so it is covered.

**Export format.** v2 `repertoire.json` `files[]` (and v1 `manifestFile`) gain an optional `lineIdentity`
holding `sourceHash`, `ids` and `rederived`. **No history is exported:** its render hashes are this
server's past saves. The import re-renders, so a mark that arrives on the current render needs no history,
and any other mark gets `CharMapKnown=false` ("to check"), never a guess. The `sourceHash` is what keeps
⟨F1⟩'s concern away: a hand-edited source cannot inherit stale ids.

## 3. The API stage 2 calls

- Pure: `chartpdf.LocateLine(li, source, anchors, lineID, fromRender, currentRender) (LineLocation, ok)`.
  It returns the line's current index and text, its runs in seg order (each with page and box), and
  `CharMap`/`CharMapKnown`, composed across history from the mark's render. `ok=false` means the line is
  deleted (lost).
- Also pure: `CarryLineIdentity`, `RederiveLineIdentity`, `LinePairs`, `BuildCharMap`,
  `LineIdentity.{Index, Describes, CharMapSince, WithRerender, ClearRederived}`.
- App: `Service.LocateChartLine(fileID, lineID, fromRender)`. It returns `ErrRenderDiverged` when the render
  is not current (P32). Also `Service.ChartLines(fileID)`, which backfills, and
  `Service.ClearChartLinesRederived(fileID)`.

## 4. Tests

- **chartpdf, `lineid_t194_test.go`:**
  - inserting a line above keeps every id;
  - a typo keeps its id, with a shifting map;
  - a word inserted before a circled word moves it, exactly;
  - a duplicate inserted before (adjacent and far) leaves the original its id;
  - a delete retires the id, and the id is never resurrected;
  - for twin lines, the later survives, and insert/delete round-trips;
  - a rewrite gets a new id, with both sides of the threshold pinned;
  - a typo and an insert in one hunk;
  - transposition by index, with the by-diff teeth;
  - mismatch re-derivation;
  - the flag persists through later saves and bypass writes;
  - `CharMapSince` composes, and refuses gaps and unknown renders;
  - the history bound;
  - `LocateLine`.
- **Teeth:** the insert-above and duplicate checks run against two naive identities: ids by index, and
  ids by text+occurrence. Each goes red.
- **chartpdf, `anchors_srcloc_t194_test.go`:** every run maps back to its line and first character, with
  seg ordinals, wrap monotonicity and coverage. Covered:
  - title, subtitle and section (indented too);
  - a chord row with `(x2)`, and its lyric on the next line;
  - bold splits, with the exact offsets;
  - non-ASCII rune offsets;
  - tab lines and a footnote;
  - a `size: 16` wrapped prose line and a wrapped chord pair;
  - two columns.

  The checker itself has teeth: char+1, line+1 and seg=0 are each caught.
- **app, `chartlines_t194_test.go`:**
  - create;
  - a save carries the ids and records the renders;
  - typo → `LocateChartLine` char map;
  - a deleted line is not located;
  - transpose (sabotaging `byIndex` turns it red);
  - backfill, clear, and mismatch;
  - delete file;
  - filerepo reload, a backup copy of `app.json`, and a pre-T194 `app.json`;
  - export → import is exact;
  - a hand-edited folder → `import`;
  - a re-import over stable ids → `import-changed`, carried;
  - an unchanged re-import → untouched.

  Sabotaging the save's carry call reddens three of them.
- **Matrix:** unchanged. `ANCHOR_RED=1` gives the same counts as main, for chartpdf, bandio and sync. No
  `knownRed` entry changes: stage 1 adds identity, but every matrix cell asserts a projected position,
  and projection is still frozen.

## 5. Open questions (for stage 2 or VLL)

- **0.6** is measured on synthetic lines. Critique §9's experiment (replaying VLL's real revisions through
  `LinePairs`) should confirm it before stage 2 relies on it.
- A **moved line** (cut and paste) gets a new id, so its marks become lost. Pairing moves would need a
  second pass over unpaired lines; it is not done.
- The **history** is bounded at 32 saves and is not exported. A mark more than 32 saves behind its chart
  gets `CharMapKnown=false`. Stage 2 could instead re-anchor marks eagerly at save time, when the server
  holds both renders.
- Source and ids are two repo writes. A crash between them is detected (`sourceHash`) and re-derived as
  `mismatch`, not prevented.
