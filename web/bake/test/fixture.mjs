// The I8 parity fixture: one page whose two layers exercise every built-in
// annotation type — freehand (with pressure), line, rect+fill (the fill+stroke
// "box" composite path), ellipse (stroke-only), text, and legacy highlight.
//
// T177 adds the line STYLES, and they belong here rather than in a test of their own: a dash is a
// pattern the two Skia builds each lay out themselves, and an arrowhead is geometry each computes from
// the stroke width, so "the same in the editor and in the bake" is exactly the claim this file's test
// makes and nothing weaker proves. Both the DIRECT stroke path and the fill+stroke offscreen composite
// are covered, because the dash is set on two different contexts and only one of them is obvious.
// All coordinates are page-relative [0,1] (I3); this is the exact shape the
// annotations API returns and studio consumes.

export const fixture = {
  overlayWidth: 700,
  pages: [{ index: 0, width: 1240, height: 1754 }],
  doc: {
    layers: [
      { id: "L1", order: 1, mandatory: false, roleTag: "" },
      { id: "L2", order: 2, mandatory: true, roleTag: "vocals" },
    ],
    objects: [
      // freehand with variable pressure
      {
        type: "freehand",
        layerId: "L1",
        page: 0,
        points: [
          { x: 0.1, y: 0.12, pressure: 0.3 },
          { x: 0.25, y: 0.28, pressure: 0.8 },
          { x: 0.42, y: 0.18, pressure: 0.6 },
          { x: 0.55, y: 0.32, pressure: 0.9 },
        ],
        style: { color: "#1a73e8", opacity: 1, width: 0.012, fontSize: 0 },
      },
      // straight line
      {
        type: "line",
        layerId: "L1",
        page: 0,
        points: [{ x: 0.15, y: 0.45 }, { x: 0.8, y: 0.5 }],
        style: { color: "#188038", opacity: 1, width: 0.006, fontSize: 0 },
      },
      // rect with BOTH fill and stroke (the offscreen-composite "box" path)
      {
        type: "rect",
        layerId: "L1",
        page: 0,
        points: [{ x: 0.2, y: 0.55 }, { x: 0.6, y: 0.72 }],
        style: { color: "#e91e63", opacity: 0.85, width: 0.01, fontSize: 0, fill: true, stroke: true },
      },
      // ellipse, stroke-only (legacy: flags absent → stroke)
      {
        type: "ellipse",
        layerId: "L2",
        page: 0,
        points: [{ x: 0.62, y: 0.2 }, { x: 0.9, y: 0.42 }],
        style: { color: "#9334e6", opacity: 1, width: 0.008, fontSize: 0 },
      },
      // legacy highlight (filled, multiply, no stroke)
      {
        type: "highlight",
        layerId: "L2",
        page: 0,
        points: [{ x: 0.1, y: 0.62 }, { x: 0.5, y: 0.66 }],
        style: { color: "#fbbc04", opacity: 0.5, width: 0, fontSize: 0 },
      },
      // T177 — dashed border, stroke-only (the direct paint path)
      {
        type: "rect",
        layerId: "L1",
        page: 0,
        points: [{ x: 0.06, y: 0.74 }, { x: 0.40, y: 0.80 }],
        style: { color: "#0b8043", opacity: 1, width: 0.005, fontSize: 0, dash: "dashed" },
      },
      // T177 — a dashed border on a FILLED box. The dash is invisible here by construction (fill and
      // border share one colour and the border is inset inside the fill — see line-style.test.mjs), so
      // this is not here to show a pattern: it is here because the fill+stroke path composites on a
      // SECOND, offscreen context, and the two builds must still agree pixel-for-pixel on it.
      {
        type: "rect",
        layerId: "L1",
        page: 0,
        points: [{ x: 0.44, y: 0.74 }, { x: 0.58, y: 0.80 }],
        style: {
          color: "#3f51b5",
          opacity: 0.8,
          width: 0.006,
          fontSize: 0,
          fill: true,
          stroke: true,
          dash: "dashed",
        },
      },
      // T177 — dotted ellipse (a zero-length dash + the round cap = a round dot)
      {
        type: "ellipse",
        layerId: "L2",
        page: 0,
        points: [{ x: 0.62, y: 0.73 }, { x: 0.92, y: 0.86 }],
        style: { color: "#c5221f", opacity: 1, width: 0.006, fontSize: 0, dash: "dotted" },
      },
      // T177 — a dashed line with an arrowhead at its far end
      {
        type: "line",
        layerId: "L1",
        page: 0,
        points: [{ x: 0.08, y: 0.91 }, { x: 0.52, y: 0.91 }],
        style: {
          color: "#e8710a",
          opacity: 1,
          width: 0.006,
          fontSize: 0,
          dash: "dashed",
          ends: { end: "arrow" },
        },
      },
      // T179 — a DIFFERENT shape at each end (arrow start, circle end): the case the old model could not
      // express, and two independent terminators the two Skia builds must place identically.
      {
        type: "line",
        layerId: "L2",
        page: 0,
        points: [{ x: 0.60, y: 0.91 }, { x: 0.62, y: 0.91 }],
        style: {
          color: "#1967d2",
          opacity: 1,
          width: 0.006,
          fontSize: 0,
          ends: { start: "arrow", end: "circle" },
        },
      },
      // T179 — a square terminator on a short line: shrinks to fit, far edge at the endpoint, in both builds.
      {
        type: "line",
        layerId: "L2",
        page: 0,
        points: [{ x: 0.66, y: 0.91 }, { x: 0.70, y: 0.91 }],
        style: {
          color: "#7b1fa2",
          opacity: 1,
          width: 0.006,
          fontSize: 0,
          ends: { start: "square", end: "square" },
        },
      },
      // T179 — a ZERO-LENGTH line with ends asked for: no direction, so no terminator. It must draw the dot
      // the tap made and raise nothing, in both renderers.
      {
        type: "line",
        layerId: "L2",
        page: 0,
        points: [{ x: 0.70, y: 0.91 }, { x: 0.70, y: 0.91 }],
        style: {
          color: "#5f6368",
          opacity: 1,
          width: 0.008,
          fontSize: 0,
          ends: { start: "arrow", end: "arrow" },
        },
      },
      // text
      {
        type: "text",
        layerId: "L2",
        page: 0,
        points: [{ x: 0.12, y: 0.82 }],
        text: "TroubaStack",
        style: { color: "#202124", opacity: 1, width: 0, fontSize: 0.045 },
      },
    ],
  },
};
