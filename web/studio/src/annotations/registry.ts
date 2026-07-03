/**
 * The annotation-type registry (T07). Collects the per-type descriptors and
 * registers each type's draw fn into ink. editor.ts and SongEditor dispatch
 * through here — the single, flat, greppable extension point.
 */
import { registerInkDraw } from "@troubastack/ink";
import type { AnnotationTypeDescriptor, ToolDef } from "./types";
import { freehandDescriptor } from "./freehand";
import { lineDescriptor } from "./line";
import { rectDescriptor } from "./rect";
import { ellipseDescriptor } from "./ellipse";
import { textDescriptor } from "./text";
import { highlightDescriptor } from "./highlight";
import { arrowDescriptor } from "./arrow";

const BASE: AnnotationTypeDescriptor[] = [
  freehandDescriptor,
  lineDescriptor,
  rectDescriptor,
  ellipseDescriptor,
  textDescriptor,
  highlightDescriptor,
];

// Dev-only arrow demo (T07): behind localStorage.devArrow === "1". Proves the
// registry is complete — a new type is 1 descriptor file + 1 entry here + the
// ink InkObjectType string. The server rejects arrow mutations until T09 adds it
// to proto + the Go string maps, so it stays flag-gated.
const DEV: AnnotationTypeDescriptor[] =
  typeof localStorage !== "undefined" && localStorage.devArrow === "1" ? [arrowDescriptor] : [];

const ALL: AnnotationTypeDescriptor[] = [...BASE, ...DEV];

/** type name → descriptor. */
export const ANNOTATION_TYPES: Record<string, AnnotationTypeDescriptor> = Object.fromEntries(
  ALL.map((d) => [d.type, d]),
);

// Register every type's draw fn into ink (idempotent for built-ins; adds arrow
// when flagged). ink renders whatever is registered, staying studio-free.
for (const d of ALL) registerInkDraw(d.type, d.draw);

/** Descriptor for a type, or undefined for an unknown/unregistered type. */
export function descriptorFor(type: string): AnnotationTypeDescriptor | undefined {
  return ANNOTATION_TYPES[type];
}

/** Drawable tools in palette order (the toolbar prepends Select itself). */
export function toolsInOrder(): ToolDef[] {
  return ALL.filter((d): d is AnnotationTypeDescriptor & { tool: ToolDef } => d.tool != null).map(
    (d) => d.tool,
  );
}
