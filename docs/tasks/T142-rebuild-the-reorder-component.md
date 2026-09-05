# T142 — Rebuild `SortableList` for touch: pointer events, an end position, auto-scroll, real feedback

**Lane:** web-core (studio). **Size:** M. **Status:** IN PROGRESS 2026-09-05 (web-core). Stage 1 DONE:
the N+1-gap primitive `reorderTo` + the RED-first flagship test (`test/sortable-reorder.test.ts`) — an
item can be dropped at the END, and the old top-edge `reorder` provably cannot reach it. No behaviour
change yet (the primitive is not wired), so zero risk to the live studio. Stage 2 (the Pointer-Events
`useSortable` rewrite: N+1 drop wired to `reorderTo`, edge auto-scroll, keyboard pick-up/move/drop + ARIA
live, focus-restore on arrow move, `touch-action`/`user-select` on the grip, input-aware indicator) is
DRAFTED (302 lines) + unit-testable with mocked geometry, but it changes the 3 call sites and BREAKS the
existing `dragTo` e2e specs — so it must be completed + **Playwright-verified in a browser** before landing
to the studio VLL uses in rehearsal; presented at the gate. Plus the UX baseline VLL asked for.

## The four defects, each confirmed in the source

`SortableList.tsx` (150 lines) is built on **HTML5 drag-and-drop**, a desktop input model. It backs the
setlist, a song's files and my-files — so one rewrite fixes every reorder surface.

| VLL's words | what the code says |
|---|---|
| *"on ne peut pas deplacer un morceau en dernier"* | `reorder` lands an item **above** the target row — "the drop hint is the row's top border". There is no row after the last, so **no drop position exists at the end**. The arrow reaches it (`canMoveDown` allows `length-1`); the drag cannot. |
| *"la fenetre ne scroll pas quand on est tout en haut ou tout en bas"* | no `scrollBy`/`scrollTop` in the component. HTML5 DnD does not auto-scroll a container; it has to be implemented. |
| *"les fleches repositionne ou on se trouve dans la page"* | nothing calls `focus()`. The moved row re-renders, focus falls to `<body>`, the browser scrolls. |
| *"en tactile une fois sur deux ca faisait une selection du texte du titre"* | no `user-select: none` / `touch-action` on the grip or row: a touch not recognised as a drag becomes a text selection. |

## What the field says (VLL: *"documente-toi sur comment font les gens"*)

**The drop indicator is the single most important affordance** — the live preview of where the item lands
if released now. Without it every drop is a gamble.

**On the ghost-vs-line question, the answer is "both, by input type":**

- **Insertion line** — better where items are small and the pointer is precise (mouse). Less frustrating,
  more accurate.
- **Ghost / semi-transparent preview at the insertion spot** — the popular choice **on mobile**, because
  finger motion is crude and targets are large.

TroubaStack has both inputs — a phone in rehearsal and a desktop for editing — so **pick the affordance
from the pointer type**, rather than choosing one and disappointing the other. That is the direct answer
to VLL's *"certains composants ont la ligne en semi-transparente"*: it is the right pattern **for touch**,
and the insertion line remains right for the mouse.

**Also established, and each maps to a defect above:**

- **Move the background items out of the way** before release — previews the outcome and makes the motion
  read as physical.
- **Keep the source context**: a gap or dimmed original where the item came from, so the list stays
  legible and an abort is obvious.
- **Never make dragging the only way.** Reorder-by-drag-only excludes keyboard users, touch users on
  cramped screens, and assistive tech. **The existing up/down arrows are the right instinct** — keep them,
  fix their focus behaviour.
- **Keyboard**: Space/Enter to pick up and drop, arrows to move, with an ARIA live region announcing the
  new position.
- **Undo** turns a drag from a high-stakes gesture into a safe one, and costs almost nothing.
- **Auto-scroll** near the container edges — the named, common failure on long lists, and exactly what VLL
  hit.

## Implementation direction

**Pointer Events**, one code path for mouse, touch and pen, replacing HTML5 DnD. Then: a drop-zone model
with **N+1 positions** for N rows (the missing end position falls out of this), `touch-action: none` and
`user-select: none` on the handle, edge auto-scroll, and arrow moves that **restore focus to the moved
row** so the page does not jump.

## Acceptance — RED FIRST (VLL)

- **An item can be dropped at the end.** Write it against today's code and watch it fail; this is the one
  that cannot pass by accident.
- Dragging near the top/bottom edge scrolls the container.
- After an arrow move, focus is still on that row's arrow, and the scroll position has not jumped.
- A touch-drag on the handle never selects text.
- Keyboard: pick up, move, drop, with the position announced.
- The three call sites (setlist, song files, my-files) all still reorder — one component, three surfaces.

## Sources

- <https://www.nngroup.com/articles/drag-drop/>
- <https://smart-interface-design-patterns.com/articles/drag-and-drop-ux/>
- <https://www.pencilandpaper.io/articles/ux-pattern-drag-and-drop>
- <https://www.saasui.design/blog/saas-drag-and-drop-reordering-ux-patterns>
- <https://blog.julik.nl/2022/10/drag-reordering>
- <https://www.smashingmagazine.com/2018/01/dragon-drop-accessible-list-reordering/>
