# T196 — Anchoring stage 3: Studio shows each mark's link state, and the bake lists what needs a look

**Lane:** web-core (core + studio) · **Status:** implemented on `fable/anchoring-stage3`, not pushed, for the gate.
**Size:** M. · **Origin:** `docs/design/anchoring/critique.md` §7 "Stage 3" (T192 simplified to the four states).
VLL decided the words (Linked / Check / Lost / Fixed) and the UI: a dot on the linked text and a thin line to
the mark. T192's ⟨D1⟩–⟨D3⟩ stand: dot + line on selection plus a "Show links" toggle; a per-file counter that
click-cycles; no manual re-linking (moving a mark re-links it, T195). **Builds on:** T195 (stage 2), which
serves every mark's `link` with its state.

## 1. What the client receives (server)

The client has no manifest, so the server says where a linked mark's text is. Two **serve-time** fields ride
inside the served `link`:

| field | meaning |
|---|---|
| `link.at` | `{page, x, y, h}`: the linked character's top-left on the CURRENT render, in [0,1] page coordinates, and that visual line's height (so the client centres the dot on the line without a manifest) |
| `link.text` | the linked source line's text **as drawn**: every visual line and run of it, without `**bold**` markers; a wrap's dropped space is restored. The client truncates it |

- **Which character** (`chartpdf.linkView`), by the link's `xRef`:
  - `char`: the character under the mark's left edge, followed through edits (the same character the
    projection follows);
  - `start` / `end`: the visual line's first character / the end of its text;
  - `margin`: no character is under the mark; the end of the line nearer to the mark (its start for a
    left-margin mark).
- **When:** only for a **Linked** or **Check** mark whose line draws on the current render. Absent on Lost
  and Fixed marks, on old-style marks (T145 anchor, no link), on marks of uploaded files, and when the chart
  has no current render (P32).
- **Same projection pass, no extra render.** `chartpdf.ServeMark` = `ProjectMark` + the view, from the same
  `project()` call. For a projected mark it uses the link re-measured on the current render; for a mark
  already on it, its own link. `projectSnapshot` (REST GET and the realtime join snapshot) calls it. The bake
  keeps calling `ProjectMarks`: nothing about links reaches the tablet (P34).
- **On the realtime echo**, so a move updates the dot live: `sync.ChartAnchorer` gains `ServeEcho`. After an
  accepted write the hub asks it for the view of the echoed object (a copy; HEAD is untouched). Only the
  view is added: the echo's geometry and state are what was stored, as in T195.
- **Never stored, never read from a client.**
  - `domain.Object.View *domain.LinkView` carries `json:"-"`. The store JSON-encodes Objects (filestore),
    and the v1 manifest is `domain.Object` as is, so neither can persist it.
  - On the wires the link is `domain.WireLink{MarkLink (flattened), At, Text}`. `WireLinkOf(o)` writes it;
    `(*WireLink).Stored()` reads it and drops `at`/`text`. Both DTOs (sync and REST) use the pair, so the
    two mirrors cannot drift.
  - Studio spreads the served object back on a move; its `link.at` is ignored like the rest of its link
    (P26).
  - The four field-completeness guards (sync, REST, band folder, bake doc) list `View` with the reason.

### The pre-bake list: an endpoint, not a bake field

`GET /api/bands/{bandId}/setlists/{setlistId}/link-check` →
`{songs: [{songId, title, files: [{fileId, filename, check, lost}]}], check, lost}`.

- **Why an endpoint.** There is no bake "preflight" today: the bake POST is a fire-and-poll kick (T103),
  and its response arrives after the decision to bake. The dialog needs the list **before** the click, and
  it already fetches per song on open. A read-only endpoint keeps the bake POST unchanged, and it cannot
  block the bake: the bake never consults it.
- **What it counts.** Songs in setlist order: breaks skipped, a song listed once. For each song, its
  generated files with at least one **Check** or **Lost** live mark, with the states **as served**
  (`projectSnapshot`, the same projection GET and the bake use). All layers count, because the bake bakes
  all layers. Old-style marks (Fixed), uploaded files and tombstones never count.
