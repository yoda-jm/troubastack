# Proposal — Studio: the song list signals which songs carry a rehearsal note, and can surface them first

**Lane:** web-core (studio, maybe a light core aggregate). **Raised by:** VLL via the mobile lane,
2026-09-14, right after the tablet→Studio send (T170 §6) was verified end to end. **Status:** proposal for
Fable to validate + spec. VLL: *"send this spec to Fable for web/core lane at some point"* — not urgent.

## What VLL asked for (verbatim, two messages)

> *"could be nice to have an indication in the song list that there is a note for a song (like a small icon)
> and the ability to filter them"*

> *"or maybe to show them first because the goal is to remove them by putting actual annotation"*

## Why this exists — the load-bearing reason

A rehearsal note is deliberately **not** an annotation (T170 §1). Its entire purpose is to be **recopied
into a real annotation and then deleted** — the friction is the feature. Once notes reach Studio as
reference underlays, the song list is where a member decides what to work on, yet nothing there says *which*
songs carry a pending note. So notes rot silently on the server, which is the exact failure the whole
recopy-then-remove design is meant to prevent. An indicator plus "notes first" ordering turns the song list
into the **worklist for clearing notes**.

## Rough shape (for Fable to spec, not a design ruling)

- **Signal:** a small icon/badge on a song row when that song has ≥1 rehearsal-note underlay (optionally a
  count). The data already exists — `GET /api/bands/{bandId}/songs/{songId}/rehearsal-notes` lists them;
  a per-band aggregate (one call returning the set of song ids that have notes, or counts) would avoid N
  calls and is probably the right core addition.
- **Surfacing:** VLL's preference is *show them first* — sort songs-with-notes to the top. A "only songs
  with notes" filter toggle is the second half of his ask; sort-first may be enough on its own.
- **The path it should open:** tapping the indicator should lead to the underlay + its toggle in the editor,
  so the loop is spot it → open → recopy into an annotation → delete the note. The affordance is a pointer
  into the existing T170 underlay UI, not a new place notes live.

## Constraints / traps

- **Read-only signal.** It must never mutate a note; a note is a reference underlay, never an object.
- **Which list?** Studio has more than one song surface (per-setlist vs the band library). Fable to decide
  where the indicator + ordering belong — likely wherever a member browses songs to edit.
- **Two lifetimes.** The indicator reflects the *server* note; the tablet copy is independent (see the
  companion proposal `stage-notes-sent-recycle-bin.md`). Clearing one does not clear the other.
