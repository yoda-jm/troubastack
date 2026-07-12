# T39 — Chart editor: live (debounced) preview as you type

**Priority:** normal (VLL, 2026-07-12: "edit the pseudo-md directly in preview
mode") · **Size:** S · **Area:** `web/studio` (the T19/T25 ChartEditor) + e2e.

## Decision (RULED, VLL-confirmed 2026-07-12)

VLL wants the preview to keep up as you type (interpretation 1 of three). This
**reverses the deliberate T25 "preview renders on demand" choice** — that choice
was to avoid per-keystroke server renders (each preview is a `chartpdf` render +
blob swap). VLL's OK sanctions the reversal; the mitigations below keep the
render rate sane. The other two readings are OFF: **WYSIWYG-on-the-PDF is
explicitly declined** (the preview is a rasterized served PDF with no
source-position mapping — disproportionate build); "just a nicer split" is
subsumed by this.

## Changes (ChartEditor, `SongDetails.tsx`)

1. **Debounced auto-preview.** When `source` changes, schedule `preview()` after
   a quiet period (~500 ms). Coalesce: a new keystroke cancels the pending timer;
   only one render fires per quiet period. The manual **Preview** button stays
   (forces an immediate render; keep its testid).
2. **In-flight coalescing (correctness — do not skip).** A render is a server
   round-trip; edits during it must not stack or land stale. Guard: if a render
   is in flight when the debounce fires, mark "dirty" and re-render once the
   current one resolves; always render the LATEST source, never an intermediate.
   Drop/ignore a resolved render whose source is no longer current (don't swap a
   stale blob over a newer one).
3. **No-thrash on errors.** A source that fails to render (bad char →
   `ErrUnsupportedChar` 400) shows the error as today and does NOT spin
   retrying the same bad text every 500 ms — only re-attempt when the source
   changes again.
4. **Blob hygiene unchanged.** The existing revoke-on-replace effect already
   prevents leaks; the higher render frequency makes it load-bearing — keep it.
5. **Scope guard:** editor-open only (the debounce lives in the ChartEditor
   component; unmount cancels the timer). Same `previewTextChart` no-persist
   endpoint — no new API, no Go change.

## Tests

- e2e: type into `chart-source`, DON'T click Preview, wait past the debounce →
  `chart-preview` updates (assert the preview object's blob/src changed, or a
  render-count testid increments). A rapid burst of edits fires ONE render after
  settle, not one per keystroke (the T25/wheel-zoom "one raster per settle"
  pattern — mirror its assertion shape). The manual Preview button still forces
  an immediate render.
- Assert the error path doesn't loop (an unsupported char shows the error once
  and stops until the source changes).

## Acceptance criteria

- Live debounced preview works; a burst of N edits = 1 render after settle (not
  N); manual Preview still works; no stale-blob swap; no error-retry loop;
  full suite green; `tsc -b studio` clean; pixels at the gate (mid-edit preview
  keeping up).

## Out of scope

- WYSIWYG editing on the rendered PDF (declined above); per-keystroke
  (undebounced) rendering; caching rendered PDFs; any server/endpoint change.
