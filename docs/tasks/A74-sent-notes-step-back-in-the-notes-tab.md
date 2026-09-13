# A74 — Sent notes step back in the Stage Notes tab

**Lane:** mobile · **Status:** specced, not started · **Origin:** VLL, 2026-09-14, watching the send work on
the tablet: *"when something is sent to the server it can probably be moved in a recycled bin (still
displayed probably but it is less important than the one already sent) in the Notes tab in Stage"*.

Validates `docs/handoff/proposals/stage-notes-sent-recycle-bin.md`, whose clause-by-clause reading of that
sentence is correct and adopted: trigger = `sentAt` set; still displayed; de-emphasized.

## ⟨D1⟩ Call it "Sent". Not a recycle bin.

The proposal's own framing is the thing to change. A recycle bin means **deleted and recoverable**; these
notes are neither deleted nor finished — they have been sent, and their remaining job (recopy in Studio, then
delete) is **not done**. A bin invites *"empty the bin"*, which would read as finishing work that is still
outstanding. The section is **"Sent"**.

This is a product word and therefore VLL's to overrule; I am ruling it rather than asking because the risk is
a musician believing work is complete, which is the same class as the "Clear page" body-text carve-out in
A73 ⟨D5⟩.

## ⟨D2⟩ Dimmed inline under a "Sent" header — not collapsed by default

He wrote *"still displayed probably"*. Collapsing hides; dimming de-emphasizes while keeping it visible, which
is what the sentence asks for. Group under a header inside the existing band→concert→song→page tree.

## ⟨D3⟩ No bulk "empty" or "clear sent"

Refused, and this is the important one. **The tablet cannot know whether Studio has recopied a note.** A bulk
clear would destroy the copy the musician can still read on stage while the Studio underlay may be sitting
there un-recopied — and per T170 §7 nothing tells the tablet either way. The existing per-note Delete stays:
one note, one informed decision. If VLL later wants a sweep, it needs a signal from Studio that does not
exist yet.

## ⟨D4⟩ "Sent" is not sticky — and the mechanism already exists

Draw on a sent note and it returns to the actionable group. **No new predicate is needed:**
`AndroidRehearsalNotes.save` already sets `sentAt = null` on every save (`:87`), so an edit invalidates the
marker rather than being compared against it. Membership of the Sent section is exactly `sentAt != null`.

That is the same predicate the T170 §6 bulk-send skip must use, which is the point: **one fact drives both,
so the list and the send cannot disagree about what is outstanding.** Do not introduce a second notion of
freshness.

## ⟨D5⟩ The nag already agrees with this split — leave it alone

`NoteIndex.isOld` is `sentAt == null && bakesSinceTouched >= NAG_BAKES`, and `bumpBakes` only ages unsent
notes. So a sent note never nags, which is already consistent with it stepping back. The proposal asked how
the bin and the nag coexist; they already do, and the answer generalises: **the tablet nags what is unsent;
Studio surfaces what is unrecopied** (T173 ⟨D6⟩). Each surface nags only what its own user can act on.

## ⟨D6⟩ Latent defect to fix while you are in this file: `updatedAt` is a monotonic clock, persisted

`NoteEntry.updatedAt` is written as `monotonicNow()` = `SystemClock.elapsedRealtime()`
(`MainActivity.kt:662`, `NotePad.kt:170`) — **milliseconds since boot** — and it is serialized into the index
on disk. After a reboot the counter restarts, so a note saved yesterday can hold a *larger* `updatedAt` than
one saved today. Any ordering, "modified since", or date display built on it is silently wrong.

**It is harmless today only because nothing reads it** — the single use is the write. That makes it a loaded
gun rather than a wound: the field's name invites exactly the comparison that is invalid, and I nearly
prescribed that comparison myself before checking the clock. Make it wall-clock epoch millis (it is persisted
metadata, which is what wall-clock is for), or rename it so its nature is unmissable. The chrono's use of a
monotonic source is correct and must not be touched.

## Acceptance

- Sent notes appear under a dimmed "Sent" header, still visible, in the existing tree.
- Drawing on a sent note moves it back to the actionable group with no new state.
- No bulk clear exists.
- `updatedAt` survives a reboot with a meaningful ordering — **shown, not argued**: save a note, restart the
  device, save another, and assert the second sorts later.
