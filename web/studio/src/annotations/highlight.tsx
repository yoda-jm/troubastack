import { drawHighlight } from "@troubastack/ink";
import {
  type AnnotationTypeDescriptor,
  filledBoxClassify,
  genericBBox,
  genericResize,
  meaningfulSpan,
  pointsStartEnd,
} from "./types";

/**
 * Legacy "highlight" — a DEMOTED type: highlighting is now the "Highlight" shape
 * preset on rect/ellipse (#5), so there is no highlight TOOL. Old seeded/persisted
 * highlight objects still render, select, move and resize like a filled box.
 */
export const highlightDescriptor: AnnotationTypeDescriptor = {
  type: "highlight",
  // no `tool` → never offered in the palette (render/edit-only).
  draw: drawHighlight,
  styleControls: ["color", "opacity", "width", "shapePreset", "fillBorder", "blend"],
  pointsForGesture: pointsStartEnd,
  isMeaningfulGesture: meaningfulSpan,
  bbox: (obj) => genericBBox(obj),
  classifyHit: (obj, x, y, pageW, pageH) => filledBoxClassify(genericBBox(obj), x, y, pageW, pageH),
  resize: (obj, handle, dx, dy) => genericResize(obj, handle, dx, dy),
};
