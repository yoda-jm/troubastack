/**
 * Arrow — the T07 registry-completeness DEMO (dev-only, localStorage.devArrow).
 *
 * Adding this whole type took exactly three sites: this descriptor file, one
 * entry in registry.ts (the DEV list), and the "arrow" string in ink's
 * InkObjectType. No edits to editor.ts's dispatchers, SongEditor's toolbar, or
 * ink's render path — which is the proof the registry is complete. The server
 * rejects arrow mutations until T09 wires proto/Go, hence the dev flag.
 */
import type { Ctx2D, InkObject, PageRect } from "@troubastack/ink";
import { distToSegment, strokeHitFrac, MIN_STROKE_HIT_PX } from "../editor";
import {
  type AnnotationTypeDescriptor,
  genericBBox,
  genericResize,
  meaningfulSpan,
  pointsStartEnd,
} from "./types";

const icon = (
  <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
    <path d="M3 13L13 3M13 3H8M13 3V8" />
  </svg>
);

/** Draw a line with a filled arrowhead at the end point. */
function drawArrow(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const a = obj.points[0];
  const b = obj.points[obj.points.length - 1];
  if (!a || !b) return;
  const ax = page.x + a.x * page.w;
  const ay = page.y + a.y * page.h;
  const bx = page.x + b.x * page.w;
  const by = page.y + b.y * page.h;
  const w = Math.max(0.5, obj.style.width * page.w);
  ctx.strokeStyle = obj.style.color;
  ctx.fillStyle = obj.style.color;
  ctx.lineWidth = w;
  ctx.beginPath();
  ctx.moveTo(ax, ay);
  ctx.lineTo(bx, by);
  ctx.stroke();
  const ang = Math.atan2(by - ay, bx - ax);
  const head = Math.max(6, w * 3.5);
  ctx.beginPath();
  ctx.moveTo(bx, by);
  ctx.lineTo(bx - head * Math.cos(ang - Math.PI / 6), by - head * Math.sin(ang - Math.PI / 6));
  ctx.lineTo(bx - head * Math.cos(ang + Math.PI / 6), by - head * Math.sin(ang + Math.PI / 6));
  ctx.closePath();
  ctx.fill();
}

export const arrowDescriptor: AnnotationTypeDescriptor = {
  type: "arrow",
  tool: { id: "arrow", label: "Arrow", icon, cursorClass: "tool-line" },
  draw: drawArrow,
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
