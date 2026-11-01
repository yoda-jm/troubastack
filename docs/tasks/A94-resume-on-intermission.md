# A94 — Stage: leaving a concert on an intermission card reopens it at the top

Owner: mobile. Filed by mobile, 2026-11-01, from the WIN01 device pass (Fable asked for the ticket). Small. Not
started; the spec needs Fable's read before code.

## What happens (emulator, main 0f7c6d68 and WIN01's head alike)

1. Open the demo concert. Turn to page 3: the "Interval" card (an intermission, T153).
2. ✕ to the concert list, then **Open** again: Stage opens on song 1, page 1 (the top), not on the card.
3. The same after process death (Home, `am kill`, relaunch): Stage comes back on the top, not the card.

Ending on a song page instead (e.g. page 4) restores that page in both cases. That is A46, and it works.

## Cause (read in code, checked on the demo bundle)

- An intermission has **no `songId`**: in the demo bundle the entry is `kind: "intermission"`, `label: "Interval"`,
  `songId` absent (`StageModel.kt` T153: "its [songId] may be empty").
- StageScreen still reports it as the position (`onPositionChange(here.songId, here.pageInSong)`), so the app stores
  `"#0"`.
- `resolveStartPage` returns **0** when `songId` is empty (`StageModel.kt`:
  `if (state.pages.isEmpty() || songId.isEmpty()) return 0`). Hence the top.

## The property to keep (the mechanism is the spec's to choose)

- Reopening a concert, and restoring it after process death, returns to the intermission card the performer left on.
- If the bundle has changed since (the card moved, or is gone), land on a page next to where it was. Never fall back
  silently to the top; the top is only for a position that cannot be placed at all, as today.
- Positions saved by older builds keep working. A song position resolves as today. A stored `"#0"` is ambiguous when
  a concert has several intermissions: the spec decides whether it means the first one or stays the top.

## Tests

- A pure test on the resolve step: an intermission position resolves to that card, with a sabotage (the empty-songId
  early return) shown to land.
- A changed-bundle case (the card removed or moved), and the old `"#0"` value.
- Device: the three steps above on the emulator.

## Not in scope

- How positions are written for songs (A46/A48), and the T153 intermission page itself.
