# T134 — `.tband` v2: the zip carries the *folder format*, so a band has ONE description outside the software

**Lane:** web-core (core/Go). **Size:** L. **Status:** spec, ruled by VLL. **Not frozen.**
**Ruled by VLL, 2026-09-04:** *"fais évoluer tband pour qu'il contienne le format dossier, mets en place
dès le début une version"* — after ruling out a second name: *"on peut appeler le format ztband ou
tbandz"* → **no.** `.tband` **is already a zip**; two zip formats with near-identical names is a trap
paid for later. One name, one format, a version field to tell them apart.

## Why: today a band has TWO descriptions outside the software

| | **folder format** (`cmd/seed`) | **`.tband` v1** (T62) |
|---|---|---|
| written by | **a human** | a server |
| read by | `cmd/seed` only (bootstrap) | the import endpoint |
| identity | **names + slugs** (`username`, song `slug`) | server UUIDs |
| band, members | ✅ `band.json` | ✅ |
| songs, key/tempo/meter/tags | ✅ `repertoire.json` | ✅ |
| concerts | ✅ `setlists.json` (T100) | ✅ |
| chart files | ✅ `<song-slug>/` folders | ✅ `<song-slug>/<filename>` (amendment 4; was `blobs/<sha256>`) |
| **annotations** | ❌ **none** — `main.go:1061`: *"a chart-only repertoire song gets none"* | ✅ head-only |
| song cues, file selections, chartSource | ❌ | ✅ |

**The gap that matters is annotations.** A real band seeded from a folder arrives with **zero**
annotations — the hand-made, valuable part. That is also what blocks expressing the demo in the folder
format, since demo songs carry annotations the repertoire path cannot describe.

**Evidence this is a live problem, not a theoretical one:** the band library contains an
`annotations.json` that **no code reads** — no `formatVersion`, not a T62 export, ignored by the seeder.
Someone produced it wanting exactly this feature. It is **17.6 KB against 554 KB of live `.jsonl`** and
**13 days stale**. If anyone believes it is a backup, it is not.

## The shape of v2

The zip carries **the folder format**, plus the machine parts that have no human-authorable form:

```
band.json          formatVersion, name, members[]  (+ the folder's own keys, ignored by the reader)
repertoire.json    songs[] {slug, title, artist, key, tempo, meter, notes, tags, files[]}
setlists.json      concerts[] {name, eventDate, venue, notes, items[]}
annotations/<slug>.json    per song: {layers[], objects[]}            ← the NEW part
cues.json          song cues, file selections                          (optional)
<slug>/<filename>  the file bytes, under HUMAN names                   ← amendment 4
```

> **A `.tband` is this directory zipped. Nothing else.** Phase 1 shipped `blobs/<sha256>` as the only
> path to bytes; **amendment 4 removes it** — one layout, folder or zip, and `unzip` gives the directory
> back. Passages below that discuss mapping to `blobs/` (Gap 1, amendment 3 §2) are **history**: they
> record why content addressing was questioned, not what to build. `blobHash` survives in
> `repertoire.json` as an **integrity field**, never as the storage key.

**Human-facing files stay name- and slug-based** — that is what makes the format portable and
hand-writable, and it is why members already match by `username` rather than by id.

### ⚠ The one thing that must NOT become name-based

**Annotation `layer.id` and `object.uuid` must survive verbatim.** T62 keeps them deliberately —
*"annotation layer ids + object uuids are KEPT"* — with `Layer.FileID` rewritten through the file map
and `OwnerID` through the member map. If v2 re-mints them, every round-trip silently reshuffles
annotation identity and the format stops being lossless. So: **the human layer is names; the annotation
payload keeps its ids.** Both, not one.

### Versioning — the version is the FILE FORMAT's, not the product's

**VLL, 2026-09-04: *"la version est celle du fichier, de nouvelles features qui s'exportent pareil ne
changent pas la version du fichier."*** This is the rule that stops version inflation, and it needs an
operational test rather than a judgement call:

> **Would a reader written for version N still parse this file correctly and lose nothing it cares
> about?** If yes — **do not bump**. If no — bump.

So: adding a field that fits the existing shape (another song property, another concert attribute) is
**not** a new version; a reader that ignores it is still correct. Moving where something lives, renaming
a file inside the zip, or changing what an existing field means **is**. A product feature that happens
to export as one more key does not touch the number.

Practically, that is why **v1 → v2 is a real bump**: the files inside the zip are rearranged, so a v1
reader would not find what it expects. It is the last such bump we should need for a long time.

