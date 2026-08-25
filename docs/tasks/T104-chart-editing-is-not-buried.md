# T104 — Editing a chart is one click, and the editor has room to work

**Priority:** normal — it's the everyday action on the product's own file type · **Size:** S
**Area:** `web/studio` (Web & Core lane). VLL, 2026-08-25: *"the edit chart is buried in a dropdown of
an optional menu and is there very small; for charts it should be fast to access… and the edit area
should be somewhat large."*

## 1. Why it's buried — the cause, not just the symptom

Chart rows and PDF rows share one component, so a chart's **primary verb inherited the PDF's
housekeeping menu**. For a PDF that menu is the whole story — rename, reorder, delete; you cannot edit a
PDF here. For a generated text chart **the source IS the file**, and editing it is the main thing you
ever do to it.

Today the path is: open the files panel → find the row → open the row's **⋯ overflow** → click
**"View source"**.

**And the label says read-only.** That is the first thing to fix regardless of the rest: someone
hunting for "edit" doesn't find it, and someone who does find it has no reason to think they can type.

There is also an asymmetry worth naming: **creating** a chart has two prominent buttons ("＋ New text
chart", "＋ New chart from lyrics") while **editing** one is behind an overflow menu under a
read-only label. You create a chart once and edit it many times.

## 2. What to build

**(a) Rename the action to "Edit chart."** Not "View source". The dialog is an editor; say so.

**(b) Put a visible Edit control on the row, for generated charts only.** The row already renders a
`text chart` chip, so it already knows the file's type — the control belongs beside that chip. One
click, no menu.

**Remove the menu item once the row control exists.** Keeping both is redundancy inside the very menu
the complaint is about. (If review disagrees and wants the menu entry kept as a secondary path, say so
at the gate — but the default is: one obvious way.)

**(c) Give the editor room.** Today `.chart-src-ta` is `min-height: 22rem` with `resize: vertical`, and
the source and preview panes each sit at `flex: 1 1 20rem; min-width: 16rem`. At `.85rem/1.5` that is
roughly **17 visible lines** for a document made of verses — and the app's answer to "I need more room"
is "drag it yourself, every time".

- The editor should **fill the height available to it** rather than sitting at a fixed 22rem.
- **Stack the source and preview panes on narrow viewports.** Two panes at `min-width: 16rem` fight on a
  laptop; side-by-side should be a wide-viewport layout, not the only layout.
- Keep `resize: vertical` as an escape hatch — just stop making it the primary mechanism.

## 3. The cost this task must pay — SEVEN e2e call sites

`getByTestId("file-menu-source")` is how **seven** e2e locations open the chart editor:
`editor-t67-chart-refresh`, `editor-transpose` (×4), `text-chart`, and `files-list-menu`. Removing the
menu item breaks all of them, so this is a small feature plus a real test migration. Budget it.

**`files-list-menu.spec.ts` needs care, not a find-and-replace.** It asserts the affordance is
**present on a chart row and ABSENT on a PDF row** — that intent is exactly the type-awareness this task
is about, so it must survive, retargeted at the new control rather than deleted.

## 4. Acceptance criteria

- Editing an existing chart is reachable in **one click from the file row**, with a label that reads as
  editing.
- The affordance appears on generated charts and **not** on PDFs — asserted, carrying
  `files-list-menu`'s intent forward.
- All seven former `file-menu-source` paths still reach the editor and still pass.
- The editor pane fills its available height; at a narrow viewport the panes stack rather than squeeze.
  Assert the narrow case — it is the one that silently regresses.
- T67 (chart re-render refresh) and T60 (transpose) behaviour unchanged — those flows go through this
  dialog and must not be disturbed by the relocation.
- `tsc -b` clean; full `make e2e`.

## 5. Out of scope

Editing from the viewer, and a dedicated editor route — both are **T105**, deliberately sequenced after
this.
