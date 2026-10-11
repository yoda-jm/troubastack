# T195 — Anchoring stage 2: rigid follow, the linking rule, the server decides

**Lane:** web-core (core) · **Status:** implemented on `fable/anchoring-stage2`, not pushed, for the gate.
**Size:** M + 3S. · **Origin:** `docs/design/anchoring/critique.md` §7 "Stage 2". VLL chose option G with
every recommendation (README): margin marks follow vertically, multi-line marks never stretch (flagged
instead), marks scale with the type size, and the four states are Linked / Check / Lost / Fixed.
**Builds on:** T193 (stage 0, freeze) and T194 (stage 1, line identity).

## 0. Scope

Stage 2 replaces the T145/T172 elastic mechanism (text + occurrence, fit to the run's box). New-style marks
are linked by the server to one source line by its stable id, and projected rigidly at serve and bake.
Old-style anchors stay frozen until stage 4. Stage 3 (the Studio UI, the pre-bake list) and stage 4 (the
migration) are not in this task; the state they need is on every wire.

## 1. The data model

`domain.Object.Link *domain.MarkLink` (`core/internal/domain/domain.go`):

| field | meaning |
|---|---|
| `state` | `linked` / `check` / `lost` / `fixed` |
| `line` | the reference line's stable id (T194); empty only when fixed |
| `wrap` | the visual line (SrcWrap) of the reference line the mark was measured on |
| `char` | the rune offset, in the line's source text, of the character under the mark's left edge (P7); only when the left edge is over the text |
| `span` | `{from, to}`: the characters the mark covers horizontally; P19 watches them |
| `xRef` | what `dx` is measured from: `char`, `start` (the mark starts left of the text), `end` (it starts right of this line's text, inside the column), or `margin` (x is page-fixed, P4) |
| `dx`, `dy` | the offset of the mark's bounding-box top-left from its reference point, in line-heights |
| `lh` | the reference line's height on `render`: the type size the link was measured at |
| `render` | the PDF blob hash the link and the mark's `Points` were measured on |
| `cols` | the chart's column count on `render` (P16) |
| `peers` | lines the follow relies on: `inside` (the bottom line a multi-line mark covers, P12) and `runner-up` (P22), each with its offset from the reference line |

- `Points` are the mark **as drawn on `link.render`**. Serve and bake project them onto the current render.
  `PointsRenderHash` names the render the Points are on: `link.render` when stored, the current render once
  served.
- `MarkLink` carries JSON tags, and it is the same shape in the store, on the realtime wire
  (`objectJSON.link`), in the REST DTO (`objectJSON.link`), in the v2 band folder (`v2Object.link`) and in
  the v1 manifest (`domain.Object` as is). One definition, so the mirrors cannot drift. Every
  field-completeness guard (sync, httpapi, bandio, bake) covers it. The bake doc deliberately does not carry
  it (P34).
- All of it is additive and `omitempty`: an unlinked mark is byte-identical on every wire.

## 2. The linking rule (`chartpdf.LinkMark`, P2–P7)

The server decides at create, move, resize, restore and import. The candidates are the visual lines on the
mark's page, **in the mark's column** (the box containing the mark's centre x, else the nearest). The
mark's box is its points' bounds, except for a text label, whose box is its anchor point: the server cannot
measure the glyphs. The reference line is chosen in this order:

1. the topmost line the box covers **entirely** (a box drawn around a line, a multi-line mark);
2. else the topmost line whose vertical centre the box covers;
3. else the line whose box contains the mark's vertical centre (a thin mark, an underline, a label's anchor
   point). Where two boxes overlap, the one whose centre is nearer wins; on a tie (0.25 lh), the line above
   wins, except for a text label (its glyphs hang below its anchor point), where the line below wins. The
   other line is the runner-up;
4. else the nearest line by vertical edge gap within **3 lh**. On a tie within 0.25 lh the line above wins;
   any line within 0.5 lh of the winner's gap is the runner-up. **No horizontal overlap is required**;
5. else **fixed** (P5).

The horizontal reference:
- **margin** when the mark is right of every line's text in its column, or left of all of it: x is
  page-fixed (P4);
- **char** when the left edge is over the reference line's text;
- otherwise **start** or **end** of that visual line.

Creating or re-linking a mark clears its old-style `Anchor`: the server's new decision supersedes it.

## 3. The rigid projection (`chartpdf.ProjectMark`, P8–P20, P22)

It applies only to a mark with a `link` that is not fixed and whose `link.render` is not the current render.
It needs `chartpdf.ChartRender{Source, Lines, Anchors, Hash}`, which `app.Service.ChartRenderForFile` builds
only when a fresh render reproduces the stored PDF (P32). Otherwise `ErrRenderDiverged`, and every mark is
served and baked as stored. Its steps:

- **Locate.** The line is found by id (`LocateLine`). If the line is deleted, or now draws nothing, the mark
  is **lost**, at its stored coordinates and with its old `PointsRenderHash` (P20).
- **Reference point.**
  - For `char`, the left-edge character is mapped through the line's character map (T194). The mark follows
    that character into whichever visual line it now sits on (P15, P16, P19).
  - For `start` and `end`, the reference is that visual line's start or end, with the wrap index clamped.
  - A margin mark keeps its x.
- **One transform for every shape:** `p' = TL' + (p − TL)·k`, where `k = lh'/lh`. `TL` is the mark's box
  top-left; `TL'` is the new reference point plus `(dx, dy)·lh'`.
  - Point order is kept, so a line keeps its direction and a stroke keeps its order (P10).
  - A stamp keeps its aspect and changes size only by k (P11).
  - A text label's `fontSize` is scaled by k (P9, B3).
  - The page is the line's new page (P17).
- **State.** The mark stays `linked`, unless one of these makes it **check**:
  - a character in `span` changed, or text was inserted inside it (P19). An edit elsewhere on the line only
    shifts the mark;
  - the history cannot give the character map (`CharMapKnown == false`: too old, or an import);
  - a peer moved by more than 0.25 lh relative to the reference line, or is gone (P12, P22);
  - a margin mark's chart changed column count (P16);
  - the link predates the file's last line-id re-derivation (T194's `Rederived`, below);
  - the stored state was already check. Check is sticky: only a move or resize clears it (P21).

## 4. Mark edits: the server decides (P23–P27)

`sync/apply.go`, reusing T191's seam re-pointed at the new link (`sync.ChartAnchorer`, implemented by
`httpapi.chartAnchorer`):
- **create:** the client's `link`, `anchor` and `pointsRenderHash` are dropped, then `AnchorMark` links
  (P26).
