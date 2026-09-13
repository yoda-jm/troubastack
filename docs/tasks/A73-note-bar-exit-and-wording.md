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

## 5. ⟨D4⟩ — **STRUCK 2026-09-14 by ⟨D5⟩. Do not do this.**

It read: *the UI settled on Erase, so rename the code's `clear` to match.* ⟨D5⟩ moved the UI the other way,
which makes `onClear`/`confirmClear` the **correct** names. **Keep them.** Kept here as the record of why the
rename was queued and why it is not happening — the only thing still owed from it is refreshing the `:1896`
doc comment, which still lists "Done" (see ⟨D1⟩).

## 5b. ⟨D5⟩ The destructive button becomes "Clear page" (VLL, 2026-09-14)

Glossary D23 resolved. The full-note delete sitting beside the ⌫ eraser *tool* becomes **"Clear page"**.

**And this is a restoration, not a preference.** I offered it as the option that reintroduced a retired word;
the history says the opposite. A70 item 12 is VLL verbatim: *"no undo, no clear, to undo you use eraser, **to
clear page** you remove it from the 'notes' tabs in Stage"* — and A70's own prose glosses the Notes-tab
delete as *that IS "clear page"*. **"Clear page" is the spec's original word for exactly this action.** What
happened since:

1. `d796a475` added the button at VLL's direction, labelled **"Clear note"**.
2. `efa9d814` renamed it to **"Erase note"** while implementing A70 ⟨D5⟩/⟨D6⟩ — **a label change no spec asked
   for**, riding along in a commit about eraser *behaviour*. A70 §3.7's bar sketch has no such button at all,
   and ⟨D5⟩ is about the swept path and the shadow, nothing else.
3. That rename put the word on the same strip as the ⌫ eraser *tool*, which is the collision the glossary
   then filed as D23 — an open question put back to VLL about a word he had already chosen.

So the decision is not "VLL changed his mind": we drifted off his word twice and asked him to adjudicate the
result. Treat ⟨D5⟩ as authoritative and restored, and note the lesson for any future strip: **a label is a
product word — it does not ride in on a behaviour commit.**

Consequence: ⟨D4⟩ reverses (above), and the confirmation dialog must follow the button:

- title `"Erase this note?"` → **"Clear this page?"**
- confirm `"Erase"` → **"Clear"**
- **body text stays exactly as it is**: *"Deletes the whole note on this page — this can't be undone."*

That last line is not cosmetic and must not be "aligned" along with the rest. On Stage a **page** is a page
of music. "Clear page" on its own can be read as clearing the score; the body sentence is the only thing that
says what actually goes away, so it keeps the word **note** and keeps naming the scope. Any rewording that
drops "the whole note on this page" needs to come back through the gate.

## 6. Acceptance

- The exit is a checkmark, still visually the primary control on the strip, and still the only exit.
- **A device row, and it is not optional here.** Enter note mode on the tablet, draw, then leave **using only
  the new glyph** — no adb, no back gesture. A seam test proves the click handler and is blind to whether a
  performer recognises the mark, which is the entire question ⟨D1⟩ turns on. Report it as seen.
- The refusal string appears in Scroll and names both working modes.
- The destructive button reads "Clear page"; its dialog says "Clear this page?" / "Clear", and its body still
  says "Deletes the whole note on this page". The code keeps `onClear`/`confirmClear` (⟨D4⟩ struck).
- No test still matching on "Done" or on "Erase note".
