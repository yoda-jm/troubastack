# T14 — Editor chrome: reach the ≤220px target (panelize)

**Priority:** after T05 · **Size:** M · **Area:** `web/studio/src/pages/SongEditor.tsx`, `styles.css`

## Context

T05 compacted the song-editor chrome from ~590px to **~372–435px** at 1440×900
(top of first `.pdf-page`; the T05 author measured 372, the review re-measured 434.5 on
the same viewport — the delta is almost certainly toolbar wrap under different font
metrics, the same environment-sensitivity T13 chases; measure your own baseline before
and after, in the same browser) and met all its qualitative goals — one-row header
(back · title · status), icon tools, deduped zoom, status out of the action rows,
contextual style controls with the no-reflow guarantee intact. But the T05 acceptance
target was **≤ ~220px**, and 372 is short of it.

The remaining height is dominated by three stacked bands above the score:

- `.editor-toolbar` (~134px): three groups — tools / style / **layer picker** — wrap to
  ~2–3 rows because the reserved-slot mechanism reserves the *maximal* control set.
- `.viewer-toolbar` (~42px): zoom + file picker + hide-layers.
- fixed topbar (67px) + card/page padding.

## Changes (proposed)

1. Move the **layer controls** (`Drawing on:` indicator, `Active layer` select,
   `+ New layer`, `Delete`, `Edit this layer`/hint) out of `EditorToolbar` into the
   **Layers side panel** (`LayersPanel`) — layer management belongs with the layers.
   Keep every `data-testid` (`active-layer`, `new-layer`, `delete-object`,
   `edit-this-layer`, `edit-layer-hint`) so e2e keeps passing by testid. This drops the
   toolbar to a single **tools + style** row.
2. Fold the **zoom** control group into the header row (right side, next to status), so
   the `.viewer-toolbar` shrinks to just the file picker (or also relocate the picker).
3. Trim the residual page/card padding above the viewer.

Target: top of first `.pdf-page` ≤ ~220px at 1440×900, **without** breaking the
zero-shift guarantee (re-measure across Select→Pen→Text→Rect and selecting/deselecting)
and with `make e2e` green.

## Acceptance criteria

- Chrome above the score ≤ ~220px at 1440×900 (screenshot/measure the `.pdf-page` top).
- Zero vertical shift on tool/selection changes (unchanged from T05).
- Every control keeps its `data-testid`; `make e2e` green; TS typecheck green.

## Out of scope

- Splitting `SongEditor.tsx` (T10). Drawing/sync/canvas behavior (T06).