- **move, resize, restore:** `RelinkMoved`.
  - The client saw the current render (its `pointsRenderHash` is current, or empty): re-link from the new
    position, as at create (P23, P24). A lost or check mark leaves that state (P21). A move into empty space
    makes it fixed.
  - It saw an older render, or none is current (a stale tab, or P32): keep the **stored** link and shift its
    offset by the move, so the next serve puts the mark where it was dropped relative to its line (P27). If
    the client's render is not even the link's own, the shift is a guess, and the mark becomes check.
- **setStyle, setText, reorder:** `RebaseMark`. Only the client's non-geometric fields are taken, and the
  stored link is kept (P25, P26). If the mark was being served projected, the projected position (what the
  client saw) is stored, with the link re-measured against the **same** line on the current render and its
  state persisted. Without this, a projected label's scaled font would be scaled twice.
- **No anchorer** (tests, a deployment without charts): a create or move stores no link, and a restyle
  copies the stored one. The hub never applies a client link.

## 5. Serve, realtime, bake, import

- **Serve:** `GET /annotations` projects through `projectSnapshot`. The realtime **join snapshot**, which
  Studio edits from, now goes through the same function (`ChartAnchorer.ServeSnapshot`). Before this task the
  WS snapshot was never projected, even under T145.
- **Realtime echo:** every write now stores current geometry and state, so the echo carries `link.state`
  as stored. The echo needs no projection.
- **Bake:** `snapshotToDoc(snap, fileID, *ChartRender)` → `ProjectMarks`, the same function as serve (P33).
  A transposed bake projects against the untransposed current render (P18, unchanged).
- **Import (P31):** both the band-folder import (`app/bandio.go`) and the HTTP import link a mark that
  arrives with **no link and no anchor**, against the chart as imported. A mark that arrives with either
  style keeps it exactly (P30).

## 6. The freeze, and old-style anchors

