# T207 — "L+B": one person sings lead AND backing in the same song

Owner: web-core. Spec: Fable, 2026-10-27. Small; no design gate, present the patch.

## Why (VLL, 2026-10-27)

> "for voice I need L+B for voice when you do both lead and backing depending on sections"

## What exists (read in code)

- The model already allows it: a line-up entry's `partIds` can hold both voice parts, and the grid
  (`lineup-grid.ts` `cellFor`) and the sheet (`sheet.go` `sheetCell`) already join the shorts with "+".
  `lineup-grid.test.ts` has a pure "L+B" case.
- **The click editor cannot produce it.** `Lineups.tsx` `cycleChannel` goes none → p0 → p1 → none, and a
  multi-selection resets to none. Only the keyboard reaches it (Shift-L then Shift-B, M4 binding 2), which a
  mouse or touch user never finds.
- **The joined text follows line-up ENTRY order**, not a fixed order. Backing set first, then lead, reads "B+L"
  in the grid and on the sheet. `sheet_changes.go` `voiceShortsByMus` compares those joined strings, so two
  consecutive songs with the same two voice parts entered in opposite orders print a false voice change
  ("A: L+B→B+L") on the engineer's sheet.

## Bindings

1. **Click cycle.** For a channel with two or more member parts, the cycle is none → each part alone, in the
   channel's member order → **all members together** → none. On a default band's Voice cell that is
   · → L → B → L+B → ·. One rule for every multi-part channel (T204 custom groups too); a single-part channel
   still toggles on/off. A partial multi-selection (2 of 3 parts) clicks to none, as today.
2. **Canonical join order.** A channel cell's text joins its parts in the **channel's member order** (the
   cycle order), whatever order they were entered in: grid, by-part projection, sheet screen, PDF and CSV. Go
   and Studio must produce the same string for the same data.
3. **No false change.** The changes column compares the musician's voice parts as a set (or in canonical
   order), so the same parts entered in opposite orders produce no voice-change line. A real change still
   prints ("A: L→L+B", "A: L+B→B").
4. Keyboard behaviour (M4 binding 2) is unchanged.
5. No label change. The cell shows the parts' shorts joined with "+", as now.

## Tests (each one teeth-checked by sabotage, the sabotage shown to have landed)

- **e2e** (extend `lineups-m4.spec.ts` or `lineups.spec.ts`): on a seeded band, four clicks on one Voice cell
  read ·→LV→BV→LV+BV→·, and the L+B state survives a reload (it reached the server). Sabotage: the old cycle,
  red at the third click.
- **Studio pure** (`lineup-grid.test.ts`): a line-up entered backing-first reads "L+B" (with the seed shorts).
  Assert that the fixture's entry order CONTRADICTS the channel order, so the test cannot pass by luck after a
  fixture edit. Sabotage: entry order, red.
- **Go** (`sheet_test.go`): the same fixture reads "L+B" in `sheetCell`, and two consecutive songs with
  {B, L} then {L, B} give no voice change line, while {L} then {L, B} gives one. Sabotage each half separately
  (the cell join, then the changes comparison).

## Out of scope

- A menu for other combinations of three or more voice parts (lead + harmony without backing). The keyboard
  Shift path still covers it.
- Tablet display of line-ups (T203 out of scope).

## Delivery

One commit, presented at the gate. Then a :8080 redeploy (VLL uses it).
