# Annotation anchoring on generated charts: behavioural spec

**Status:** draft for VLL's decision, 2026-10-11. **Base:** origin/main `a936c93d`. T191 and T192 are on hold pending this decision.
**Companion files:** `critique.md` (options and recommendation), `coverage.md` (matrix and gaps),
`red-first-baseline.md` plus `red-first.patch` (the executable matrix and today's red/green), and `explication-fr.md` (for VLL).

This document has two halves:

- **Part A, as built:** what the code does today. Each claim cites origin/main file:line, or a matrix cell that was run.
- **Part B, the proposed contract:** the numbered properties P1–P34 that the red-first tests assert. Each one gives an input state, an action, and an observable result with a tolerance.

Words used throughout:

- **chart:** a generated chart, typeset from text by `core/internal/chartpdf`. An **uploaded PDF** has no source text and is out of scope (P1).
- **line:** one source line of the chart text as drawn. That means the title, subtitle, a section label, a chord row (with its `(x2)` note), a lyric, a text line (all of its **bold** segments), one paragraph of footnote, or a tab line. Since T168 a long line can be drawn as several **visual lines** (segments).
- **run:** one box in the render manifest (`chartpdf.Anchor`). It covers a whole visual line, or one bold segment, or the `(x2)` note.
- **line-height (lh):** the height of the reference run's box. **cw:** that run's average character width, (X1−X0)/rune count.
- **link states**, as Studio would show them (T192; the words are VLL's to choose):
  - **linked:** follows its line;
  - **to check:** followed its line, but something about it is uncertain (proposed new state);
  - **lost:** its line is gone, so it stays where it was;
  - **fixed:** never linked.

---

## Part A. As built (origin/main)

### A1. The data

- `domain.Object` has `Type` (1 freehand, 2 line, 3 rect, 4 ellipse, 5 text, 6 highlight, 7 icon). See `core/internal/domain/domain.go:23-34`.
  - Its `Points` and `Page` are a **cache** of the projection for one render, and `PointsRenderHash` names that render (`domain.go:311-316`).
  - `JumpTo` makes an icon a jump source (`domain.go:317-322`).
- `SourceAnchor` = `RunText` + `Occurrence` + `CharStart`/`CharEnd` + optional `Offset{RelY0,RelY1}` + optional `Span` (`domain.go:226-262, 285-288`).
  - `Occurrence` is 1-based. It counts **runs with identical text**, in source order, across the whole chart (`anchor_project.go:130-142, 210-223`; `Seq` at `chart.go:185-204`).
  - `CharStart`/`CharEnd` are rune indices **clamped to the run's text** (`anchor_project.go:146-151`).
- Studio's point layout per tool: freehand = full path; line, rect, ellipse, highlight and icon = [start, end] drag corners; text = [top-left] only (`web/studio/src/editor.ts:130-145`, `annotations/*.tsx` `pointsForGesture`).

### A2. Which text runs exist (`chart.go`)

These are recorded by `rec(...)`:
- title and subtitle (`chart.go:848, 857`);
- section label (`:1000`);
- chord row (`:1016`) and its `(x2)` note as a **separate** run (`:1026`);
- the lyric under a chord row (`:1035`);
- a text line, **one run per bold segment** (`:1051-1061`);
- each wrapped footnote line (`:1119`);
- each tab line (`chart_tab.go:229`).

Long lines wrap per column (T168: pairs at `chart.go:597-608`, prose at `:630-636`). **Each segment is its own run, with its own text.** Header directives (`size:`, `fit:`, `columns:`, parsed at `chart.go:925`) are not drawn. Two consequences:
- A box's horizontal extent is the **text's** width, not the column's. A run for a short title is only as wide as the title.
- In a chord+lyric pair, the lyric's box and the next pair's chord-row box overlap vertically by about 0.15 lh. The boxes abut, with no gap.

### A3. Linking at creation

- **Realtime create:** `sync/apply.go:80-85` calls `hub.anchorer.AnchorMark`, which reaches `httpapi/anchorer.go:44-63` and then `chartpdf.AnchorObject`.
- **HTTP annotation import:** `httpapi/annotations.go:282-300` does the same.
- **Band-folder import** (`app/bandio_v2.go:595-627`) **never links.** It copies whatever anchor the folder carries. A coordinates-only folder mark stays fixed for ever. The cell `*/import_links_unlinked_mark` is RED; this is how VLL's five unlinked icons arose.
- Linking only happens against a render that reproduces the stored PDF byte for byte (`app/service.go:1328-1334`). Otherwise the mark stays unlinked silently.

`AnchorAt` (`anchor_project.go:104-162`):
1. **On-run:** take the first run whose box contains the mark's bbox **centre** (`:105-115`). This records no offset.
2. **Else beside** (T172): take the nearest run on the page that **overlaps the mark horizontally** (by text width), within `maxOffsetRunHeights = 3.0` (`:63, :72-102`, overlap test `:82`). Record `Offset` in that run's heights (`:153-160`).
3. **Else fixed** (nothing recorded).

It **never sets `Span`**. `AnchorObject` returns early if an anchor already exists (`:233`).

### A4. Re-projection (serve and bake)

- `Reproject` (`:250-268`) runs at serve (`httpapi/annotations.go:158-177`, read-only) and at bake (`bake/baker.go:760-777` through `bake/annotations.go:142`). It acts on any anchored mark whose hash ≠ the current render.
- `Project` (`:169-206`) finds the run by (text, occurrence). Then:
  - the box is the run's character span, at the **run's own height**;
  - or that box moved by `Offset`;
  - or it is widened to `Span`'s run, which never happens because nothing sets `Span`.
- If the run is gone, the mark keeps its frozen coordinates and its stale hash: **lost** (`:259-261`).
- `remap` (`:273-290`):
  - **a mark with ≤ 2 points becomes exactly that box**, as `[(x0,y0),(x1,y1)]`;
  - a path is scaled bbox→bbox.

### A5. Mark edits

`sync/apply.go:105-115` (move, resize, setStyle, setText) applies the client's object **as sent**, its anchor included. It never re-links, never consults the anchorer, and trusts a client anchor. A client that omits the anchor field unlinks the mark (cell `sync setStyle/missing_link_not_dropped` RED).

### A6. Bake, transposition, page count, export

- **Transposition:** the bake renders the transposed source (`baker.go:710-729`) but projects marks against the **untransposed** render (`:760-777`). The code assumes "transpose preserves line count, so geometry is identical" (`:714`).
  - Measured: lyric, title, section and tab run geometry is identical to 1e-6 for G→Ab, F♯ and B♭ (experiment; cell `*/transpose_bake` GREEN).
  - **Untested:** a chord row at the exact wrap limit whose chords grow. T168 wraps chord rows by monospace column count, so a growing chord could re-wrap.
- **Page count:** a mark on a page the render no longer has **fails the bake** with a message (`baker.go:814-822`; `TestBake_ReflowOrphanedOverlay_FailsBake`). A mark on an existing page but no longer on its words bakes silently.
- **Export/import** carries `anchor` and `pointsRenderHash` (`bandio_v2.go:178-182, 367-374, 610-627`; `TestBandFolder_RoundTripsEveryObjectField`).

### A7. What today's code does to each shape

Measured by the red-first matrix, `ANCHOR_RED=1`, 2026-10-11. Fixture: synthetic chart "The Open Road", default size.

| shape | links at create | after an edit that moves its line |
|---|---|---|
| freehand (loop around a word, ±0.5 lh) | on-run | **squashed** into the line box (height ±0.5 lh → 1 lh); x snapped to a character |
| line / arrow (drawn right-to-left) | on-run or beside | **re-drawn top-left → bottom-right**: direction flipped; a straight on-line stroke becomes a **diagonal** across the line; width clamped to the text |
| rect (±0.4 lh, 2 chars wider than text) | on-run | **collapses** to the line's own box; margins lost |
| ellipse (ring around one word) | on-run | collapses to the word's box at line height |
| text (one point) | on-run or beside | snaps to the line's top edge and to a character boundary (about 0.1 lh / ≤0.5 char) |
| highlight (drawn over the words) | on-run | follows within tolerance: it is the one shape the model fits |
| icon (1.6 lh stamp) | on-run | **re-sized** to the line box (about 40% smaller); a 0.5 lh stamp beside a line is re-sized to **zero width, so invisible** |
| jump pair | each icon on its own | each collapses like an icon; `JumpTo` survives |
| any shape in a margin / right of short lines | **never** (T172 overlap rule) | stays while its line moves |
| any shape, after **move/resize** | (keeps the creation link) | snaps back relative to the **old** line; the resize is undone |
| any shape, linked line **typo-fixed** | (exact text gone) | **lost**: frozen silently |
| any shape, **duplicate** of its line inserted above | | jumps to the copy |
| any shape, its long line **re-wrapped** (size / fit / columns) | | **lost**: frozen silently |

Totals: of 202 chartpdf cells, 126 are red. Of 7 sync cells, 6 are red. Of 16 bandio cells, 8 are red. See `red-first-baseline.md`.

Live evidence (VLL's data, 2026-10-10/11, no names):
- 15 anchored marks. 13 already diverge from their own anchor's projection by 0.008–0.133 of the page. 10 were moved or resized after linking.
- 3 boxes over about 3 lines, and 1 box over 2 lines, will collapse to 1 line at the next edit.
- 1 line mark is lost.
- 5 icons have no link.
- Nothing is shown in Studio.

---

## Part B. Proposed contract: "rigid follow by line"

### B1. The model in one paragraph

Every mark on a chart is attached to one **reference line**. The attachment is by the line's **identity**: a stable id carried through edits by a line diff. It is never by text plus occurrence. When the chart changes, the mark is **moved rigidly** with that line: translated, and scaled only by the chart's type-size ratio. It is **never reshaped, stretched, flipped or resized to fit text.**

When the mark's own line was edited, the mark also shifts sideways by the displacement of the character at its left edge. Whenever the server cannot be sure, the mark still moves with its line and is flagged **to check**:
- the line's characters under the mark changed;
- the lines inside a multi-line mark moved apart;
- the mark was nearly equidistant from two lines that then separated;
- the column layout changed under a margin mark.

When the line is deleted, the mark stays where it was and is flagged **lost**. Nothing is ever moved onto other words silently.

### B2. Properties

Tolerances, unless stated otherwise:
- **follow** = every point within **1 cw** horizontally and **0.25 lh** vertically of its expected position, in the **new** render's units;
- **exact** = within **0.001** of the page.

Bracketed after each property:
- **[today]**: holds, violated, or partial;
- **[cells]**: the matrix events that test it (column names in `coverage.md`).

**Scope**

- **P1.** A mark on an uploaded PDF never gets a link, and no chart edit or bake changes its coordinates (exact). [holds] [n/a: PDF]
- **P32.** The server links or re-projects only against a render that reproduces the stored PDF byte for byte; otherwise every mark keeps its stored coordinates (exact). [holds: `service.go:1328`]

**Linking at creation**

- **P2. On text.** A mark created over a line's text links to that line. "Over" means the mark's box contains the vertical centre of one or more visual lines in its column. The reference is the **topmost** such line. [holds for single lines] [`create_on_run`]
- **P3. Beside text.** A mark that covers no line, in the text area of a column, links to the nearest visual line in that column by vertical edge gap, if within **3 lh**. On a tie within 0.25 lh, the line above wins. Horizontal overlap with the text is **not** required. [partial: today it requires overlap with the text's width] [`create_beside_run`, `beside_then_edit`]
- **P4. Margin (DECISION).** A mark entirely in a page margin, or right of every line's text, links to the line it is level with, under the same vertical rule as P3. Its horizontal page position is then **fixed**, because there is no character under it, and only its vertical position follows. [violated: never links] [`create_margin`, `margin_then_edit`]
- **P5. Far.** A mark farther than 3 lh from every line in its column is **fixed**. No edit changes its coordinates (exact). [holds] [`create_far`, `far_then_edit`]
- **P6. Multi-line.** A mark covering several lines links to the **topmost** covered line. [today: centre rule, see P12] [`multiline_reflow`]
- **P7. What is recorded:**
  - the reference line's id;
  - its visual segment;
  - the **character index** under the mark's left edge, if the left edge is over the line's text, otherwise none;
  - the mark's vertical offset from the line's top, in lh;
  - the runner-up line's id if it was within 0.5 lh of the same distance (see P22).

  The mark's own geometry is kept as drawn, relative to that origin. [violated: today records a clamped character span and an offset, and loses the mark's own extent]

**Following**

- **P8. Rigid follow.** After a chart edit in which the reference line still exists, every point of the mark is at its old position plus the reference line's displacement (follow tolerance). The mark's point count and order are unchanged, and it is on the line's new page. [violated for every shape except highlight and text; see A7] [`insert_line_above`, `delete_line_above`, `section_added_above`, `restyle_then_edit`, `multiline_reflow`]
- **P9. Type size.** When the type size changes (`size:`, `fit:`, or the auto-fit that `columns: 2` implies), the mark is scaled by k = new lh / old lh about its reference origin, along with its offset and its own size (follow tolerance in new units). [violated by collapse] [`size_change`, `fit_change`]
- **P10. Shape preserved.** A line or arrow keeps its start and end (direction). A freehand stroke keeps its point order and shape. Neither is ever mirrored or re-fitted to a box. [violated: lines flip] [all line and freehand cells]
- **P11. Stamps and labels keep their size.** An icon or a text mark keeps its aspect ratio. Its size changes only by k (±10%). It is never re-sized to a line box, and a small stamp never becomes zero-width. [violated: icons shrink by about 40% or vanish] [all icon, jump and text cells]
- **P12. No stretch (DECISION).** A multi-line mark never stretches or shrinks when the lines it covers move apart or together. It follows its topmost line (P8) and becomes **to check**. [today: a box centred on a run collapses; one centred in a gap keeps its size by luck] [`multiline_insert_inside`]
- **P13. Unrelated edits.** An edit that does not move the reference line leaves the mark's coordinates unchanged (exact). [violated: any edit re-projects, so boxes collapse and text snaps] [`edit_unrelated_line`]
- **P14. Duplicates.** Inserting or deleting a line whose text is identical to the reference line, anywhere, does not change which line the mark follows. [violated: occurrence shift] [`duplicate_inserted_before`]
- **P15. Re-wrapping.** When a long line wraps differently (size, fit, columns, or its own edit), a mark whose left edge is over the text follows **that character** to whichever segment it lands on (follow), and stays **linked**. [violated: lost] [`size_change_wrapped_line`, `columns_change`]
- **P16. Columns.** Switching between one and two columns moves an on-text mark with its character (P15) into the other column. A margin mark follows vertically and becomes **to check**. [violated: lost on any line long enough to wrap in the narrower column] [`columns_change`]
- **P17. Pages.** A mark follows its line to another page, and its page index is the line's new page. [holds where nothing collapses] [`page_count_shrink`]
- **P18. Transposition.** A transposed bake draws every mark at the same page coordinates as the untransposed chart (exact), including marks on chord rows. [holds; the chord-row wrap-limit case is untested] [`transpose_bake`]

**When the reference line itself changes**

- **P19. Edited line (typo).** If the reference line was edited but the line diff still pairs it with its old self, the mark stays **linked**. It follows the line (P8), shifted sideways by the displacement of its left-edge character. It becomes **to check** only if a character under the mark's horizontal extent changed. [violated: lost] [`typo_linked_line`, `typo_then_edit`]
- **P20. Deleted line.** If the reference line was deleted (the diff pairs it with nothing), the mark keeps its coordinates (exact). It is **lost**, is counted in Studio, and is never re-attached to other text automatically. [holds for the coordinates; the counter is T192] [`delete_linked_line`]
- **P21. Recovery.** A lost or to-check mark that the user moves re-links at its new position (P23) and leaves the lost or to-check state. [n/a today: lost marks re-link only with T191]
- **P22. Ambiguity.** If at link time a runner-up line was recorded (P7), and an edit moves the reference and the runner-up by displacements that differ by more than 0.25 lh, the mark follows its reference and becomes **to check**. [new]

**Mark edits**

- **P23. Move.** After a move, the mark links from its new position as if just created (P2–P5). A later chart edit uses the new link (follow), and a move into empty space makes it **fixed**. [violated: the old link is kept] [`move_then_edit`; sync `move/*`]
- **P24. Resize.** The same as P23, with the new size. [violated] [`resize_then_edit`; sync `resize/*`]
- **P25. Restyle and text change.** setStyle and setText keep the **stored** link and do not move the mark. [partial: they keep whatever the client sends] [`restyle_then_edit`; sync `setStyle/*`, `setText/*`]
- **P26. The server decides.** On every mutation the server ignores a client-sent link. A mutation that omits the link does not unlink the mark. [violated] [sync cells]
- **P27. Stale view.** A move made against an older render (its render hash is not current) keeps the previous link, and the next move fixes it (T191). [not tested]

**Jump pairs, pages, folders, bake**

- **P28. Jump pairs.** Each icon of a jump pair links independently (P2–P5) and follows its own line (P8, P11). Re-projection never alters `JumpTo`, and the bake resolves the jump to the destination's current page. [partial: pairing holds, icons collapse] [all `jump_pair` cells; `TestResolveJumps_PairResolvesToPageAndAnchor`]
- **P29. Page shrink.** When the chart loses pages, linked marks follow (P17). A fixed or lost mark whose page no longer exists **fails the bake** with the song named (holds today, `baker.go:814-822`), and Studio lists such marks before any bake (new). [`page_count_shrink`]
- **P30. Export/import round trip.** A band-folder export then import returns every mark's link and render hash unchanged, and `JumpTo` with them. [holds] [`*/export_import_round_trip`]
- **P31. Import links.** A mark imported without a link onto a chart whose render reproduces the blob is linked by P2–P5 at import, on both the band-folder and the HTTP path. [violated on the band-folder path] [`*/import_links_unlinked_mark`]
- **P33. Serve equals bake.** For the same stored marks and render, Studio (serve) and the tablet bundle (bake) place every mark at identical coordinates (exact). [holds by construction: one `Reproject`; `TestSnapshotToDoc_ReprojectsStaleMark`]
- **P34. Visible state.** On a chart, Studio shows each selected mark's state (linked, to check, lost, fixed) and a per-file count of lost and to-check marks. None of this ever reaches a bake, a PDF download or Stage. [not built: T192]

### B3. Per-shape rules: the only places shapes differ

| shape | left-edge character (P7) | size under k (P9, P11) | special |
|---|---|---|---|
| freehand | min x of the path | the whole path is scaled by k | point order kept (P10); a scribble over 3 lines does **not** stretch (P12) |
| line / arrow | min x of the two ends | scaled by k | ends move **together**, never independently (decision; see critique §5.4); direction kept (P10) |
| rect, ellipse, highlight | box left | box scaled by k | own margins kept; never clamped to the text width |
| text | the anchor point (top-left) | `style.fontSize` × k | the text is not re-measured; multi-line text: see B5 |
| icon | box left | box scaled by k, aspect kept | never re-sized to the line (P11) |
| jump pair | each icon on its own | as icon | `JumpTo` untouched (P28) |

Everything else (P2–P8, P13–P34) is **shape-agnostic**. That is what makes the model testable: one transform, applied the same way to every point list.

### B4. Event table: what happens, by event

| event | reference line still exists? | result |
|---|---|---|
| create | — | link by P2–P5 |
| move / resize | — | re-link by P2–P5 (P23, P24); fixed if far |
| restyle / setText | — | link unchanged (P25) |
| edit an unrelated line, on the same page above | yes | translate by the line's dy (P8) |
| edit an unrelated line below | yes, unmoved | no change at all (P13) |
| insert or delete lines, add a section | yes | translate (P8); new page if it moved (P17) |
| typo in the reference line | paired by diff | follow + left-edge shift; to check if a character under the mark changed (P19) |
| rewrite or delete the reference line | no pair | stays; lost (P20) |
| duplicate of the reference line inserted | yes (identity by id) | unaffected (P14) |
| `size:` / `fit:` | yes | translate + scale k (P9); characters follow across re-wrap (P15) |
| `columns:` | yes | on-text: follows its character into the other column (P16); margin: vertical follow + to check |
| transposition (bake) | yes | identical coordinates (P18) |
| page count shrinks | yes / no | follow / bake blocked + listed (P29) |
| multi-line, lines inside move apart | yes | follow the top line, no stretch, to check (P12) |
| band-folder export/import | — | carried (P30); unlinked marks linked at import (P31) |
| bake | — | the same projection as Studio (P33) |

### B5. Related defects found on the way (not anchoring, file separately)

- **Multi-line text marks.** Studio's `textBBox` measures one row per `\n` (`editor.ts:263-286`), but ink's `drawText` is a single `fillText` (`web/ink/src/index.ts:730-739`). So a multi-line text mark's selection box and its drawing disagree. P11 scales `fontSize`, and the line count does not matter to anchoring.
- **A missing field means unlink.** A client that omits `anchor` on any mutation unlinks the mark (A5). Whether the tablet app or any non-Studio client can send object mutations is unverified. **Experiment:** grep `app/` for the sync mutation shape.

### B6. What implementing it means (summary; tasks in `critique.md` §7)

1. **Manifest:** each run carries its source line index, its segment index, and the character offset of the segment within the line. `rec` is called inside `layout()`, which iterates the source lines, so the data is available.
2. **Line ids:** each generated file keeps a list of stable line ids, parallel to its source lines. `SaveChartSource` (`service.go:1340`) already holds the old and new source. It diffs them (Myers or patience, line-level) and carries ids across, so a paired line keeps its id and new lines get new ids. It also records each paired line's character diff, which P19 needs. Any source write that bypasses this (folder import, migration) re-derives ids by matching text, then by order, and marks every affected mark **to check**.
3. **The link record:** `SourceAnchor` v2 = {lineId, segment, leftChar?, dyLh, runnerUpId?}, plus the mark's geometry as drawn (its existing `Points`, in the coordinates of the render named by `PointsRenderHash`). The projection is translate plus scale; `remap`'s box branch is deleted.
4. **apply.go:** move and resize re-link on the server (T191 ⟨D1⟩, re-specified for P23–P27). setStyle and setText keep the stored link.
5. **Band-folder import** links unlinked marks (P31). The folder carries line ids alongside the source, or ids are re-derived at import.
6. **Studio** shows the four states (T192, simplified).
