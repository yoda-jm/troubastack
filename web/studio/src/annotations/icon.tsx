/**
 * T51 — icon stamp descriptor: place a tinted glyph (the T50 set) as a page-space
 * annotation. Geometry is a bbox (like rect); the glyph id rides in the object's
 * `text` (set from the floating glyph palette when the Icon tool is active — see
 * IconGlyphPalette). `draw` is ink's shared `drawIcon`, so studio + the bake render
 * identically from the one `glyphs.json`.
 */
import { drawIcon } from "@troubastack/ink";
import {
  type AnnotationTypeDescriptor,
  filledBoxClassify,
  genericBBox,
  genericResize,
  meaningfulSpan,
  pointsStartEnd,
} from "./types";

const icon = (
  <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.5">
    <rect x="2.5" y="2.5" width="11" height="11" rx="2" />
    <circle cx="8" cy="8" r="2.4" fill="currentColor" stroke="none" />
  </svg>
);

export const iconDescriptor: AnnotationTypeDescriptor = {
  type: "icon",
  tool: { id: "icon", label: "Icon", icon, cursorClass: "tool-icon" },
  draw: drawIcon,
  // Tint + opacity only — no stroke width / shape preset / font.
  styleControls: ["color", "opacity"],
  pointsForGesture: pointsStartEnd,
  isMeaningfulGesture: meaningfulSpan,
  bbox: (obj) => genericBBox(obj),
  classifyHit: (obj, x, y, pageW, pageH) => filledBoxClassify(genericBBox(obj), x, y, pageW, pageH),
  resize: (obj, handle, dx, dy) => genericResize(obj, handle, dx, dy),
};
