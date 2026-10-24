# T203 — Who does what, per song: line-ups, line-up search, and the sound engineer's sheet

Owner: web-core. Spec: Fable, 2026-10-24.
Status: **new design. Present the data model + an editor sketch at the gate BEFORE building** (as A91 did).
Three **[VLL]** decisions, implement on their defaults.

## Why (VLL, 2026-10-24)

> "in my bossa band not everybody plays always or the same things or sing lead or backing … I am looking for something
> where we can search (just like tags: I want the songs where <A> sings lead and <B> plays autoharp), but also
> something that can produce a summary for the sound engineer. Currently I can add a txt document per song where I
> describe it and give the tstage printed with only that, but this is far fetched."

The names in this spec are placeholders. Never commit real band, member or song names.

## What exists (read in code)

- `Song` has `Tags`, `Notes`, `Key`, `Tempo`, `Meter`. **Nothing says who plays or sings what in a song.**
- The `.tband` member record has `plays`: free-text "instrument prose", documentation the reader ignores. It is per
  member, not per song.
- Studio's song filter (`BandDetail.tsx`) uses **chips** (tags, title/artist text) with AND semantics and caret
  suggestions (`suggestForWord`, `foldText`). That is the place to extend.
- `internal/setlistpdf` (T158) prints a setlist's running order. Its own doc calls it "the sheet that gets taped to
  the floor or handed to a sound engineer". It is the natural base for the sound sheet.

## Model

### Musicians (band level)

The people who can appear in a line-up.

```
Musician { id, name, userId? }        // userId links to a band member's account when there is one
```

- Every band member appears automatically, linked by userId.
- **Non-account musicians can be added by name** (a guest percussionist, a musician who never installs the app).
  A musician can later be linked to an account when that person joins.
- The roster is managed in band settings by an admin.

### Parts (band-level vocabulary)

What someone can do in a song.

```
Part { name, kind: voice|instrument|other, short?, input?: mic|di|line|none }
```

- `short` is the sheet abbreviation ("LV", "BV", "Gtr", "AH"); if empty it is derived from the name.
- `input` feeds the sound sheet's input list.
- Parts are entered like tags: free text with suggestions from the band's vocabulary, folded with `foldText`. The
  vocabulary is editable in band settings (rename or merge, the way the TagsPanel already does it for tags).
- Seed suggestions: Lead vocal, Backing vocal, Guitar, Bass, Keys, Percussion, Drums.

### Line-up (per song): **explicit, no inheritance** [VLL]

```
Song.lineup: [ { musicianId, parts: [partName…], optional: bool, note? } ]
Song.soundNote: string        // what the sound engineer needs for this song ("intro voice + guitar only")
```

- A musician absent from the list does not play that song.
- `optional` means the song works without them. That makes "what can we play without B?" answerable.
- **Default (implement): explicit per song**, with fast entry tools. That is predictable, which is VLL's recurring
  complaint about the app.
- **Alternative:** a band default line-up plus per-song exceptions. Faster to type, harder to predict.

## Studio

### Editing

On the song page there is a **"Line-up"** panel, a compact grid:
- one row per musician, with the parts as chips (with suggestions), an "optional" toggle and a note;
- an "add a musician to this song" control;
- a "Sound note" field.

Bulk tools:
- **"Copy line-up from…"**: pick another song.
- In the song list: **select songs → "Set a part…"**, for example "B · Autoharp" added to the 12 selected songs; and
  **"Remove a part…"**.

### Search: chips, AND, like tags

New chip kinds:

| Chip | Matches |
|---|---|
| `A · Lead vocal` | A has that part |
| `A` | A plays or sings anything |
| `Lead vocal` | anyone has that part |
| `without B` | B is absent, or optional |

- Typing suggests musicians, parts, and musician·part pairs that actually occur.
- Chips combine with tag chips and text, all AND, the same as today.
- VLL's example is the two chips `A · Lead vocal` + `B · Autoharp`.

### Sound sheet (per setlist)

On the setlist page: **"Sound sheet"**, shown on screen, with **PDF** (A4 landscape, reusing `writeA4PDF` /
`setlistpdf` conventions) and **CSV**.

1. **Header:** band, setlist name, date, venue.
2. **Input list:** for each musician in the running order, all their parts across the set, with `input` (mic / DI /
   line). For example "A — Lead vocal (mic), Guitar (DI)". This is what the engineer patches.
3. **Matrix:**
   - rows are songs in running order: number, title, key, tempo;
   - columns are musicians;
   - a cell holds that musician's part abbreviations for the song ("LV+Gtr", "BV", "AH"), and an empty cell means
     not playing;
   - an optional part is shown in parentheses.
4. **"Changes" column:** the difference from the previous song ("B: Gtr→AH", "C out", "D in"), plus the song's
   sound note.
5. Intermissions are separator rows. On-call songs go in a section of their own, after the main set.

### `.tband`

Musicians, parts and line-ups round-trip: musicians by stable id (see the WIP note on ids), and line-up entries by
musician id. Add it to the round-trip test.

## Out of scope (later tasks, kept in mind by the design)

- **Per-gig variations [VLL]:** an absence or a substitute for one concert, as a setlist-item override of the
  line-up, feeding that gig's sound sheet. Default: later. The model must allow an item-level `lineup` override
  without migration.
- **Tablet:** "You: Autoharp · Lead: A" on the song's first page, and maybe default layer visibility per the
  member's parts, unifying the old `RoleTag` reading role.
- **Smart playlists** on these chips (the "meta-playlist" idea), always frozen before a concert.
- **One-off pre-fill** of line-ups from VLL's existing per-song text documents: Fable does it through the API once
  this lands, and VLL corrects it.

## [VLL] decisions (defaults stand unless he says otherwise)

1. **Musicians without an account:** allowed, by name, and linkable later. Default **yes**.
2. **Line-up per song:** explicit, with copy and bulk tools. Default **explicit** (no default line-up + exceptions).
3. **Per-gig variations:** default **later**, as a separate task.

## Tests

- **Model and API:**
  - CRUD for musicians, parts and line-up;
  - only admins edit the roster and vocabulary; members edit line-ups like other song metadata;
  - renaming a part updates every line-up;
  - merging two parts unions them;
  - linking a guest musician to an account keeps its line-ups.
- **Search (pure):**
  - each chip kind;
  - AND combinations with tags;
  - `without B` with B absent, optional, or required;
  - VLL's two-chip example.
- **Sound sheet (pure, then PDF):**
  - the input list is the union over the set;
  - the matrix cells;
  - the changes column across intermissions (compared with the last song before the break) and for the first song
    (no "changes");
  - on-call songs listed separately;
  - the PDF is deterministic.
- **`.tband` round-trip.**
- **Studio e2e:** edit a line-up, search with two chips, open the sound sheet.

## Delivery

1. **Design at the gate:** the model, the editor sketch, the search grammar, the sound sheet mock-up.
2. Model + API + `.tband`.
3. Editor + bulk tools + search.
4. Sound sheet.

Present each step.
