/**
 * TroubaStudio — the canonical editor (ARCHITECTURE.md I10).
 *
 * Studio is the COMPLETE editor and runs standalone in any browser. The mobile
 * app embeds this exact build in a webview; the editor is never reimplemented
 * natively (I10). All rich interactive state (tools, selection, optimistic
 * objects, zoom) lives HERE; core stays a calm authority that accepts/echoes
 * objects (I6, 07-boundaries-and-no-duplication.md).
 *
 * WET / DRY SPLIT (I9, see ../../docs/design/03-rendering-and-ink.md):
 *  - DRY layer = PDF raster + all committed objects. Always rendered by this
 *    web layer, via @troubastack/ink (I8). Re-rasterize the PDF page (pdfjs)
 *    per zoom level for crisp text; never CSS-scale the old bitmap.
 *  - WET layer = the single in-progress freehand stroke only. Drawn here on a
 *    second `desynchronized` canvas (pointerrawupdate + getCoalescedEvents).
 *    This in-browser wet path is CANONICAL and ALWAYS EXISTS (I10) — it is the
 *    fallback everywhere and the only path on desktop.
 *
 * NATIVE OVERLAY = optional accelerator (I9, I10): on mobile in the app, a
 * feature-detected native surface may render the wet stroke for lowest stylus
 * input→photon latency, handing the stroke back to this web layer on commit.
 * It is NOT required; absence ⇒ in-browser wet path. The native overlay
 * mirrors @troubastack/ink under a pixel-parity test — it is the only
 * sanctioned re-implementation (I8).
 *
 * FIRST BUILD STEP: the web-ink spike (03-rendering-and-ink.md) — PDF.js +
 * @troubastack/ink + a low-latency canvas, judged for stylus feel on the real
 * target Android tablet. That spike validates the assumption the whole client
 * architecture rests on. Build it before committing to the native overlay; do
 * not delete the in-browser wet path.
 */

import { buildStrokeGeometry, renderStroke } from "@troubastack/ink";

// Reference the one renderer so the dependency boundary (I8) is encoded even
// in the stub. studio NEVER reimplements stroke rendering.
void buildStrokeGeometry;
void renderStroke;

// TODO: mount the editor into #app — PDF.js dry layer, in-browser wet canvas,
// tool/selection state, optimistic object outbox reconciled to core echoes (I6).
throw new Error("TODO");
