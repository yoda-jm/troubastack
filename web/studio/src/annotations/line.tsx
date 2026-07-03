import { drawLine } from "@troubastack/ink";
import { distToSegment, strokeHitFrac, MIN_STROKE_HIT_PX } from "../editor";
import {
  type AnnotationTypeDescriptor,
  genericBBox,
  genericResize,
  meaningfulSpan,
  pointsStartEnd,
} from "./types";

const icon = (
  <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round">
    <path d="M3 13L13 3" />
  </svg>
);

export const lineDescriptor: AnnotationTypeDescriptor = {
  type: "line",
  tool: { id: "line", label: "Line", icon, cursorClass: "tool-line" },
  draw: drawLine,
  styleControls: ["color", "opacity", "width"],
  pointsForGesture: pointsStartEnd,
  isMeaningfulGesture: meaningfulSpan,
  bbox: (obj) => genericBBox(obj),
  classifyHit: (obj, x, y, pageW, pageH) => {
    const a = obj.points[0];
    const c = obj.points[obj.points.length - 1];
    if (!a || !c) return "none";
    const tol = Math.max(strokeHitFrac(obj, "x", pageW, pageH), strokeHitFrac(obj, "y", pageW, pageH));
    const d = distToSegment({ x, y }, a, c);
    if (d <= tol) return "strong";
    const weakPad = pageW > 0 ? (MIN_STROKE_HIT_PX + 6) / Math.min(pageW, pageH) : 0.02;
    if (d <= tol + weakPad) return "weak";
    return "none";
  },
  resize: (obj, handle, dx, dy) => genericResize(obj, handle, dx, dy),
};
