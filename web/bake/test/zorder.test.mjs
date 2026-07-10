/**
 * T31 — bake must honor per-object z-order (I8 parity with studio).
 *
 * Studio's dry render sorts a layer's objects by `order → createdAt → uuid`
 * (compareObjectZ, T27 stage 2); the bake renderer must apply the SAME sort or a
 * studio bring-to-front is silently absent from the baked overlay. Asserted the
 * same way studio's editor-zorder spec does: two overlapping opaque rects whose
 * `order` INVERTS document order — the overlap pixel must be the high-`order`
 * object's color.
 */
import { test } from "node:test";
import assert from "node:assert/strict";
import { loadImage, createCanvas } from "@napi-rs/canvas";
import { renderOverlays } from "../dist/index.js";

async function overlapPixel(doc, fx, fy) {
  const overlays = renderOverlays(doc, [{ index: 0, width: 800, height: 600 }], { width: 400 });
  assert.equal(overlays.length, 1, "one layer, one page → one overlay");
  const img = await loadImage(Buffer.from(overlays[0].png));
  const cv = createCanvas(img.width, img.height);
  const ctx = cv.getContext("2d");
  ctx.drawImage(img, 0, 0);
  const d = ctx.getImageData(Math.floor(fx * img.width), Math.floor(fy * img.height), 1, 1).data;
  return { r: d[0], g: d[1], b: d[2], a: d[3] };
}

const rect = (uuid, color, extra = {}) => ({
  uuid,
  layerId: "L1",
  type: "rect",
  page: 0,
  // Both rects cover the centre (0.5, 0.5): A spans x 0.2–0.6, B spans x 0.4–0.8.
  points:
    extra.left === "A"
      ? [{ x: 0.2, y: 0.3 }, { x: 0.6, y: 0.7 }]
      : [{ x: 0.4, y: 0.3 }, { x: 0.8, y: 0.7 }],
  style: { color, opacity: 1, width: 0.005, fill: true, stroke: false },
  ...extra,
});

const isReddish = (p) => p.a > 200 && p.r > p.b + 60;
const isBluish = (p) => p.a > 200 && p.b > p.r + 60;

test("order inverts document order: the high-order object wins the overlap", async () => {
  const doc = {
    layers: [{ id: "L1", order: 0 }],
    objects: [
      // A (red) comes FIRST in the array but carries the HIGHER order → must
      // render ON TOP. Pre-T31 (document-order rendering) painted B last → blue.
      rect("aaaa-red", "#e11d48", { left: "A", order: 5, createdAt: 100 }),
      rect("bbbb-blue", "#2563eb", { left: "B", order: 0, createdAt: 200 }),
    ],
  };
  const px = await overlapPixel(doc, 0.5, 0.5);
  assert.ok(isReddish(px), `overlap must be the high-order red, got ${JSON.stringify(px)}`);
});

test("equal order falls back to createdAt, then uuid (the compareObjectZ contract)", async () => {
  // Same order; B (blue) is OLDER (createdAt 100) and appears LAST in the array —
  // createdAt must win over array position: A (red, createdAt 200) renders on top.
  const byCreated = {
    layers: [{ id: "L1", order: 0 }],
    objects: [
      rect("aaaa-red", "#e11d48", { left: "A", order: 1, createdAt: 200 }),
      rect("bbbb-blue", "#2563eb", { left: "B", order: 1, createdAt: 100 }),
    ],
  };
  // Array order puts red first; createdAt sort must re-order blue first → red on top.
  byCreated.objects.reverse(); // blue first in the array now
  const p1 = await overlapPixel(byCreated, 0.5, 0.5);
  assert.ok(isReddish(p1), `createdAt tiebreak: red (newer) on top, got ${JSON.stringify(p1)}`);

  // Same order + createdAt → uuid decides ("zzzz" sorts after "aaaa" → on top).
  const byUuid = {
    layers: [{ id: "L1", order: 0 }],
    objects: [
      rect("zzzz-blue", "#2563eb", { left: "B", order: 1, createdAt: 100 }),
      rect("aaaa-red", "#e11d48", { left: "A", order: 1, createdAt: 100 }),
    ],
  };
  const p2 = await overlapPixel(byUuid, 0.5, 0.5);
  assert.ok(isBluish(p2), `uuid tiebreak: zzzz (blue) on top, got ${JSON.stringify(p2)}`);
});
