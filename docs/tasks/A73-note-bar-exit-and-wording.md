# A73 — The note bar: a checkmark for the exit, and the strip's wording pass

**Lane:** mobile · **Status:** specced, not started · **Origin:** VLL relaying **a performer's** feedback,
2026-09-14: *"in the rehersal note mode in stage, a user prefer a checkmark instead of a done to end the
rehersal mode"*.

Worth recording that the provenance is a **user who is not the author**. That is the rarest kind of feedback
we get on Stage and it arrived about the one control that cannot be got wrong (§2).

Four pending items sit on the same strip. One pass, one device check.

## 1. The bar today

`StageScreen.kt:1900` `NoteBar` — `✎ ⌫ · three widths · four colours · [spacer] · "Erase note" · [Done]`.
Left half is glyphs, right half is words.

## 2. ⟨D1⟩ The exit becomes a checkmark — and keeps its emphasis

Replace the word with a checkmark. **Keep it a filled `Button`.** This is the load-bearing half of the
decision: `StageScreen.kt:754` records that in note mode the ✕/⚙ chrome is deliberately hidden *because*
"exit is the note bar's Done" — so **this control is the only way out of a mode in which touch draws instead
of turning pages.** Demoting it to a bare chip like `✎`/`⌫` would make it read as a fourth tool; a performer
who cannot find it is stuck mid-song. Glyph in, prominence unchanged.

**The glyph is the lane's to pick**, against a property rather than a codepoint: it must read unmistakably as
*finish* at arm's length, on a dark stage, at the size the neighbouring chips use. A thin `✓` on a filled
container often does not; a heavier mark or a Material `Check` may. **Prove it on the device, not in the
IDE** — see §6.

## 3. ⟨D2⟩ A bare glyph has no accessible name — give it one

Today the label *is* the accessible name. A checkmark alone leaves the sole exit unnamed for TalkBack and
for every automated check that finds the control by text. Set a content description, and **update any test
that locates this button by the string "Done"** rather than deleting the assertion.

## 4. ⟨D3⟩ D20 — the refusal names one of the two modes that work

`StageViewModel.kt:186` refuses with **"Notes: switch to page mode"**. The guard is
`fitMode == FitMode.SCROLL`, so notes are allowed in **Page and Width** both; the string names one and
lowercases it besides. Use **"Notes: switch to Page or Width"**. Verified against the guard, not the prose.

## 5. ⟨D4⟩ Fold in the Clear → Erase alignment

The UI settled on **Erase**; the code still says clear — `onClear`, `confirmClear` (`:881`, `:1900`, `:1902`,
`:1941-1946`) and the doc comment at `:1896` still lists "Done". Rename to match the words on screen and
refresh the comment. Mechanical, but this is the touch it was queued for.

**Not in scope: the "Erase note" button's own wording** (glossary D23 — the destructive full-note delete
sitting next to the ⌫ eraser *tool*). That is a product word and VLL's call; if his answer arrives before
this starts, fold it in, otherwise leave the string exactly as it is.

## 6. Acceptance

- The exit is a checkmark, still visually the primary control on the strip, and still the only exit.
- **A device row, and it is not optional here.** Enter note mode on the tablet, draw, then leave **using only
  the new glyph** — no adb, no back gesture. A seam test proves the click handler and is blind to whether a
  performer recognises the mark, which is the entire question ⟨D1⟩ turns on. Report it as seen.
- The refusal string appears in Scroll and names both working modes.
- No occurrence of `clear` left in the note-bar code path; no test still matching on "Done".
