# T04 — UI: simplify pages (progressive disclosure, lists, navigation)

**Priority:** 4 · **Size:** M · **Area:** `web/studio/src/pages/*`, `components/*`, `styles.css`

## Context

The management pages work but read as engineer-built forms, not a calm consumer tool.
Concrete problems, all verified against the running app:

1. **Creation forms are permanently expanded above every list.** `Bands.tsx` puts the
   "New band name + Create band" form *above* the bands list; `BandDetail.tsx` does the
   same for "Invite" (inside the Members card) and "Song title / Artist / Add song"
   (above the songs list). Tools like Google Drive/Docs put content first and creation
   behind a single **"+ New"** button; the form appears only on demand.
2. **The members list rows look broken.** Each row is a `space-between` flex of
   (avatar, name, @username, role-pill), which centers the name in the middle of the row
   with a huge gap after the avatar. It should read as one left-aligned identity block:
   avatar, then name with `@username` muted beside/under it, actions and role at the right.
3. **Section navigation is naked links.** Band page has bare `Setlists Settings` links
   under the title. These are top-level sections of a band and should be visually
   distinct (tab strip or buttons in the page header row).
4. **The invite control leaks internals.** The identifier-type `<select>` offers
   `username / email / uuid` — no end user should ever be shown "uuid". Default to a
   single text field ("username or email") and auto-detect; keep uuid acceptance
   server-side only.
5. **Pill overload / inconsistent casing.** Lowercase pills (`admin`, `member`,
   `shared`, `personal`, `live`, …) look like debug tags. Keep pills only where they
   carry state a user acts on; render them in sentence case with the neutral chip tokens
   from T03.

## Changes

1. Add a small reusable disclosure pattern (e.g. a `NewItemButton`/inline component in
   `web/studio/src/components/`): renders a `+ New band`-style primary button; on click
   swaps to the inline form with autofocus + Escape/Cancel to collapse. Apply it to:
   - Bands page ("+ New band")
   - Band detail → Songs card ("+ Add song")
   - Band detail → Members card ("Invite member")
   Keep the exact submit flow and API calls; keep existing `data-testid`s on the revealed
   inputs/buttons so e2e specs keep working — specs must only gain an extra "click the
   + button" step where needed (update the specs accordingly, e.g. via a shared helper).
2. Restyle the member row as described (left-aligned identity block). CSS + small JSX
   reshuffle in `BandDetail.tsx`; no data changes.
3. Replace the bare Setlists/Settings links with a header tab row (CSS class, e.g.
   `.section-tabs`), highlighting the active section on those routes.
4. Simplify the invite control to one input; detect email by `@`, otherwise username.
   If the API requires an explicit kind, keep sending it — just derive it.
5. Pill sweep: sentence-case labels, neutral chip styling; drop pills that repeat what
   the row already says.
6. **Native widget accent** (T03 leftover): add `accent-color: var(--accent);` to
   `:root` in `web/studio/src/styles.css`. Native range sliders (opacity/width/text
   size) and checkboxes (Fill/Border, layer visibility) currently render the browser's
   default blue in both themes — this one declaration brings them onto the single
   accent. Verify in the song editor in light *and* dark mode.

## Acceptance criteria

- `make demo` walkthrough as `marie`: bands page shows the list first with a single
  "+ New band" button; band page members render as tidy left-aligned rows; songs card
  content-first; invite is one field + button; Setlists/Settings are visually tabs.
- No creation flow lost: create band, add song, invite each still work end-to-end.
- In the song editor, the style sliders and checkboxes render in the accent color, not
  browser-default blue (check light and dark).
- `make e2e` green after spec updates; every changed spec's diff is limited to the new
  disclosure step / selector location, not to assertions about outcomes.
- TS typecheck green.

## Out of scope

- The song editor page (T05).
- New features (search, sorting, avatars redesign).
- Backend/API changes.
