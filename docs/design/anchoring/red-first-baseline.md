# Red-first baseline: the anchoring matrix run against origin/main

**Base:** origin/main `a936c93d` (2026-10-11). **Patch:** `red-first.patch` (4 new files, 1143 lines; tests plus one test helper):
- `core/internal/chartpdf/anchor_matrix_test.go`: `TestAnchorMatrix/<shape>/<event>`, 202 cells;
- `core/internal/sync/anchor_matrix_sync_test.go`: `TestAnchorMatrixSync/<kind>/<case>`, 7 cells;
- `core/internal/app/bandio_anchor_matrix_test.go`: `TestAnchorMatrixBandio/<shape>/<case>`, 16 cells;
- `core/internal/testutil/anchorred.go`: `AnchorKnownRed(t, prop, task)`, which skips unless `ANCHOR_RED=1`.

Each cell asserts the **desired** property (P-ids from `anchoring-spec.md` Part B, rigid-follow model), not today's behaviour. Known-red cells are listed one per line in a `knownRed` table in each file, as `{properties, fix label}`. **A fix flips a cell by deleting its line.**

## Commands and counts

Run in a throwaway worktree off origin/main `a936c93d`, from `core/`:

```
gofmt -l internal/                      # empty
go vet ./internal/chartpdf/ ./internal/sync/ ./internal/app/ ./internal/testutil/   # clean

# CI mode (what lands): known-red cells skip
go test ./internal/chartpdf/ ./internal/sync/ ./internal/app/ -run AnchorMatrix -count=1 -v
  → 85 PASS, 140 SKIP, 0 FAIL  (exit 0)
go test ./internal/chartpdf/ ./internal/sync/ ./internal/app/ ./internal/testutil/ -count=1
  → ok chartpdf 1.8s, ok sync 0.02s, ok app 28.1s   (full packages, nothing else affected)

# Full picture
ANCHOR_RED=1 go test ./internal/chartpdf/ ./internal/sync/ ./internal/app/ -run AnchorMatrix -count=1 -v
  → 85 PASS, 140 FAIL  (exit 1)
```

| package | cells | GREEN | RED |
|---|---|---|---|
| chartpdf (`TestAnchorMatrix`) | 202 | 76 | 126 |
| sync (`TestAnchorMatrixSync`) | 7 | 1 | 6 |
| app / bandio (`TestAnchorMatrixBandio`) | 16 | 8 | 8 |
| **total** | **225** | **85** | **140** |

The gated set equals the red set exactly. In CI mode nothing fails and exactly 140 cells skip. With `ANCHOR_RED=1`, exactly those 140 fail.

## chartpdf matrix (rows = events, columns = shapes)

DEC marks a **DECISION** cell: the test asserts the recommended answer to an open question for VLL:
- `create_margin` and `margin_then_edit`: P4, margin marks follow vertically;
- `multiline_insert_inside`: P12, no stretch.

N-A means a stamp or a text label has no multi-line form.