`formatVersion` already exists (`BandExportFormatVersion = 1`) and T62's ruling was **reject anything
else, no migration code**. Keep the field; change the ruling **narrowly**:

- **Write v2 always.**
- **Read v1 and v2.** v1 → v2 is a pure rearrangement of the same information — no data is invented or
  lost — so the reader is a mapping, not a migration engine. Refusing v1 would strand exports that
  already exist for the price of a small function.
- **Reject anything else with a 400**, unchanged.
- Add a test that a **v1 fixture still imports**, and one that an unknown version is refused. The v1
  fixture is the whole point of keeping the field: without it, "we support v1" is an assertion.

## `.tband` is a "latest" — history loss is the RULING, not a caveat

**VLL, 2026-09-04: *"je pense que tband doit etre un latest (perte de l'historique)."*** Settled: an
export carries the **current state**, and the annotation history stays behind in the server's `.jsonl`
logs. `bandio.go:142` already does this (*"Tombstones are dropped (head-only, no history)"*) — this
ruling makes it intentional rather than incidental, so nobody later "fixes" it by trying to ship the
event log.

That also decides a design question v2 might otherwise have reopened: **`annotations/<slug>.json` holds
layers and objects at head, not a replay log.** Simpler file, smaller zip, and it round-trips because
ids are preserved.

**But it must therefore never be called a backup.** Most of the 554 KB → 17.6 KB difference in the
library's stray file is head-vs-history, not staleness — I attributed it to staleness first and was
wrong. Someone who exports, deletes, and re-imports
loses their annotation history and will not know until they look for it.


## AMENDMENT (VLL, 2026-09-04) — the unzipped directory is the canonical form

**VLL: *"pour le repertoire band je pense qu'un repertoire avec .tband sera un tband dezippé, c'est plus
facile a historiser, lire et differ."*** Accepted. A band directory **is** a `.tband` unzipped; the zip
is the transport, the directory is the thing you read, diff and version.

**His stated reason understates it.** `.tband` is a *latest* by his own earlier ruling — it carries no
past at all. Putting the directory under git **gives the format the history it deliberately dropped**,
for free, on top of a snapshot format. That is the real argument.

**With the limit stated plainly:** git historises **head states**, not the annotation event log. The
server's `.jsonl` logs (one song alone holds 531 KB) do not come back this way. An export is still not a
backup.

### The measurement that shapes all three gaps

The band library is **37.6 KB of JSON against 821 MB of everything else** — the describable part of a
band is **0.005 %** of its weight. So **the JSON wants git and the charts do not.** Split by *nature*,
not by container.

### Gap 1 — blob naming: the zip and the directory cannot share it

`blobs/<sha256>` is right for a zip (dedup, integrity) and **wrong for a directory a human browses** —
nobody reads a folder of hashes, and it is a regression against today's `<song-slug>/<filename>`.

**Do not bake the hash layout into the writer.** Keep whatever emits blobs behind a seam so the
directory form can lay the same bytes out under human names while the zip keeps content addressing.
Both must round-trip to the same manifest.

### Gap 2 — the names/ids boundary must be explicit, not emergent

The human-facing files are name- and slug-keyed (`username`, song `slug`, `filename` — settled by
⟨D1⟩/⟨D2⟩). **The annotation payload keeps `layer.id`, `object.uuid`, `object.layerId` verbatim.** Write
that boundary down in the format doc as a rule, not as an observation: the next person will otherwise
"tidy" the ids away for consistency and end losslessness silently.

### Gap 3 — canonical JSON, or every round-trip is a fake diff

If the directory is canonical and the zip derived, `zip(dir)` and `unzip(tband)` must be **exact
inverses**. Any normalisation drift — key order, indentation, trailing newline — turns a no-op
round-trip into a diff, and the whole "easy to diff" argument dies. Pin the encoding. The project has
precedent: `bundle.json` is proto3 **canonical** JSON for exactly this reason.

### Correction — the band-level annotations file is NOT an orphan

I reported earlier that the `annotations.json` in the band library "protects nothing". **Half of that
was wrong**, and VLL was right to push back.

Its shape — `{band, exportedAt, songs:[{title, layers, objects}]}` — is exactly the per-song payload of
`POST /api/bands/{bandId}/songs/{songId}/annotations/import`, **wrapped per band**. And the timing shows
it was used: the file is stamped `16:35`, and seven annotation logs were written at **`16:36`**.

