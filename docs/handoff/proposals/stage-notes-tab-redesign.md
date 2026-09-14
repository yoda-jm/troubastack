# Proposal — Stage Notes tab: re-spec the whole page (information design, compactness, sent = secondary)

**Lane:** mobile (`app/androidApp`, the A70 Notes tab). **Raised by:** VLL, 2026-09-14, while re-testing the
T170 §6 send on-device. **Status:** proposal for Fable to **own and spec the whole page**. VLL, verbatim:

> *"the send to studio are less important, and here it is hard to know, the whole page (compacity, how it is
> displayed, ...) should be re-specced by Fable"*

and earlier, on the same page:

> *"look at the page, you can only display 2 notes, we should find a way to be a little bit more compact there"*

and (the sent-notes idea this proposal now absorbs):

> *"when something is sent to the server it can probably be moved in a recycled bin (still displayed probably
> but it is less important than the one already sent) in the Notes tab in Stage"*

This **supersedes** `stage-notes-sent-recycle-bin.md` (folded in below).

## The problem, in VLL's terms

The Notes tab is the tablet-side worklist for rehearsal notes. Today it does not read as one:

1. **Not compact** — each note was a tall card; only ~2 fit on screen. (I shipped a stopgap single-row layout
   on `task/notes-tree` @ `c7cca638` so it's usable, but VLL wants the *page* designed, not patched.)
2. **Wrong emphasis** — "send to Studio" actions sit visually equal to the note itself, and *"here it is hard
   to know"* — the note's real state (drawn / sent / stale-vs-current-bake / awaiting-recopy) doesn't read at
   a glance. Sending is a **secondary** action; the note and its status are the primary content.
3. **Sent ≠ done, but sent ≠ urgent either** — once a note reaches Studio its remaining job is to be recopied
   there and deleted; the tablet can't know when that happened. Sent notes should drop to a de-emphasized
   "sent" resting place (still visible), keeping not-yet-sent notes prominent.

## What Fable should spec (the whole page)

- **Information hierarchy** — what is primary (the note, its page, its state) vs secondary (send/re-send).
  De-emphasize the send affordances; make state legible without reading a row of buttons.
- **Compactness** — a density that fits many notes; the current one-row-per-note is a floor to improve on,
  not the target.
- **Sent bin** — sent notes de-emphasized/collapsed but still shown; the two-lifetimes trap stands (see
  below).
- **The tree** — band → concert → song → page grouping (T143-style collapse) stays useful; spec how it
  coexists with the sent/unsent split and the density goal.

## Constraints / traps to carry into the spec

- **Two lifetimes.** A sent note on the tablet is *still live in Studio*. Deleting the tablet copy does **not**
  delete the Studio underlay, and Studio's *Done, remove* never reaches the tablet (T170 §7). Any "clear sent"
  / bin-empty affordance must not imply the Studio copy is gone; nothing here can auto-delete.
- **Per-user is already correct server-side** (verified 2026-09-14): a note is keyed `(ownerUserID, songID,
  pageInSong)` with `ownerUserID = the authenticated caller`, so one member never overwrites another's
  underlay. The identity prompt (`takenAs` ≠ signed-in) exists because `takenAs` is display-only and the owner
  is always the sender. The page design should make that ownership legible where it matters.
- **The functional send is done and verified** (overwrite prompt, identity prompt, bulk skip-already-sent);
  this proposal is about the page's *design*, not the send behaviour. A redesign should preserve those flows.
