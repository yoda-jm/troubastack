# T211 — Line-ups that fill the screen, on desktop and tablet, mouse or finger

Spec: Fable, 2026-11-14. Lane: **web-core (9c)**, three stages, each presented on its own. Mobile (1d) checks
stage 1 in the app's WebView before it lands.

The look and behaviour are settled with VLL on a working prototype:
- **Reference:** `docs/design/lineups-a-prototype.html`. Open it in a browser and resize the window.
  - The names and songs in it are invented.
  - The dashed "Prototype" box only switches the example band size; it is not part of the design.
- **Rule:** where this spec and the prototype disagree, this spec wins. Otherwise, build what the prototype does.

## Why (VLL, 2026-11-14)

> "look at this: …/lineups no well rendered on desktop, not easy to browse, should make sense in both tablets and
> desktop to display and edit, with or without touch to navigate"

Then, choosing between three directions:

> "I clearly prefer A, maybe a popup modal when clicking on an edit icon before the song name can be nice"

VLL asked me to decide where the scrollbars go. My decision is below, and he asked for this task with it.

Later rulings from the review of the prototype:
> "it misses the edit part/edit musician things, where I can swap colors, add instruments"
> "voice round chips with None written in the middle and 2 small arrows in the left and right … clicking in the
> middle you have a dropdown"
> "in the table click to switch for the voice is enough, chip is only for the detailled view"

## What is wrong today (measured 2026-11-14 on a copy of the :8080 data)

The band measured has 5 musicians (1 hidden), 13 parts and 48 songs, so the grid shows 15 sub-columns. It was
measured with the deployed build (3890545f) in Chromium.

| Viewport | Grid box width | Table width | Rows visible at load | Box bottom (its horizontal scrollbar) |
|---|---|---|---|---|
| 1280×720 | 912 | 1132 | 7 | 1000, off screen |
| 1440×900 | 912 | 1132 | 12 | 1180, off screen |
| 1920×1080 | 912 | 1132 | 17 | 1360, off screen |
| 800×1280 tablet | 704 | 1132 | 23 | 1558, off screen |

1. **The page is a 1040 px column.** `.page` has `max-width: var(--maxw)` (1040px). On a 1920 px screen the grid
   gets 912 px for a 1132 px table: the last musician is cut off and you scroll sideways next to 1000 px of empty
   margin.
2. **401 px of crumb, tabs, title, filter, buttons and hidden bar** sit above the first row.
3. **Two scrolls, one inside the other.**
   - The box's bottom edge, and with it the horizontal scrollbar, is below the screen at load, at every size.
   - The wheel scrolls the songs inside the box.
   - To scroll sideways you must first scroll the page (344 px).
4. **The song column is 315 px wide** (28% of the table), set by one 46-character title.
5. **Each musician header carries four controls** (◀ ▶ hide + part).
6. **83% of the cells are "·"** (599 of 720). With 2.3 musicians a song across 15 columns, who plays what is lost
   in the dots.

## Stage 1 — the frame and the grid (web/studio only)

### The frame: the grid owns the window
- **Full width, full height.** On the Line-ups route only, the band page drops its 1040 px cap. The route becomes
  a column exactly as tall as the window below the top bar (`100dvh - var(--topbar-h)`; in the app's WebView,
  where there is no top bar, the whole WebView). **The page itself never scrolls**, on any size.
  - Other band tabs keep today's layout.
  - The full-bleed editor routes are untouched.
- **One compact header row:**
  - "← Bands" and the section tabs share one row.
  - The visible "Line-ups" `<h1>` goes, because the active tab names the page. Keep an `<h1>` for screen readers
    (visually hidden) with `data-testid="lineups-title"`, so the existing specs still find it.
- **One toolbar row** (it wraps on a narrow screen): the filter (it takes the free width), By part, + musician,
  Parts, and the hidden chip.
  - The T210 "Hidden: ‹name› ↺" bar becomes a chip in this row. Keep its test ids (`lineups-hidden-bar`,
    `lineups-hidden-name`, `lineups-unhide`).
  - Filter chips and suggestions keep working; chips may take a second row.