So a band-level annotations interchange **already exists in practice**, hand-driven, undocumented. v2
should adopt its shape — with **one fix: key songs by `slug`, not `title`.** A rename silently breaks a
title-keyed mapping, and titles are the field most likely to be corrected.

What remains true from my earlier note: it is **head-only**, so it cannot restore that 531 KB of
history — and nothing that ships should call it a backup.

## AMENDMENT 2 (VLL, 2026-09-04) — filename carries the shortname, folders get migrated, import stays gated

### ⟨D4⟩ REVISED — an absent `formatVersion` is an ERROR

**VLL: *"pour les dossiers actuels on fera les modifications pour les rendre compatibles."*** So the
importer does **not** need a legacy branch. My earlier requirement (absent version → glob the files) is
withdrawn and replaced by a stricter one:

**A folder or archive without `formatVersion` is refused, with a message naming the missing field and
the expected value.** Not a default, not a fallback. The failure it prevents — importing songs with no
charts, silently — becomes loud instead of handled, and there is no lenient path to maintain.

Keep the fixture test, with its assertion flipped: a folder without `formatVersion` is **rejected**.

### The migration is two edits per folder, and it is free *today*

Measured against the real library (2 band folders, no band data reproduced here):

| Folder | `formatVersion` | annotations |
|---|---|---|
| A | absent | `annotations.json`, 9 songs, **keyed by `title`**, 16 objects |
| B | absent | none |

So the work is: **add `formatVersion: 2`**, and **re-key the annotations by `slug`** into `annotations/`.

**The re-key resolves 9/9 with no ambiguity and no misses right now.** That is the argument for doing it
immediately rather than later: title→slug is a lookup that works until the first title is corrected, and
then it silently drops that song's marks. Migrating while the mapping is total costs nothing; migrating
after a rename costs annotations nobody notices are gone.

**The migration tool must fail on an unresolved title, never skip it.** A skipped song looks exactly
like a song that had no annotations.

### The file list: derive it in a directory, declare it in a zip

A hand-authored folder should not have to maintain a `files[]` that the directory already states. So:
**in the directory form the file list IS the directory**; in the zip, the manifest declares entries
because the zip is generated. This also removes the silent-chartless-import class entirely — you cannot
forget to list a file that is found by being there.

The constraint this puts on Gap 3 stands and must be honoured: derivation has to be **deterministic and
sorted**, or `zip(dir)` stops being an exact inverse and the diffability argument dies.

### Export filename ⟶ shortname (VLL's flow, accepted)

**VLL: *"l'export sauve un tband, je recommanderais un slug comme name, mais si l'utilisateur le change
alors ce sera le shortname."*** Accepted, and it is the cleanest answer to "the server has no shortname
field": **the filename is the shortname.**

- Export names the file from a **slug of the band name**.
- If the user saves it under a different name, **that name becomes the `shortname`** on import.
- Import falls back to a slug of `name` when the filename carries nothing distinct.

This gives the field a home without adding a server column for it, and it matches what the folders
already do — their directory name *is* their shortname today.

### Importing a band folder = zip it, then the normal `.tband` path

**VLL: *"pareil pour l'import des repertoire trouba bands est de zipper les folder et utiliser l'import
tband."*** Accepted — one code path, no second importer.

**With one hard condition: the zip flow must not bypass `ImportPreview`.** Importing a band **creates
accounts on your server from a file somebody handed you**. The preview is the only thing standing
between "a colleague sent me their band" and "a file I was sent decided who exists here". A convenience
wrapper that zips a folder and posts it straight through would remove that check precisely where it
matters most — the case where the input came from outside.

### How users are created on import (settled, from the code — for the format doc)

**Passwords never travel.** `.tband` carries no credential of any kind. `ImportPreview` classifies every
member first, and each one is dispositioned:

- **`create`** — a **passwordless** account. It cannot be logged into until an admin issues a reset via
  `POST /api/bands/{bandId}/members/{userId}/password-reset`.
- **`invite`** — the person is invited, not created.
- **`skip`** — not imported.

**`invite` and `skip` drop that member’s personal content**, which is a real data consequence and
belongs in the format doc rather than in the code alone.

## AMENDMENT 3 (2026-09-04) — corrections found by migrating the real folders

I migrated the two real band folders to v2. Four corrections, three of them to my own earlier rulings.

1. **"Derive the file list from the directory" (amendment 2) is WRONG — declare it.** One folder holds a
   `__pycache__` directory with no repertoire entry; under derivation it imports as a **song** carrying a
   `.pyc` as its chart. **The repertoire is the index: a directory is a song only if its slug is
   declared.** The migration tool derives; the format declares.
