// T50/T51 — AUTHORING source for the cue/stamp glyph set. Hand-authored SVG-ish
// primitives in a 24×24 box (y-down). This file is NEVER shipped and NEVER parsed at
// runtime: `gen-glyphs.mjs` flattens every arc/curve/circle/rect to polylines and
// emits the runtime contract `glyphs.json` (the ONE artifact TS ink, Go bake, and the
// studio picker consume — no SVG parser anywhere at runtime). Edit here, then run
// `node gen-glyphs.mjs` and commit both this file and the regenerated glyphs.json.
//
// Each glyph is an ordered list of shapes:
//   ["path", d]                    stroked polyline(s) (subpaths split on M)
//   ["path", d, true]              filled polygon(s)
//   ["circle", cx, cy, r]          stroked ring
//   ["circle", cx, cy, r, true]    filled dot
//   ["rect", x, y, w, h, rx]       stroked (rounded) rectangle
//   ["rect", x, y, w, h, rx, true] filled (rounded) rectangle
//
// AUTHORITY for the id list + intent: docs/tasks/T50-song-cues.md §2. The 18 ids
// (order = the studio picker order); `note` is the required unknown-id fallback.

export const BOX = 24; // viewBox side; the generator normalizes coords to a 1×1 box.
export const STROKE_WIDTH = 1.6; // box units; normalized by the generator (÷ BOX).

export const GLYPHS = {
  "guitar-electric": [
    ["path", "M15.5 3.2l5.3 5.3-2 2-1.6-1.6-4.2 4.2"],
    ["path", "M11 15.5a4.4 4.4 0 1 1-4.4-4.4c1.3 0 2 .4 2.6 1l3.4 3.4c.6.6 1 1.3 1 2.6"],
    ["circle", 6.8, 15.4, 1.4],
  ],
  "guitar-acoustic": [
    ["path", "M14.5 3.5l3.5 3.5-2.4 2.4"],
    ["path", "M13 11.2c1.7 1.7 1.9 4.2.5 5.9a5.7 5.7 0 1 1-6.6-8.9c1.7-.9 3.5-.6 4.8.6"],
    ["circle", 9.4, 14.6, 1.9],
  ],
  "guitar-classical": [
    ["path", "M14.8 3.5l3.2 3.2-2.2 2.2"],
    ["path", "M12.6 11c1.9 1.9 2.2 4.7.6 6.6a6.2 6.2 0 1 1-7.2-9.7c2-1 4-.6 5.4.7"],
    ["circle", 8.7, 14.9, 2.4],
    ["circle", 8.7, 14.9, 0.7],
  ],
  bass: [
    ["path", "M16.5 2.6l4.9 4.9-1.8 1.8-1.5-1.5-4.6 4.6"],
    ["path", "M13 14.6a4.1 4.1 0 1 1-3.8-4c1.2-.1 1.9.3 2.5.9l3.1 3.1c.6.6 1 1.3.9 2.5"],
    ["circle", 9, 14.7, 1.3],
    ["circle", 18.4, 4.3, 0.8],
    ["circle", 20, 5.9, 0.8],
  ],
  ukulele: [
    ["path", "M15.5 4l2.9 2.9-1.9 1.9"],
    ["path", "M12.4 10.6c1.4 1.4 1.6 3.4.4 4.8a4.7 4.7 0 1 1-5.4-7.3c1.4-.8 2.9-.5 4 .5"],
    ["circle", 9.6, 13.2, 1.5],
  ],
  autoharp: [
    ["path", "M3.5 7.5l16-2.5v11l-13 2z"],
    ["path", "M6 8.7l11.5-1.6M6 11l11.5-1.5M6 13.3l11.5-1.4"],
  ],
  melodica: [
    ["rect", 3.5, 8, 14.5, 7, 1],
    ["path", "M18 10.5h2.5v2H18"],
    ["path", "M6.5 15v-2.4M9 15v-2.4M11.5 15v-2.4M14 15v-2.4"],
  ],
  keys: [
    ["rect", 3, 7, 18, 10, 1],
    ["path", "M7 7v10M11 7v10M15 7v10M19 7v10"],
    ["path", "M5.6 7v5.5h2.4V7M9.6 7v5.5H12V7M13.6 7v5.5H16V7", true],
  ],
  cajon: [
    ["rect", 5, 3.5, 14, 17, 1.2],
    ["circle", 12, 9.5, 2.6],
  ],
  bongo: [
    ["path", "M4 9.5a3.6 3.6 0 0 1 7.2 0v6a3.6 3.6 0 0 1-7.2 0z"],
    ["path", "M12.4 10.4a3 3 0 0 1 6 0v4.6a3 3 0 0 1-6 0z"],
  ],
  djembe: [
    ["path", "M6.5 4.5h11l-1.4 5c-.4 1.3-1.1 2-1.1 3.2 0 2 1 3 1 5.3h-7c0-2.3 1-3.3 1-5.3 0-1.2-.7-1.9-1.1-3.2z"],
    ["path", "M6.5 4.5h11"],
  ],
  guiro: [
    ["rect", 4, 8.5, 13, 7, 3.5],
    ["path", "M7 9.2v6M9.3 9v6M11.6 9v6M13.9 9.2v6"],
    ["path", "M17.5 12h3"],
  ],
  cuica: [
    ["circle", 12, 12, 7],
    ["path", "M12 5.2v13.6"],
    ["circle", 12, 12, 1.2, true],
  ],
  shaker: [
    ["rect", 7.5, 3.5, 9, 17, 4.5],
    ["circle", 10.5, 9, 0.9, true],
    ["circle", 13.5, 11.5, 0.9, true],
    ["circle", 10.8, 14, 0.9, true],
    ["circle", 13.4, 16, 0.9, true],
  ],
  "egg-shaker": [
    ["path", "M12 3.5c3.3 0 5.5 3.8 5.5 8.3S15.3 20.5 12 20.5 6.5 16.3 6.5 11.8 8.7 3.5 12 3.5z"],
    ["circle", 10.6, 10, 0.85, true],
    ["circle", 13.4, 12.5, 0.85, true],
    ["circle", 11, 14.6, 0.85, true],
  ],
  tambourine: [
    ["circle", 12, 12, 7.5],
    ["circle", 12, 12, 4.5],
    ["circle", 12, 4.5, 1, true],
    ["circle", 19.5, 12, 1, true],
    ["circle", 12, 19.5, 1, true],
    ["circle", 4.5, 12, 1, true],
  ],
  mic: [
    ["rect", 9, 2.5, 6, 11, 3],
    ["path", "M6 11a6 6 0 0 0 12 0"],
    ["path", "M12 17v4M9 21h6"],
  ],
  warning: [
    ["path", "M12 3.6L21.4 20H2.6Z"],
    ["path", "M12 9.4V14.9"],
    ["circle", 12, 17.6, 1, true],
  ],
  note: [
    ["path", "M9 18V6l9-2v10"],
    ["circle", 6.5, 18, 2.5],
    ["circle", 15.5, 16, 2.5],
  ],
};

// The picker/consumer order and the required fallback id.
export const GLYPH_IDS = Object.keys(GLYPHS);
export const FALLBACK_ID = "note";
