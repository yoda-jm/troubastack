# T69 — Self-heal a generated chart whose rendered PDF blob is missing (404 recovery)

**Lane:** web-core (core + studio) · **Size:** M · **Status:** SPEC'd 2026-08-03 (VLL production 404s) · **Priority:** HIGH (data-availability — files 404 with no recovery) · **Depends on:** nothing open

## The report (VLL, production box `troubashare.leligeour.net:8080`, Studio `45d2ca2`)

On a song page, files 404: `GET /api/files/{id}` and `GET /api/files/{id}?rev=4` both return 404 ("pdf not found"). The file RECORD exists (it appears in the song's file list with a revision, hence the `?rev=4`), but the download 404s.

## Diagnosis (grounded — NOT the current code)

- `DownloadSongFile` (`service.go`) 404s when `GetSongFile` fails **or** `blobs.Get(f.BlobHash)` fails. Since the file shows in the list with rev 4, the RECORD exists (`GetSongFile` succeeds) → the 404 is **`blobs.Get` failing: the PDF blob is missing from the blob store.**
- **The current code does not orphan blobs** — reproduced on `45d2ca2` (Fable, 2026-08-03): re-render a chart 4×, edit one of two blob-sharing files, delete a blob-sharing file, delete a blob-sharing song → the blob is always kept while any file references it (`derefBlob` checks `FilesWithBlob` after the file is updated to the new hash). `troubacore gc`/`PruneOutputs` operates only on `bakesDir` (baked concerts), never the blob store. So the missing blobs on VLL's box are **orphaned historical data** (an older build's deref bug since fixed, or a partial backup/restore that kept the repo — file records + chart sources — but lost blob files).
- **Recoverable:** the 404'd files are GENERATED charts (they carry a rev), and the chart SOURCE is stored in the repo (`GetChartSource`), **separate from the rendered blob**. So a generated chart whose PDF blob is missing can be re-rendered from its stored source — the content is not lost. (An UPLOADED PDF whose blob is missing IS lost — bytes gone, needs re-upload.)

## The fix — make a generated chart un-404-able (its render is derivable from source)

A generated chart's blob is a *cache* of `Render(source)`; the source is the source of truth. So a missing blob should self-heal, never dead-end.

1. **Auto-heal on download (core, primary).** In `DownloadSongFile` (or the download handler), when `blobs.Get(f.BlobHash)` fails AND the file is `Generated` AND `GetChartSource(f.ID)` returns a source: **re-render (`chartpdf.Render`), `blobs.Put`, update the file's `BlobHash`/`Size` (revision NOT bumped — it's the same logical revision, just re-materialized), and serve the fresh bytes.** The 404 becomes a transparent recovery on the next view. Idempotent; content-addressed Put means re-rendering identical source restores the exact prior blob hash if the render is deterministic (it is — pinned dates), so the `?rev` URL stays valid.
2. **Graceful handling for the genuinely-lost case (studio).** If the blob is missing AND (not generated, or no source) — an uploaded PDF whose bytes are gone — the viewer must NOT dead-end on a raw 404. Show a clear inline state ("This file's data is missing" + for uploaded files, "re-upload to restore"), keep the file LIST and the rest of the song usable (other files still open). Don't let one broken file blank the editor.
3. **Batch repair command (core, ops).** A `troubacore repair-blobs` (or fold into `gc`) that scans all SongFile records, finds those whose blob is missing, and for generated charts re-renders from source to restore them; reports the uploaded-file casualties it can't fix. VLL runs it once on his box to heal all broken charts in one pass (immediate production unblock beyond the per-view auto-heal).

## Out of scope
- Recovering uploaded (non-generated) PDF bytes whose blob is gone — genuinely lost; the fix is graceful reporting + re-upload, not magic.
- Root-causing VLL's specific historical data loss (the current code doesn't reproduce it; this task makes the symptom self-healing regardless of cause).
- Changing the deref logic (it's correct — verified).

## Acceptance
1. **Core auto-heal test:** create a generated chart, delete its blob directly from the store (simulate the orphan), `DownloadSongFile` → succeeds (re-rendered from source), served bytes = `Render(source)`; the file's BlobHash is restored. A NON-generated file with a deleted blob → still 404/clear error (nothing to heal). Both backends.
2. **Repair command:** seed a store with a generated chart whose blob is removed → `repair-blobs` restores it (downloadable after); reports the count fixed + any unfixable uploaded files.
3. **Studio:** a file whose render 404s (non-generated, unhealable) shows the inline "data missing" state, the file list still renders, other files still open (no blank editor). e2e or component test.
4. `go test ./...` + gofmt + vet green; tsc/build clean; no dist churn.

## Notes for the executor
- The auto-heal (#1) is the one that directly un-404s VLL's box on next load — prioritize it. The repair command (#3) heals his box in one pass without waiting for each file to be viewed. #2 (studio graceful) covers the truly-lost uploaded case.
- IMMEDIATE manual workaround for VLL before this lands (no new code): for each 404'd chart, open its source in the editor and click **Save chart** — `SaveChartSource` re-renders from the current source + re-stores the blob, restoring the file. (Confirm the "edit source" path is reachable when the render 404s — the details-panel file row should offer it independent of the viewer.)
- Present at the gate; cite VLL 2026-08-03 (via Fable).