2. **The directory must never contain hashes.** `blobs/<sha256>` is fine for the zip and unusable for a
   browsable folder — 154 files would become hash names. The **packer** maps `<slug>/<filename>` →
   `blobs/<sha256>` at zip time and fills `blobHash`. Without it, "hand-authored folder" means "authored
   by something that can compute SHA-256".
3. **`role` collides.** Folder = free text for what a person plays. v2 = permission enum. Unknown strings
   degrade silently to `member`. Keep the prose under `plays`; the format doc must state the collision.
4. **Two shapes the format cannot yet take**: the folder keeps `admin` beside `members` (v2 wants everyone
   in `members[]`, or it drops the only privileged account), and a `personal` layer carries **no owner**
   while v2 requires one. An export must never have to guess an owner.

## PHASE 2 SPECIFICATION — the packer (written after building one and measuring it)

Phase 1 landed the wire format. Phase 2 is the piece that makes VLL's flow real: *"l'import des
repertoire trouba bands est de zipper les folder et utiliser l'import tband."* I built a working packer
as a scratch tool and ran both real band libraries through it end to end, so the requirements below are
measured rather than predicted.

### ⟨P1⟩ The mapping belongs to the TOOL. The stored folder keeps its own vocabulary.

**This is the requirement I violated myself, so it is first.** I migrated the two real folders *in place*
into v2's member shape and silently broke the seeder, which reads `display`, `role` as **free text naming
the instrument**, and `conductor`. Renaming them produced members with no display name, no instrument and
no conductor promotion — **no error, just wrong**.

So: the folder keeps `display`, prose `role`, `conductor`, and `admin` beside `members`. The **packer**
emits v2's `displayName`, the `role` enum (from `admin` + `conductor`), and `members[]` with the admin
folded in. Nothing about the v2 shape is stored in the directory.

**Regression test, non-negotiable:** one fixture folder must both **seed** (`cmd/seed`) and **pack**
successfully. If a change makes one work and the other fail, that test is the thing that says so.

### ⟨P2⟩ Verify hashes, never trust the manifest

If `repertoire.json` declares a `blobHash`, the packer **recomputes it from the file on disk and refuses
on mismatch**, naming the file. A stale manifest that packs quietly imports the *wrong bytes* under a
right-looking name — the failure nobody notices until a musician opens the wrong chart on stage.

Measured: dedup is real and worth having — 107 files packed to **106 blobs**, two charts byte-identical.

### ⟨P3⟩ The repertoire is the index

Only a `<slug>/` directory whose slug is **declared in `repertoire.json`** becomes a song. Measured in the
real library: one folder contains a `__pycache__` directory and three stray root files (a `.py`, a `.js`,
a timestamped `.bak`). Under any glob-the-directory rule, `__pycache__` imports **as a song** carrying a
`.pyc` as its chart. Declared-only makes that impossible by construction — no deny-list to maintain.

### ⟨P4⟩ Fail on anything unresolved; never skip

Applies to every mapping the packer performs — an annotation title that resolves to zero or several
repertoire entries, a `files[]` entry with no file on disk, a layer `file` ref matching no filename.
**A skipped song is indistinguishable from a song that had nothing.** Measured: title→slug resolves 9/9
today, which is exactly why it must fail loudly the first time it does not.

### ⟨P5⟩ Round-trip is an exact inverse

`unzip(pack(dir))` reproduces the directory byte-for-byte. Pin the JSON encoding — indentation, key
order, trailing newline — or a no-op round-trip shows as a diff and the "easy to diff" argument dies.
Layer ids, object uuids and object→layer bindings stay verbatim, asserted **per song**: a band-wide id set
collapses reused ids (the real library names every song's layer `L0`, which turned 41 identifiers into a
falsely-healthy 33).

### ⟨P6⟩ Size: report before uploading, not after

`MaxImportBytes` is 512 MB. Measured: the larger library packs to **133.9 MB** from 158 MB declared, so
real bands fit comfortably — but the packer should state the packed size and refuse *locally* when it
would exceed the limit, rather than discovering it in an upload.

### Acceptance