- **"Jump to" chips** (one per visible musician, in the musician's colour) appear **only when the grid is wider
  than its box**. A click scrolls the box so that musician's header sits just right of the song column. Respect
  `prefers-reduced-motion`.
- **The grid box takes all the remaining height** (`flex: 1; min-height: 0; overflow: auto`). This replaces
  T210's `max-height: calc(…)` for this page.
  - `LineupScroll` gains a fill mode.
  - The Sound sheet keeps T210's bounded box: it is out of scope here.
  - T210's touch fix must survive: the page stays as wide as the screen, and the box is what scrolls.
- **The Parts drawer**, until stage 3 replaces it, may sit between the toolbar and the grid. Cap it (`max-height:
  40vh; overflow: auto`) so the grid keeps room.
- A thin status line at the bottom says what the grid holds and how to use it (see the prototype). It is optional;
  drop it if it costs you.

### Scrollbars (my decision, at VLL's request)
- **Inside the grid box, never on the page.** On a tablet, a page wider than the screen breaks the frozen header:
  that is the T210 bug.
- **Right and bottom only**, the convention of every spreadsheet (Excel, Google Sheets, Numbers, Airtable).
  - No duplicate bar at the top: that is a workaround for tables that run past the bottom of the screen, and this
    one never does.
  - No bar on the left: that is for right-to-left languages only.
- **Only beside what scrolls.** The vertical track starts under the frozen header; the horizontal track starts
  right of the frozen song column.
  - On `pointer: fine` only: style the box's `::-webkit-scrollbar` with Studio tokens (both themes), with
    `::-webkit-scrollbar-track:vertical { margin-top: <header height> }` and `:horizontal { margin-left: <song
    column width> }`. The prototype sets the two values from JS as CSS variables after layout.
  - Chromium (Edge, WebView2, Android WebView) and Safari honour this. Firefox keeps its full-length bar, which is
    fine.
  - Do not set `scrollbar-width` or `scrollbar-color` on the box: either one makes Chromium ignore the
    `::-webkit-scrollbar` rules.
- **Touch screens keep their thin, fading overlay bars.** Three edge shadows stand in for them, and also show on
  desktop:
  - one at the right edge while columns remain to the right;
  - one just right of the song column once you have scrolled sideways;
  - one under the header once you have scrolled down.

  They are `pointer-events: none`, update on scroll, and sit clear of the scrollbars.

### The grid
- **The song column has a fixed width** (prototype: 248 px with a mouse, 210 px with a finger, 172 px under
  860 px). Long titles end in "…", and the full title shows on hover (`title`).
  - In stage 1 a ✎ button sits before each title. Stage 2 wires it.
  - A single title must never again set the column's width.
- **Empty cells are empty: no "·".**
  - A played cell is a rounded tag in the musician's colour, showing the same text as today (`L`, `B`, `L+B`, the
    part's short, or the group's carried shorts).
  - Rows alternate a faint stripe.
  - A 2 px line separates musicians.
  - On `hover: hover`, an empty cell shows a faint "+", and the row under the pointer is highlighted.
- **Musician colours.** Each musician has a colour from a fixed palette of 8 (prototype tokens `--m0`…`--m7`, each a
  background/ink pair for light and for dark).
  - **Stage 1** derives the colour client-side: musicians in display order take the palette in order, modulo 8.
    No data change. Stage 3 makes it a saved, swappable choice.
  - The musician's header cell, their tags, the Jump-to chip and the menus use it.
  - Every pair must pass 4.5:1 for the tag text in both themes. Show the numbers.
- **Musician header:** the name, then ⋯. The ⋯ menu holds:
  - Edit musician… (stage 1: opens today's instruments popup, `lineup-assign`);
  - ◀ Move left, Move right ▶ (keep `lineup-musician-left` and `lineup-musician-right`);
  - Hide everywhere (keep `lineup-hide`).

  Admin-only, as today. The menu must paint above the second header row: the prototype needed a z-index for that.
- **Clicking a cell switches it, exactly as today.**
  - A one-part column toggles.
  - A Voice column, or any column that holds several parts on one track, steps through T204's cycle (none → L → B
    → L+B → none, in general none → each part → all → none). VLL ruled: in the grid, a click is enough. No arrows
    and no list in the grid.
  - The keyboard stays as it is: the arrows move (M4, with `revealInLineupScroll`); L/B set the voice, the same key
    again clears it, and Shift toggles membership.
