# T68 — Persist the open file in the URL (F5 no longer resets to the first file)

**Lane:** web-core (studio only) · **Size:** S/M · **Status:** SPEC'd 2026-08-03 (VLL request) · **Depends on:** nothing (independent of T67; both touch Viewer.tsx — rebase/coordinate)

## What VLL asked for

On a hard refresh (F5) the song editor resets to the **first file** of the song, losing whichever file you had open. VLL: *"a hard refresh F5 I think I am placed back on the first file of the song again — it could be nice if the URL embed in what file we are."* Put the open file in the URL so a refresh restores it.

## Facts (verified)

- Route `/bands/:bandId/songs/:songId` → `SongEditor` (`web/studio/src/App.tsx:35`), which reads only band/song from `useParams` (`SongEditor.tsx:25`) — no file param, no query string anywhere in the editor tree.
- The open file is in-memory only: `const [selectedFileId, setSelectedFileId] = useState<string | null>(null)` (`Viewer.tsx:103`); `selectedFile = files.find(f => f.id === selectedFileId)` (`Viewer.tsx:190`). Resets to `null` every mount.
- Initial pick on load: `Viewer.tsx:224-257` chooses `firstPdf` (else first viewable) → `setSelectedFileId(first.id)` (`:241-247`) — the "resets to first" behavior. Same fallback in `refreshMyFiles` (`:214-219`).
- Selection also drives layer/annotation filtering (`Viewer.tsx:317-318,343,389,417,490-496,850`), so a restored file id must resolve to a real file BEFORE those run.

## The fix — URL-backed selection via a `?file=<id>` query param

- Use `useSearchParams` (react-router — already the router) in the editor. **Do NOT make it a route segment** — it's local editor state, so `SongEditor.tsx` needs no route change.
- **Initialize** `selectedFileId` from `?file` (`Viewer.tsx:103`) instead of `null`.
- **In the my-files load** (`Viewer.tsx:240-247`) and `refreshMyFiles` (`:214-219`): prefer the URL's file id **if it exists in `mine.files`**; otherwise fall back to the existing first-PDF/first-viewable rule (so a stale/foreign `?file` degrades gracefully to today's behavior, never a broken state). Restore INSIDE the existing my-files load so the selection resolves to a real file before the layer/annotation effects run — do not add a separate effect that races them.
- **On selection change** write the param: the file-strip click (`Viewer.tsx:1236` `onClick={() => viewable && setSelectedFileId(f.id)}`) and the load/refresh paths push the id into the query string. Use `replace` (not push) so switching files doesn't spam browser history / hijack the Back button — Back should still leave the editor, not step through file selections. (Confirm this feels right; `replace` is the safe default.)
- Empty/absent `?file` → today's behavior exactly (first PDF). Deleting the open file → falls back to first (the existing refresh rule handles a vanished selection).

## Out of scope
- The stale-render bug (that's T67; both touch Viewer.tsx — coordinate the diffs).
- Persisting zoom/scroll/page position in the URL (only the open FILE — scope to VLL's ask; a follow-up could add `?page=` if wanted).
- App/iOS.

## Acceptance
1. e2e (studio): open a song with ≥2 files, select the SECOND file, F5 / reload → the viewer restores the SECOND file (not the first). The URL carries `?file=<id>` after selection. Red-first: fails today (resets to first).
2. Graceful fallback: a URL with `?file=<nonexistent-id>` loads the first PDF (no crash/blank); no `?file` → first PDF (unchanged behavior).
3. Switching files updates the URL via `replace` (Back button still exits the editor, doesn't walk file history) — assert history length doesn't grow per file switch.
4. Layer/annotation filtering still keys on the restored file correctly (open the second file's own layers after refresh, not the first's).
5. tsc + studio build clean; `editor*` / viewer e2e stay green; no dist churn.

## Notes for the executor
- Keep it to the FILE only. The restored id must be validated against the loaded pool before use (decision above) so a bad param can never wedge the viewer.
- Present at the gate; cite VLL 2026-08-03 (via Fable). If landing alongside T67, rebase so the two Viewer.tsx changes compose cleanly.
