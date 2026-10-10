# Anchoring test coverage: matrix, existing tests, gaps

**Base:** origin/main `a936c93d`. The property ids are those of `anchoring-spec.md` Part B.

**Existing tests** were found by grepping these paths for anchor, reproject, reflow, orphan, occurrence, span, offset and `pointsRenderHash`:
- `core/internal/chartpdf/*_test.go`;
- `core/internal/sync`, `core/internal/bake`, `core/internal/app`, `core/internal/httpapi`, `core/internal/domain`;
- `web/studio/e2e`, `web/studio/test`.

**Studio has no anchoring test at all.** Every `anchor` hit in `web/studio/e2e` and `web/studio/test` is about a text mark's top-left point, zoom anchoring or `HTMLAnchorElement`.

The **matrix cells** are the new red-first tests in `red-first.patch`; their baseline is in `red-first-baseline.md`.

## 1. Shape × event matrix: the governing property ids

P10 (direction and stroke order), P11 (stamp and label size) and P28 (jump pairing) are added for the shapes they constrain. DEC marks a property that is a DECISION for VLL.

| event | freehand | line | rect | ellipse | text | highlight | icon | jump-pair icon | matrix cells (red-first.patch) |
|---|---|---|---|---|---|---|---|---|---|
| create on a run | P2 P7 P10 | P2 P7 P10 | P2 P7 | P2 P7 | P2 P7 P11 | P2 P7 | P2 P7 P11 | P2 P7 P11 P28 | `create_on_run` |
| create beside a run | P3 P7 P10 | P3 P7 P10 | P3 P7 | P3 P7 | P3 P7 P11 | P3 P7 | P3 P7 P11 | P3 P7 P11 P28 | `create_beside_run` |
| create far away / in a margin | P5 · P4 (DEC) P10 | P5 · P4 (DEC) P10 | P5 · P4 (DEC) | P5 · P4 (DEC) | P5 · P4 (DEC) P11 | P5 · P4 (DEC) | P5 · P4 (DEC) P11 | P5 · P4 (DEC) P11 P28 | `create_far, create_margin` |
| move | P23 P26 P27 P8 P10 | P23 P26 P27 P8 P10 | P23 P26 P27 P8 | P23 P26 P27 P8 | P23 P26 P27 P8 P11 | P23 P26 P27 P8 | P23 P26 P27 P8 P11 | P23 P26 P27 P8 P11 P28 | `move_then_edit; sync move/*` |
| resize | P24 P26 P8 P10 | P24 P26 P8 P10 | P24 P26 P8 | P24 P26 P8 | P24 P26 P8 P11 | P24 P26 P8 | P24 P26 P8 P11 | P24 P26 P8 P11 P28 | `resize_then_edit; sync resize/*` |
| restyle (and setText) | P25 P26 P10 | P25 P26 P10 | P25 P26 | P25 P26 | P25 P26 P11 | P25 P26 | P25 P26 P11 | P25 P26 P11 P28 | `restyle_then_edit; sync setStyle/*, setText/*` |
| edit an unrelated line | P13 · P8 (lines above) P10 | P13 · P8 (lines above) P10 | P13 · P8 (lines above) | P13 · P8 (lines above) | P13 · P8 (lines above) P11 | P13 · P8 (lines above) | P13 · P8 (lines above) P11 | P13 · P8 (lines above) P11 P28 | `edit_unrelated_line; insert/delete_line_above; section_added_above` |
| edit the linked line (typo) | P19 P21 P10 | P19 P21 P10 | P19 P21 | P19 P21 | P19 P21 P11 | P19 P21 | P19 P21 P11 | P19 P21 P11 P28 | `typo_linked_line, typo_then_edit` |
| delete the linked line | P20 P21 P10 | P20 P21 P10 | P20 P21 | P20 P21 | P20 P21 P11 | P20 P21 | P20 P21 P11 | P20 P21 P11 P28 | `delete_linked_line` |
| insert a duplicate line before it | P14 P10 | P14 P10 | P14 | P14 | P14 P11 | P14 | P14 P11 | P14 P11 P28 | `duplicate_inserted_before` |
| multi-line mark + reflow | P6 P12 (DEC) P22 P10 | P6 P12 (DEC) P22 P10 | P6 P12 (DEC) P22 | P6 P12 (DEC) P22 | N/A | P6 P12 (DEC) P22 | N/A | N/A | `multiline_reflow, multiline_insert_inside` |
| size / fit / columns change | P9 P15 P16 P10 | P9 P15 P16 P10 | P9 P15 P16 | P9 P15 P16 | P9 P15 P16 P11 | P9 P15 P16 | P9 P15 P16 P11 | P9 P15 P16 P11 P28 | `size_change, fit_change, columns_change, size_change_wrapped_line` |
| transpose in a bake | P18 P10 | P18 P10 | P18 | P18 | P18 P11 | P18 | P18 P11 | P18 P11 P28 | `transpose_bake` |
| page-count shrink | P17 P29 P10 | P17 P29 P10 | P17 P29 | P17 P29 | P17 P29 P11 | P17 P29 | P17 P29 P11 | P17 P29 P11 P28 | `page_count_shrink` |
| export/import round trip | P30 P31 | P30 P31 | P30 P31 | P30 P31 | P30 P31 | P30 P31 | P30 P31 | P30 P31 P28 | `bandio */export_import_round_trip, */import_links_unlinked_mark` |
| bake | P33 P29 P34 P10 | P33 P29 P34 P10 | P33 P29 P34 | P33 P29 P34 | P33 P29 P34 P11 | P33 P29 P34 | P33 P29 P34 P11 | P33 P29 P34 P11 P28 | `existing bake tests (see below)` |