- **Gate:** member-gated, like every read it is made of (the setlist, each song's files and annotations).
  The dialog itself is admin-only.

## 2. What the writer sees (Studio)

- **The dot and the line (⟨D1⟩).**
  - When exactly one mark is selected on a generated chart and it is Linked or Check, a dot is drawn at
    `link.at` (the character's left edge, at mid-line), with a thin dotted line to the nearest point of the
    mark's box edge.
  - A mark on its own words shows the dot alone: the line is drawn only when the dot is outside the box by
    3 px or more.
  - The box follows a live drag, so the line stretches with it; the dot stays on the text.
  - **Colours:** the Studio accent (`var(--accent)`) at 0.8 opacity, over a paper-white halo. The chart page
    is white paper in both themes, so the halo is deliberately not themed. It keeps the dot and line legible
    over blue chords and black lyrics.
  - **Check** is the same tie with a **hollow** dot: the mark still follows its line, but asks for a look.
- **"Show links"** is a pill in the top bar next to "Edit chart", on generated charts only. It draws the tie
  for every Linked or Check mark on the visible layers. It is off by default and remembered per browser
  (`localStorage` `trouba_show_links`, every access in try/catch; broken storage reads as off).
- **The details** are a slim pill under the style row, shown for one selected mark on a generated chart:
  the state word, then VLL's sentence, verbatim:
  - Linked: *Linked to "Pack a little light for the…"*. The line's text is cut to at most 32 characters,
    at a word boundary, with an ellipsis.
  - Check: *Check: the text under this mark changed, or the lines it spans moved apart. Move it to
    confirm.*
  - Lost: *Lost: the line it was on was deleted. Move it onto the right words to link it again.*
  - Fixed: *Fixed position on the page.*
  - **Old-style** (no link, on a generated chart): Fixed, plus *Placed before linking existed; move it to
    link it.*
- **The per-file counter (⟨D2⟩)** sits right after the open file's tab in the bottom bar: **"N to check"**
  (`1 to check`, `3 to check`). It counts **Check + Lost** marks on the visible layers. Clicking it switches
  to the Select tool, selects the next mark in reading order (page, then top, then left; wrapping) and
  centres it on screen. It is absent when there is nothing to count, and on uploaded files.
- **Chrome only.** The tie is DOM inside the page's aria-hidden `.selection-overlay`, like the P206 jump
  segment. It is never painted into the annotation canvas. Bakes, the file PDF (FILEPDF), note composites
  and Stage all render server-side from stored marks, so none of them can contain it.
- **The bake dialog** (P205's BakeDialog) fetches the link-check on open. When it lists anything, an amber
  note names each song and file with its counts (`1 Check · 1 Lost`), says the tablet cannot show these
  states and that the marks bake where they are. The Bake button is unaffected.
- **Uploaded files:** no indicator, no counter, no details line.

## 3. Decisions to flag at the gate

1. **"N to check" counts Lost too.** One counter for "needs a look". The bake list separates the two words.
2. **Any link-less mark on a generated chart reads "Placed before linking existed".** That is also true of a
   mark created while the chart's render diverged (P32, rare), for which the note is inexact.
3. **The dot is the character's left edge at mid-line,** not its top-left corner: a dot on the top edge
   floats above the letters. The server sends the top-left plus the height, as specified, and the client
   centres the dot.
4. **On-run marks show a dot without a line** (the dot is on the box's edge). Beside, margin and multi-line
   marks show both.
5. **Check's dot is hollow.** This is not in VLL's words; it is easy to drop.
6. **The counter counts visible layers only** (a hidden layer's mark cannot be shown when selected). The bake
   list counts every layer.
7. **The details pill shows for read-only selections too** (inspecting someone else's mark).

## 4. Tests

**Go**
- `chartpdf/linkview_t196_test.go`. Expectations come from the renderer's runs:
  - on-run → the left-edge character;
  - beside (an underline below its line) → on the line, not the mark;
  - right of a short line → the line's end;
  - left margin → the line's start;
  - multi-line → the top line's start;
  - after an edit, from the same pass → the dot moves by exactly the inserted word, and the text is the
    edited line's; a typo under the mark → Check, with a view;
  - no view for lost, fixed, old-style and unlinked marks;
  - the text of a bold line and of a wrapped line.
- `httpapi/linkview_t196_test.go`, through the real websocket, hub, anchorer and REST:
  - GET serves `link.at` and `link.text`, and the join snapshot serves the same;
  - old-style and uploaded-file marks carry no link;
  - after a source edit, the view is on the new render;
  - the create, move and restyle **echoes** carry the server's view; a forged client `at`/`text` is
    ignored;
  - HEAD stores no view;
  - the DTO writes the view and never reads it;
  - `link-check` counts (1 Check, 2 Lost; linked, fixed, old-style and deleted marks are not counted; a
    song listed twice and a break are handled);
  - the pure count skips uploads and clean files.
- `sync/linkview_t196_test.go`: the realtime wire writes the view and never reads it.
- `domain/linkview_t196_test.go`: the stored mutation JSON has no view; `Clone` copies it; `WireLink`
  round-trips.

**Teeth.** Each sabotage was applied in place, the tests went red, and the code was restored:

| sabotage | first red line |
|---|---|
| dot at the line start, ignoring the linked character | `on-run: at {… X:0.0381 …}, want … x 0.1480` |
| view from the stored link instead of the re-measured one | `after the edit: at {… X:0.1446 …}, want … x 0.2084`; httpapi `GET after an edit before the word: link.at {… X:0.06804 …}, want … x 0.09798` |
| the echo without `ServeEcho` | `create echo: no link.at on the wire` |
| link-check ignores Check | `link-check {… Check:0 Lost:2}, want 1 check and 2 lost` |
| `View` without `json:"-"` | `stored mutation JSON carries the view: …"View":{"At":…`; httpapi `stored JSON carries the view` |
| the sync wire reads a client's view | `objectFromJSON read a client's view: &{At:{Page:2 X:0.125 …}` |

**Studio unit** (`test/mark-link.test.ts`, `test/link-ties.dom.test.tsx`):
- states and the legacy Fixed;
- the product words, verbatim, and truncation;
- the counter's population, order and cycling;
- the tie geometry (nearest edge, no line inside or right next to the box);
- the storage (off by default, broken storage is off, never throws);
- the rendered dots and lines.

**Studio e2e** (`e2e/link-states.spec.ts`), on a generated demo chart (synthetic words). The marks are
imported; their positions come from a probe copy of the chart, through the server's own `link.at`.
- **Select a linked highlight:** the dot and line render, the dot sits on line 1, and a **pixel probe** of
  the screen finds accent at the dot. **Deselect:** the dot and line are gone, and so is the accent at that
  pixel.
- **"Show links":** three ties; still on after a reload; off removes them. Toggling sends **no** websocket
  mutation.
- **Delete the linked line:** "1 to check"; a click selects that mark; the details read Lost, with no dot.
  **Drag it onto a line:** Linked, a dot (from the echo), and the counter is gone.
- **A typo under a circle:** "1 to check"; the click selects the circle as Check, with a dot.
- **An old-style mark:** Fixed, with the note.
- **The bake dialog:** one row, `1 Check · 1 Lost`, the tablet note, and Bake still enabled.
- **No indicator in a bake:** the file PDF is downloaded through the rail button with "Show links" off and
  on. The request is the same; the bytes are compared when the server has the bake toolchain (poppler and
  the web/bake worker). Without it, the test says so in an annotation.

**E2E teeth** (sabotaged and restored):

| sabotage | red |
|---|---|
| no tie for the selected mark | tests 1, 3, 4: `link-dot` count 0, want 1 |
| the bake list hidden | test 5: `bake-link-check` not found |
| the counter counts Lost only | test 4: `link-attention` not found |
| "Show links" not remembered | test 2: `aria-pressed` "false" after the reload |

**Gates:** see the gate note for the numbers.

## 5. Not in this task

- Stage 4, the migration that re-links old-style marks.
- Fuzzy re-linking.
- Any indicator in Stage or the app.
