/**
 * T177 — the line styles reach the BAKED PIXELS.
 *
 * The parity test next door proves the bake and the editor agree. It cannot prove the style was drawn:
 * two renderers that both ignore `dash` agree perfectly. So this file asserts the pixels a dash and an
 * arrowhead actually change, each against the SAME object without the style — the discriminating half.
 *
 * Populations, not lucky pixels: each claim counts the pixels inside a region only the styled version can
 * reach, so an antialiased edge cannot pass or fail it by a sub-pixel.
 */
import { test } from "node:test";
import assert from "node:assert/strict";
import { loadImage, createCanvas } from "@napi-rs/canvas";
import { renderOverlays } from "../dist/index.js";

const PAGE = { index: 0, width: 800, height: 600 };
const OVERLAY = { width: 400 }; // → a 400×300 overlay; page-relative x·400, y·300

/** Render one object and hand back its overlay as RGBA. */
async function rasterize(obj) {
  const doc = { layers: [{ id: "L1", order: 1, mandatory: false, roleTag: "" }], objects: [{ uuid: "o1", layerId: "L1", page: 0, ...obj }] };
  const overlays = renderOverlays(doc, [PAGE], OVERLAY);
  assert.equal(overlays.length, 1, "one layer, one page → one overlay");
  const img = await loadImage(Buffer.from(overlays[0].png));
  const cv = createCanvas(img.width, img.height);
  const ctx = cv.getContext("2d");
  ctx.drawImage(img, 0, 0);
  return ctx.getImageData(0, 0, img.width, img.height);
}

/** Count pixels in [x0,x1]×[y0,y1] whose alpha clears `minAlpha` (default: solidly painted). */
function inkIn(img, x0, y0, x1, y1, minAlpha = 200) {
  let n = 0;
  for (let y = Math.round(y0); y <= Math.round(y1); y++) {
    for (let x = Math.round(x0); x <= Math.round(x1); x++) {
      if (img.data[(y * img.width + x) * 4 + 3] >= minAlpha) n++;
    }
  }
  return n;
}

/** Count pixels along a horizontal run that are essentially UNPAINTED — a dash's gaps. */
/** Alpha of a single pixel. */
function sampleAlpha(img, x, y) {
  return img.data[(Math.round(y) * img.width + Math.round(x)) * 4 + 3];
}

function gapsAlong(img, x0, x1, y, maxAlpha = 20) {
  let n = 0;
  for (let x = Math.round(x0); x <= Math.round(x1); x++) {
    if (img.data[(Math.round(y) * img.width + x) * 4 + 3] <= maxAlpha) n++;
  }
  return n;
}

const LINE = { type: "line", points: [{ x: 0.2, y: 0.5 }, { x: 0.8, y: 0.5 }] };
const base = { color: "#000000", opacity: 1, width: 0.01, fontSize: 0 }; // 4px stroke at this overlay
// The line runs from x=80 to x=320 at y=150.

test("a solid line is continuous; a dashed one is not; a dotted one is gappier still", async () => {
  const solid = await rasterize({ ...LINE, style: { ...base } });
  const dashed = await rasterize({ ...LINE, style: { ...base, dash: "dashed" } });
  const dotted = await rasterize({ ...LINE, style: { ...base, dash: "dotted" } });

  // Measured strictly INSIDE the segment so the ends cannot contribute.
  const solidGaps = gapsAlong(solid, 90, 310, 150);
  const dashedGaps = gapsAlong(dashed, 90, 310, 150);
  const dottedGaps = gapsAlong(dotted, 90, 310, 150);

  assert.equal(solidGaps, 0, "a solid line must have no holes — if this is non-zero the probe is wrong, not the renderer");
  assert.ok(dashedGaps > 0, `dashed line has no gaps (${dashedGaps}) — the pattern never reached the bake`);
  assert.ok(dottedGaps > dashedGaps, `dotted (${dottedGaps}) should leave more of the line bare than dashed (${dashedGaps})`);
});

test("a dashed BORDER reaches a rect", async () => {
  const rect = { type: "rect", points: [{ x: 0.2, y: 0.3 }, { x: 0.8, y: 0.7 }] };
  // The border's top edge sits at y=90; the stroke is inset by half its width, so sample its centre.
  const solid = await rasterize({ ...rect, style: { ...base } });
  const dashed = await rasterize({ ...rect, style: { ...base, dash: "dashed" } });
  assert.equal(gapsAlong(solid, 100, 300, 92), 0, "a solid border must be continuous (probe check)");
  assert.ok(gapsAlong(dashed, 100, 300, 92) > 0, "the dash never reached the rect border");
});

test("on a FILLED box a dash changes nothing, and that is the shape not a miss", async () => {
  // Worth pinning, because the renderer does set the pattern on the offscreen composite and someone
  // reading this later will look for its effect: fill and border share ONE colour, and the border is
  // inset INSIDE the fill, so a dash's gaps expose the same colour they interrupt. The pattern is
  // unobservable on a filled box by construction — not dropped, and not something to "fix" by clipping
  // the fill, which would put a ragged edge on every dashed box.
  const rect = { type: "rect", points: [{ x: 0.2, y: 0.3 }, { x: 0.8, y: 0.7 }] };
  const boxSolid = await rasterize({ ...rect, style: { ...base, fill: true, stroke: true } });
  const boxDashed = await rasterize({ ...rect, style: { ...base, fill: true, stroke: true, dash: "dashed" } });
  assert.deepEqual(
    Array.from(boxDashed.data),
    Array.from(boxSolid.data),
    "a dashed filled box rendered differently from a solid one — if that is now intended, this test is the place to say so",
  );
});

