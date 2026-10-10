# T192 — Studio shows what a mark is linked to: a dot, a thin line, and lost links counted

**Lane:** web-core (core + studio) · **Status:** specced 2026-10-11, not started · **Depends on:** T191 (a
move re-anchors). Without it the dot would show a link that the next move silently contradicts. ·
**Origin:** VLL, 2026-10-11: *"quand un truc est anchored il faudrait pouvoir le voir ou avoir un indice
(genre un point avec un trait fin)"*. He accepted Fable's three recommendations the same day.

## 1. Why

On a generated chart, a mark may be **linked** to a line of the source (T145/T172 anchors), **lost** (it has
an anchor, but the text it named is gone: a typo fixed in that line, the line deleted, or the chords of a
linked chord row changed), or **fixed** (no anchor: placed more than 3 run-heights from text, or on an
uploaded PDF). Today none of this shows. VLL's "Capo" mark sat by the title but was linked to the first verse
line two lines lower. He only found out when an edit moved it.

## 2. What the writer sees (VLL's choices, ⟨D1⟩–⟨D3⟩)

**⟨D1⟩ On selection, plus a "Show links" toggle.**
- When **one** mark is selected on a generated chart and it is **linked**:
  - draw a small **filled dot** at the start of the letters it is linked to (the anchored span on its run);
  - draw a **thin dotted line** from the dot to the nearest edge of the mark's selection box.
- A mark linked **beside** its run (T172 offset) gets the same dot and line, so the line visibly crosses to
  the line it depends on, even two lines away.
- A **"Show links"** toggle in the editor toolbar draws the dot and line for **every** linked mark on the page.
  It is off by default and remembered per browser (`localStorage`, `try/catch`, broken storage meaning off).
- Colours: the dot and line in the **Studio accent** at reduced opacity. They must read on light and dark
  themes, and over chord-blue and lyric-black text.
- **Chrome only.** Never in a bake, a PDF download (FILEPDF), a note composite (T186) or Stage.

**⟨D2⟩ Lost links are counted without selection.**
- On a generated file with at least one lost link, show a quiet counter by that file's name in the
  bottom bar: **"N lost links"** ("1 lost link"). Clicking it selects the first lost mark and scrolls to it.
  Clicking again moves to the next. Each lost mark, while selected, shows a **hollow orange dot** where its
  run *was* last placed (its cached `Points` box edge) and no line.
- No counter on uploaded PDFs, where there is nothing to lose.

**The mark's details** (wherever the editor shows a selected object's properties today; add a line if it shows
none):
- linked → *Linked to "C'est un endroit qui…"*: the run text, cut at ~32 characters with an ellipsis;
- beside → the same text, plus *(above)* or *(below)*;
- lost → *Link lost: the text it was on changed. Move it to re-link it.*
- fixed → *Fixed position on the page.*

These are product words. **Use them exactly; VLL renames, not the lane** (`a-label-is-a-product-word`).

**⟨D3⟩ No manual link choice.** No dragging the dot onto another line. With T191, moving the mark re-links it
to the nearest run. The "lost" message says so.

## 3. Where the state comes from: the server decides

The client has no anchor manifest and must not guess. The serve path that already re-projects marks
(`chartpdf.Reproject` at serve) knows the state of each mark. Expose it per object on generated files:
- `link: "linked" | "lost" | "fixed"`:
  - **linked** = has an anchor and `Project` resolved it on the current render;
  - **lost** = has an anchor and `Project` failed (run gone);
  - **fixed** = no anchor.
- For **linked** marks only, `linkBox`: the anchored span's box on its run, in [0,1] page coordinates, **before**
  the T172 offset (where the dot goes), plus `linkText` (the run text, which the client truncates).
- **The realtime echo carries the same fields**, so the dot follows a move live. T191 recomputes the anchor
  on a move, and the echo must state the new link without a reload.
- **Uploaded files:** omit the fields entirely. The client treats absent as "no indicator, no counter".
- **No new storage.** These are computed at serve, never persisted.

## 4. Not in this task

- Fuzzy re-linking after a typo fix, or disambiguating repeated lines by section. Both are named in VLL's
  annotations doc as later options.
- Manual link editing (⟨D3⟩).
- Any indicator in Stage or the app.

## 5. Acceptance

**Go (serve):** on a fixture chart:
- a mark on a run → `linked`, with `linkBox` inside that run's box and `linkText` equal to the run text;
- a mark beside a run (offset) → `linked`, with `linkBox` on the run, **not** on the mark;
- edit that run's text and re-render → `lost`, with no `linkBox`;
- a mark far from text → `fixed`;
- an uploaded PDF → no fields;
- **Teeth:** report every anchored mark as `linked` without calling `Project`. The `lost` case goes red. Print
  the line.
- The realtime move echo (after T191) carries the recomputed `link` / `linkBox`.

**Studio e2e (generated chart):**
- Select a highlight: the dot and line render, and a pixel probe finds accent pixels at `linkBox`'s start.
  Deselect: they are gone.
- "Show links" on: every linked mark shows its line. Reload: still on.
- Edit the source to change a linked line's text, then save: the counter reads "1 lost link". A click selects
  that mark and shows the hollow dot, and its details read the lost message.
- Move the lost mark onto another line (T191 re-links): the counter disappears and the dot shows on the new
  line.
- No indicator pixels in a bake of the same chart. Compare against a bake with the toggle off; they must be
  identical bytes.
- **Screenshots** at the gate: a selected linked mark, "Show links" on, and the lost counter plus the hollow
  dot, in light and dark. Demo data only.

**Regression:** go (`-race`, gofmt), studio typecheck and unit, and the annotation, sync and bake e2e are green.
