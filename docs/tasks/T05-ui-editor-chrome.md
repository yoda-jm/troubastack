# T05 — UI: compress the song-editor chrome

**Priority:** 5 · **Size:** M/L · **Area:** `web/studio/src/pages/SongEditor.tsx`, `styles.css`

## Context

On a 1440×900 screen the actual score starts ~590px down the page. The stack today:
sticky top bar → centered "← Back to band" link → huge centered `<h1>Wonderwall</h1>` →
a two-row style toolbar → a second viewer-toolbar row (zoom / file tabs / hide-layers) →
finally the PDF. In an editor, the document is the product — compare Google Docs, which
fits identity + title + toolbar in ~110px. Additional noise: every style control is
always visible (color, opacity, width, shape presets, fill/border checkboxes, blend
select, text size) whether or not it applies; status pills ("draw: select", "17 objects",
"live") sit between action controls; "Fit width" appears twice (in the zoom `<select>`
and again as a pill readout); all tool buttons are text-only.

Careful constraint: the existing "stable footprint" mechanism (`.style-slot-off`,
`.draw-hint-off` — hidden-but-space-reserved slots so the toolbar never reflows when the
selection changes) exists to keep the viewer from jumping. Preserve the no-reflow
guarantee while making the bar smaller.

## Changes

1. **Merge the header into one compact bar.** Back-link (`←`), song title (left-aligned,
   ~1.1rem, medium weight), and the connection/"live" status on the right, all in a
   single row directly under the top bar. Delete the centered h1 block. Target: chrome
   above the score ≤ ~220px at 1440×900 (measure with a screenshot).
2. **One toolbar row, grouped.** Reorganize the style toolbar into groups separated by
   thin dividers: [tools] [color+width+opacity] [shape options] [text size] [layer
   picker]. Give the tool buttons icons (inline SVG, no icon-font dependency) with
   `title=` tooltips and `aria-label`s; keep the text label only for the active tool if
   that helps clarity.
3. **Contextual visibility with stable footprint.** Text size shows only for the Text
   tool / a selected text object; shape presets + fill/border/blend only for shape tools
   or a selected shape — using the existing reserved-slot technique so total bar height
   is constant. The bar must not change height when switching tools or selections
   (this is asserted informally today by e2e; re-verify).
4. **De-duplicate zoom.** One zoom group: `− [readout/select] +`. Remove the redundant
   "Fit width" pill.
5. **Move status out of the action rows.** "17 objects" and "live"/connection state
   belong in the header bar right side (from change 1) or the layers panel — not between
   buttons.
6. Keep all `data-testid` attributes on the controls (move them with the elements);
   update `web/studio/e2e/*.spec.ts` where structure genuinely changed.

## Acceptance criteria

- Screenshot check at 1440×900 (`make demo`, marie → Wonderwall): first page of the score
  is visible above the fold; chrome above the score measures ≤ ~220px.
- Switching tools (Select→Pen→Text→Rect) and selecting/deselecting objects produces
  **zero vertical shift** of the viewer (record `getBoundingClientRect().top` of the
  first `.pdf-page` before/after in the browser console, or add a quick e2e assertion).
- Every toolbar button has an accessible name (axe or manual `aria-label` audit).
- `make e2e` green; TS typecheck green.

## Out of scope

- Any change to drawing behavior, sync, or the canvas stack (T06 owns the wet path).
- The layers/annotations side panels' internals (light pill restyle only, per T03/T04).
- Splitting the file (T10) — keep the refactor minimal here.
