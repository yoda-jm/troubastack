# T173 — The Studio song list shows which songs carry a rehearsal note, and puts them first

**Lane:** web-core · **Status:** specced, not started · **Origin:** VLL via the mobile lane, 2026-09-14,
right after the tablet→Studio send was verified: *"could be nice to have an indication in the song list that
there is a note for a song (like a small icon) and the ability to filter them"* / *"or maybe to show them
first because the goal is to remove them by putting actual annotation"*.

Validates `docs/handoff/proposals/studio-song-list-note-affordance.md`. The proposal's reasoning is adopted
as written: a rehearsal note exists **to be recopied into a real annotation and then deleted**, and until now
nothing in Studio said which songs had one waiting, so notes rot — the exact failure T170 §1 is built to
prevent. This turns the song list into the worklist for clearing them.

## ⟨D1⟩ The band library list, not the setlist page

Put the badge and the ordering where a member **browses songs in order to edit them**. Recopying a note is
editing work, not performance prep; a setlist is the wrong frame and would put a nag in front of someone
getting ready to play.

## ⟨D2⟩ Badge with a count, and sort-first. The filter is second, not struck.

Both halves of VLL's ask are legitimate and he offered them as alternatives (*"or maybe"*), so choosing is
allowed — and his reason picks the winner: *"the goal is to remove them"*. **Sort-first serves removal
better than a filter**, because a filter is a mode you must decide to enter while sorting puts the work under
your nose. Ship the badge (with the count, since the aggregate returns it anyway) and songs-with-notes on
top. **The filter is deferred, not refused** — revisit if a band's library grows long enough that sorting
alone stops helping.

## ⟨D3⟩ One aggregate call per band

Not N calls. One endpoint returning the song ids that have notes, with counts. The per-song list
(`GET …/songs/{songId}/rehearsal-notes`) stays what the editor uses.

## ⟨D4⟩ Read-only, always

The list may never mutate a note. A note is a reference underlay, never an object — no "clear from here", no
bulk delete on this surface.

## ⟨D5⟩ The badge is a pointer into the existing underlay UI

Tapping it leads to the underlay and its toggle in the editor, so the loop closes where it already lives:
spot it → open → recopy into an annotation → delete the note. Do not build a second place notes live.

## ⟨D6⟩ Say what the badge counts, because zero does not mean "all clear"

It counts notes **sitting in Studio**, nothing else. Two consequences to write into the UI copy and the code
comment rather than leave for a reader to discover:

- A note the tablet has **not sent** is invisible here. The tablet is the only surface that knows about it —
  and it is the surface that nags for it (`NoteIndex.isOld` only ages **unsent** notes). See A74.
- Once someone uses **Done, remove** in Studio, the badge clears while **the tablet still holds its copy**
  (T170 §7: removal never reaches the tablet). So an empty list means "nothing waiting in Studio", never
  "nobody has notes".

**The division of labour, stated once so neither surface drifts:** the tablet nags what is **unsent**;
Studio surfaces what is **unrecopied**. Each nags only what its own user can act on.
