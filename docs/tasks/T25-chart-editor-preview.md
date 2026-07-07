# T25 — Chart editor: rendered PDF preview pane (T19 decision 3, deferred)

**Priority:** medium (UX completeness) · **Size:** S · **Area:** `core` (one endpoint), `web/studio`

## Context

T19's resolved design decision 3 specified a **two-pane** editor card ("plain
`textarea` + rendered PDF preview via the existing file viewer, refresh on save").
The landed editor (`9058aa9`) is textarea-only; the deviation was admitted in a code
comment ("Live PDF preview is a later nicety") but not flagged at the gate. The
review (reviews.md 2026-07-07) accepted the editor as a usable v1 and filed this
follow-up: the write→see loop is the whole point of a formatted chart editor, and
everything needed already exists server-side (`chartpdf.Render` is pure).

**Design decisions (resolved):**
1. **Preview endpoint, no persistence:** `POST /api/bands/{b}/songs/{s}/text-charts:preview`
   (member-gated like the other chart endpoints) takes `{source}` and returns
   `application/pdf` bytes rendered by `chartpdf.Render` — nothing stored, no blob,
   no file record. Unrenderable source → 400 with the `ErrUnsupportedChar` message
   (the editor shows it where the save error shows today).
2. **Refresh on demand, not per keystroke:** a "Preview" button (and refresh on
   save) — rendering is cheap but per-keystroke round-trips are noise; v1 explicitly
   does NOT debounce-live-render.
3. **Display via `<object type="application/pdf">`/blob URL** in the second pane —
   the browser's native PDF view is enough; do NOT wire the annotation viewer into
   the editor card (annotations belong to the saved pool file, not a preview).

## Changes

1. Core: the preview handler (reuse the create-path validation; no repo access
   beyond the membership/song scope checks). Endpoint test: member gets PDF bytes,
   non-member 404/403 per the T08 pattern, bad chars 400.
2. Studio: second pane in `ChartEditor` (stack on narrow widths), Preview button,
   blob-URL lifecycle (revoke on unmount/refresh).
3. e2e: extend `text-chart.spec.ts` — type, Preview, assert the object/iframe is
   present with a blob URL (pixel assertions stay out; the Go golden tests own
   rendering).

## Acceptance criteria

- Typing a chart and clicking Preview shows the rendered PDF beside/below the
  textarea without creating a pool file (file list unchanged until Save).
- Invalid characters surface the server's message on Preview as well as Save.
- `make test` + studio typecheck + e2e green.

## Out of scope

- Live/debounced rendering; client-side rendering; annotation viewer integration;
  any change to the save/LWW contract (T19's shipped API is frozen).
