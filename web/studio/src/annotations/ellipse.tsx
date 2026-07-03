import { drawEllipse } from "@troubastack/ink";
import { insideBox, moveMarginFrac } from "../editor";
import {
  type AnnotationTypeDescriptor,
  genericBBox,
  genericResize,
  meaningfulSpan,
  pointsStartEnd,
} from "./types";

const icon = (
  <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.6">
    <ellipse cx="8" cy="8" rx="6" ry="4.5" />
  </svg>
);

export const ellipseDescriptor: AnnotationTypeDescriptor = {
  type: "ellipse",
  tool: { id: "ellipse", label: "Ellipse", icon, cursorClass: "tool-ellipse" },
  draw: drawEllipse,
  styleControls: ["color", "opacity", "width", "shapePreset", "fillBorder", "blend"],
  pointsForGesture: pointsStartEnd,
  isMeaningfulGesture: meaningfulSpan,
  bbox: (obj) => genericBBox(obj),
  classifyHit: (obj, x, y, pageW, pageH) => {
    const b = genericBBox(obj);
    const cx = (b.minX + b.maxX) / 2;
    const cy = (b.minY + b.maxY) / 2;
    const rx = (b.maxX - b.minX) / 2;
    const ry = (b.maxY - b.minY) / 2;
    if (rx > 1e-6 && ry > 1e-6) {
      const mX = moveMarginFrac("x", pageW, pageH);
      const mY = moveMarginFrac("y", pageW, pageH);
      const nx = (x - cx) / (rx + mX);
      const ny = (y - cy) / (ry + mY);
      if (nx * nx + ny * ny <= 1.0) return "strong";
    }
    if (insideBox(b, x, y, 0.012, 0.012)) return "weak";
    return "none";
  },
  resize: (obj, handle, dx, dy) => genericResize(obj, handle, dx, dy),
};
