# T88 — The icon palette should hug the page, not the far viewport edge

**Priority:** normal · **Size:** S–M · **Area:** `web/studio` (`styles.css`, the viewer/toolbar,
a small pure helper + its unit test, e2e). Lane: Web & Core.

VLL, 2026-08-21: *"the icon bar should not be full left if the page is smaller, it should be anchored
at the page left (outside), of course if the zoom is too big it is aligned in the viewport left"*.

## The bug, pinned

`.icon-palette` (T51's glyph strip for the Icon tool, `web/studio/src/styles.css:683`) is docked to
the viewer, not to the score:

```css
.icon-palette { position: absolute; z-index: 7;
  top: calc(var(--chrome-h, 3.5rem) + 3.6rem); left: .75rem; … }
```

A PDF page is centred in the viewer and is usually much narrower than a desktop viewport, so the
palette ends up stranded far to the left with a wide empty gutter between it and the score it stamps
onto. The eye has to travel the whole gap on every stamp.

## The fix — the same clamp T85b already established

This is exactly the shape VLL approved for the beat frame in T85b, one axis. Reuse the idea, and the
measurement discipline:

```
paletteLeft = clamp(
  pageLeft - gap - paletteWidth,   // preferred: just OUTSIDE the page's left edge
  viewportLeft + margin,           // floor: never off-screen when zoomed in
  …
)
```

- **Page narrower than the viewport** (the common case): the palette's right edge sits `gap` to the
  left of the page's left edge — beside the score, not covering it.
- **Zoomed in until the page reaches or passes the viewport edge**: the clamp takes over and the
  palette rests at `viewportLeft + margin`, which now means it overlaps the page. **That is
  intended** — VLL asked for it explicitly. The palette is already click-through except on its
  buttons, so the score underneath stays drawable.

### Measurement discipline (non-negotiable)

Follow `positionFrame` in `web/studio/src/useBeat.ts`: **two `getBoundingClientRect()` reads total**
(first and last `.pdf-page`), never one per page. Every page div is mounted — T45 virtualizes
rasterizing, not the divs — so a per-page read on a 20-page part is 20 forced layouts. This was the
T85b review nit; do not reintroduce it here.

Reposition on the things that actually move the page: **zoom change, scroll, resize, page/part
change**. It does not need a rAF loop — the palette is static between those events, unlike the beat.

### Put the arithmetic in a pure function

Add `iconPaletteLeft(page, viewport, paletteWidth, gap, margin): number` next to `frameBox` (or
beside it in the same module) and unit-test it directly. Three cases at minimum:

| case | expectation |
|---|---|
| narrow page, wide viewport | `left === page.left - gap - paletteWidth` |
| page zoomed past the left edge (`page.left < margin`) | `left === viewport.left + margin` |
| exactly enough room for the palette and no more | takes the preferred position, not the clamp |

A pure function means the interesting behaviour is provable without a browser, and the e2e only has
to confirm it is wired up.

## Acceptance criteria

- Unit test on the helper covering the three cases above, plus a degenerate viewport (zero/negative
  width must not produce `NaN` or a negative `left`).
- **e2e, in the editor with the Icon tool selected:**
  - wide viewport / unzoomed page → `paletteRect.right <= pageRect.left` (it is *outside* the page);
  - zoomed until `pageRect.left < margin` → palette clamps to the viewport edge and stays fully
    on-screen (`paletteRect.left >= 0`).
- **Teeth-check:** hard-code the old `left: .75rem` behaviour back and confirm the first e2e
  assertion goes red. Record it.
- The palette still renders only for the Icon tool, is still click-through except on its buttons, and
  its **vertical** position is unchanged.
- The glyph buttons remain reachable and stampable after repositioning (stamp one and assert the
  object lands) — moving a container has broken pointer routing before.
- `tsc -b studio` clean; **full `make e2e`** on the isolated ports, not a subset.
- Dangling-testid sweep if any testid moves.

## Out of scope

- Redesigning the palette's contents or the Icon tool itself.
- Applying the same treatment to the top/bottom pills — they are deliberately centred and
  width-capped, which already reads as attached to the page.
- Right-side placement or a user-configurable side. If VLL wants the palette to flip to whichever
  gutter is wider, that is a follow-up; this task anchors it left.
