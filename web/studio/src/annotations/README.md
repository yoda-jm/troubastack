# Annotation types

Per-type behavior lives in one **descriptor** per type; `registry.ts` collects
them and `editor.ts` / `SongEditor.tsx` dispatch through the registry instead of
scattered `if (obj.type === …)` ladders. `web/ink` renders via a draw-fn lookup
(no `switch`) that the registry populates at startup, so ink stays framework- and
studio-free.

A descriptor (`AnnotationTypeDescriptor` in `types.ts`) collects that type's
already-existing functions: `draw` (the `web/ink` per-type fn), `pointsForGesture`,
`isMeaningfulGesture`, `bbox`, `classifyHit`, `resize`, plus toolbar metadata
(`tool`) and `styleControls` (drives contextual toolbar visibility). Generic
building blocks (`genericBBox`, `genericResize`, `filledBoxClassify`, …) live in
`types.ts` so most descriptors are a few lines.

## Adding a type (TS/client side) — 3 steps

1. **Descriptor file** `web/studio/src/annotations/<type>.tsx` exporting a
   `<type>Descriptor: AnnotationTypeDescriptor` (its `draw` fn, hit/geometry, and
   — if drawable — a `tool` with an inline-SVG icon).
2. **Register it**: add the descriptor to the list in `registry.ts`.
3. **Wire string**: add the type to `InkObjectType` in `web/ink/src/index.ts`.

That's all the client needs — no edits to `editor.ts`, `SongEditor.tsx`, or ink's
render path. See `arrow.tsx` for a worked example (a dev-only demo gated behind
`localStorage.devArrow === "1"`).

## Server side (separate — T09)

The Go core treats annotation types opaquely (no per-type logic in
engine/store/sync), so no Go behavior changes. But to persist/sync a new type you
must add it to **proto** and the Go string maps — tracked in **T09
(proto reconciliation)**. Until then the server rejects mutations of the new type,
which is why the arrow demo is dev-flagged.