| event (P-ids) | freehand | line | rect | ellipse | text | highlight | icon | jump_pair |
|---|---|---|---|---|---|---|---|---|
| `create_on_run` (P2) | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN |
| `create_beside_run` (P3) | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN |
| `create_margin` (P4) | RED (DEC) | RED (DEC) | RED (DEC) | RED (DEC) | RED (DEC) | RED (DEC) | RED (DEC) | RED (DEC) |
| `create_far` (P5) | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN |
| `move_then_edit` (P23 P8) | RED | RED | RED | RED | RED | RED | RED | RED |
| `resize_then_edit` (P24 P8) | RED | RED | RED | RED | GREEN | RED | RED | RED |
| `restyle_then_edit` (P25 P8) | RED | RED | RED | RED | GREEN | GREEN | RED | RED |
| `edit_unrelated_line` (P13) | RED | RED | RED | RED | RED | GREEN | RED | RED |
| `insert_line_above` (P8) | RED | RED | RED | RED | GREEN | GREEN | RED | RED |
| `delete_line_above` (P8) | RED | RED | RED | RED | GREEN | GREEN | RED | RED |
| `section_added_above` (P8) | RED | RED | RED | RED | GREEN | GREEN | RED | RED |
| `typo_linked_line` (P19) | RED | RED | RED | RED | RED | RED | RED | RED |
| `typo_then_edit` (P19 P8) | RED | RED | RED | RED | RED | RED | RED | RED |
| `delete_linked_line` (P20) | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN | RED |
| `duplicate_inserted_before` (P14) | RED | RED | RED | RED | RED | RED | RED | RED |
| `multiline_reflow` (P6 P7 P8) | GREEN | RED | GREEN | GREEN | N-A | GREEN | N-A | N-A |
| `multiline_insert_inside` (P12) | GREEN (DEC) | RED (DEC) | GREEN (DEC) | GREEN (DEC) | N-A | GREEN (DEC) | N-A | N-A |
| `size_change` (P9) | RED | RED | RED | RED | GREEN | GREEN | RED | RED |
| `fit_change` (P9) | RED | RED | RED | RED | GREEN | GREEN | RED | RED |
| `columns_change` (P16 P15) | RED | RED | RED | RED | RED | RED | RED | RED |
| `size_change_wrapped_line` (P15) | RED | RED | RED | RED | RED | RED | RED | RED |
| `transpose_bake` (P18) | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN |
| `page_count_shrink` (P17 P29) | RED | RED | RED | RED | GREEN | GREEN | RED | RED |
| `beside_then_edit` (P3 P8) | GREEN | RED | GREEN | GREEN | GREEN | GREEN | RED | RED |
| `margin_then_edit` (P4 P8) | RED (DEC) | RED (DEC) | RED (DEC) | RED (DEC) | RED (DEC) | RED (DEC) | RED (DEC) | RED (DEC) |
| `far_then_edit` (P5) | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN | GREEN |

Cross-cutting properties, asserted in every cell where they apply:
- P10: line and freehand point order and direction (line and freehand columns);
- P11: stamp and text size (icon, jump_pair and text columns);
- P28: `JumpTo` survives (jump_pair column).

## Why each red is red (grouped; one line per failure family)