**`anchoringFrozen` is removed**, with `ServedObjects`, `AnchorNewMark`, and the whole old mechanism:
`AnchorAt`, `Project`, `Reproject`, `AnchorObject`, `remap` (`anchor_project.go`), `MigrateObject(s)`
(`anchor_migrate.go`, dormant since T145), `cmd/migrate-anchors` (the T145 runner), and
`cmd/t193-freeze-served`. The last one was a one-off, run at the T193 deploy; its re-run found 0.

New-style links project. **Old-style anchors stay frozen.** A mark with an `Anchor` and no `Link`:
- is served and baked at its stored coordinates, and the `Anchor` is carried on every wire and export;
- is never interpreted for projection, and nothing reads `RunText`/`Occurrence` any more;
- keeps its `Anchor` through a restyle;
- when the user moves or resizes it, gets a new link and loses the old anchor: that is the server's new
  decision.

Stage 4 re-links the rest with VLL's approval.

## 7. Other changes

- **A line-id race, fixed.** Stage 2 reads source and ids on every serve. In `saveChartSource`, a reader
  landing between `SetChartSource` and the id carry would see ids that do not describe the source. It would
  re-derive and **store** fresh ones, and the carry would start from those, so every link on the chart would
  read lost. Now:
  - `saveChartSource` holds the file's line lock across the file, source and id writes (and, since the
    review, the eager re-placing of §11 F3);
  - `ChartLines`, and the new `ChartRenderForFile`, read the source under the same lock;
  - `LocateChartLine` goes through `ChartRenderForFile`.
- **Render cache.** `ChartRenderForFile` memoises `ChartAnchorsIfCurrent` by (source hash, blob hash), as
  an LRU of 256 entries (review F5). Every realtime move and restyle needs the manifest. Measured on a
  100-line chart: about 10 ms uncached, about 80 µs cached. Both keys are content hashes, so an entry is
  never stale.
- **T194's `Rederived` flag** is consulted per mark (`predatesRederive`). It is **cleared after a save
  re-places every linked mark of the file** (review F3): each link is then measured on a render after the
  re-derivation, so the flag has nothing left to say. If any mark could not be re-placed, the flag is kept.

## 8. Decisions on spec points that were wrong or ambiguous (flag at the gate)

1. **P2's "topmost covered line" is wrong for chord/lyric pairs.** A rectangle drawn ±0.4 lh around a lyric
   covers the centre of its own chord row above. P2 read literally would link the circle to the chord row,
   so a word inserted before the circled word (P19) would not move it. Chosen: the topmost line covered
   **entirely**, else the topmost centre covered. Multi-line marks still follow their top line.
