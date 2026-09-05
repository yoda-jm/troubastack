// T151 — force pdf.js onto the MAIN-THREAD canvas for the live song-editor render.
//
// With no render options, pdf.js is free to pick an OffscreenCanvas path. The Android WebView
// (Chromium 151) mishandles it: the editor's live canvas came up BLANK on the tablet — no JS error, only
// font warnings — while the same URL renders in a desktop browser. `isOffscreenCanvasSupported: false`
// forces the main-thread canvas path (the one the WebView paints) and has no desktop perf cost. This is
// spread into every `getDocument(...)` call in usePdfDocument, so the fix is defined once and pinned by a
// unit test (T151 red-first) rather than a per-commit tablet check.
export const PDF_RENDER_OPTIONS = { isOffscreenCanvasSupported: false } as const;