## 2. Each property: existing coverage, status, gap sketch

Status means coverage of the **desired** property:
- **covered:** an existing test, or a GREEN matrix cell, asserts it;
- **partial:** something asserts part of it, or asserts it only for a lucky fixture;
- **not covered:** nothing asserts it, or the only test pins the opposite.

| P | property (short) | existing tests today | matrix cells (patch) | status | gap: test sketch (layer: vector that fails under today's bug) |
|---|---|---|---|---|---|
| P1 | an uploaded PDF never links or moves | `httpapi/anchorer_t145_test.go:TestChartAnchorer_BestEffortDegradesToNoAnchor`; `bake/annotations_reproject_t145_test.go:TestSnapshotToDoc_ReprojectsStaleMark` (uploaded file untouched) | — | covered | — |
| P2 | a mark on text links to that line (topmost covered) | `chartpdf/anchor_forward_t145_test.go:TestAnchorObject_And_Reproject`; `anchor_offset_t172_test.go:TestOnRunAnchorIsUnchanged`; `httpapi/anchorer_t145_test.go:TestChartAnchorer_AnchorsMarkOnGeneratedChart`; `sync/anchor_hook_t145_test.go:TestCreateRoutesThroughAnchorer`, `TestNilAnchorerLeavesCreateUnanchored` | `*/create_on_run` (8 GREEN) | partial | **Go unit:** a rect around 3 lines **centred on the middle line** must link to the TOP line. Today it links to the middle one. |
| P3 | a mark beside text links to the nearest line within 3 lh, no horizontal-overlap requirement | `anchor_offset_t172_test.go:TestUnderlineAnchorsToTheLineAbove`, `TestUnderlineStaysUnderItsLineAtANewTypeSize` | `*/create_beside_run` (8 GREEN), `*/beside_then_edit` (5 GREEN, 3 RED) | partial | **Go unit:** a "capo 2" text mark right of a short title (no overlap with the title's text width) must link to the title. Today it links to a verse line or to nothing. |
| P4 (DEC) | a margin mark follows its line vertically, x fixed | **pinned opposite:** `anchor_offset_t172_test.go:TestNoHorizontalOverlapStaysUnanchorable` | `*/create_margin`, `*/margin_then_edit` (16 RED) | not covered | in the patch |
| P5 | a far mark is fixed and never moves | `anchor_offset_t172_test.go:TestRefusalIsKept` | `*/create_far`, `*/far_then_edit` (16 GREEN) | covered | — |
| P6 | a multi-line mark links to its topmost line | — | `*/multiline_reflow` (4 GREEN by fixture, 1 RED) | partial | **Go unit:** the same box with its centre ON a line (not in a gap), plus an insert inside: it must follow the top line. Today it collapses. |
| P7 | the record keeps the mark's geometry, left-edge character, offset and runner-up | `domain/anchor_clone_t172_test.go:TestCloneDeepCopiesNestedAnchorOptionals` (structure only) | implied by every follow cell | not covered | **Go unit**, on the new link type: create → round-trip through the wire / folder / clone, then project with no change: exact. |
| P8 | rigid follow when lines above change | `anchor_project_test.go:TestSourceAnchor_SurvivesReflow`; `anchor_forward_t145_test.go:TestAnchorObject_And_Reproject` (a mark equal to its run box) | `*/insert_line_above`, `*/delete_line_above`, `*/section_added_above`, `*/restyle_then_edit` (text and highlight GREEN, the rest RED) | partial | in the patch (rect with ±0.4 lh margins; arrow drawn right-to-left; 1.6 lh stamp) |
| P9 | type size: scale by k about the line origin | `anchor_offset_t172_test.go:TestUnderlineStaysUnderItsLineAtANewTypeSize`; `anchor_forward_t145_test.go:TestAnchorObject_And_Reproject` (size 8 → 16) | `*/size_change`, `*/fit_change` | partial | in the patch |
| P10 | a line keeps its direction; a freehand path keeps its order and shape | — | every line and freehand follow cell (all RED) | not covered | in the patch. Today's teeth: `line/insert_line_above` start lands at char 5 instead of char 30. |
| P11 | stamps and labels keep their size and aspect | — | icon, jump and text follow cells; `icon/beside_then_edit` (width 0, invisible) | not covered | in the patch |
| P12 (DEC) | a multi-line mark never stretches, and is flagged when its inner lines move apart | **pinned opposite:** `anchor_offset_t172_test.go:TestSpanCoversBothEnds` (elastic span) | `*/multiline_insert_inside` | partial (geometry only) | **Go unit**, after the states exist: the same cell asserts state = to check. |
| P13 | an unrelated edit leaves the mark exact | — | `*/edit_unrelated_line` (7 RED, highlight GREEN) | not covered | in the patch (exact tolerance 0.001) |
| P14 | a duplicate inserted before does not change the followed line | `anchor_columns_t146_test.go:TestAnchors_OccurrenceIsLayoutIndependent_T146`, `TestAnchors_SingleColumnOccurrenceUnchanged_T146` (occurrence across a RE-LAYOUT only, not an insertion) | `*/duplicate_inserted_before` (8 RED) | not covered | in the patch |
| P15 | after a re-wrap, the mark follows its character and stays linked | — | `*/size_change_wrapped_line` (8 RED) | not covered | in the patch |
| P16 | columns: an on-text mark follows its character into the other column; a margin mark is flagged | `anchor_columns_t146_test.go:TestAnchors_MarkKeepsItsLineAcrossTheColumnRelayout_T146` (short lines that do not wrap) | `*/columns_change` (8 RED: a 47-character lyric wraps in two columns) | partial | in the patch |
| P17 | a mark follows its line to another page | `anchor_project_test.go:TestSourceAnchor_SurvivesReflow`; `anchor_forward_t145_test.go:TestAnchorObject_And_Reproject` | `*/page_count_shrink` (text and highlight GREEN) | partial | in the patch |
| P18 | a transposed bake keeps identical coordinates | `transpose_test.go:TestTransposeGeometryInvariant` (page and line count), `TestTransposeAlignmentAnchors` (chord columns) | `*/transpose_bake` (8 GREEN) | partial | **Go unit:** a chord row exactly at the T168 wrap limit, transposed C→Db; the run count and lyric geometry must be equal. **bake:** a transposed bake after a renderer change (the stored blob from an older render): today the transposed raster comes from the new binary while the marks sit on the old geometry. |
| P19 | typo: stays linked, follows, shifts by its left character, flagged only if the characters under it changed | **pins today:** `anchor_columns_t146_test.go:TestAnchors_ProjectRefusesWhenTheRunIsGone_T146` (any change of text → refuse; still right for a DELETED line) | `*/typo_linked_line`, `*/typo_then_edit` (16 RED) | not covered | **Go unit**, also needed: insert a word **before** a circled word on the same line; the circle shifts right by that word (±1 cw). Today it is lost. |
| P20 | a deleted line leaves the mark lost and exact, and it is counted | `anchor_columns_t146_test.go:TestAnchors_ProjectRefusesWhenTheRunIsGone_T146`; `anchor_forward_t145_test.go` (lost pass-through) | `*/delete_linked_line` (7 GREEN; jump RED via the destination collapse) | partial | **Studio e2e** (T192): delete the linked line; the counter reads "1 lost link". |
| P21 | moving a lost or to-check mark re-links it and clears the state | — | — | not covered | **sync + Go:** delete the line (lost), move the mark onto another line, edit above; it follows the new line. Today it stays lost. |
| P22 | runner-up: flagged when the two candidate lines separate | — | — | not covered | **Go unit:** an underline in the overlap between a lyric and the next chord row; insert a pair between them; state = to check. |
| P23 | move re-links | — | `*/move_then_edit` (8 RED); sync `move/relinks_server_side`, `move/old_link_not_kept` (RED) | not covered | in the patch. Also **Studio e2e** (T191 acceptance): drag a highlight from line 3 to line 6, insert above, reload; it is on line 6. |
| P24 | resize re-links | — | `*/resize_then_edit` (7 RED); sync `resize/relinks_server_side` (RED) | not covered | in the patch |
| P25 | restyle and setText keep the stored link | — | `*/restyle_then_edit`; sync `setStyle/*`, `setText/*` (`echoed_link_kept` GREEN, 3 RED) | partial | in the patch |
| P26 | the server decides; an omitted field does not unlink | — | sync `move/relinks_server_side`, `setStyle/keeps_stored_link`, `setStyle/missing_link_not_dropped` (RED) | not covered | in the patch |
| P27 | a move against a stale render keeps the old link | — | — | not covered | **httpapi seam** (it needs the real render hash): move with `pointsRenderHash` = an old render's hash; the stored link is unchanged. |
| P28 | jump pairs: each icon independent, `JumpTo` intact, the bake resolves the current page | `bake/jumps_test.go:TestResolveJumps_PairResolvesToPageAndAnchor` and 6 siblings; `bake/baker_p206_test.go:TestBake_JumpPairBecomesAPageJump`; `app/bandio_p206_test.go:TestBandExportImport_CarriesJumpPairing_P206` | every `jump_pair/*` cell | partial | **bake:** a jump pair with the destination's line moved to page 2 by an insert; the hotspot target is page 2. |
| P29 | page shrink: linked follow; orphans block the bake and are listed in Studio | `bake/overlay_reflow_t145_test.go:TestBake_ReflowOrphanedOverlay_FailsBake` | `*/page_count_shrink` | partial | **Studio e2e:** a fixed mark on page 2, the chart shrinks to 1 page; Studio lists it before the bake. |
| P30 | export/import round trip | `app/bandio_object_fields_test.go:TestBandFolder_RoundTripsEveryObjectField`; `sync/wire_object_fields_test.go:TestSyncWire_CarriesEveryObjectField`; `httpapi/wire_object_fields_test.go:TestAnnotationsDTO_CarriesEveryObjectField`; `httpapi/anchor_wire_t145_test.go:TestObjectJSON_AnchorRoundTrip` | `bandio */export_import_round_trip` (8 GREEN) | covered | — (the new link type must keep these field-completeness guards green) |
| P31 | import links unlinked marks | `httpapi` import path (`annotations.go:282-300`; no dedicated test found) | `bandio */import_links_unlinked_mark` (8 RED) | not covered | in the patch, plus **httpapi:** the same vector through the HTTP import |
| P32 | link only against a render that reproduces the blob | `app/chart_anchors_current_t145_test.go:TestChartAnchorsIfCurrent`; `httpapi/anchorer_t145_test.go:TestChartAnchorer_BestEffortDegradesToNoAnchor` | — | covered | — |
| P33 | serve equals bake | `bake/annotations_reproject_t145_test.go:TestSnapshotToDoc_ReprojectsStaleMark` (bake side only) | — | partial | **httpapi + bake:** the same snapshot through `reprojectSnapshot` and `snapshotToDoc`; identical points. |
| P34 | Studio shows the states; never in a bake | — | — | not covered | **Studio e2e** (T192 acceptance), plus a byte-equal bake with the indicator toggle on and off. |

## 3. Existing tests that pin today's behaviour and will need to change

- `chartpdf/anchor_offset_t172_test.go:TestOnRunAnchorIsUnchanged` asserts that an on-run mark projects to **the run's own box**. That is the collapse (P7, P8). Under the new link it becomes "an on-run mark projects to its own drawn box".
- `chartpdf/anchor_offset_t172_test.go:TestSpanCoversBothEnds` asserts elastic stretching to the span's last run. It contradicts P12 if VLL accepts "no stretch".
- `chartpdf/anchor_offset_t172_test.go:TestNoHorizontalOverlapStaysUnanchorable` asserts the margin refusal. It contradicts P4 if accepted.
- `chartpdf/anchor_columns_t146_test.go` (4 tests) asserts identity by text + occurrence. They become line-id tests. `TestAnchors_ProjectRefusesWhenTheRunIsGone_T146` stays valid **for a deleted line** only.
- `chartpdf/anchor_migrate_test.go:TestMigrateObjects_ReverseAnchorsFromCorrectRender` covers dormant code (`MigrateObjects`, unused since T145). Delete both with the old link.
- `sync/anchor_hook_t145_test.go:TestCreateRoutesThroughAnchorer` stays (the create seam), but its stub stamps `Occurrence: 0`, which is not a valid occurrence. That is harmless now, and wrong under any validating link type.

## 4. How much test surface each option implies (for the critique)

- **Today's elastic model, fixed in place:** every one of the 202 chartpdf cells is a distinct behaviour, because each shape has its own projection path. Add 7 sync, 16 bandio and about 6 Studio e2e cells, for about 230.
- **Rigid follow (recommended):** the same 202 cells run as a cheap regression net, but they reduce to about 26 event classes plus 8 shape-invariance checks. Add 7 sync, 16 bandio, the gap cells above (P19 word-shift, P21, P22, P27, P33) and about 6 Studio e2e cells, for about 60 distinct behaviours.
- **Never link + flag:** the follow cells invert to "exact" (about 26 shape-agnostic cells collapse to about 5). Add P1, P29, P30 and P32, plus about 6 flag cells (flagged after an insert above, not flagged after an edit below, the pre-bake warning), for about 15. The red-first patch would be rewritten, not landed as is.
