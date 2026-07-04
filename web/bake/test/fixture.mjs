// The I8 parity fixture: one page whose two layers exercise every built-in
// annotation type — freehand (with pressure), line, rect+fill (the fill+stroke
// "box" composite path), ellipse (stroke-only), text, and legacy highlight.
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