2. **P22's example geometry does not exist at the default size.** A lyric's box and the next chord row's box
   do not overlap: there is a gap of about 0.06 lh. A chord row does overlap its **own** lyric. The rule
   covers both cases: the overlap zone (step 3) and a thin gap (step 4's 0.5 lh runner-up).
3. **Ties for a text label go to the line below.** Its point is its top-left, and its glyphs hang below.
4. **A margin mark under a type-size change** (P4 against P9): its left edge keeps its x, and it scales by k
   about that edge. The matrix tests margin marks only with k = 1.
5. **P19 makes a typo fix under a circled word "check".** In the matrix, the freehand, line, rect and
   highlight typo cells are check, while ellipse, text, icon and jump stay linked. The expectation is
   computed from the stage-1 character map, not from the implementation. VLL may find this noisy; the
   alternative is to flag only when the mark's left-edge character changed.
6. **A text label's P19 extent is its first character**, because the server does not measure glyphs.
7. **A move whose `pointsRenderHash` is empty is treated as current.** Studio always sends back the hash it
   was served.
8. **The stroke width is not scaled by k.** It is a pen property, not geometry. B3 does not say.
9. **Projection is eager at save, and lazy everywhere else** (reversed by the review, F3). The first version
   was lazy only, and it decayed: a mark untouched while its chart was saved more than `maxLineHistory` (32)
   times lost its character map. It landed 4 characters off, and "check". `SaveChartSource` now re-places
   every linked mark of the chart on the new render (§11). Serve, bake and the WS snapshot still project
   whatever a save did not re-place. The state is persisted at every write: create, move, resize, restyle,
   and each save.

## 9. Tests

**The matrix** (`ANCHOR_RED=1 go test ./internal/chartpdf/ ./internal/sync/ ./internal/app/ -run
AnchorMatrix`):

| package | main `91427500` (T193 freeze, T194) | this branch |
|---|---|---|
| chartpdf `TestAnchorMatrix` | 34 pass / 208 red | **288 pass / 0 red** (242 + 46 review F4 cells) |
| sync `TestAnchorMatrixSync` | 1 / 6 | **15 / 0** (7 cells rewritten, 8 added) |
| app `TestAnchorMatrixBandio` | 8 / 8 | **24 / 0** (8 `old_style_anchor_round_trip` added) |

Every `knownRed` table is empty, the two DECISION families included:
- `create_margin` and `margin_then_edit` (P4);
- `multiline_insert_inside`, which now asserts state **check** (P12).

**Seams changed in the chartpdf harness:**
- create calls `LinkMark`;
- move and resize call `RelinkMoved`, and restyle calls `RebaseMark`, each against the stored object, with a
  forged client link;
- edit is a real save: `CarryLineIdentity` plus `ProjectMarks`;
- "linked to" compares line ids;
- the state comes from `link.state`.

Two expectations were corrected:
- **`columns_change` and `size_change_wrapped_line`:** they expected a mark to stay on the line's first
  visual segment, even when its character wraps onto the second. A jump icon at character 28 was expected at
  x = 0.47, past the narrowed column. They now expect what P15 says: the mark follows its left-edge
  character to its new segment. The position is found through the runs' `SrcLine`/`SrcChar`, not through
  the link.
- **Typo cells:** they expect check exactly where P19 says.

The sync cells now drive a **real** anchorer: the same chartpdf calls the httpapi adapter makes.

**Old tests rewritten or deleted, with the reason:**

| old test | now | why |
|---|---|---|
| `anchor_offset_t172_test.go` `TestOnRunAnchorIsUnchanged` | `link_t195_test.go` `TestLink_OnRunMarkKeepsItsOwnBox` | it pinned "projects to the run's own box", which is the collapse; it now asserts the mark's own box, translated exactly |
| `TestSpanCoversBothEnds` | `TestLink_MultiLineNeverStretches` | it pinned elastic stretching, which P12 (VLL) inverts: no stretch, check on insert or delete inside, linked on insert above |
| `TestNoHorizontalOverlapStaysUnanchorable` | `TestLink_MarginMarkFollowsVertically` | it pinned the margin refusal, which P4 (VLL) inverts. Both margins are covered, and x is exact |
| `TestUnderlineAnchorsToTheLineAbove`, `TestUnderlineStaysUnderItsLineAtANewTypeSize`, `TestRefusalIsKept` | `TestLink_UnderlineLinksToTheLineAbove`, `…StaysUnderItsLineAtANewTypeSize` (now also checks the length scales by k), `TestLink_FarIsFixed` | the same properties, on the new rule |
| `anchor_columns_t146_test.go` (4 tests: occurrence) | `TestLink_MarkKeepsItsLineAcrossTheColumnRelayout`, `TestLink_RepeatedLinesLinkToTheirOwnID`, `TestLink_DeletedLineIsLostAndStays` | occurrence no longer exists. Its invariant restated for ids: each copy of a repeated line keeps its own id, in any layout. The "refuses when the run is gone" test becomes P20: lost, exact |
| `anchor_forward_t145_test.go`, `anchor_project_test.go` | `TestLink_FollowsItsLineOntoAnotherPage` | they pinned the re-fit to the run's box. Now: follows onto the new page, scaled by k, own shape; unlinked and already-current marks are untouched |
| `anchor_migrate_test.go` | deleted | it tested dormant code (`MigrateObjects`), deleted with it, as critique §7 says |
| `httpapi/anchor_freeze_t193_test.go` (`TestAnchorFreezeServe`, `TestAnchorFreezeCreateNoAnchor`) | `httpapi/anchor_stage2_t195_test.go` `TestAnchorStage2Serve`, `TestAnchorStage2CreateAndImport` | the freeze is replaced. **Old-style marks are still pinned frozen, shape by shape**; new-style marks are served at the projection, moved, with `link.state` in the payload; the WS snapshot equals GET (P33); create and import link to the target line; pre-linked imports of either style are kept |
| `httpapi/anchorer_t145_test.go` | folded into `TestAnchorStage2CreateAndImport` | it pinned the old anchor (`RunText`) |
| `bake/anchor_freeze_t193_test.go`, `bake/annotations_reproject_t145_test.go` | `bake/anchor_stage2_t195_test.go` `TestAnchorStage2Bake`, `TestSnapshotToDoc_ProjectsLinkedMarks` | a real bake: linked marks bake at exactly the serve projection (P33) and moved; old-style marks bake at their stored coordinates |
| `cmd/t193-freeze-served/main_test.go` | deleted with the command | it was a one-off, already run |
| `sync/anchor_hook_t145_test.go` | kept, updated | the create seam now also asserts that the hub drops a client link before the anchorer sees it |

**New property tests** (`chartpdf/link_t195_test.go`):
- P19: a word inserted **before** a circled word shifts it by exactly that word and it stays linked; a typo
  under it makes it check;
- P22: runner-up separation is check, and an insert above both stays linked;
- P9: a text label's font scales;
- P21: a lost mark that is moved re-links and follows;
- P25: a restyle of a projected label neither moves nor re-scales it, even after a second serve;
- the `Rederived` rule.

**At the httpapi seam:** P27, `TestRelinkMark_StaleRenderKeepsTheLink`, with the real render hashes. A
stale-tab move keeps the line and lands where it was dropped; the same move from a current tab re-links.

**Teeth.** Each property was sabotaged in place, the tests went red, and the code was restored:

| property | sabotage | first red line |
|---|---|---|
| rigid follow | `ny0 := oy0` | `rect-on: box … y[0.1614,0.1972], want … y[0.1825,0.2183]` |
| move re-links | a move keeps the stored link | `freehand-on: point 0 at (0.3722,0.2004), want (0.3722,0.2386)` (8 `move_then_edit` cells) |
| no flip | a 2-point mark is re-drawn min → max | `line-on: point 0 at (0.1036,0.2024), want (0.4312,0.2024)` |
| typo keeps the link | an edited line is treated as gone | `freehand-on: status lost after the edit, want check` (8 `typo_linked_line` cells) |
| duplicate doesn't steal it | resolve to the first line with the same text | `rect-on: box … y[0.1232,0.1590], want … y[0.1996,0.2354]` (8 `duplicate_inserted_before` cells) |
| the server decides (P26) | the hub applies a restyle as sent | `the edit changed the link to &{… Line:client-line …}; it must keep the stored line` |
| import links (P31) | the band import skips linking | `m1: imported onto a generated chart with link <nil>, want linked to the line it sits on` |

**Gates:** `go vet ./...` and `gofmt -l .` are clean, and `go test -race -count=1 -timeout 45m ./...` is
green (21 packages ok, exit 0).

## 11. Review fixes (GO with fixes, on `065ae932`)

**F1 (blocking): a lost, fixed or old-style mark moved through the real client never re-linked.**
- **Cause.** Studio echoes the served object (`translateObject` spreads `{...obj}`), so it sends back the
  `pointsRenderHash` it was served. Serve stamped the current hash only on projected marks. For every other
  mark, `RelinkMoved` read the move as a stale tab (P27).
- **Fix 1.** `projectSnapshot` stamps the current render's hash on **every** object served against a
  current render. Their stored coordinates are exactly what is drawn on that render.
- **Fix 2.** `RelinkMoved` treats a move as current when the client's hash equals the stored one and the
  stored mark is not projectable (no link, fixed, or lost). This covers the realtime echo of a restyle,
  which carries the stored hash.
- **Tests.** The bent tests now send what the real client sends:
  - `TestLink_LostMarkMovedRelinks` sends the served hash;
  - sync `oldstyle/move_relinks` uses an old stored hash;
  - new sync cell: `fixed/move_with_echoed_hash_relinks`.

  The reviewer's websocket → hub → real anchorer → REST vectors are now permanent, in
  `httpapi/anchor_e2e_ws_t195_test.go`. They cover rigid follow, move re-links, lost/fixed/old-style moved
  → linked, no flip, typo, duplicate, margin, and uploaded PDF. Two cells are added: a move from a restyle
  echo, and the 40-save vector.

**F2: a forged imported link broke GET for the whole song.** A line-height of 5e-324 made k infinite, and
the JSON encode failed.
- `chartpdf.ValidLink` requires:
  - a known state and `xRef`;
  - a line unless fixed;
  - finite numbers, with `lh` in [0.002, 0.2] and `|dx|, |dy|` ≤ 50;
  - sane character offsets and peers.
- Both import paths (HTTP and band folder) drop an invalid link and re-link the mark (P31).
- `project()` returns the stored mark as **lost** if any result is non-finite.

**F3: lazy projection decayed after 32 saves. Now eager rebase** (`app/chartmarks_t195.go`).
- **The rebase.** `SaveChartSource`, under the file's line lock and after the id carry, re-places every
  live linked mark on that chart's layers (`chartpdf.RebaseOnRender`). Each changed mark is written as a
  normal `move` mutation, attributed to the saver, with `BaseVersion = the version read` and `Version + 1`.
- **Concurrency.** A concurrent edit wins the engine's LWW check. The rebase then re-reads that mark and
  re-bases the edit, up to 2 retries, so the edit is never overwritten.
- **Lost marks.** A lost mark's state is persisted, at its stored coordinates.
- **The flag.** The `Rederived` flag is cleared when every mark was re-placed (§7).
- **Wiring.** `Service.WithAnnotationEngine(eng)`, set in `cmd/troubacore`. Tools that never call it
  (`mkbaked`, tests) stay lazy.
- **Known gap.** These writes do not go through the realtime hub, so a peer with the song open gets no
  echo. Its next GET or reconnect shows the re-placed marks. Meanwhile, a move from that stale view is P27's
  stale-view case: the link is kept, the offset is shifted, and the state is check.
- **The save** now renders once with `RenderWithAnchors` (the same bytes), reusing the manifest for the
  rebase.

**F4: two sabotages passed every matrix cell.** 46 new cells:
- `typo_lengthens_before` (`Leave the` → `Leave all the`, +4 characters before most marks);
- `typo_shortens_before` (`porch lihgt` → `lihgt`, −6);
- `typo_after` (`goes down` → `goes downtown`);
- for margin marks, `margin_line_lengthened` and `margin_line_shortened` (the margin mark's own line).

The harness computes the expected shift and the expected "check" from the stage-1 character map. "Check"
now also covers text inserted inside a mark's extent.

**F5.**
- The render cache is an LRU (above).
- `lineIDMu` is now **one lock per file** (`lockLines`).
- `derefBlob` runs after the line lock is released.

**Decision 4 follow-up.** A scaled margin stamp that would cross the paper's edge keeps its scaled size and
slides inward (`TestLink_ScaledMarginStampStaysOnThePage`).

**Kept:** decision 5 (a typo under the mark → check) and decision 8 (the stroke width is not scaled).

**Red → green.** Each fix was sabotaged in place and its tests went red, then the code was restored:

| fix | sabotage | red line |
|---|---|---|
| F1 | serve keeps the old hash, no `RelinkMoved` fallback | `P21: a lost mark moved onto a line (through the real client echo) is lost, want linked`; `P23: a fixed mark … is fixed`; `§6: an old-style mark … has link <nil>` |
| F2 | imports trust every link, no finite guard | `a forged imported link broke GET /annotations: 200, 0 bytes` |
| F3 | the save does not re-place | `after 41 saves the untouched icon is check, want linked`; `save 1: the mark was not re-placed on the new render` |
| F3 | a rebase that loses the LWW race gives up | `the re-placing gave up after losing the race` |
| F4 | ignore the character map | `freehand-on: point 0 at (0.3722,0.1793), want (0.4246,0.1793)` (13 cells) |
| F4 | right margin referenced to the line's end | `icon-margin: box x[0.9629,0.9868] …, want x[0.8581,0.8819]` (16 cells) |
| decision 4 | no clamp | `the scaled stamp spans x [0.9500, 1.0150]: off the page` |

## 10. Open questions

- **0.6 similarity** (T194) and the P19 "check on any change under the mark" rule should be replayed on
  VLL's real revisions (critique §9) before stage 3 shows the states.
- **Stage 3 needs the state of old-style marks.** They have no `link`; Studio should show them as Fixed
  until stage 4. Alternatively, stage 4 runs before stage 3 is visible.
- **A chart whose render diverged** (a renderer change, no rerender yet) creates unlinked marks. They stay
  coordinates-only until stage 4, or until the user moves them after a rerender.
- The `inside` peer watches only the **bottom** covered line. An insert between two inner lines also moves
  the bottom one, so it is caught. A pure re-order inside a mark is not.
