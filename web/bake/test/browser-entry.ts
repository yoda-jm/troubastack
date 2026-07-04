/**
 * Browser side of the I8 parity harness (bundled into Chromium by parity.test.mjs).
 *
 * This is the STUDIO dry path: it draws annotation objects with @troubastack/ink's
 * `renderObjects` onto a real browser <canvas> — exactly what studio's dry layer
 * does — and hands the raw pixels back. The bake worker draws the same objects with
 * the same ink onto a Skia node canvas; the test asserts the two agree per-pixel.
 *
 * It carries NO @napi-rs/canvas import — it must run in bare Chromium. The bundle
 * exposes its API on `window.TroubaBakeParity`.
 */

import { renderObjects, setTextFontFamily, type InkObject } from "@troubastack/ink";

interface RawImage {
  width: number;
  height: number;
  /** RGBA bytes, row-major, length = width*height*4. */
  data: number[];
}

/** Pin ink's text font to the family the parity harness loaded via FontFace. */
function setup(fontFamily: string): void {
  setTextFontFamily(fontFamily);
}

/** Render one layer's objects onto a transparent w×h canvas; return raw RGBA. */
function renderOverlay(objs: InkObject[], w: number, h: number): RawImage {
  const canvas = document.createElement("canvas");
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("no 2d context in browser");
  // Mirror bake: disable hinting so text rasterizes from pure geometry (I8 parity).
  (ctx as unknown as { textRendering?: string }).textRendering = "geometricPrecision";
  renderObjects(ctx, objs, { x: 0, y: 0, w, h });
  const img = ctx.getImageData(0, 0, w, h);
  return { width: w, height: h, data: Array.from(img.data) };
}

(window as unknown as { TroubaBakeParity: unknown }).TroubaBakeParity = {
  setup,
  renderOverlay,
};
