import { drawRect } from "@troubastack/ink";
import {
  type AnnotationTypeDescriptor,
  filledBoxClassify,
  genericBBox,
  genericResize,
  meaningfulSpan,
  pointsStartEnd,
} from "./types";

const icon = (
  <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.6">
    <rect x="2.5" y="4" width="11" height="8" rx="1" />
  </svg>
);

export const rectDescriptor: AnnotationTypeDescriptor = {
  type: "rect",
  tool: { id: "rect", label: "Rect", icon, cursorClass: "tool-rect" },
  draw: drawRect,
  styleControls: ["color", "opacity", "width", "shapePreset", "fillBorder", "blend"],
  pointsForGesture: pointsStartEnd,
  isMeaningfulGesture: meaningfulSpan,
  bbox: (obj) => genericBBox(obj),
  classifyHit: (obj, x, y, pageW, pageH) => filledBoxClassify(genericBBox(obj), x, y, pageW, pageH),
  resize: (obj, handle, dx, dy) => genericResize(obj, handle, dx, dy),
};
