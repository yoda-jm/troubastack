# A73 — The note bar: a checkmark for the exit, and the strip's wording pass

**Lane:** mobile · **Status:** specced 2026-09-14, never started; **reactivated 2026-10-02** (VLL: *"Clear page, garde ce mot"*), with ⟨D6⟩ added · **Origin:** VLL relaying **a performer's** feedback,
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

## 5c. ⟨D6⟩ In two-up, "Clear page" must clear a page that has a note (added 2026-10-02)

Found by the mobile lane during the A78 device pass (gate, 2026-09-30). In a two-up spread **both** pages are
editable, but `onClear` clears `state.pages[state.current]`. A note drawn on the *other* page is never
cleared: the dialog closes and the note stays. Hand-erasing does not rescue it either, because the eraser
leaves a faint anti-aliased fringe (18 px at alpha ≤ 29/255 measured), so the empty → delete rule never
fires. Today such a note can only be removed from the Notes tab.

**The rule:** "Clear page" acts on a visible page **that carries a note**.
- **One page of the spread has a note:** clear that one. The dialog is unchanged.
- **Both pages have a note:** the dialog asks which. Title **"Clear which page?"**, one button per page
  naming it (**"Page 3"** / **"Page 4"**, the numbers the reader sees), plus Cancel. No "both" button: one
  note at a time is the safe default for an action that cannot be undone. If clearing both turns out to be
  wanted, that is VLL's call.
- **Neither has a note:** the button is disabled. Today it opens a dialog that then does nothing, which is
  the same silent failure in another form.
- Single-page view: unchanged, the current page.

Whichever page is cleared, keep A77's armed mirror exactly as it is: it receives the **cleared** page's entry.

**Not in scope:** the eraser's fringe. "Clear page" is the reliable way to remove a note, and fixing it makes
the fringe harmless. Lowering the empty threshold is a separate decision with its own risk: a faint real mark
would vanish.

## 6. Acceptance

- The exit is a checkmark, still visually the primary control on the strip, and still the only exit.
- **A device row, and it is not optional here.** Enter note mode on the tablet, draw, then leave **using only
  the new glyph** — no adb, no back gesture. A seam test proves the click handler and is blind to whether a
  performer recognises the mark, which is the entire question ⟨D1⟩ turns on. Report it as seen.
- The refusal string appears in Scroll and names both working modes.
- The destructive button reads "Clear page"; its dialog says "Clear this page?" / "Clear", and its body still
  says "Deletes the whole note on this page". The code keeps `onClear`/`confirmClear` (⟨D4⟩ struck).
- No test still matching on "Done" or on "Erase note".
- ⟨D6⟩ **device row in two-up:** draw on the **non-current** page, tap "Clear page", and the note is gone
  (index entry removed, ✎ badge gone). This is the exact case that failed. Then draw on both pages: the dialog
  asks which, and clearing one leaves the other. A pure test picks the page from (spread, which pages have
  notes); the device row proves the page the reader sees is the one cleared.
- Line numbers above (`:1900`, `:754`, `:186`) date from 2026-09-14; find the code by name (`NoteBar`,
  `onClear`, the refusal string), not by line.
