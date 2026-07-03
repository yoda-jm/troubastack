import { drawText } from "@troubastack/ink";
import {
  clamp01,
  insideBox,
  moveMarginFrac,
  textBBox,
  textHitPad,
} from "../editor";
import {
  type AnnotationTypeDescriptor,
  genericBBox,
  pointsAnchor,
  resizeBox,
} from "./types";

const icon = (
  <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round">
    <path d="M3.5 4h9M8 4v9M6 13h4" />
  </svg>
);

export const textDescriptor: AnnotationTypeDescriptor = {
  type: "text",
  tool: { id: "text", label: "Text", icon, cursorClass: "tool-text" },
  draw: drawText,
  styleControls: ["color", "opacity", "textSize"],
  pointsForGesture: pointsAnchor,
  isMeaningfulGesture: (path) => path.length >= 1,
  bbox: (obj, measure) => (measure ? textBBox(obj, measure) : genericBBox(obj)),
  classifyHit: (obj, x, y, pageW, pageH, measure) => {
    const b = measure ? textBBox(obj, measure) : genericBBox(obj);
    const tp = textHitPad(measure);
    const mX = moveMarginFrac("x", pageW, pageH);
    const mY = moveMarginFrac("y", pageW, pageH);
    if (insideBox(b, x, y, mX, mY)) return "strong";
    if (insideBox(b, x, y, tp.padX, tp.padY)) return "weak";
    return "none";
  },
  resize: (obj, handle, dx, dy, measure) => {
    const b = measure ? textBBox(obj, measure) : genericBBox(obj);
    const r = resizeBox(b, handle, dx, dy);
    const nextFont = Math.max(0.005, obj.style.fontSize * (r.sy > 0 ? r.sy : 1));
    return {
      ...obj,
      points: [{ x: clamp01(r.minX), y: clamp01(r.minY) }],
      style: { ...obj.style, fontSize: nextFont },
    };
  },
};
