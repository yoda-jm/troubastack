# T151 — The song editor is blank in the app's WebView (pdf.js live render)

**Lane:** mobile (Android WebView host). **Size:** S. **Status:** ✅ **CLOSED 2026-09-06** — root-caused and fixed at `0aec4b90` (Fable 5.1 lane): the app's
WebView got `wrap_content` LayoutParams from Compose's `AndroidViewHolder`, so Chromium's
`AwLayoutSizer` forced a zero-height layout viewport and **every `vh`-family unit resolved to 0**;
Studio's editor is sized in `100svh` with `overflow:hidden`, so it collapsed to nothing. Explicit
`MATCH_PARENT` fixes it (100vh = 595, was 0).

**Device-verified by VLL on the tablet, 2026-09-06** — *"le correctif de la lane fable extra est bon"*,
then *"visuellement confirmé"* (the song editor in the app WebView). That is the acceptance leg; it is
the only check that could close this, and no test in the suite could have.

The studio pdf.js option (`6be53580`) stays — correct on its own terms, never the cause.

## What VLL reported

*"concerts and bands works, songs editor works in a browser, but displays a blank page in the webview in the app."*

## Device investigation (mobile lane, 2026-09-05, on the tablet, build `9bab3c92`)

Walking the app WebView with adb screenshots + logcat:

- **Everything that is plain DOM renders fine in the WebView:** Concerts, Bands, the setlist detail form,
  the song list, and the inline per-song key/tempo/note editor.
- **Clicking a song NAME opens the editor**, which loads `pdfjs-*.js` from the connected server
  (`http://…:8080/assets/pdfjs-*.js`). The only console output is pdf.js **font warnings** (*"Cannot load
  system font: Helvetica-Bold … PDF rendering"*) — **no JS error at any level**. The page is **blank white**.
- **The differentiator is live PDF rendering.** The Stage *performer* view renders the chart perfectly —
  but that is a **baked raster** (pre-rendered WebP). The song editor renders the **source PDF live via
  pdf.js to a canvas**, and that canvas comes up blank in the Android **WebView (Chromium 151)** while it
  works in a desktop browser at the same URL.

So this is not a broken build, a missing route, or a JS crash — it is **pdf.js's live canvas render
producing a blank result specifically under the Android WebView.**

## The likely fix (web-core, cleanest)

This is the classic "pdf.js paints blank in an Android WebView" class. The usual cause is pdf.js choosing an
`OffscreenCanvas` / worker path the WebView mishandles. Passing **`isOffscreenCanvasSupported: false`** to
the pdf.js render/get-document call (or otherwise forcing the main-thread canvas) is the standard fix and
carries no perf cost on desktop. Web-core owns the editor's pdf.js usage; please confirm the render options.

## Fallback (mobile), only if the studio-side fix is insufficient

`webView.setLayerType(View.LAYER_TYPE_SOFTWARE, null)` forces a software raster path that renders pdf.js
canvases, but it degrades canvas performance app-wide — a last resort, not the first move.

## Done when

- Opening a song's editor in the app WebView shows the chart (not a blank page), on the tablet.
- Verified on device by the mobile lane (the exact reproduction is captured above) after the web-core fix lands.

## Out of scope

The baked performer render (works). The setlist/song-list DOM (works).
