# T206 — Concert page: "Running order" | "Sound sheet" as tabs, with a shared header and the exports where they belong

Owner: web-core. Spec: Fable, 2026-10-26. Small UI. **After the RED-MAIN fix.**

## Why (VLL, relayed by troubastack-8b)

> "the button for the concert sheet is bogus, it should probably be a tab and not a link just next to a button, try
> to improve the design"

## What exists (read on main)

- `SetlistDetail.tsx` (~168–190): the header actions hold an "Export PDF" `<button>` (the T158 running-order PDF)
  and, next to it, a react-router `<Link className="secondary">` "Sound sheet" to `…/setlists/:s/sheet`. They look
  different.
- The wrapper is `phead-actions`, but `styles.css` (~1932) styles only `.phead .head-actions`, so the row is unstyled.
- `SoundSheet.tsx` (~135–138) has its OWN header (a "← Concert" crumb and the h1 "Sound sheet — <name>") instead of
  the concert's.

## Design (troubastack-8b's proposal, accepted with rulings)

1. **One concert header on both routes.**
   - Factor the concert `phead` (name, date and venue, chips: on call, archived, edited-since-bake) into a component
     used by both the running-order route and the sheet route.
   - `SoundSheet` drops its own crumb and h1.
2. **A route-based tab bar under the header:** "Running order" (the current page) | "Sound sheet".
   - Use `NavLink`s, so `/sheet` stays deep-linkable and the browser Back button works.
   - Use **underline tabs**, in the `.drawer-tab` family, not the `.details-tab` pills.
   - `aria-current` on the active tab; keyboard reachable.
3. **The exports live in the tab they export:**
   - **Running order tab:** the running-order PDF (T158), plus the existing bake controls (Bake, Download .tstage,
     and the member-scoped concert PDF dialog from T202).
   - **Sound sheet tab:** its existing PDF / CSV / order toggle / patch controls (T203).
   - Nothing is left floating in the shared header except page-level state (chips).
4. **Fix the unstyled actions row:** a single class name, styled. Either rename `phead-actions` to what the CSS
   targets, or style the new class; one rule, not two.
5. **Labels are product words** (docs-stage list, VLL's call):
   - "Running order PDF" (today "Export PDF");
   - the T202 "Download PDF" / "Print for…";
   - "Sound sheet PDF" / "CSV";
   - the tab names.

   Propose them; don't settle them.

## Tests

- **e2e:**
  - the tab bar shows on both routes;
  - clicking "Sound sheet" changes the URL to `/sheet` and marks the tab active;
  - Back returns to "Running order";
  - a direct load of `/sheet` shows the shared header and the active Sound sheet tab;
  - each tab shows only its own exports.
- **Update the affected specs:** `sound-sheet.spec.ts` (the `sound-sheet-link` test id becomes the tab), and the
  sheet-title checks in `sound-sheet.spec` and `lineups-sticky.spec` (the h1 is now the concert's).
- **Sticky headers (T203-FIX3/4):** the tab bar must not break the sticky grid header offset (`--topbar-h`). Either
  the tabs scroll away normally, or they become part of the sticky offset; say which, and test that the sheet's
  header still sits right under the topbar after scrolling.

## Labels settled by VLL (2026-11-01, "Jeu B": the musicians' words)

| Where | Shipped | Settled |
|---|---|---|
| Eyebrow above the concert name (`ConcertLayout`) | "Setlist" | **"Concert"** |
| Tabs (`ConcertTabs`) | "Running order" \| "Sound sheet" | **"Setlist" \| "Sound sheet"** |
| The running-order tab's PDF button (`SetlistDetail`) | "Running order PDF" | **"Setlist PDF"** |
| Sound-sheet exports (`SoundSheet`) | "PDF" / "CSV" | **"Sound sheet PDF" / "Sound sheet CSV"** |

- Unchanged: T202's "Download PDF" / "Print for…".
- The in-tab heading `<h2>Running order</h2>` (`SetlistDetail`) is not in the set. If it reads as a duplicate of
  the tab under the new name, say so at the gate rather than renaming it on your own.
- Update the comments that quote the old words (ConcertTabs, ConcertLayout's binding 2 note, SetlistDetail,
  styles.css) and the e2e text assertions. Keep the testids.

## Follow-up settled by VLL (2026-11-01): the in-tab heading, and its count

- `SetlistDetail`'s `<h2>Running order</h2>` becomes **"Main set"**. It pairs with "★ Bench · on call" below it.
- **The count next to it counts only songs.** Today it is `main.length`, which includes intermissions: the demo
  concert shows "5 songs" for 4 songs and 1 intermission. It becomes "4 songs · 1 intermission", using
  "intermission", Studio's own word for the kind, singular or plural as needed, and leaving out the intermission
  part when there is none ("4 songs").
- Tests: a pure count function (songs vs intermissions, the 0, 1 and n cases), with a sabotage; the e2e asserts the
  count text on the demo concert.