- **the stamp is re-sized to the line box, not kept at its own size** (17): `icon/delete_line_above`, `icon/edit_unrelated_line`, `icon/fit_change`, `icon/insert_line_above`, `icon/page_count_shrink`, `icon/restyle_then_edit`, `icon/section_added_above`, `icon/size_change`, `jump_pair/delete_line_above`, `jump_pair/delete_linked_line`, `jump_pair/edit_unrelated_line`, `jump_pair/fit_change`, `jump_pair/insert_line_above`, `jump_pair/page_count_shrink`, `jump_pair/restyle_then_edit`, `jump_pair/section_added_above`, `jump_pair/size_change`
- **the box collapses to the line box: its own margins are lost, width clamped to the text** (16): `ellipse/delete_line_above`, `ellipse/edit_unrelated_line`, `ellipse/fit_change`, `ellipse/insert_line_above`, `ellipse/page_count_shrink`, `ellipse/restyle_then_edit`, `ellipse/section_added_above`, `ellipse/size_change`, `rect/delete_line_above`, `rect/edit_unrelated_line`, `rect/fit_change`, `rect/insert_line_above`, `rect/page_count_shrink`, `rect/restyle_then_edit`, `rect/section_added_above`, `rect/size_change`
- **the line text changed, so the exact-text link is lost and the mark freezes** (16): `ellipse/typo_linked_line`, `ellipse/typo_then_edit`, `freehand/typo_linked_line`, `freehand/typo_then_edit`, `highlight/typo_linked_line`, `highlight/typo_then_edit`, `icon/typo_linked_line`, `icon/typo_then_edit`, `jump_pair/typo_linked_line`, `jump_pair/typo_then_edit`, `line/typo_linked_line`, `line/typo_then_edit`, `rect/typo_linked_line`, `rect/typo_then_edit`, `text/typo_linked_line`, `text/typo_then_edit`
- **the line re-wrapped, so its run text changed: link lost, mark frozen** (16): `ellipse/columns_change`, `ellipse/size_change_wrapped_line`, `freehand/columns_change`, `freehand/size_change_wrapped_line`, `highlight/columns_change`, `highlight/size_change_wrapped_line`, `icon/columns_change`, `icon/size_change_wrapped_line`, `jump_pair/columns_change`, `jump_pair/size_change_wrapped_line`, `line/columns_change`, `line/size_change_wrapped_line`, `rect/columns_change`, `rect/size_change_wrapped_line`, `text/columns_change`, `text/size_change_wrapped_line`
- **re-drawn top-left to bottom-right: direction flipped; an on-line straight stroke turns diagonal; width clamped to the text** (11): `line/beside_then_edit`, `line/delete_line_above`, `line/edit_unrelated_line`, `line/fit_change`, `line/insert_line_above`, `line/multiline_insert_inside`, `line/multiline_reflow`, `line/page_count_shrink`, `line/restyle_then_edit`, `line/section_added_above`, `line/size_change`
- **never linked (margin), so it stays put while its line moves** (8): `ellipse/margin_then_edit`, `freehand/margin_then_edit`, `highlight/margin_then_edit`, `icon/margin_then_edit`, `jump_pair/margin_then_edit`, `line/margin_then_edit`, `rect/margin_then_edit`, `text/margin_then_edit`
- **the scribble is squashed into the line box (and x snapped to a character)** (8): `freehand/delete_line_above`, `freehand/edit_unrelated_line`, `freehand/fit_change`, `freehand/insert_line_above`, `freehand/page_count_shrink`, `freehand/restyle_then_edit`, `freehand/section_added_above`, `freehand/size_change`
- **no horizontal overlap with any line, so it never links (T172 overlap rule)** (8): `ellipse/create_margin`, `freehand/create_margin`, `highlight/create_margin`, `icon/create_margin`, `jump_pair/create_margin`, `line/create_margin`, `rect/create_margin`, `text/create_margin`
- **the pre-move link is kept, so the mark snaps back relative to its OLD line** (8): `ellipse/move_then_edit`, `freehand/move_then_edit`, `highlight/move_then_edit`, `icon/move_then_edit`, `jump_pair/move_then_edit`, `line/move_then_edit`, `rect/move_then_edit`, `text/move_then_edit`
- **jumps to the inserted copy (occurrence is counted by identical text)** (8): `ellipse/duplicate_inserted_before`, `freehand/duplicate_inserted_before`, `highlight/duplicate_inserted_before`, `icon/duplicate_inserted_before`, `jump_pair/duplicate_inserted_before`, `line/duplicate_inserted_before`, `rect/duplicate_inserted_before`, `text/duplicate_inserted_before`
- **the pre-resize link is kept, so the resize is undone at the next edit** (7): `ellipse/resize_then_edit`, `freehand/resize_then_edit`, `highlight/resize_then_edit`, `icon/resize_then_edit`, `jump_pair/resize_then_edit`, `line/resize_then_edit`, `rect/resize_then_edit`
- **a stamp narrower than one character collapses to zero width (invisible)** (2): `icon/beside_then_edit`, `jump_pair/beside_then_edit`
- **the text snaps to the line top and to a character boundary** (1): `text/edit_unrelated_line`

## sync and bandio cells