test("a terminator paints where the shaft alone cannot reach, at the right end", async () => {
  // The shaft is 4px wide → never further than 2px off-axis. A terminator is ~6px half-width, so the band
  // 3..7px off-axis just behind an endpoint is reachable ONLY by a terminator.
  const plain = await rasterize({ ...LINE, style: { ...base } });
  const arrowEnd = await rasterize({ ...LINE, style: { ...base, ends: { end: "arrow" } } });
  const arrowStart = await rasterize({ ...LINE, style: { ...base, ends: { start: "arrow" } } });
  const both = await rasterize({ ...LINE, style: { ...base, ends: { start: "arrow", end: "arrow" } } });
  const mixed = await rasterize({ ...LINE, style: { ...base, ends: { start: "circle", end: "square" } } });

  const nearEnd = (img) => inkIn(img, 305, 153, 320, 157);
  const nearStart = (img) => inkIn(img, 80, 153, 95, 157);

  assert.equal(nearEnd(plain), 0, "an undecorated line must not reach the terminator band (probe check)");
  assert.equal(nearStart(plain), 0, "…at either end");

  assert.ok(nearEnd(arrowEnd) > 0, "no terminator at the END");
  assert.equal(nearStart(arrowEnd), 0, "an END terminator must not also appear at the start");

  assert.ok(nearStart(arrowStart) > 0, "no terminator at the START");
  assert.equal(nearEnd(arrowStart), 0, "a START terminator must not also appear at the end");

  assert.ok(nearEnd(both) > 0 && nearStart(both) > 0, "both ends must be decorated");

  // T179's new case: DIFFERENT shapes at each end, both present.
  assert.ok(nearStart(mixed) > 0 && nearEnd(mixed) > 0, "a circle-start + square-end line must decorate both ends");
});

test("T179: the shaft is TRIMMED to the terminator base — no spine, no dash gap in the head", async () => {
  // The bug VLL hit: a shaft stroked tip-to-tip runs UNDER a filled head. At opacity < 1 it composites as
  // a darker spine; dashed, a gap can land inside the head. The fix trims the shaft to the base, so on a
  // DASHED half-opacity arrow the pixels between the base and the tip are painted ONLY by the head — one
  // uniform triangle, never a shaft artifact. We assert that region is fully, uniformly opaque-for-its-alpha.
  const dashHalf = { ...base, opacity: 0.5, dash: "dashed", ends: { end: "arrow" } };
  const img = await rasterize({ ...LINE, style: dashHalf });
  // The head occupies roughly the last ~13px of the 4px-stroke line (3.2×4). Sample a band just inside the
  // tip, on-axis, where a tip-to-tip shaft would have overlapped the head (→ darker) or left a dash gap.
  // The line runs x 80..320 at y=150; the head spans the last ~13px (3.2×4), tip at x=320. Sample
  // ON-AXIS (y=150) inside the head but behind the tip, where a tip-to-tip shaft WOULD have overlapped.
  let holes = 0, doubled = 0, n = 0;
  for (let x = 309; x <= 317; x++) {
    const a = sampleAlpha(img, x, 150);
    n++;
    if (a === 0) holes++;              // a dash gap that reached inside the head
    if (a > 170) doubled++;            // shaft+head composited darker than a single 0.5 layer (~128)
  }
  assert.ok(n > 0, "probe covered no pixels");
  assert.equal(holes, 0, "a dash gap landed inside the arrowhead — the shaft was not trimmed");
  assert.equal(doubled, 0, "the shaft shows through the head as a darker spine — it was not trimmed");
});

test("an END SHAPE this renderer does not know draws nothing at all", async () => {
  const unknown = await rasterize({ ...LINE, style: { ...base, ends: { end: "diamond" } } });
  assert.equal(
    inkIn(unknown, 305, 153, 320, 157),
    0,
    "an unrecognised end shape was approximated — a mark nobody asked for, on a page someone plays from",
  );
});

test("the degenerate ends a stray tap makes: no terminator bigger than its line, and no crash", async () => {
  const short = {
    type: "line",
    points: [{ x: 0.5, y: 0.5 }, { x: 0.52, y: 0.5 }],
    style: { ...base, ends: { start: "arrow", end: "arrow" } },
  };
  const img = await rasterize(short);
  assert.equal(inkIn(img, 175, 140, 195, 160, 40), 0, "ink spilled well before the line's start");
  assert.equal(inkIn(img, 220, 140, 240, 160, 40), 0, "ink spilled well past the line's end");
  assert.ok(inkIn(img, 198, 146, 210, 154, 40) > 0, "the short line vanished entirely (probe check)");

  const dot = {
    type: "line",
    points: [{ x: 0.5, y: 0.5 }, { x: 0.5, y: 0.5 }],
    style: { ...base, width: 0.02, ends: { start: "arrow", end: "arrow" } },
  };
  const dotImg = await rasterize(dot);
  assert.ok(inkIn(dotImg, 196, 146, 204, 154, 40) > 0, "the tap's dot is missing");
  assert.equal(inkIn(dotImg, 215, 140, 240, 160, 40), 0, "a zero-length line grew a terminator out of nowhere");
});
