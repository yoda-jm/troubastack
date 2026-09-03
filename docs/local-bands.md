# Seed your own band from a local folder

The demo dataset (`make demo`) is fixed, copyright-safe content. To run TroubaStack with **your
own** band instead — its members and repertoire — drop a folder under `bands/` and seed it. This
never touches Go code, and **`bands/` is gitignored in full**: real repertoires contain
copyrighted PDFs and lyrics, so nothing under `bands/` is ever committed.

## Layout

```
bands/
  <slug>/                 one folder per band (the slug is your handle for it)
    band.json             band + members (required)
    repertoire.json       the song list (optional)
    <song-slug>/          per-song source files (optional)
      chart.pdf           sheet music / tab — any *.pdf, uploaded in filename order
      lyrics.txt          lyrics in the chart dialect (# Title / ## Section / lines)
      guitar-bass.txt     …any number of *.txt parts (see below)
```

**Each `*.txt` in a song folder is its own chart part**, named after the file (so
`guitar-bass.txt` → a part called "guitar-bass" in the pool). `lyrics.txt` is created first (the
song's default part); the rest follow in sorted filename order. So a song can carry *Lyrics* and
*Guitar/Bass* side by side, and the folder reproduces both. Rename a part by renaming its file.

A song with no PDF and no `*.txt` is created **metadata-only** (title/artist only) — that's fine,
fill it in later.

### `band.json`

```json
{
  "name": "My Band",
  "shortname": "myband",
  "kind": "Band",
  "notes": "optional blurb",
  "admin":   {"username": "alice", "display": "Alice", "role": "bass + vocals"},
  "members": [
    {"username": "bob",  "display": "Bob",  "role": "drums"},
    {"username": "cara", "display": "Cara", "role": "guitar + vocals"}
  ]
}
```

`name` and `admin.username` are required. `shortname` defaults to the folder name. `kind` defaults
to `"Band"`.

### `repertoire.json`

```json
{
  "songs": [
    {"slug": "song-one", "title": "Song One", "artist": "…", "key": "G", "tempo": 120,
     "tags": ["cover"], "notes": ""},
    {"slug": "song-two", "title": "Song Two", "artist": "…"}
  ]
}
```

Per song, `bands/<slug>/<song-slug>/` supplies the source files (`*.pdf`, `lyrics.txt`).

## Running it

```sh
make band=myband       # boots a server seeded with ONLY that band, then serves it
```

- Everyone signs in with the seed password (`demo` by default).
- The band gets its **own data dir** under the runtime root (T129):
  `$TROUBA_HOME/troubadata-<shortname>` — default `~/.local/share/troubastack/troubadata-<shortname>`,
  **outside the source tree** so a `git clean` can't erase it — so recreating the server rebuilds your
  band cleanly and separately from the demo. Reset with
  `rm -rf ~/.local/share/troubastack/troubadata-<shortname>` (or wherever `TROUBA_HOME` points).
- `make` with no arguments still prints the help; the `band` target only becomes the default when
  `band=` is set.

## Isolation from the demo

Every folder-discovered band is **personal**: a plain `make demo` / `go run ./cmd/seed` seeds only
the built-in demo groups and **never** a personal band or its members. A personal band is built
only when explicitly selected — `-band <shortname>` (what `make band=` uses) or `-only <name>`.

## Environment

- **`TROUBA_BANDS_DIR`** — override where bands are discovered; wins outright. Unset (T129), the
  search is `$TROUBA_HOME/bands` first (default `~/.local/share/troubastack/bands`, **outside the
  source tree**), then the historical in-tree locations (`../bands`, `bands`, …) so an existing
  checkout keeps working. Keeping the real library outside the repo — e.g. `~/troubastack-bands` —
  means a `git clean -xdf` can never touch it.
- **`TROUBA_HOME`** — the runtime root for all non-repo data; default
  `${XDG_DATA_HOME:-~/.local/share}/troubastack`. Nothing under it is regenerable from the
  repository, so back it up like the irreplaceable data it holds.
