# T72 — A text chart's pool name is derived from the title and clobbered on every edit (BUG)

**Priority:** high — data-losing from the user's point of view (a rename they made is
silently undone) · **Size:** S · **Area:** `core/internal/app/service.go` (+ a service
test). Found in live use, 2026-08-19.

## The bug (verified in the code)

`CreateTextChart` sets `Filename = chartpdf.Title(source) + ".pdf"` (`service.go:1175`),
and the chart-source **update** path sets it *again* from the title on every save
(`service.go:1255`). Consequences:

1. **A user rename is reverted.** Rename the part to `Guitar/Bass` via
   `PATCH …/files/{id}`, then edit the chart and save: the filename snaps back to
   `<Song Title>.pdf`. Confirmed live. A song with two chart parts cannot keep them
   distinguishable — which is exactly what multi-part songs need.
2. **The name is misleading.** A text chart is source, not an upload; calling it
   `<Title>.pdf` implies an uploaded PDF and duplicates the song title in the pool.

## Design (decided)

- **The default is a create-time default, not an invariant.** `CreateTextChart` sets
  `Filename = chartpdf.Title(source)` — **no `.pdf` suffix** (the pool renders it as a
  generated chart; the extension adds nothing but a false implication).
- **The update path must not touch `Filename` at all.** Saving a source updates the blob,
  size and revision. The name belongs to the user from the moment the file exists. This is
  the actual fix; everything else here is tidying.
- **No migration.** Existing files keep whatever name they have (including `…​.pdf`); a
  rename now sticks. Rewriting historical filenames would be a second surprise, and the
  names are user-visible data we don't own.
- **Renaming stays the existing `PATCH …/files/{id}` path** — no new endpoint, no new field.

## Acceptance criteria

- **Red-first service test**, the reported scenario end to end: create a text chart →
  `PATCH` its filename to `Guitar/Bass` → update the chart source → the filename is still
  `Guitar/Bass`, and the blob hash / size / revision did change. This test must fail on
  today's code before the fix.
- A newly created text chart's default filename is the chart title with **no `.pdf`**;
  a chart with no `# Title` still falls back to `Chart` (via `chartpdf.Title`).
- Two text charts on one song keep distinct names across edits of either.
- Nothing else about the file model changes: `Generated`, `ContentType`, `DisplayOrder`
  semantics untouched; the bake/pool ordering is unaffected.
- `gofmt -l core` clean; `go vet`; `make test` green.

## Out of scope

- Renaming existing charts in place (no migration, by decision above).
- Any Studio change — the rename UI already exists and starts working once the server stops
  overwriting it. If a UI gap turns up, file it separately.
- Multi-part seeding (that is **B15**, which depends on this).
