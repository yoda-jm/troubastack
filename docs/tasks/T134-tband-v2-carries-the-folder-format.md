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

### Versioning — VLL: *"mets en place dès le début une version"*

`formatVersion` already exists (`BandExportFormatVersion = 1`) and T62's ruling was **reject anything
else, no migration code**. Keep the field; change the ruling **narrowly**:

- **Write v2 always.**
- **Read v1 and v2.** v1 → v2 is a pure rearrangement of the same information — no data is invented or
  lost — so the reader is a mapping, not a migration engine. Refusing v1 would strand exports that
  already exist for the price of a small function.
- **Reject anything else with a 400**, unchanged.
- Add a test that a **v1 fixture still imports**, and one that an unknown version is refused. The v1
  fixture is the whole point of keeping the field: without it, "we support v1" is an assertion.

## ⚠ Say what an export is — it is a snapshot, not a backup

`bandio.go:142`: *"Tombstones are dropped (head-only, no history)."* That is correct for migration, and
it is **not** a backup: the live `.jsonl` logs hold the full history, and an export holds the current
state. Most of the 554 KB → 17.6 KB difference is this, not staleness — I attributed it to staleness
first and was wrong.

**Whatever UI offers this must not call it a backup.** Someone who exports, deletes, and re-imports
loses their annotation history and will not know until they look for it.

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
