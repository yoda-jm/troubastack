# T151 — The song editor is blank in the app's WebView (pdf.js live render)

**Lane:** mobile (Android WebView host). **Size:** S. **Status:** ✅ ROOT-CAUSED + FIXED ON DEVICE 2026-09-06
(`13a8e6e8`, branch `fix/t151-webview-layout-params`, awaiting land) — **every `vh`/`svh`/`dvh` unit is 0 inside the
app's WebView** because the WebView is added by Compose's `AndroidView` with `wrap_content` LayoutParams, which
Chromium's `AwLayoutSizer` turns into `ForceZeroLayoutHeight` (a 0-height layout viewport). Studio's full-bleed
editor is sized in `100svh` + `overflow:hidden`, so its whole shell collapsed to 0px around a fully rendered DOM.
Fix: explicit `MATCH_PARENT` LayoutParams in `WebViewHost`. Verified on the tablet via DevTools (`100vh` 0 → 595)
and by eye. The pdf.js option (`6be53580`) stays; it was never the cause. History below kept for the record.
**Previously:** ⛔ REOPENED 2026-09-05 (`60290a1b`) — the studio pdf.js fix landed (`6be53580`, keep it) but is
NOT the cause: `EditScreen.kt:98` is the app's ONLY WebView, so the "DOM renders fine in the WebView"
contrast never existed. Re-routed to mobile as a WebView surface/compositing bug. Previously: fixed
2026-09-05 (web-core) — `getDocument` now spreads `PDF_RENDER_OPTIONS` (`isOffscreenCanvasSupported: false`)
at `usePdfDocument.ts:153`, forcing the main-thread canvas the WebView paints. Red-first guard
(`test/pdf-render-options.test.ts`) pins the flag AND asserts every getDocument call spreads it (no tablet
needed). Did NOT use the mobile `LAYER_TYPE_SOFTWARE` fallback. **Awaiting the mobile device-verify leg**
on the tablet (the acceptance). Originally from VLL on the tablet + a mobile device-investigation. **Routing:** filed by mobile, routed to
web-core — the reproduction is a WebView, but the failing code is the studio editor's live PDF render.

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