- A fixture folder **seeds and packs** (⟨P1⟩'s regression test).
- A declared `blobHash` that disagrees with the file refuses the pack, naming the file.
- A stray directory with no repertoire entry is **not** imported as a song.
- An unresolvable annotation title / missing file / dangling layer ref each fail with the name in the
  message.
- `unzip(pack(dir))` is byte-identical to the source directory.
- Round-trip preserves layers and objects **by id, per song**.
- T63's zip-bomb bounds and all-or-nothing import unchanged, tests still green.

### Note on the import preview

Earlier I told the lane the zip flow "must not bypass `ImportPreview`" as though the service enforced it.
**It does not** — `ImportBand` with nil dispositions is accepted and mints accounts, because `create` is
the documented default for an unknown username. What *is* enforced is the T62 takeover fix. So the
preview stays a **UI obligation** in whatever surface drives the packer; do not build as though the
service layer already guarantees it, and do not weaken the takeover check.

## AMENDMENT 4 (VLL, 2026-09-04) — ONE `.tband`: folder or zip, JSON + `slug/filename`, no `blobs/`

**VLL: *"je veux un seul type de tband (fichier ou folder), uniquement avec les json + slug/fichier,
j'aimerais que tout le monde utilise ca (demo, -band, ...)"*** This supersedes the blob layout that phase
1 shipped. **Revise v2 in place** — it landed today and nothing outside the repo consumes it — but as a
deliberate change: the reader still rejects v1, and the fixtures move with it.

### The layout, entire

```
band.json          formatVersion, name, members[]        (+ the folder's own keys, ignored by the reader)
repertoire.json    songs[] with files[] (filename, contentType, size, blobHash, displayOrder)
setlists.json      setlists[] with items[].song = slug
annotations/<slug>.json    { layers[], objects[] } — ids verbatim
<slug>/<filename>          the bytes, under human names
```

A `.tband` **is that directory zipped**. Nothing else. `unzip` gives the directory back.

### Why this is the right shape, measured

- **The packer stops being a translation**, so the two forms cannot drift.
- **"Hand-authored" becomes true.** `TestBandImport_HandAuthoredV2Folder` currently calls `blob.HashOf`
  to build its own fixture — a person cannot write a folder that needs SHA-256.
- **The seeder's duplicate reader dies.** `cmd/seed` hand-mirrors the manifest (*"mirrors .tband
  manifestItem.onCall"*). Two readers of one folder is exactly what let a migration break seeding while
  import stayed green. One reader serves demo, `-band` and import.
- **Dedup was buying nothing:** 107 real files → **106** blobs. One duplicate.

### ⚠ ⟨P7⟩ MANDATORY — entry names become user data

`blobs/<hex>` names were **machine-generated**, so an archive could not choose them. Under
`slug/filename` they are attacker-controllable, and `bandio.go` has **no traversal defence today** —
`sanitizeFilename` only names the export download.

**An entry is accepted only when its name is EXACTLY `<slug>/<filename>` for a slug and filename declared
in `repertoire.json`.** Matched against the manifest — not cleaned, not resolved, not `filepath.Clean`ed.
The manifest itself refuses any slug or filename containing a path separator, `..`, a leading `/`, a
drive letter, or a NUL. A zip entry that is neither a declared file nor a known JSON name is **refused**,
not ignored. Keep `blobHash` as an **integrity field** verified after reading: dropping content
addressing must not drop the checksum.

**Test it as an attack, not as a shape:** an archive declaring an innocent manifest while carrying
`../../etc/x`, an absolute path, and a name differing only by Unicode normalisation must each be refused,
by an assertion that reads as a refusal rather than an absence.

### Everything else stands

⟨P1⟩–⟨P6⟩ from the phase 2 section are unchanged, and ⟨P1⟩ (the mapping belongs to the tool, with a
fixture that both seeds and packs) matters more under this ruling, not less — it is now the *same* reader
doing both.

**Correction to something I asserted twice:** I called the seeder *"the only end-to-end exercise of the
public REST surface"*. That is **overstated** — `core/internal/httpapi` holds **26 test files, 111 test
functions**, covering register, bands, invite-links, songs and files. What the seeder uniquely provides
is not route coverage but a **real-binary, real-store, ordered scenario**. Worth keeping one thin path of;
not worth blocking a simplification over.

## AMENDMENT 5 (VLL, 2026-09-04) — the seeder keeps only what a file cannot express

**VLL: *"le seeder peut etre les phases que le fichier ne peut pas faire + just l'import du tband a la
fin."*** Right split, and it makes the demo's content data instead of Go literals. **One ordering
constraint decides whether it works.**

### ⚠ ⟨P8⟩ IMPORT FIRST, PASSWORDS SECOND — the obvious order silently destroys content

The natural reading is "create the demo users, then import the band". **That loses every member's
personal annotations.**

`classifyMembers` marks any username already on this server (and not the caller) as **`Existing`**, which
is **consent-required: invite or skip only** — the T62 takeover fix, and it is correct. But
`DispositionInvite` and `DispositionSkip` both **drop that member's personal content** (annotations, cues,
selections — `bandio.go:533`, `:587`). So pre-creating `marie` and friends makes them un-attachable, and
the import quietly seeds a demo with shared and conductor content only, every personal layer gone. It
does not fail; it increments `DroppedLayers` and returns 200.

