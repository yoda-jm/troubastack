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
Part { name, kind: voice|instrument|other, short?, input?: mic|di|line|none, shared?: bool }
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

## The grid: VLL's layout (2026-10-24). Read this before the Studio and sheet sections; it shapes both.

> "un tableau avec chaque ligne un morceau et ensuite des larges colonnes avec les noms et en sous-colonnes des
> instruments ou voix — ça marche moins bien si la basse passe d'une personne à l'autre, mais d'un autre côté le même
> instrument (chant lead) peut changer aussi, ou une même personne peut passer de chant lead à backing"

**Resolution: sub-columns are CHANNELS (what gets patched), not parts.**

- **Rows** are songs.
- **Wide columns** are musicians. **Sub-columns:**
  - **ONE "Voice" sub-column per person**, whatever voice part they sing. It is one mic. The cell shows the voice
    part's short form: **L** (lead), **B** (backing), or empty. A person going from lead to backing is the same
    column with a different letter, which is exactly what the engineer needs (a level change, not a repatch).
  - **One sub-column per instrument part that person plays anywhere in the shown songs** (one DI or instrument
    mic). The cell is filled or empty; "(…)" means optional.
- **SHARED instruments** (VLL, 2026-10-24: "on risque de ne pas recâbler la basse (ou les bongos)… la colonne de la
  basse pourrait être une 3e personne « R / V » à part"):
  - one physical instrument on stage is passed between players and stays on the SAME channel;
  - it gets **its own wide column, apart from the people**, headed with the instrument and its players
    ("Bass — R / V");
  - its cell says **WHO** plays it in that song (initials, or the short name), just as a Voice cell says L or B;
  - the changes column distinguishes the two cases: "Bass: R → V (same channel)" means no repatch, while a
    PERSONAL instrument changing hands means two sources, so a repatch.
- **Model consequence:** the PART decides the column.
  - Every `voice` part of a person shares one "Voice" sub-column under that person.
  - A **personal** `instrument` part gets a sub-column under each person who plays it.
  - A **shared** part (`Part.shared = true`: one physical source on stage) gets one column of its own, whoever plays
    it.
  - `shared` is set per band, in the parts vocabulary: bass is shared in one band and personal in another. Default
    false.
  - `input` is per part (a voice is mic; an instrument part says mic, di or line).
  - **The per-song line-up is unchanged** ("R: Bass"). Only the grid projection routes a shared part into its own
    column.

**A second projection of the same data, for musicians: "by part".** One toggle, same rows.
- Columns are parts: Lead vocal, Backing vocal, Bass, Autoharp…
- Cells are names: "who sings lead here?", "who has the bass?".
- The same data, no second source.

**The grid IS the main editor** (a band-level "Line-ups" page), not only a report:
- rows are all songs, and the existing chips filter them (tags, line-up chips, "in setlist X");
- **click a Voice cell** to cycle empty → L → B → empty, with other voice parts from a small menu;
- **click an instrument cell** to toggle it; **click a shared-instrument cell** to pick who plays it, from a menu of
  musicians, or empty; long-press or the menu sets optional and the note;
- **"+ part" on a musician's header** adds a sub-column (an instrument part for them);
- **"+ musician"** adds a wide column;
- **keyboard:** arrows move between cells, Space toggles, L or B set the voice.

The per-song panel on the song page stays, as the same data seen for one row.

**The sound sheet uses this exact person-major layout**, filtered to the setlist's running order, with only the
sub-columns used in that set, plus the changes and sound-note columns. The CSV has one column per sub-column
("A · Voice", "A · Guitar", …).

**Review will check:**
- Lead↔Backing for one person: one column, the letter changes.
- A shared bass moving between two people: ONE column, the cell's player changes, and the changes line says "same
  channel".
- A personal instrument moving between two people: two sub-columns, and the changes line says "repatch".
- Toggling a part's `shared` re-routes the grid with no edit to any line-up.
- One person with two instruments in one song: both cells filled.
- A sub-column unused in a set does not appear on that set's sheet.

## Headers, patch order, unamplified parts (VLL, 2026-10-24, third amendment)

> "je mettrais des colspan… la possibilité de le mettre dans l'ordre de la patchlist du concert… et il y a aussi les
> instruments non sonorisés, peut-être une ligne sous la première avec l'entrée de la table (avec un nom propre)"

**Multi-row header** (grid, sheet and PDF):
- **Row 1, owner:** the musician name, or the shared instrument's header ("Bass — R / V"). It is **colspanned over
  runs of adjacent columns with the same owner**, in the CURRENT column order. In stage order that is one span per
  musician. In patch order a musician's columns may not be adjacent, so their name spans each run separately, never
  across someone else's column.
- **Row 2, channel:** "Voice", "Guitar", "Autoharp", "Shaker", …
- **Row 3, console input** (sound sheet, and the grid when a setlist is selected): the input number and the strip
  name from the concert's patch, e.g. "In 4 · A GTR". It reads "— acoustic" for an unamplified part and "not
  patched" for a channel with no patch entry.

**Patch list: per CONCERT (setlist), because the console changes with the venue.**
- **Model:** `Setlist.patch: [{ channel, input: int, name: string }]`. It is additive, and **reserve the field in
  stage 1**.
  - `channel` is a stable channel key: `voice:<musicianId>` | `inst:<musicianId>:<partId>` | `shared:<partId>`.
  - `name` is the console strip name (short, free text).
- Studio, on the sound sheet:
  - edit input numbers and strip names inline;
  - **"Copy patch from…"** another concert, with matching by channel key;
  - "Auto-number" fills 1..N in the current order.
- **Column order: [Stage order | Patch order]** (a toggle on the sheet).
  - Stage order is the band's `Musician.order` (M4).
  - Patch order sorts patched channels by input number, then unpatched channels, then unamplified ones.
- The **input list** section of the sheet IS the patch list, sorted by input:
  - one line per patched channel: in, strip name, musician or shared instrument, part, mic/DI/line;
  - then "Not patched yet: …" if any;
  - unamplified parts never appear there.

**Unamplified parts** are `Part.input = none` (shaker, triangle, an unmiked acoustic):
- they **stay in the grid and the sheet**, because musicians need to know who plays them;
- they show as a sub-column whose input row reads "— acoustic";
- they are never given a patch entry or a channel key, and are excluded from the input list;
- in both orders they sit after the amplified channels of the same owner (stage order) or at the end (patch order).

**Review checks:**
- colspans over adjacent runs only, in both orders;
- patch order with a musician split across non-adjacent inputs → two spans;
- an unamplified part has no input and no patch line;
- "Copy patch from…" matches by channel key, and a new channel reads "not patched";
- a part toggled to `shared` changes its channel key from `inst:` to `shared:`, so the patch shows it as not patched
  (an honest signal, not a silent mismatch).

## Studio

### Editing

The band-level grid above is the main editor. On the song page there is also a **"Line-up"** panel, a compact grid:
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
3. **Matrix** (the person-major channel grid above, filtered to the set):
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