- **Shared parts** keep their wide who-plays column and its select (`lineup-cell-shared`), styled to fit the new
  look.
- **By part** gets the same frame. Its own column layout is unchanged.

### Stage 1 targets (desktop on chromium; tablet on the `mobile` project)
Seed a band of the measured shape:
- 4 visible musicians with 15 sub-columns, plus 1 hidden;
- 48 songs, 2–3 players a song;
- one title of at least 46 characters.

Then a wide band: 7 visible musicians, 25 sub-columns.

| Check | Target |
|---|---|
| The page never scrolls | `documentElement.scrollHeight <= innerHeight` and `scrollWidth <= innerWidth`, at every size below |
| The box ends on screen | the box's `getBoundingClientRect().bottom <= innerHeight` at load, at every size |
| No sideways scroll when it fits | measured shape at 1440×900 and 1920×1080: box `scrollWidth <= clientWidth` |
| More songs visible | measured shape: ≥ 15 rows at 1280×720, ≥ 20 at 1440×900 (today 7 and 12) |
| Song column | its width doesn't change with a 46- versus a 10-character longest title |
| Empty is empty | no cell's text is "·" |
| Wide band at 1440×900 | box `scrollWidth > clientWidth`, right shadow on, Jump to visible; Jump to the last musician puts its header's left edge at the song column's right edge (±2 px) |
| Shadows | after scrolling right: left shadow on; at the far right: right shadow off; after scrolling down: top shadow on |
| Tablet 800×1280 (`mobile` project, real CDP touch drag as in T210) | `innerWidth` stays 800; a drag inside the box moves `scrollTop`, `scrollY` stays 0, and the header stays at the box's top |

**Sabotages, each red on its own assertion:**
- put back `max-width: var(--maxw)` on the route: the 1440 no-sideways check;
- put back T210's `max-height: calc(…)` instead of the fill: the box-ends-on-screen or rows check;
- show "·" in empty cells: the empty check;
- drop the `scrollLeft > 0` condition on the left shadow: the shadows check;
- show Jump to unconditionally: the Jump to check on the measured shape.

**Present with screenshots taken with real scrollbars** (Playwright hides them by default:
`ignoreDefaultArgs: ['--hide-scrollbars']`): 1440×900 and 1920×1080 with the measured shape, 1440×900 with the
wide band at load and scrolled, dark theme, and the tablet. Fictional names only.

**Existing specs** (`lineups*.spec.ts`, `lineups-hidden.spec.ts`, `lineups-sticky*.spec.ts`):
- update them to reach the controls through ⋯ where they moved;
- do not weaken an assertion;
- every behaviour they covered stays covered.

**Mobile (1d) checks stage 1 in the app** before it lands, the way T210 A was measured: the Android emulator WebView
on the PR build, measuring `innerWidth`, the box rect, and a real touch drag. A desktop browser cannot see the
WebView bug class.

## Stage 2 — ✎ the song in detail

- **Opening and closing:**
  - ✎ before a title opens that song's line-up in a dialog: a native `<dialog>` with `showModal()`.
  - With a mouse it is centred; on `pointer: coarse`, or under 640 px, it is a bottom sheet (up to 78dvh).
  - Escape, Close, and a click on the backdrop close it, and focus returns to the ✎ that opened it.
- **Content:** "Song n of N" (N = the rows the filter shows), the title, then one row per visible musician.
- **Voice** is a round chip `‹ value ›`. The value sits in the middle: None, Lead, Backing, Both, or the part
  names when the band has other voice parts.
  - The arrows step through None → each part → all (Both), wrapping round.
  - A click or tap in the middle opens the list, with the current choice ticked.
  - The chip takes the musician's colour while they sing.
  - Arrows are ≥ 44 px wide with a finger.
  - The list must be a child of the dialog: a fixed popover outside a modal dialog is inert. The prototype moves
    it in.
