# T210 — Line-ups on a tablet: a header and song column that stay put, and hiding a player

Spec: Fable, 2026-11-13. Two parts, two lanes:
- **Part A** (the frozen header and song column) → **mobile (1d)**.
- **Part B** (hiding a player) → **web-core (9c)**.

Part A is a fix, so build it and present it. Part B is a new data model, and VLL has settled its behaviour (below).
Build B's core half first, then its Studio half once A has landed: both parts touch `Lineups.tsx`, `SoundSheet.tsx`
and `styles.css`.

## Why (VLL, 2026-11-13)

> "in stage, the studio part, when going to lineup of the band or of a setlist, the top (name/instrument) and the
> left (songs) are not anchored when you scroll, not easy to see what we modifiy, I also want the possibility to hide
> a player (if he's not here for example) in both general and setlist"

Asked whether hiding should be saved or only affect his screen, he answered:

> "everywhere for everybody is possible ? it is just as if he does't exist, but 1 click away of bringing him back ?"

## Part A — the header and song column must stay put on a touch screen (mobile)

### What exists, and why it fails

T203-FIX3 (4a3b6cee) made the Line-ups grid, the by-part view and the Sound sheet (`.lineup-grid`) sticky **against
the page's scroll**: `thead { position: sticky; top: var(--topbar-h) }`, and the corner and song cells use
`position: sticky; left: 0`. There's deliberately no overflow wrapper. `e2e/lineups-sticky.spec.ts` passes, but it
runs desktop Chromium and scrolls with `window.scrollTo`.

**Fable measured it** (headless Chromium, a static page with the same CSS: 14 musicians × 2 sub-columns, 60 songs):

| | layout viewport (`innerWidth`) | after a one-finger drag | header seen at |
|---|---|---|---|
| desktop 1280×800, wheel | 1280 | page `scrollY` 853 | top 0: stuck |
| tablet 800×1280, `isMobile` + touch | **2252** (the table's width) | visual viewport moved 831 px down, `scrollY` still 0 | **−745: off screen** |
| the same tablet, grid in an `overflow:auto` box with a bounded height | **800** | the box scrolls (`scrollTop` 295) | **at the top of the box: stuck** |

On a mobile engine, a page wider than the screen widens the **layout** viewport to the content. Sticky positions
against the layout viewport, which no longer scrolls, so the finger only pans the visual viewport across a page that
never scrolls. Both the header and the song column ride away together, as VLL describes. A desktop browser doesn't do
this, which is why the e2e is green.

### The change

- **Measure first, on the app:** the Android WebView on the emulator, through DevTools over adb. Use a demo rig on its
  own port with throwaway data, as for A99; never :8080's data, never the tablet.
  - Record `innerWidth`, `visualViewport.{width,scale,pageTop}`, `scrollY`, and the thead's rect after a touch drag,
    on the Line-ups grid and on a Sound sheet, **before and after**.
  - If the app shows something the emulation did not, stop and report.
- Wrap each `.lineup-grid` (the Line-ups grid, the by-part table, the Sound sheet) in **one shared scroll box**:
  - `overflow: auto`;
  - a bounded height that fills the screen under the topbar (`100dvh` minus `--topbar-h`; `dvh` works in the app since
    T151);
  - inside it, `thead { top: 0 }`. The corner and song cells keep `left: 0`, now relative to the box.
- **What must keep working:**
  - The **"+ part" popup** is `position:absolute` inside a header cell, and an overflow box clips it. It must open fully
    visible, at the right-hand column too. If the box clips it, render the popup outside the box (fixed, positioned
    from the trigger's rect); don't make the box's overflow visible.
  - **M4 keyboard navigation:** an arrow key onto an off-screen cell scrolls the box to it.
  - Column drag-reorder still works.
  - The page above the grid (controls, parts list) still scrolls normally. On desktop the box must not feel like a
    trap: the wheel chains to the page at the box's ends (no `overscroll-behavior: contain` on desktop unless you
    measure a reason).
  - **Windows app:** WebView2 behaves as a desktop engine. Check it once in the WIN08 smoke or say why not.

### Tests (each sabotaged)

- **Keep `lineups-sticky.spec.ts`,** rewritten to scroll the box rather than the window. Keep its FIX4 and FIX6
  assertions: the first Voice header keeps its tint and scrolls past the corner.
- **A NEW mobile project:** `isMobile` + `hasTouch`, 800×1280 at DPR 2, and a **real touch drag** (`Input.dispatchTouchEvent`
  through CDP; not `scrollTo`, not the wheel). After the drag, on both the Line-ups grid and the Sound sheet:
  - the thead's top is inside the visible area, at the box's top ±2 px;
  - the first song cell is at the box's left ±2 px after a horizontal drag;
  - `innerWidth` equals the viewport width: the page itself no longer widens.
- **Sabotage:** remove the box. The new mobile test must go **red** and the desktop test may stay green. Report both.
  That proves the new test sees what the old one could not.

## Part B — hide a player, for everybody, one click to bring them back (web-core)

### Behaviour (VLL's answer)

Two scopes, both **saved on the server and seen by every member**. Nothing is deleted: the song line-ups stay exactly
as they are, so bringing a player back restores everything.

- **Band (Line-ups page): "Hide" on a musician's header.**
  - That musician disappears from the band Line-ups grid, the by-part view, and **every concert's Sound sheet, its CSV
    and PDF, and Auto-number**, "as if he doesn't exist".
  - Line-up search **suggestions** skip him; a typed chip still matches the stored line-ups. Search reads data, not the
    view.
- **Concert (Sound sheet): "Not at this concert" on an owner header.**
  - The same, for that concert only: its sheet, CSV, PDF and Auto-number.
- **Bringing them back is one click, from where you are:**
  - Above the Line-ups grid: a bar **"Hidden: ‹name› ↺ · ‹name› ↺"**.
  - Above the Sound sheet: **"Not at this concert: ‹name› ↺"**, plus **"Hidden from the band: ‹name› ↺"** for the band
    scope. The label says it's band-wide, so a click from a concert page that un-hides for the whole band isn't a
    surprise.
  - The bar shows whenever anyone is hidden, for every member, so a missing column is never silent.
- **Who can change it:** the same people who can edit line-ups (admins, `isAdmin` in `Lineups.tsx`). The others see the
  result and the bar, without the buttons.
- **Copying a concert:** a duplicated concert does **not** copy the absences, because a new date means a new
  attendance. **Copy patch from…** doesn't copy them either.
- **The tablets:** unchanged. Line-ups never reach the bake or proto (checked: no `lineup` in `proto/` or
  `app/shared/commonMain`).

### Model

- `Musician.Hidden bool` (`json:"hidden,omitempty"`): band scope.
- `Setlist.AbsentMusicianIDs []string` (`json:"absentMusicians,omitempty"`): concert scope. An ID that's no longer a
  musician is ignored, never an error, and dropped on the next write.
- **API:**
  - the musician PATCH takes `hidden`;
  - the setlist PATCH takes `absentMusicians`;
  - both admin-only, as the line-up writes are.
- **`.tband` (bandio v2):** both survive an export → import round trip, with a round-trip test. An older file without
  them imports as nobody hidden.

### One filter, at the source, inside the shared projection

**The rule:** before projecting, remove the hidden or absent musicians **and their line-up entries**. Everything
downstream then follows with no special cases:
- their columns go;
- a shared part only they played in a song shows nobody in that song;
- a sub-column nobody uses in the set drops ("unused ⇒ not shown", T203);
- the changes column stops naming them.

- The filter goes **inside** the Go sheet projection (screen, CSV, PDF, Auto-number) and the TS `buildGrid`.
  - It must not go in a caller; a caller-side filter is how one of the four outputs drifts.
  - `testdata/lineup-sheet-vectors.json` gains cases that both sides load. Placeholder names only.
- **The vectors must include:**
  - a band-hidden player;
  - a concert-absent player;
  - both at once;
  - a shared part whose only player is hidden, in one song and then in the whole set, so the column drops;
  - a "changes" line that named the hidden player;
  - an absent ID that's not a musician.
- **Auto-number skips the absent players' channels.** Pin it: three channels, the middle owner absent, numbering
  1, 2 with no gap.

### Tests (each sabotaged)

- **Vectors:** move the filter into one caller → the other side's vectors go red.
- **The API:** a non-admin's PATCH of `hidden` or `absentMusicians` gets 403.
- **The round trip:** drop the field from the v2 writer → red.
- **An e2e test:**
  - hide from the Line-ups page;
  - the column is gone on the grid and on a concert's sheet;
  - the CSV has no column for him;
  - one click on ↺ brings him back everywhere.
  - Then the same for the concert scope, and check that a second concert is unaffected.

## The gate

- **Part A:**
  - the before and after measurement from the app;
  - both tests;
  - the box-removed sabotage, red on mobile.
- **Part B:**
  - the core half can be presented on its own: model, API, round trip, vectors;
  - the Studio half comes after A has landed.
- Neither part should touch `app/`. If one does touch `app/shared`, present it with the usual Windows PR run.
