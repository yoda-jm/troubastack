# T79 — One way to add a file: three entries, one shape — plus good default names

**Priority:** normal · **Size:** M · **Area:** `web/studio` + a small `core` change for the
download filename. Agreed in discussion with VLL 2026-08-19/20.

## Why

The three ways to add a part are three different interaction models today:

| entry | today |
|---|---|
| `new-text-chart` | jumps **straight into the editor** with a hardcoded `# New chart` stub; asks nothing |
| `new-lyrics-chart` | opens a **dialog**: name, URL + fetch, paste, sections toggle, Create |
| upload | an **inline form** with a file input and an Upload button |

All three produce the same thing — a new part in the pool — so they should look and land the
same. VLL wants the three entries kept (not merged), made homogeneous.

## Design (decided)

1. **One button group, one dialog shell.** The three entries sit together in the Files
   header, styled identically, and each opens the *same* dialog shell: a name field, a
   source area (differing per entry), one primary action, and the same landing behaviour —
   the new file is appended to the pool, visible immediately in the T78 list.
2. **Good defaults, rename later** (VLL: *"nice default, rename later if the user wants
   something else"*) — so no path forces the user to name anything up front:
   - **upload** → the uploaded filename **without its extension**;
   - **from lyrics** → the name the user typed, else the fetched song's title;
   - **from scratch** → **the song's own title**, not `New chart`.
3. **Fix the naming wart this creates today.** T72 correctly made the pool name a create-time
   default that never re-derives — but the from-scratch path creates the file while the
   source still reads `# New chart`, so retitling the chart to *Hotel California* leaves the
   row reading **"New chart"** forever. Defaulting the stub to the song's title (both the
   `# Title` line and the pool name) fixes it at the source. Do **not** "follow the title
   while it is still the default" — that reintroduces the clobbering T72 removed.
4. **Extensions are stripped when a file lands.** A part is a part; `.pdf`/`.txt` in the pool
   name is noise, and text charts already dropped it in T72. Applies to new files only —
   **no migration** for existing names (VLL confirmed; consistent with T72).
5. **Re-append the extension at the HTTP boundary.** Investigated (2026-08-19), the exposure
   is exactly one place:
   - `.tband` export names entries `band.json` + `blobs/<hash>` — **unaffected**;
   - `.tstage` bundle blobs are keyed by song/page/layer + hash — **unaffected**;
   - Studio's `download=` links are only bake artefacts with their own extensions —
     **unaffected**;
   - **`webapi.go:920` serves song-file bytes as `Content-Disposition: inline;
     filename="<Filename>"`.** It is `inline` (the viewer renders it), but that name is the
     hint the browser uses for *Save as* from the PDF viewer — so a stripped name would save
     as `Hotel California` with no extension.

   **Decision:** keep the stored/display name clean and derive the extension from
   `ContentType` when writing the header (`application/pdf` → `.pdf`). Storage stays tidy;
   saving from the viewer still yields a usable file.

## Acceptance criteria

- The three entries are visually and behaviourally homogeneous: same placement and styling,
  same dialog shell, same primary action, same landing (appended, visible, named).
- Default names per §2, verified for each entry; a from-scratch chart on song *Hotel
  California* lands named **Hotel California** (not "New chart"), and its `# Title` matches.
- Uploading `scan_001.pdf` lands as **`scan_001`**; uploading `Hotel California.pdf` lands as
  **`Hotel California`**.
- Server: `Content-Disposition` for a song file carries `<name>.pdf` for a PDF even though the
  stored name has no extension — unit test on the header for a stored name with and without
  an extension (existing files keep theirs and must not double up: `x.pdf` → `x.pdf`, never
  `x.pdf.pdf`).
- No migration: an existing file named `Foo.pdf` is untouched.
- Existing lyrics-dialog behaviour (fetch/paste/sections/create) and its testids survive; the
  T71 search row, when it lands, drops into the same shell.
- Testids for the unified affordances; e2e covering each of the three entries end-to-end
  (create → appears in the list with the expected default name).
- `tsc -b studio` clean; `make e2e` green; `gofmt -l core`, `go vet`, `make test` for the
  server half.
- Before/after screenshots of the Files header + each dialog in the handoff.

## Out of scope

- Merging from-scratch and from-lyrics into one entry (proposed; VLL chose to keep three).
- The matrix (parked) and per-member `my-files`.
- The list/row-menu presentation itself (**T78**).
- Renaming existing files or any bulk rename tool.