- **Each instrument** of the musician is an on/off button (`aria-pressed`) showing the part's name and short.
- **Previous / Next** walk the filtered rows (disabled at the ends).
- Every change writes through the same `patchLineup` path as the grid, and the grid behind updates at once.
- The hidden-musician note from T210 appears here too (their parts stay saved).
- **Tests:**
  - ✎ opens the right song.
  - The right arrow takes the voice None → Lead, and after closing, the grid cell reads `L`.
  - The middle opens the list; choosing Both makes the cell `L+B`.
  - An instrument toggle round-trips through `GET /api/bands/:id/songs`.
  - Next lands on the next filtered song.
  - Escape returns focus to ✎.
  - `mobile` project: it is a bottom sheet, and the arrows' boxes are ≥ 44 px.
- **Sabotages:** swap the arrows' directions; mount the list outside the dialog (it no longer receives clicks);
  write to the wrong song index. Each reds its own assertion.

## Stage 3 — editing musicians and parts (core + web/studio)

### Musician colour becomes data
- **The fields:**
  - `Musician.Color` (`json:"color,omitempty"`), a palette key: `blue`, `orange`, `green`, `purple`, `rose`,
    `teal`, `amber` or `olive`. Any other value is refused with 400.
  - Musician PATCH takes `color`, admin-only like the other musician edits (403 otherwise).
- **The rule, one function in Go and its TS mirror:**
  - musicians in display order;
  - a saved colour wins;
  - an unsaved one takes the first palette key not saved by anyone, in palette order;
  - when the palette runs out, it wraps.
- **Swap:** setting a colour that another musician has (saved or derived) gives that musician the editor's old
  effective colour, in the same write. Both are then saved. One request, one store write: no client-side two-step.
- **Shared vectors:** the Go test and the TS test load the same file (`testdata/musician-colour-vectors.json`).
  Cover:
  - none saved;
  - some saved;
  - a swap with a derived colour;
  - more than 8 musicians;
  - a hidden musician (still holds their colour, so restoring them doesn't repaint anyone).
- `.tband` v2 carries `color` both ways, with a round-trip test. A file without it imports as nothing saved.

### The musician editor
Clicking a musician's name, or ⋯ → Edit musician…, opens a dialog (a bottom sheet with a finger). It holds:
- the name;
- the colour: 8 swatches, each showing who has it; picking a taken one swaps, and a line says so ("Swapped: Alex
  now has orange.");
- the instruments:
  - ✕ removes one: today's rule (refused while it is used; offer "Clear those N and remove") moves here;
  - "+ ‹part›" adds an instrument the band already has, or a new one from a name input (kind inferred as today);
- ◀ Move left / Move right ▶;
- Hide everywhere.

**+ musician** opens the same dialog empty, and the musician is created when it is saved. This replaces the "+ part"
popup.

### The Parts panel
Parts opens a panel on the right (a bottom sheet with a finger). The grid stays usable beside it. It holds today's
drawer controls:
- add a part (name, short, kind, shared);
- reorder (▲▼ and drag);
- the track select ("+ New track…" inline);
- the tracks list (rename, delete with the revert confirmation, the warnings);
- the merge notice.

Same rules and endpoints as today.

### Stage 3 tests
- **Go:**
  - an unknown key gets 400;
  - a non-admin gets 403;
  - a swap writes both musicians in one store write;
  - the vectors;
  - the `.tband` round trip.
- **TS:** the same vectors through the mirror.
- **e2e:**
  - rename shows in the header;
  - picking a colour another musician has swaps both headers' colours, and survives a reload;
  - removing a used instrument shows "Clear those N and remove" and clears exactly N cells;
  - adding an instrument adds its column;
  - + musician creates a column;
  - in Parts, putting two of a musician's parts on one new track merges their columns into one that cycles.
- **Sabotages:**
  - a client-side two-PATCH swap instead of the server's: the "one write" Go test is red;
  - drop the hidden musician from the colour rule: the hidden vector is red;
  - remove without the used check: the N-cells e2e is red.

## Out of scope, noted for later
- The Sound sheet in the same frame and colours. It is the obvious next step once this has landed. Ask VLL first.
- Musician colours anywhere outside Line-ups (sheet, PDF, tablet).

## The gate
- Each stage is presented on its own: a `reviews.md` entry, a PR run with every gating job, the sabotages shown
  red, and screenshots.
- Web/studio-only stages need no Windows job. Stage 3 touches core, so `go` must be green, plus `web`.
- 1d checks stage 1 in the emulator before stage 1 lands.
- After each land, refresh :8080 (both stamp fields).