**So the order is:**

1. **Register ONE account** — the importer, who becomes the band admin. This one needs a password,
   because someone must be able to log in and drive the import.
2. **`POST /api/bands/import`** the `.tband`, dispositioning every other member `create`. They arrive as
   **passwordless** accounts **with their personal content intact**.
3. **Then give them passwords**, over the public API: admin issues
   `POST /api/bands/{bandId}/members/{userId}/password-reset` → the returned token is submitted to
   `POST /api/password-reset/{token}` with the demo password. Two calls per member, no private surface.

### What a file genuinely cannot express (so it stays in the seeder)

- **Passwords.** `.tband` carries no credential of any kind, by design. A demo you can log into needs
  step 3 above.
- **Baked concerts.** v1's rule stands — bakes are rebuilt on the target, never carried.
- **Anything derived**: rendercache, generated-chart artefacts that are regenerated on demand.

Everything else — band, members, repertoire, files, setlists, annotations — is in the file.

### What this costs, honestly

The `-band` and demo paths stop driving register → create band → invite → accept → create song → upload
in sequence. Given the 111 `httpapi` tests, that is a loss of scenario, not of coverage.

**And VLL closes even that** — *"on peut toujours deriver un test d'integration a partir des donnees de
demo si vraiment on veut tester toute l'api."* Once the demo is a folder, the long REST path is a
**function of that data**: walk the folder and replay it as register → create band → invite → accept →
create song → upload → import annotations.

**This inverts the concern rather than answering it.** A hand-written scenario drifts from the content it
was written against; a derived one cannot, because there is one source. So the sequence is not something
to preserve at a cost — it becomes cheaper and truer than what exists today. **Concern closed; I raised
it three times and it was wrong to keep raising it.** Build it only if someone wants it, not as a
condition on the split.

## Then, and only then: the demo

Once annotations are expressible, move the demo's groups out of Go literals (`groupDef`) into a folder
under the same format. **That is the real completeness test** — anything the demo cannot express is a
remaining gap in the format, discovered rather than predicted.

**Keep the seeder as an end-to-end test.** It drives `register → login → create band → invite → accept →
create song → upload file` over the public REST API and its own header calls it *"an end-to-end smoke
test of the normal web surface"*. If the demo becomes pure data, something must still exercise that
path, or we trade a test for convenience without noticing.

## Do not

- **Do not introduce a second extension** (`ztband`, `tbandz`). One name, versioned inside.
- Do not re-mint annotation ids "for consistency" with the name-based layer.
- Do not include baked concerts (v1's rule stands — rebake on the target).
- Do not weaken T63's zip-bomb bounds (`maxImportEntries`, `maxDecompressedBytes`) or the
  all-or-nothing validation.

## Done when

- A v2 export round-trips: export → import → **the same layers and objects, by id**, not merely the same
  count.
- **A v1 fixture still imports**, asserted by a test; an unknown **or absent** `formatVersion` is refused
  (amendment 2 — absent is an ERROR, not a legacy fallback).
- A band seeded from a folder that includes `annotations/` arrives **with its annotations** — the case
  that is impossible today.
- **`unzip(tband)` IS the directory** — no hash anywhere in it, byte-identical to the source folder
  (amendment 4), and hand-writable without computing a checksum.
- **⟨P7⟩ entry names, tested as an attack**: an archive whose manifest looks innocent but whose entries
  carry `../../etc/x`, an absolute path, or a Unicode-normalisation twin is **refused**, each by an
  assertion that reads as a refusal.
- **⟨P1⟩ one fixture folder both SEEDS and PACKS** — the regression that catches a v2 change breaking
  `cmd/seed`.
- **⟨P8⟩ import before passwords**: a seeded demo comes up with its personal layers intact
  (`DroppedLayers == 0`), which pre-creating the members would silently lose.
- T63's limits and the all-or-nothing import are unchanged, with their tests still green.
- `gofmt -l core` empty; `go test ./...` green with the count stated.
