# T197 — Anchoring stage 4: re-link the old marks, one by one, with VLL's approval

**Lane:** web-core (core) · **Status:** implemented on `fable/anchoring-stage4`, not pushed, for the gate.
**Size:** S–M. · **Origin:** `docs/design/anchoring/critique.md` §7 "Stage 4", T191 §3 and §3b (VLL reviews
every mark before it is repaired). **Builds on:** T195 (stage 2: `domain.Object.Link`, `chartpdf.LinkMark`,
`Service.ChartRenderForFile`). The tool's shape (data dir, server-lock refusal, writes through the engine)
follows the T193 one-off, `cmd/t193-freeze-served` (deleted in T195; `git show 19a5b656`).

## 1. What it does

`core/cmd/t197-relink` re-links the marks that predate the stage-2 model. Since T195 those stay frozen at
their stored coordinates and never follow their text.

**Candidates:** every live mark on a **generated** chart that has **no** `Link`:
- an old-style anchored mark (`Anchor` set);
- a coordinates-only mark (neither; e.g. the icons restored from a band folder).

Marks on uploaded PDFs, and marks that already carry a `Link`, are never candidates.

**The proposal** is stage 2's own rule, `chartpdf.LinkMark`, applied to the mark's **current stored
position** on the chart's current render. That render must reproduce the stored PDF; otherwise
(`ErrRenderDiverged`) the mark is listed as skipped. The result is linked (to which line), fixed, or, if
serve would say so, check.

## 2. Usage

```
t197-relink -data <dataDir> -out <reviewDir>              # the default (-dry): writes nothing to the data dir
t197-relink -data <dataDir> -apply -approve <idsFile>     # writes the approved marks only
```

Both modes refuse while a server holds the data dir (`filerepo.LockDir`, the same `app.lock` check as
t193). Stdout carries ids and numbers only.

### Dry run

Writes `<reviewDir>/marks.json`, one entry per candidate:
- `objectId`, `songId`, `fileId`, `type`, `text` (a label's text or an icon's glyph), `page`, `box` (stored
  points' bounds);
- `oldState`: `old-style` or `unlinked`; for old-style, `oldAnchor` (run text, occurrence, and where the
  pre-T195 projection puts it);
- `proposed`: `linked` / `check` / `fixed` / `skipped`; `lineId`, `lineNumber`, `lineText`, `wrap`,
  `xRef`, `dx`, `dy` (line-heights), `margin`, `peers`;
- `reason`: one English sentence; `notes`.

And `<reviewDir>/<objectId>.png`: a crop of the stored chart page (the file's blob, `pdftoppm -r 110`),
around the mark plus about 3 line-heights, at least 600 px wide:
- **red**: the mark's box (a text label's box is estimated from its font size);
- **blue**: the proposed line's run box, and a thin tie from its reference point to the mark's top-left;
- **dashed orange** (old-style only): where the old anchor's stage-0-era projection would put the mark,
  using the pre-T195 `Project` ported verbatim in `oldproject.go`. When the old run is gone, or projects to
  another page, no orange box is drawn and the entry says so. Orange is drawn last, so where it coincides
  with the red box the red shows between the dashes.

A fixed or skipped mark also gets `<objectId>-page.png`: its crop shows no text, so the whole page gives
the context.

`marks.json` and the crops carry real chart text. They go in a private scratch directory and into the
private review page (T191 §3b), **never** into the repo or a gate entry.

The dry run is read-only. A chart that has no stored line ids yet (they are backfilled lazily, at the first
stage-2 read) gets provisional ids in memory, and the entry says so. The apply backfills them for real
through `ChartRenderForFile`. The line **text** is the same either way.

### Apply

The approval file lists one object id per line, optionally `<id>:fixed` to store Fixed instead of the
proposal. Blank lines and `#` comments are ignored; anything else is an error before anything is opened.

For each candidate whose id is approved:
- the link is **recomputed** on the current render (`ChartRenderForFile`); the dry run is not trusted;
- the mark is written through the engine as a normal mutation (`KindMove`, version + 1, a new revision):
  Points and Page unchanged, the old `Anchor` dropped, `PointsRenderHash` = the current render, and the link
  must pass `chartpdf.ValidLink`. The tool refuses if the proposal would move the mark.

An unapproved candidate is never written. An approved id that is not a candidate (already linked,
deleted, on an uploaded PDF, unknown) is listed as skipped. A second dry run no longer lists the written
marks.

## 3. Tests (`cmd/t197-relink/main_test.go`)

A temporary file-backed data dir, synthetic only (`testutil.FreezeChart`):
- an old-style anchored rect stored half a line below its old projection;
- an unlinked icon;
- a right-margin rect;
- a far icon (proposal: fixed);
- an already-linked mark;
- a mark on an uploaded PDF.

`TestRelink` checks:
- the dry run lists exactly the four candidates with the right old state, proposal, line id and margin flag,
  writes decodable crops ≥ 600 px wide with red always, blue only when linked, orange only when old-style,
  writes a page image for the fixed mark, and leaves `app.json` and the song logs byte-identical;
- with the server's lock held, both modes return `errLive` and nothing changes;
- apply with `m-old`, `m-far`, `m-margin:fixed` and three non-candidates writes exactly three revisions.
  Each written mark keeps its points, page, type and style, loses its anchor, carries the current hash and
  the expected link (`m-margin` forced fixed). The unapproved icon, the linked mark and the uploaded mark
  are deep-equal to before;
- a second dry run lists only the unapproved icon.

`TestReadApprovals` covers the file format and the flag errors.

**Teeth:** with the approval filter disabled (`if false && !approved`), `TestRelink` goes red:
`revision 8 -> 12, want exactly 3 new revisions`, `m-icon was not approved as a candidate, yet it changed`,
`second dry run candidates [], want [m-icon]`.

## 4. The dry run on a copy of VLL's data (2026-10-12)

Run on a scratch copy (`app.json` and `songs/` copied; `blobs/` and `bakes/` symlinked read-only). Nothing
was applied. 27 candidates on 13 generated charts:

| old state | proposed linked | proposed fixed | skipped |
|---|---|---|---|
| old-style | 15 | 0 | 0 |
| unlinked | 11 | 1 | 0 |

- No chart's render diverged, so nothing was skipped.
- The 15 old-style marks are the 15 in critique §7. The unlinked ones are the 5 icons of T191 §3b (4
  linked, 1 fixed) plus 6 text labels and 1 freehand mark with no anchor.
- One proposal is a margin mark. For one old-style mark the old anchor's run no longer exists, so it has no
  orange box.
- All 13 charts have no stored line ids yet, so every line id in the dry run is provisional; the apply
  backfills them.

## 5. Next

1. Build the private review page from `marks.json` and the crops (approve / skip per mark, `:fixed` as a
   third choice), and let VLL review.
2. With his list: stop the server, take a dated backup of `app.json` and `songs/`, dry-run once more, run
   `-apply -approve`, dry-run again (the approved marks must be gone), and restart.
3. Later cleanup (critique §7): stop carrying old-format `Anchor` fields on the wire.
