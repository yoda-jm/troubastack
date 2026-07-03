import { drawFreehand } from "@troubastack/ink";
import { distToPolyline, strokeHitFrac, MIN_STROKE_HIT_PX } from "../editor";
import {
  type AnnotationTypeDescriptor,
  genericBBox,
  genericResize,
  pointsFull,
} from "./types";

const icon = (
  <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round">
    <path d="M10.5 2.5l3 3-8 8-3.5.6.6-3.5z" />
    <path d="M9.5 3.5l3 3" />
  </svg>
);

export const freehandDescriptor: AnnotationTypeDescriptor = {
  type: "freehand",
  tool: { id: "freehand", label: "Pen", icon, cursorClass: "tool-freehand" },
  draw: drawFreehand,
  styleControls: ["color", "opacity", "width"],
  pointsForGesture: pointsFull,
  isMeaningfulGesture: (path) => path.length >= 2,
  bbox: (obj) => genericBBox(obj),
  classifyHit: (obj, x, y, pageW, pageH) => {
    const tol = Math.max(strokeHitFrac(obj, "x", pageW, pageH), strokeHitFrac(obj, "y", pageW, pageH));
    const d = distToPolyline({ x, y }, obj.points);
    if (d <= tol) return "strong";
    const weakPad = pageW > 0 ? (MIN_STROKE_HIT_PX + 6) / Math.min(pageW, pageH) : 0.02;
    if (d <= tol + weakPad) return "weak";
    return "none";
  },
  resize: (obj, handle, dx, dy) => genericResize(obj, handle, dx, dy),
};
