# T182 — The band's tags: one panel to browse, rename, merge and delete them

**Lane:** web-core · **Status:** specced, not started · **Origin:** VLL, 2026-09-28: *"on a le vocabulaire
d'un groupe quelque part ? ça pourrait aussi faire un fast search access et aussi servir à renommer ou
supprimer un tag"*. He then accepted all three of my proposals: renaming onto an existing tag merges, any
member may do it, and the number of songs touched is always confirmed first.

**Depends on:** T180, for `GET /api/bands/{id}/tags` (under review @ `d8c64a4c`), and T181, for the search
chip that a click adds (§3). Start after T180 lands. The click in §3 needs T181.

## 1. What exists, and one thing that does not

- T180's `TagCounts` returns the band's tags with song counts, one entry per **distinct stored spelling**
  (`encore` and `Encore` are two entries). This panel reads it.
- Any band **member** can already change a song's tags (`UpdateSong` only checks membership).
- **No history exists for song metadata.** `UpdateSong` overwrites the song in place; only annotations
  keep a history. A tag rename or delete cannot be undone from the app. **That is why the confirmation in
  §5 is not optional.** It is the only safety this feature has.

## 2. ⟨D1⟩ A "Tags" panel on the band page

A panel beside Members and Songs. It lists every tag with its count, most-used first, **one row per stored
spelling**. Spellings are *not* grouped here, unlike T181's search. This panel is where the reader cleans up
duplicate spellings, so it must show them. Each row has **Rename** and **Delete**.

A band with no tags shows one muted line (*No tags yet — add them from a song's details.*), not an empty
panel.

## 3. ⟨D2⟩ A click on a tag filters the song list

Clicking a tag's name adds it as a T181 chip to the song list's search box and scrolls to that list. Chips
already in the box stay (AND, as T181 says). This is the "fast search access".

T181's box only appears above 12 songs. **When a chip is present, the box must show whatever the song
count**, otherwise a small band clicks a tag and sees nothing to explain the shorter list.

## 4. ⟨D3⟩ Rename, and merge when the new name already exists

One server call, `POST /api/bands/{id}/tags/rename {from, to}`:

- `from` is an **exact stored spelling**: the row the reader clicked. If no song carries it any more, return
  `ErrNotFound` so a stale panel says so, not a silent "0 changed".
- `to` is trimmed, must be non-empty, and must **not contain a comma**. A comma is T180's tag delimiter, so a
  tag containing one could never be typed again.
- On every song carrying `from`, replace it **in place** with `to`, so its position among the song's tags is
  kept. If the song already carries `to`, drop the duplicate, so the song ends up with **one** `to`. That is
  the merge.
- **Merging happens on exact equality only.** If `to` folds equal to a *different* existing spelling
  (typing `ENCORE` when `encore` exists), the dialog offers that existing spelling first: *Use "encore"
  (2 songs)?* This is T180's adopt-existing rule applied here, so renaming cannot mint a third spelling.
  Changing only the case of a tag that has no other spelling (`encore` to `Encore`) is a plain rename and is
  allowed.
- The response is the number of songs actually changed.

## 5. ⟨D4⟩ Delete, and the confirmation both actions share

`POST /api/bands/{id}/tags/delete {tag}` removes that exact spelling from every song carrying it. It
returns the number of songs changed, and `ErrNotFound` as in §4.

Before either write, the dialog states the consequence in numbers, using the count from the panel:
- rename: *"slow blues" will become "blues" on 4 songs.*
- merge: *"Encore" will be merged into "encore": 1 song changes, "encore" will be on 3.*
- delete: *"needs work" will be removed from 5 songs.*

**Every dialog adds that this cannot be undone.** Afterwards, report the server's number, not the dialog's.
If another member changed tags in the meantime, the two differ and the server's number is the true one. Then
refetch the panel and the song list.

## 6. ⟨D5⟩ Server: one lock, one write, any member

- **Authorisation:** band membership, the same right as editing one song's tags. A non-member gets
  `ErrForbidden`.
- **All or nothing.** Apply the change to every affected song in **one repo operation**: one lock and one
  `flush()` in `filerepo`, and the same in `memrepo`. Do **not** loop over `UpdateSong`. That loop flushes
  once per song, so a failure part-way leaves some songs renamed and some not, with no history to repair from.
- The server computes the affected songs **from the store at write time**. It never trusts a song list sent
  by the client.

## 7. Not in this task

- Undo or a history of song metadata. That is real, and it is a larger change than this task.
- Renaming across bands. Everything here stays within one band.
- Tags on Stage.
- Making the counts or the panel live-update when another member edits. Refetching after your own action
  is enough.

## 8. Acceptance

- **Merge dedups within a song.** Use a fixture where one song carries **both** `Encore` and `encore`. Renaming
  `Encore` to `encore` leaves that song with exactly one `encore`. **Assert that the fixture has such a song**,
  or this line proves nothing.
- The rename keeps position: `[a, from, b]` becomes `[a, to, b]`. Other tags and other songs are untouched,
  asserted on a song that does **not** carry `from`.
- A `to` that folds equal to a different existing spelling triggers the adopt-existing offer, and accepting it
  stores the existing spelling.
- `to` with a comma, or empty after trimming, is `ErrInvalidInput`. An unknown `from` is `ErrNotFound`.
- A non-member gets `ErrForbidden` on both endpoints.
- The returned number equals the songs actually changed.
- **All or nothing, teeth-checked.** The property: after a failed write, no song has changed, **both** when
  read back through the service **and** after reloading the repo from disk. Watch the in-memory side:
  `filerepo` mutates `r.d.Songs` before `flush()`, so a failed flush can leave memory changed while disk is
  not. How you guarantee the property is your call.
  Inject the failure **after the first affected song would have been persisted**, so that a per-song loop
  would already have written one song. A failure on the very first write cannot tell a loop from a batch.
  Then sabotage the implementation into a loop over `UpdateSong`, confirm the test goes **red**, and print the
  sabotaged line to prove the sabotage landed.
- Clicking a tag in a band with fewer than 12 songs shows the box with its chip, and the list is filtered.
- The e2e asserts on the **stored tags** after a rename, not only on the refreshed panel.
