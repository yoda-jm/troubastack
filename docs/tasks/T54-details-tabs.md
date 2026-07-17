# T54 — Song Details panel: tabs by audience (Band / Mine / Admin)

**Priority:** normal (VLL 2026-07-17; scheme A part 2) · **Size:** studio S · **Area:**
`web/studio` (song-editor Details panel). Presentation/IA only — no API/model change.

## Ruling (Fable, docs/handoff/reviews.md 2026-07-17 + docs/design/09-global-vs-personal-ia.md)

Global-vs-personal legibility scheme **(A)**, T54 = **(ii) tabs by audience**: the
audience split becomes the tab structure, solving both the illegible global/personal
mix AND the long-scrolling Details modal (VLL's phone pain) in one move.

## Design

The editor Details panel (`details-panel`, opened by the Details pill) becomes tabbed:
- **"Shared with the band" 👥** — `Metadata` + `Files` (the shared pool: upload / ＋ new
  text chart / manage).
- **"Just for you" 👤** — `MyFilesEditor` (personal selection/order) + `MyCuesEditor`.
- **Admin** — `DeleteSong` (admin-only). Rendered only for admins; last. (Kept as a tab
  so the destructive action is out of the way; a footer action was the allowed
  alternative — tab chosen so delete isn't ever-present under the other tabs.)

Requirements (Fable's workflow-trap pin):
1. **Mine's "My files" is self-sufficient:** its add-from-pool picker lists the FULL
   current pool including files uploaded moments ago under the Band tab. Achieved by
   mounting only the active tab's content, so switching to Mine re-fetches the pool.
2. **Upload = Band concern; selection/ordering = Mine concern** (the audience rule).
3. **Tab state remembered per session** (`sessionStorage`); default = Band (metadata is
   the commonest first read). A non-admin never lands on Admin.
4. **Headers carry 👥/👤;** existing section `data-testid`s preserved where sections
   moved — e2e updates to navigate tabs, assertions unchanged.
5. **Phone (412px) is the win:** no long scroll inside any single tab.

## Acceptance

- Details opens on the Band tab (Metadata + Files); My files/My cues live under Mine;
  delete under Admin (admins only). Tab choice persists across close/reopen in a session.
- The Mine → My files picker shows a file uploaded under Band without a reload.
- Existing editor e2e (lyrics-import, song-cues, files-delete, song-details) pass after
  tab-navigation updates; reorder/cues/files behaviors unchanged.
- `tsc -b` clean; pixels light+dark at the gate (both tabs, phone width).

## Out of scope

T55 (zone-at-draw-time chip) + the Band/Mine chip sweep (scheme A parts 3) — separate.
