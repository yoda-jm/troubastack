# B14 — Seed your OWN band from a local folder (`bands/<slug>/`, never committed)

**Priority:** normal (VLL 2026-08-16: seed his real band locally, rebuildable on every
recreate, kept out of the public demo) · **Size:** S/M — **the implementation already
exists**, uncommitted in the primary worktree; this task lands it as a *product feature*
with the naming, docs and tests it needs · **Area:** `core/cmd/seed`, `Makefile`,
`.gitignore`, `docs/`.

## Context

The seed's groups were hardcoded demo content. The uncommitted work adds a generic hook:
a gitignored `bands/` folder, one subfolder per real band, discovered at seed time. That
is a genuinely general capability — *any* user can seed their own band from a folder
without touching Go — so it should land as a documented feature, not as a personal
one-off. Reviewed 2026-08-16 (`docs/handoff/reviews.md`): no personal band data leaks
into code; the only issue is that a personal band's handle is used as the example string.

**Land it after B13 (done, `42c4ed9`)** — the `cmd/seed/main.go` overlap is gone, so this
branches cleanly off `main`.

## Design (decided — mostly ratifying what exists)

1. **Discovery.** `loadLocalBands()` scans `<bandsDir>/*/band.json`; `localBandsDir()`
   resolves `../bands`, `bands`, … relative to the seed's cwd, overridable with
   **`TROUBA_BANDS_DIR`**. No folder → nothing loaded (demo-only seed, unchanged).
2. **Manifest.** `band.json`: `name` (required), `shortname`, `kind` (default `"Band"`),
   `notes`, `admin{username,display,role}` (required), `members[]`. `repertoire.json`:
   `songs[]` of `{slug,title,artist,key,tempo,notes,tags}`. Per song, `bands/<band>/<slug>/`
   supplies `*.pdf` parts (sorted, deterministic) and an optional `lyrics.txt` authored in
   the chart dialect, fed through the existing text-chart path (T19). A song with neither
   is created **metadata-only** — that is intended, not an error.
3. **Isolation from the demo — the safety property.** Every discovered band is marked
   `personal: true`, and a plain `seed` / `make demo` seeds **only non-personal groups**.
   A personal band is built only when explicitly selected: `-band <shortname>` or
   `-only <name-substring>`. Its members are registered only then, too.
4. **Runner.** `make band=<shortname>` seeds that band into its own data dir
   (`core/troubadata-<shortname>`) and serves it, so recreating the server rebuilds it
   cleanly. `make` with no args still prints help (`.DEFAULT_GOAL` only flips when `band`
   is set).
5. **`bands/` is gitignored in full** — real repertoires contain copyrighted PDFs and
   lyrics. Nothing under it may ever be committed.
6. **Naming (the one review change):** replace the personal-band example everywhere with a
   neutral placeholder — the `-band` flag help, the Makefile comment/usage line, and the
   code comment must read `<shortname>` / `myband`, never a real band's handle. A public
   repo should not carry someone's band name as sample text.

## Acceptance criteria

- a grep for the real band's name and handle over the committed tree returns **nothing** (the feature is
  generic; the example strings are neutral).
- Fresh clone with **no** `bands/` folder: `make demo` and `cd core && go run ./cmd/seed`
  behave exactly as before (demo groups only) — assert in a test that `loadLocalBands()`
  on a missing dir returns `(nil, nil, nil)`.
- **Demo isolation test (the important one):** with a temp `TROUBA_BANDS_DIR` containing a
  fixture band, a default seed run selects **zero** personal groups and **none** of that
  band's members; `-band <shortname>` selects exactly that band and its people. Table-driven
  over the three selection modes (`-band`, `-only`, neither).
- Manifest handling: missing `band.json` in a subfolder is skipped silently; malformed JSON
  and a manifest missing `name`/`admin.username` fail with a path-qualified error; absent
  `repertoire.json` yields no songs; `shortname` falls back to the folder name.
- `-band` with no match exits with the actionable error naming `band.json shortname`.
- Deterministic: the PDF glob is sorted; re-running the seed against the same server is
  idempotent (same rule as the demo groups).
- **Docs:** a `docs/` section (README or a short `docs/local-bands.md`, linked from the
  root README) covering the folder layout, a complete minimal `band.json` +
  `repertoire.json` example, `make band=<shortname>`, `TROUBA_BANDS_DIR`, the data-dir
  convention, and an explicit note that `bands/` is **never committed** and may hold
  copyrighted material.
- `gofmt -l core` clean; `go vet`; `make test` green.

## Out of scope

- Any real band's data (it lives only in the contributor's gitignored folder).
- A UI for creating bands from folders; import/export of band folders; syncing them.
- Changing the demo dataset itself.

## Sequencing

1. Neutralize the example strings; land the loader + `-band` + Makefile target + `.gitignore`.
2. Add the isolation/manifest tests (fixture band under a temp `TROUBA_BANDS_DIR`).
3. Write the docs section. One commit.
