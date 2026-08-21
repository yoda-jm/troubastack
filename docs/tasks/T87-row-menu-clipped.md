# T87 — The per-file "⋯" menu is clipped away by its own container

**Priority:** high — it is a **dead control**, not a cosmetic flaw · **Size:** S–M ·
**Area:** `web/studio` (`components/RowMenu.tsx`, `styles.css`, e2e). Lane: Web & Core.

VLL, 2026-08-21: *"the 3 dot around a file to have more info is inside its container so it is not
over the around, so we cannot see it and clicking"*.

## The bug, pinned

`RowMenu` (`web/studio/src/components/RowMenu.tsx`) renders its panel as
`.row-menu-panel { position: absolute; right: 0; top: calc(100% + 4px) }` inside
`.row-menu { position: relative }`. On the song editor's **Files** list
(`pages/song-editor/SongDetails.tsx:500`, `<RowMenu testId="file-menu">`) the row sits inside

```
section.card.details-section   ← styles.css:1063  { padding: 0; overflow: hidden; }
  └ div.details-body
      └ div.file-row
          └ div.row-menu → div.row-menu-panel   (absolutely positioned, CLIPPED here)
```

`overflow: hidden` on `.details-section` makes it the panel's clipping ancestor. The panel is laid
out *below* the trigger, so on the lower rows — and on the last row always — it is cut off at the
section edge: invisible, and its items unclickable because they are not painted.

**Do not simply delete the `overflow: hidden`.** It is almost certainly there so the collapsible
card's rounded corners clip its body. Check that first: if removing it is genuinely safe, the corner
rounding must still be correct on an open section with a long file list, top and bottom. Even then,
prefer the portal below — it fixes the whole class rather than this one instance, and `RowMenu` is a
shared component that will land inside another scroll container sooner or later.

## The fix

Render the panel in a **portal on `document.body`**, `position: fixed`, positioned from the
trigger's `getBoundingClientRect()`. No ancestor can clip a fixed-position child of `<body>`.

Flip when it would leave the viewport:
- **vertically** — if `triggerRect.bottom + panelHeight + margin > innerHeight`, place it *above* the
  trigger instead;
- **horizontally** — the panel is right-aligned to the trigger today; keep that, but clamp so
  `left >= margin` and `right <= innerWidth - margin`.

Close (or reposition) on `scroll` (capture: true, so nested scrollers count) and on `resize`. A menu
left hanging over unrelated content after a scroll is worse than the bug being fixed.

### Two traps this refactor sets — both must be handled, and both are testable

1. **Click-outside will start eating the menu's own clicks.** The current handler is
   ```ts
   if (!wrapRef.current?.contains(e.target as Node)) setOpen(false);
   ```
   Once the panel is portalled it is no longer inside `wrapRef`, so pressing a menu item fires
   `mousedown` → "outside" → `setOpen(false)` → the panel unmounts → the `click` never lands on it
   and **the action silently does nothing**. Keep a second ref on the panel and treat *either*
   container as inside.
2. **Escape will stop working.** `onKeyDown` is on the wrapper `<div>`; a portalled panel is
   elsewhere in the DOM, so its key events do not bubble to it. Move Escape to a `document`
   `keydown` listener while open.

## Acceptance criteria

- **Regression test first, and it must fail on today's code.** An e2e that opens the file menu on the
  **last** file row of a song with several files and asserts the panel is both visible and
  *actionable* — click a real item (e.g. rename) and assert its effect, not merely that the node
  exists. A `toBeVisible()` alone can pass on a clipped element; assert the panel's bounding box lies
  within the viewport, and click through it.
- **Teeth-check:** re-add the clipping (or revert to `position: absolute`) and confirm the new test
  goes red. Record that you did.
- Both traps above covered by tests: clicking a menu item performs its action (trap 1); Escape closes
  the panel (trap 2); clicking genuinely outside still closes it.
- Menu still closes after an action fires (the `close` render-prop contract is unchanged).
- Keyboard/ARIA unchanged: `aria-haspopup`, `aria-expanded`, `role="menu"`, `role="menuitem"`.
- **Dangling-testid sweep**: `file-menu`, `file-menu-rename`, `file-menu-*` and every other `RowMenu`
  testid must survive; no `data-testid` removed from `web/studio/src` may keep a reference in
  `web/studio/e2e` or `web/studio/walkthrough`.
- `tsc -b studio` clean; **full `make e2e`** on the isolated ports (:8091/:5174) — not a subset.

## Out of scope

- Redesigning the Files list or the menu's contents.
- The other `RowMenu` call sites' *contents* — but the fix is in the shared component, so verify at
  least one non-file call site still opens correctly.