| cell | result | first failure |
|---|---|---|
| `TestAnchorMatrixBandio/ellipse/export_import_round_trip` | GREEN |  |
| `TestAnchorMatrixBandio/ellipse/import_links_unlinked_mark` | RED | m1: imported onto a generated chart with no link, though it sits on a line of text |
| `TestAnchorMatrixBandio/freehand/export_import_round_trip` | GREEN |  |
| `TestAnchorMatrixBandio/freehand/import_links_unlinked_mark` | RED | m1: imported onto a generated chart with no link, though it sits on a line of text |
| `TestAnchorMatrixBandio/highlight/export_import_round_trip` | GREEN |  |
| `TestAnchorMatrixBandio/highlight/import_links_unlinked_mark` | RED | m1: imported onto a generated chart with no link, though it sits on a line of text |
| `TestAnchorMatrixBandio/icon/export_import_round_trip` | GREEN |  |
| `TestAnchorMatrixBandio/icon/import_links_unlinked_mark` | RED | m1: imported onto a generated chart with no link, though it sits on a line of text |
| `TestAnchorMatrixBandio/jump_pair/export_import_round_trip` | GREEN |  |
| `TestAnchorMatrixBandio/jump_pair/import_links_unlinked_mark` | RED | m1: imported onto a generated chart with no link, though it sits on a line of text |
| `TestAnchorMatrixBandio/line/export_import_round_trip` | GREEN |  |
| `TestAnchorMatrixBandio/line/import_links_unlinked_mark` | RED | m1: imported onto a generated chart with no link, though it sits on a line of text |
| `TestAnchorMatrixBandio/rect/export_import_round_trip` | GREEN |  |
| `TestAnchorMatrixBandio/rect/import_links_unlinked_mark` | RED | m1: imported onto a generated chart with no link, though it sits on a line of text |
| `TestAnchorMatrixBandio/text/export_import_round_trip` | GREEN |  |
| `TestAnchorMatrixBandio/text/import_links_unlinked_mark` | RED | m1: imported onto a generated chart with no link, though it sits on a line of text |
| `TestAnchorMatrixSync/move/old_link_not_kept` | RED | a move kept the pre-move link "stored line" (the measured bug: the mark jumps back at the next edit) |
| `TestAnchorMatrixSync/move/relinks_server_side` | RED | a move applied the client's link "client line"; the server must decide it (re-link or clear) |
| `TestAnchorMatrixSync/resize/relinks_server_side` | RED | a resize kept the pre-resize link unchanged; it must re-link with the new extent |
| `TestAnchorMatrixSync/setStyle/echoed_link_kept` | GREEN |  |
| `TestAnchorMatrixSync/setStyle/keeps_stored_link` | RED | a restyle changed the link to client line; it must keep the stored "stored line |
| `TestAnchorMatrixSync/setStyle/missing_link_not_dropped` | RED | a restyle sent without the anchor field unlinked the mark; the server must keep the stored link |
| `TestAnchorMatrixSync/setText/keeps_stored_link` | RED | a text change changed the link to client line; it must keep the stored "stored l |

## Read these GREENs carefully (green by fixture, not by design)

- **highlight** is green in most follow cells because the fixture highlight is drawn exactly over the line box (0.05..0.95 lh), so the re-fit to the line box changes it by less than the tolerance. A highlight drawn taller than its line collapses like `rect`.
- **text** is green where only the vertical position is compared and the point already sits within 0.25 lh of the line top. `text/edit_unrelated_line` (exact tolerance) is red: the label snaps to the line top.
- **multiline_reflow and multiline_insert_inside** are green for boxes and freehand because the fixture's centre falls in the gap between two lines. That routes it to T172's offset path, which keeps the height. The same box centred **on** a line collapses (`rect/insert_line_above`). Today's behaviour depends on where the centre happens to land.
- **transpose_bake** is green: the run geometry of the transposed render is identical for the fixture (G→A). A chord row at the T168 wrap limit is not covered (critique §9).
- **delete_linked_line** is green: today a lost mark is frozen, not moved, which is the desired coordinate behaviour. The "counted in Studio" half of P20 is not observable at this layer.

## Harness seams (what changes when a fix lands)

The chartpdf matrix drives production functions at three seams, marked `SEAM` in the file:
1. **create:** `AnchorObject`;
2. **move/resize/restyle:** a mirror of `sync/apply.go`'s default branch **as it is today**: the client's points, the old anchor, the current hash;
3. **edit:** `RenderWithAnchors` + `Reproject`.

When T191 (or Stage 2 of the critique) lands:
- seam 2 must call the new re-link function;
- seam 3 must call the new projection, which will need the **old** source too for the line diff: the harness already keeps it, as `old`.

The sync cells observe the real `handleMutation` path and assert only on the applied mutation, so they stay valid whichever implementation lands.
