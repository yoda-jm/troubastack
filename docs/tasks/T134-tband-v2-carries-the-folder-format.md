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
| chart files | ✅ `<song-slug>/` folders | ✅ `blobs/<sha256>` |
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
band.json          name, shortname, kind, notes, admin, members[]     ← folder format, unchanged
repertoire.json    songs[] {slug, title, artist, key, tempo, meter, notes, tags, files[]}
setlists.json      concerts[] {name, eventDate, venue, notes, items[]}
annotations/<slug>.json    per song: {layers[], objects[]}            ← the NEW part
cues.json          song cues, file selections                          (optional)
blobs/<sha256>     file bytes, content-addressed                       ← unchanged from v1
```

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
- **A v1 fixture still imports**, asserted by a test; an unknown `formatVersion` is refused with 400.
- A band seeded from a folder that includes `annotations/` arrives **with its annotations** — the case
  that is impossible today.
- The zip is hand-inspectable: unzip it and the JSON is the folder format, readable and diffable.
- T63's limits and the all-or-nothing import are unchanged, with their tests still green.
- `gofmt -l core` empty; `go test ./...` green with the count stated.
