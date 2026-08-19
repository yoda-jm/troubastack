# B15 — Seed **all** a song's chart parts from its folder (`<slug>/*.txt`)

**Priority:** normal · **Size:** S · **Area:** `core/cmd/seed` (B14's loader) + `docs/local-bands.md`.
**Depends on T72** (a seeded part's name must survive; today the source-update path would
overwrite it, and the semantics of the default name change there).

## Context

B14 wires **one** `lyrics.txt` per song folder. A real song now carries several chart parts
(e.g. *Lyrics* and *Guitar/Bass*), so a band folder cannot reproduce its own songs: the
second part is authored online and lost on every recreate — which defeats the point of B14
("recreating the server rebuilds it cleanly").

## Design (decided)

- **Every `<slug>/*.txt` becomes its own text-chart part**, in sorted filename order, each
  created through the existing `POST …/text-charts` path.
- **The part's pool name is the file's basename without `.txt`**, set once at create (T72
  makes that stick). The contributor names the part by naming the file — no extra manifest
  syntax, and it round-trips: what you see in the pool is the file you edit.
- **`lyrics.txt` keeps priority**: if present it is created **first** (DisplayOrder 0) so the
  default part of a song stays the lyrics chart, as today. Remaining `*.txt` follow in sorted
  order.
- **Back-compatible**: a folder with only `lyrics.txt` behaves exactly as it does now; a
  folder with no `.txt` is unchanged (PDF parts and/or metadata-only).
- PDFs are untouched — `*.pdf` parts continue to load as before, ordering unchanged.

## Acceptance criteria

- Loader test (temp `TROUBA_BANDS_DIR` fixture, in the B14 style): a song folder with
  `lyrics.txt` + `guitar-bass.txt` yields two chart sources, `lyrics.txt` first, names
  `lyrics` and `guitar-bass`; a folder with only `lyrics.txt` yields exactly one (unchanged);
  a folder with none yields none.
- Live check in the handoff (the B14 precedent): seed a fixture band with a two-part song and
  show both parts in the song's pool with their distinct names, **and** that re-running the
  seed is idempotent — no duplicate parts.
- The demo seed is untouched: `go test ./cmd/seed/` green including the B13 anchor/ink suites
  and B14's `TestSelectGroups` isolation property.
- `docs/local-bands.md` updated: the per-song folder section documents that **each `*.txt` is
  one chart part named after the file**, with `lyrics.txt` first.
- `gofmt -l core` clean; `go vet`.

## Out of scope

- Per-part metadata (key/tempo overrides per part) — the song carries that.
- Ordering control beyond "lyrics first, then sorted" (rename the files if you want a
  different order).
- Uploading a part as anything other than a text chart.
