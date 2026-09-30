# A78 — Notes tab: "Send all" and the collapsible concert come back; "Clear sent" empties the Sent group

**Lane:** mobile · **Status:** specced, not started · **Origin:** VLL, 2026-09-30: *"send all of rehearsal
notes has disappeared … at node level and top level, and you cannot collapse it anymore (not an
accordion)"*.

## 1. Cause: my A75 ⟨D1⟩ folded the control away with the header

A75 ⟨D1⟩ says a grouping level draws a header **only when it has two or more children**. The code implements
that (`renderSection` in `MainActivity.kt`: `if (multiBand)` / `if (multiConcert)`). But the header was
also **the only place "Send all" lived** (`GroupHeader(onSendAll = …)`), and the only thing that collapses.
A75's own table says so (*"concert header … holds a 'Send all' button"*), and ⟨D1⟩ folded it anyway. So:

- **Unsent notes in one band and one concert** → no header is drawn, so there is **no Send all anywhere and
  nothing to collapse**. That is VLL's ordinary case: one rehearsal's notes.
- **No top-level Send all has ever existed.** The widest bulk send was the band header, and it only appears
  with 2+ bands.

This is a spec defect, not an implementation one. The mobile lane built A75 as written.

## 2. ⟨D1⟩ A top-level "Send all (N)", always there when there is something to send

Put **one line above the list**, in the `Column` that already holds the status and offline lines, not as a
lazy item. It reads **"Send all (N)"**, where N is the number of **unsent** notes across every band and
concert. It sends them all through the existing bulk path (`onSendAll` → the identity check →
`launchBulk`), so A75's tested `NotesBulkOutcome` decision is reused, not duplicated.

- Shown when N ≥ 1. With N = 0 there is nothing to send, so no line.
- Offline: shown **disabled**, with the existing "Connect (Bakes tab)…" line under it. It does not vanish,
  because a control that disappears offline reads as the same bug.
- While sending: *Sending…*, as the header button does today.

## 3. ⟨D2⟩ The concert header is always drawn; the band header still folds

Amend A75 ⟨D1⟩ for **the concert level only**: the **concert header is drawn even when it is the only
concert**. It is the unit a rehearsal produces: it collapses, and it carries the node-level "Send all". The
band level keeps ⟨D1⟩ (header only with 2+ bands), and so does the song level.

The density cost is **one 48 dp row** here, plus the ⟨D1⟩ line, so **at most two rows**. A75 bought back
much more than that, and these are the rows that carry the controls VLL is missing. When the band header is folded away, fold the band name into the
concert header (*"[band] · [concert]"*), so the context is not lost.

The "Sent" group is unchanged. It is already a header, collapsed by default (A75 ⟨D3⟩).

## 4. ⟨D3⟩ "Clear sent": a bulk clear for the Sent group only (VLL, 2026-09-30)

VLL: *"we should not forget a 'empty trash' lookalike in the sent one (at top level? intermediate?)"*. **This
reverses A74's "no bulk clear"**, which was my ruling and not his. Its reason was that *the tablet cannot know
whether Studio recopied*, and it still holds. It is now carried by the confirmation below instead of by the
absence of the button. VLL knows his own workflow; a sent note is by definition safe in Studio.

**Where, both levels, mirroring "Send all":**
- on the **"Sent" header** (top level): **"Clear sent (N)"**, every sent note on this tablet;
- on each **concert header inside Sent** (always drawn, ⟨D2⟩): **"Clear"**, that concert's sent notes.
  With 2+ bands, the band headers inside Sent get it too, exactly where the unsent side has *Send all*.

**The word is "Clear", not "Empty" or a bin.** A74's point stands: these are not deleted and recoverable, so
a trash metaphor would lie. "Clear" is already VLL's word for removing a note (*Clear page*).

**Only the Sent group.** Never on the unsent side: an unsent note exists nowhere else, and clearing it in
bulk would destroy the only copy.

**It is local, always, even while auto-upload is on.** It removes the tablet copies only. **The Studio copies
are untouched in every state**, including inside an A77 window. A77's mirror-delete belongs to clearing a
**live page in Stage**, not to tidying this list. If the path you use to delete would reach
`ArmedUploader`, that is a defect.

**Confirm with the consequence in numbers:** *"Remove 7 sent notes from this tablet? Studio keeps its
copies. On Stage, these pages will no longer show them."* The second sentence is A74's reason, said to the
person who can judge it. On confirm, the status line says *"Removed 7 notes from this tablet"*.

## 4b. Not in this task

- No bulk clear on unsent notes (above).
- The leaf rows and the bulk-send outcome logic are unchanged.

## 5. Acceptance

- **One band, one concert, 2 unsent notes** (VLL's case): the top-level line reads *Send all (2)*, and a
  concert header is drawn with its own *Send all*. Tapping the header collapses and expands the notes. **This
  is the case A75 broke, so it must be the test's fixture, not the multi-concert one.**
- Two bands: the top-level N counts both, and each band header still has its own *Send all*.
- The top-level send goes through the same decision as a node send. Pin it by asserting that a foreign-identity
  note triggers the *Send under your account?* dialog from the top-level line too.
- Offline: the line is present and disabled.
- **Clear sent, local only.** With auto-upload **on**, *Clear sent* removes the tablet copies and makes **no**
  server call. Assert this on the transport (no delete request), not only on the list. A concert-level
  *Clear* removes only that concert's sent notes; an unsent note in the same concert survives. **The fixture
  must contain that unsent note**, or the test cannot fail.
- There is no *Clear* anywhere on the unsent side.
- **Device pass on VLL's tablet** (landscape, 686 dp), measured the way A75 measured it. A75's bar was
  **≥ 8 notes visible**. This task adds at most two rows, so the bar is **≥ 6**. Report the actual number. If it
  falls below 6, report it; do not shrink the 48 dp touch targets to make it fit.
