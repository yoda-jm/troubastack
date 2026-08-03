# T67 — "Save chart" shows a stale render (revision-agnostic file URL + no refetch + no cache headers)

**Lane:** web-core (core + studio) · **Size:** M · **Status:** SPEC'd 2026-08-03 (VLL bug report) · **Priority:** HIGH (correctness — a user edits a chart and never sees the change; risks baking a stale chart) · **Depends on:** nothing open

## The bug (VLL, verified)

Edit a generated text-chart's source (e.g. add a chord), click **Save chart** → the source IS persisted **and the server DOES re-render the PDF** (`Service.SaveChartSource`, `core/internal/app/service.go:1233` — `chartpdf.Render(source)` at :1244, new blob, `f.Revision++`, `UpdateSongFile`, `SetChartSource`). But the viewer keeps showing the OLD render, **even after a hard refresh (F5)**. Only a cache-bypassing reload (Ctrl+Shift+R) shows the new one.

## Root cause (three compounding client/transport gaps — all verified)

1. **The file URL is revision-agnostic:** `api.fileUrl = /api/files/${fileId}` (`web/studio/src/api.ts:544`) — same URL for every revision of a file.
2. **`downloadFile` sets NO cache headers:** `core/internal/httpapi/webapi.go:895-906` sets only Content-Type + Content-Disposition. No `Cache-Control`, `ETag`, or `Last-Modified`. So the browser's heuristic HTTP cache freely serves the previously-fetched bytes for that identical URL — which is why even F5 stays stale (F5 revalidates but there's nothing to revalidate against; only Ctrl+Shift+R bypasses).
3. **The viewer never refetches after save:** the chart editor's `onDone` (`SongDetails.tsx:339-342`) reloads only the `Files` panel's own list (`api.listFiles`), never the Viewer's file state. The Viewer's `selectedFile` object reference doesn't change, so `usePdfDocument`'s load effect (keyed on the `SongFile` object, fetch at `usePdfDocument.ts:132` using `selectedFile.id` only) never re-runs.

## The fix (make a new revision a new URL, refetch after save, cache safely)

**Do all three — they reinforce each other; #1 alone fixes both the stale-after-F5 AND the no-refetch-in-session symptom by construction:**

1. **Revision-aware file URL (studio + wherever fileUrl is used).** `api.fileUrl(fileId, revision?)` → `/api/files/${fileId}?rev=${revision}` when a revision is known. A re-rendered chart bumps `revision`, so the URL changes → the browser can't serve the stale cached bytes AND `usePdfDocument`'s fetch URL changes → the load effect re-runs. Thread the selected file's `revision` into the fetch (`usePdfDocument.ts` — key/fetch on `selectedFile.revision` too, not just `.id`). Grep every `fileUrl(`/`/api/files/` caller (viewer, download links, preview) and pass the revision where the SongFile is in hand.
2. **Cache headers on `downloadFile` (core, `webapi.go:895-906`).** Set `Cache-Control: no-cache` (revalidate) OR, better paired with #1's `?rev=`, `Cache-Control: public, max-age=…, immutable` (a `{id, rev}` URL is genuinely immutable — the bytes for that revision never change). Add an `ETag` (the blobHash is a perfect strong ETag) and honor `If-None-Match` → 304. Pick one coherent policy: **recommended = `?rev` immutable caching + ETag=blobHash** (fast, correct, no stale — a new rev is a new URL). Same treatment for the analogous inline-PDF responses if they share the path.
3. **Refetch the viewer after a chart save.** Route the chart-editor save completion to the Viewer's `refreshMyFiles` (`Viewer.tsx:214`), not just the Files panel's local `load`. After `SaveChartSource` returns the bumped `SongFile`, the viewer's `files`/`selectedFile` must pick up the new `revision` so the render refetches in-session (no manual refresh needed). Keep the current selection (don't jump to another file on save).

## Out of scope
- Any change to the server RE-RENDER (it's correct — this is transport/cache/refetch only).
- The transposed-chart / bake paths (they already produce fresh bytes; but if they share `fileUrl`, they inherit the revision fix for free — fine).
- App/iOS — the app performs baked bundles, not live chart files; no app work.

## Acceptance
1. **e2e (studio):** open a generated chart in the viewer; edit the source (add a line/chord), Save chart; **the viewer's rendered page updates WITHOUT a manual refresh** (assert the page re-rendered — e.g. the fetched file URL now carries the new `?rev`, or the rendered page's content/box changed). Red-first: fails on today's revision-agnostic URL + no-refetch.
2. **Hard-refresh (F5) freshness:** after a save, a full reload shows the NEW render (assert the served file responds with the revision-aware URL / correct cache headers so F5 can't serve stale). A test hitting `downloadFile` asserts the cache/ETag headers are present.
3. httpapi: `downloadFile` returns the chosen cache policy + `ETag`; `If-None-Match` with the current blobHash → 304.
4. `go test ./...` + gofmt + vet green; tsc + studio build clean; existing text-chart e2e (`text-chart`, `editor-transpose`) stay green; no dist churn.

## Notes for the executor
- The cleanest single lever is #1 (revision in the URL) — it makes "new render = new URL" true, which fixes BOTH the browser-cache staleness and the effect-doesn't-re-run staleness at once. #2 (headers) and #3 (refetch wiring) make it robust and in-session-instant. Do all three.
- Verify no OTHER consumer relies on the bare `/api/files/{id}` URL being stable (e.g. a cached blob key elsewhere) — grep before changing the signature.
- Present at the gate; cite VLL 2026-08-03 (via Fable).
