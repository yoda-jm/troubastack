# T07 — Annotation-type descriptor registry (TS side)

**Priority:** 7 · **Size:** M · **Area:** `web/ink`, `web/studio/src/editor.ts`, `SongEditor.tsx`

## Context

Adding a new annotation type today (say, an arrow) means editing ~15 scattered sites:
the `switch (obj.type)` in `web/ink/src/index.ts` (~line 147), the `InkObjectType`
union, the per-type `if` ladders in `editor.ts` (`classifyHit` ~line 413,
`pointsForTool` ~121, `isMeaningfulGesture` ~154, `objectBBox`, `resizeObject`), plus
`TOOLS`, `buildWet`, `toInkObject`, and toolbar visibility logic in `SongEditor.tsx`,
plus the proto/Go sites (out of scope here — see T09). There is no single extension
point.

The Go core deliberately treats annotation types opaquely (no per-type logic in
engine/store/sync), so the *only* per-type knowledge lives in TS — which makes a TS
registry the complete fix for the client side.

Design goal: **not a plugin framework** — just one module per type collecting the
already-existing per-type functions, and one registry map. Simple, greppable, flat.

## Changes

1. Define a descriptor interface (in `web/studio/src/annotations/types.ts` or similar):

   ```ts
   interface AnnotationTypeDescriptor {
     type: InkObjectType;              // wire/string name
     tool?: {                          // absent for legacy/non-drawable types
       id: Tool; label: string; icon: ReactNode | string;
       cursor: string;                 // css cursor for the edit canvas
     };
     pointsForGesture(start: Pt, current: Pt, path: Pt[]): Pt[];   // from pointsForTool
     isMeaningfulGesture(points: Pt[], pageW: number, pageH: number): boolean;
     hitTest(obj, point, ctx): HitResult | null;                   // from classifyHit
     bbox(obj): BBox;                                              // from objectBBox
     resize(obj, handle, delta): InkObject;                        // from resizeObject
     draw(ctx2d, obj, page: PageRect, measure: TextMeasure): void; // from web/ink drawX
     styleControls: Array<"color"|"width"|"opacity"|"shapePreset"|"fillBorder"|"blend"|"textSize">;
   }
   ```

   Adjust signatures to what the existing functions actually take — the interface should
   *fit the current code*, not force a rewrite.
2. Create one descriptor module per existing type: `freehand`, `line`, `rect`,
   `ellipse`, `text`, and legacy `highlight` (render-only, no tool — it's a demoted
   preset). Move the bodies of the existing per-type branches into them **verbatim**
   where possible; this is a mechanical relocation, not a redesign.
3. Registry: `const ANNOTATION_TYPES: Record<InkObjectType, AnnotationTypeDescriptor>`,
   plus a `toolsInOrder()` helper for the toolbar.
4. Rewire the dispatch sites to registry lookups:
   - `web/ink` `renderObject`: look up `draw` (ink must not import studio — either keep
     ink's draw functions in ink and have descriptors reference them, or accept draw fns
     being *registered into* ink at startup; prefer the former: descriptors live in
     studio and import ink's exported per-type draw functions, which ink also uses
     internally for its own switch → then ink's switch is the one permitted duplicate.
     Simpler alternative if you can keep ink dependency-free: export ink's per-type draw
     functions and delete the switch by having `renderObject` take a lookup table with a
     default. Choose the option with the smallest diff and document it in the PR.)
   - `editor.ts`: `classifyHit` / `pointsForTool` / `isMeaningfulGesture` /
     `objectBBox` / `resizeObject` become thin registry dispatchers.
   - `SongEditor.tsx`: `TOOLS`, cursor classes, `buildWet`, `toInkObject`, and the
     style-control visibility switch read from the registry.
5. Prove it: as the last commit of the PR, add a **demo type** `arrow` end-to-end on the
   TS side (descriptor + draw function; wire value can temporarily reuse the generic
   points+style shape). It should require: 1 new descriptor file + 1 registry entry +
   (for now) the string added to the TS `InkObjectType` union. If it needs more sites
   than that, the registry isn't complete — fix that instead of patching around it.
   Note: until T09 adds the type to proto and the Go string maps, the server will reject
   arrow mutations — so keep the arrow tool behind a dev flag
   (`localStorage.devArrow === "1"`) and state this in the PR.

## Acceptance criteria

- `git grep -n "switch (obj.type)\|switch (o.type)" web/` shows at most the single
  documented dispatch point (or none).
- All existing tools draw/select/move/resize exactly as before — `make e2e` green.
- The arrow demo works behind its dev flag locally (draw, select, move, resize).
- Adding-a-type is documented in a short `web/studio/src/annotations/README.md`
  (list the 3 required steps + the proto/Go steps referencing T09).
- TS typecheck green across the workspace.

## Out of scope

- proto/Go changes (T09), native/Kotlin anything, bake.
- New user-visible types beyond the dev-flagged arrow demo.
