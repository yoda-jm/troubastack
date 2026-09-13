# Proposal — Stage Notes tab: once sent, a note falls into a de-emphasized "sent" bin

**Lane:** mobile (`app/androidApp`, the A70 Notes tab). **Raised by:** VLL, 2026-09-14, while watching the
tablet→Studio send work on-device. **Status:** proposal for Fable to validate + spec. Builds on the Notes
tree + send/bulk-send just verified on `task/notes-tree`.

## What VLL asked for (verbatim)

> *"when something is sent to the server it can probably be moved in a recycled bin (still displayed probably
> but it is less important than the one already sent) in the Notes tab in Stage"*

Reading it clause by clause: **trigger** = a note's `sentAt` is set; **action** = move it to a "recycle
bin"; **still displayed** = don't hide it; **less important** = de-emphasize it relative to the notes that
still need action.

## Why — the Notes tab is the tablet-side worklist

After a bulk "Send all", every note reads "sent" and the list stops telling you what still needs doing. A
note's urgency drops the moment it reaches Studio: its remaining job is to be recopied in Studio and then
deleted, and the tablet can't know when that happened. So the useful split is **not-yet-sent** (actionable
here: draw more, send) kept prominent, vs **sent** (done on this side) tucked into a quiet, still-visible
section.

## Rough shape (for Fable to spec)

- **Grouping:** a "Sent" section — either a top-level split ("To send" above, "Sent" below) or per-node
  within the existing band→concert→song→page tree. Sent likely **collapsed by default** and/or dimmed;
  unsent stays expanded and prominent.
- **Interplay with existing nags:** the A70 `⚠` nag and the "older than the current bake" warning still
  apply to a sent note (a stale note still wants recopy+delete). Fable to decide how the bin and the nag
  coexist.
- **"Recycle bin" framing — the trap to get right:** a sent note in the bin is **still live in Studio.**
  Deleting the tablet note does **not** delete the Studio underlay (two independent lifetimes — see
  `studio-song-list-note-affordance.md`). So a bulk "empty bin" / "clear sent" action, if offered, must not
  imply the Studio copy is gone; it only clears the tablet's local copies. And there is no signal that
  Studio has recopied, so nothing here can auto-delete — it stays a manual, informed choice.

## Open questions for Fable

- Default-collapsed section vs dimmed-inline — which reads as "less important" without hiding?
- Does "recycle bin" imply a delete affordance beyond the existing per-note Delete, or just a resting place?
- Should re-sending (already supported) pull a note back out of the bin, or is "sent" sticky?
