# T152 — The band export drops `shortname`, `kind` and `notes`

**Lane:** web-core (core). **Size:** S. **Status:** spec, 2026-09-05. Found while exporting VLL's live band
back into his folder.

## What happens

`ExportBand` writes a `band.json` with **`{exportedAt, formatVersion, members, name}`**. The folder it came
from has **`{formatVersion, kind, members, name, notes, shortname}`**.

So a round-trip **loses three declared fields**, and one of them is load-bearing:

- **`shortname`** — the handle a human uses (`make band=<shortname>`), the key `cmd/seed` matches on, and
  **the identity T150 is about to build on**. An export/import cycle silently unnames the band.
- **`kind`** and **`notes`** — author-declared, quietly dropped.

This is **T139 again, one field up**: an identifier that is *declared* by the author must survive the
round-trip. There we fixed it for song slugs; the band's own handle still evaporates.

## How it was caught, which is the part worth keeping

I exported VLL's band to write three server-side setlist additions back into his folder. Rather than
overwrite, I diffed first — and `band.json` was in the changed set. **Had I copied the export wholesale, his
`shortname` would have been erased** and the next `make band=altoband` would have failed to match anything.

**So: never restore a folder from an export without diffing it first**, and this task is what makes that
caution unnecessary.

## Required

- Export emits every declared field it read: `shortname`, `kind`, `notes` alongside the rest.
- Import already tolerates their absence; that stays true for older exports.
- `exportedAt` is fine as export-only metadata — it describes the act, not the band.

## ⟨R1⟩ Red first

- **Round-trip a folder that declares all six fields and assert the exported `band.json` still carries
  them.** Red today on three. Assert the *values*, not just presence.
- A second import of the export produces a band that `-band <shortname>` still matches — the failure a
  musician would actually meet.
- **Teeth-check:** drop one field from the exporter and confirm the test names that field.

## Out of scope

Adding an `id` to `band.json` — that is **T150 ⟨D2⟩**, and it will need the same round-trip guarantee, so
land this first or the identity it introduces will be erased by the very next export.
