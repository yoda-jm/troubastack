# T100 — A local band folder can define its concerts (`make band=gvo` seeds a setlist)

**Priority:** normal — VLL asked for it directly · **Size:** S · **Area:** `core/cmd/seed`. Lane: Web & Core.
**Extends B14** (local band folders).

VLL, 2026-08-24: *"add the concert I've created in the gvo seed"*.

## 1. The finding: it isn't data entry, the seed can't express a concert

`bands/<slug>/` (B14, gitignored) holds `band.json` + `repertoire.json` + per-song source files, and
`make band=<shortname>` seeds it. But the loader builds:

```go
groups = append(groups, groupDef{
    name: man.Name, kind: kind, admin: man.Admin.Username,
    members: memberNames, songs: songs, personal: true, shortname: shortname,
})
```

`groupDef.setlist` is **left zero**, and `seedSetlist` is gated on `g.setlist.name != ""`. So
`make band=gvo` creates the band, its members and its 46 songs — **and never a setlist**. There is
nowhere in `band.json` or `repertoire.json` to put one: `bandManifest` is `{name, shortname, kind,
notes, admin, members}` and nothing more.

The demo bands *do* get setlists, but only because they're hard-coded `setlistDef` literals in
`main.go` (e.g. "Spring Concert"). A local band has no equivalent.

So this is a small feature, not a config edit.

## 2. What to build

**A `setlists.json` in the band folder** — plural from the start, and a separate file from
`repertoire.json`:

- Plural because VLL already asked "what if I want to follow 2 concerts?" — Sat and Sun for the same
  band is the normal case, not an edge one. A single object in `band.json` would have to be widened
  later.
- Separate because `band.json` is identity, `repertoire.json` is the song list, and a concert is a
  third thing with its own lifecycle (it changes per gig; the repertoire doesn't).

**Reference songs by `slug`, not by title.** `repertoire.json` already keys every song by a `slug`
that is also its folder name (`dirty-old-town`, `jaime-plus-paris`). A slug is stable across a retitle
and unambiguous across two songs with the same name; matching on a display title is neither. An
unknown slug is an error naming the slug — silently dropping a song from a gig list is the worst
failure this task can have.

**Match the `.tband` manifest's field names exactly** (`core/internal/app/bandio.go`). `ExportBand`/
`ImportBand` already round-trip setlists losslessly through `manifestSetlist` {name, eventDate, venue,
notes, items} and `manifestItem` {songRef, position, keyOverride, tempoOverride, notes, onCall,
transposeChords}. `setlists.json` is a hand-edited *definition* where `.tband` is a machine *snapshot*,
so they stay separate mechanisms — but the vocabulary must not fork. Two deliberate deviations, both
in service of hand-editing: `song` (a repertoire slug) replaces `songRef` (an internal songID), and
**array order replaces explicit `position`**. Everything else keeps the manifest's name and meaning.

Note that `onCall` and `transposeChords` exist in `manifestItem` but *not* in seed's `overrideDef`.
Support them in `setlists.json` — a gig list that can't mark an on-call song is missing the thing that
makes it a gig list.

`groupDef.setlist` is a single `setlistDef` today; widen the local path to seed **each** entry. Keep
the demo path working unchanged.

## 3. What must NOT change

- **`bands/` stays gitignored.** The GVO concert's content — songs, dates, venue — is real band data
  and stays local. **Only the mechanism is committed**, and it must be generic: nothing in
  `cmd/seed` may name Good Vibes Only or any of its songs.
- **A plain `seed` / `make demo` must still skip personal bands.** They're `personal: true`; adding
  setlists must not change that gate, or a demo run would start emitting a real band's gig list.
- The demo groups' hard-coded setlists keep working byte-identically.

## 4. Acceptance criteria

- `make band=gvo` (or any local band) with a `setlists.json` creates each setlist with its items in
  array order, and applies the overrides.
- **A missing `setlists.json` is normal**, not an error — the band seeds exactly as it does today. This
  is the back-compat case and it needs a test.
- **A slug not in the repertoire fails loudly, naming the slug.** Silently dropping a song from a gig
  list is the worst possible failure here.
- **A plain `seed` still skips personal bands** — assert it, since this task adds new data to the thing
  being skipped.
- Two setlists in one file both seed, with distinct names/dates.
- `gofmt -l core` clean; `go test ./...` green.

## 5. Also fix while you're here

`bands/good-vibes-only/repertoire.json`'s own note claims *"this file is committed so `make gvo`
rebuilds the song list from a fresh clone"* — but `.gitignore:68` ignores `bands/` wholesale, so it is
**not** committed and a fresh clone rebuilds nothing. Either the note or the ignore rule is wrong.
Don't guess which: **flag it at the gate with a recommendation** and let VLL rule — it's his data and
his call whether a repertoire of titles-and-artists is safe to commit.

## 6. Out of scope

- Baking those setlists, or anything about the bake.
- Committing any GVO content.
- A UI for editing local band folders.

## 7. The data is already written

`bands/good-vibes-only/setlists.json` exists (gitignored) and holds VLL's concert as read off the
running instance's store on 2026-08-24: **"Hésingue en Fête", 2026-09-05, two items** — `dirty-old-town`
then `jaime-plus-paris` — no venue, no notes, no overrides. It is the acceptance fixture: when this
task lands, `make band=gvo` must recreate exactly that setlist.

Two caveats for whoever implements this. It is a **snapshot** — VLL may have added songs since, so
re-read the store rather than trusting the file's contents. And venue/notes are empty because he left
them blank in the UI, **not** because the field is missing: `app.Setlist` has both, `omitempty`.
