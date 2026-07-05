# T17 — Editor chrome: collapse the style bar behind a disclosure (reach ≤220px)

**Priority:** after T14 (supersedes T14's height goal) · **Size:** M · **Area:**
`web/studio/src/pages/song-editor/Toolbar.tsx`, `styles.css`

## Context — why this exists (T14 finding)

T14 proposed reaching the ≤220px chrome target by "panelizing": move the layer
picker into the Layers panel, fold zoom into the header, trim padding. That approach
was implemented and **measured at 1440×900 — it nets only ~10px** (372 → ~363), and
was reverted (no code landed). The measurements show why the panelize approach can't
move the needle:

| Band above the score | Height | Notes |
|---|---|---|
| global sticky `.topbar` + `.page`/card padding + inter-band gaps | **~160px** | Not editor-scoped — the app shell. Page `padding-top` already trimmed to 8px (T05). |
| `.editor-header` (back · title · status) | ~27px | Already one shallow row. |
| `.editor-toolbar` | **~134px** | = `tool-palette` 30 + **`style-controls` ~90px (3 wrapped rows)** + padding. |
| `.viewer-toolbar` (files · hide-layers) | ~42px | One row. |

Key findings, all confirmed by measurement (Playwright @1440×900):

- **Relocating the layer picker is height-neutral.** It shared a wrapped line inside
  `.editor-toolbar`; removing it left the toolbar at 134px because `style-controls`
  dominates. It also *hides* Drawing-on / Active-layer / New / Delete when the sidebar
  is collapsed — a UX regression for ~0px. **Don't do it.**
- **Folding zoom into the header is height-neutral.** Zoom shared the `.viewer-toolbar`
  row with the file controls; removing it didn't drop the row.
- The single dominant, editor-controllable cost is **`style-controls` (~90px, 3 rows)**,
  which is wide because the no-reflow design (T05) keeps *every* style slot mounted
  (`visibility:hidden` reserves the maximal control set) so the footprint never changes
  across tool/selection. At 1440px that maximal set can't fit in fewer than ~3 rows.

So ≤220 is only reachable by making `style-controls` occupy **one thin row** — i.e.
put the bulky/contextual controls behind a **disclosure/popover** instead of reserving
them inline.

## Changes (proposed)

1. Reduce the always-visible style bar to a **single row**: keep the target/tool
   indicator + color swatches + a compact color/opacity, and move the rest (Width,
   the shape-style block — presets/Fill/Border/Blend, Text size) into a **disclosure**
   (a "More…" popover / expandable panel anchored to the toolbar) that opens *over* the
   score without displacing it.
2. Preserve the **zero-shift guarantee** a different way than reserve-all-inline: the
   collapsed bar is a fixed single-row height regardless of tool/selection, and the
   disclosure is an overlay (absolute/popover), so opening it never reflows the viewer.
   Re-verify across Select→Freehand→Text→Rect→Ellipse and select/deselect, at both
   1440px and the narrow (≤760px, sidebar-stacked) width — same discipline as T13.
3. Keep **every `data-testid`** (`style-width`, `style-fill`, `style-stroke`,
   `style-blend`, `preset-*`, `style-font`, …) attached to the equivalent control
   inside the disclosure so the e2e suite keeps passing by testid; update specs only if
   a control must be opened first (add a click on the disclosure trigger).

## Design decision to resolve (Fable)

This **relaxes the "no-reflow by reserving every slot inline" mechanism** (T05/772be41)
in favor of "no-reflow by fixed-height bar + overlay disclosure." That is a deliberate
trade — the reserve-all approach is *why* the bar is 3 rows tall. Confirm this direction
(vs. living with ~360px chrome) before building. The ≤160px app-shell band (sticky
topbar + page padding) is out of scope and sets a hard floor: with a one-row style bar
(~28px), expect roughly **~230–260px** total — at or just above 220; treat ≤~240 as the
practical target unless the topbar is also revisited (separate task).

**Decision (2026-07-05, architect): GO — direction confirmed.** The invariant we
actually guarantee is *"the score never shifts"*; reserve-all-inline was one mechanism
for it, not the invariant itself. A fixed-height single-row bar + overlay disclosure
preserves the invariant and is the only measured lever to the target. Constraints:

1. **≤~240px is the accepted target** at 1440×900; state the achieved number in the
   close-out. Do not contort the design to hit a literal 220 — the app-shell floor is a
   separate task if we ever want it.
2. **Inline-set preference:** target/tool indicator + color swatches + opacity as
   proposed; **prefer keeping Width inline too if the single row fits at 1440px** — it
   is the most-touched drawing control after color. If it doesn't fit, the disclosure
   is acceptable.
3. The zero-shift guarantee must be held by an **e2e spec, not a manual check** —
   extend the T13-style footprint assertions to cover tool changes, select/deselect,
   and disclosure open/close (the disclosure must be an overlay: `position:absolute`/
   popover, never in-flow).
4. The disclosure must close on outside click/Escape and must not trap drawing input —
   opening it, adjusting, and drawing again should cost one click, not a mode switch.

## Acceptance criteria

- Chrome above the score materially reduced at 1440×900 (measure `.pdf-page` top);
  target ≤~240px, ideally ≤220 (state the achieved number).
- Zero vertical shift on tool/selection changes AND on opening/closing the disclosure,
  at 1440px and ≤760px.
- Every style control keeps its `data-testid`; `make e2e` green; TS typecheck green.

## Out of scope

- The global sticky topbar / app-shell padding (the ~160px floor).
- Splitting `Viewer.tsx` (T15). Drawing/sync/canvas behavior (T06).
