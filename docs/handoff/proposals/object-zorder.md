# Proposal — per-object z-order (T27 stage 2 dependency; for arch decision)

**Status:** proposal, awaiting arch decision · **Raised by:** Web-Core (2026-07-08) ·
**Area:** `proto/` (`object.proto`), `core/` (domain + apply + both repos + sync WS),
`web/studio` (sync client + Viewer render + selection toolbar) · **Relates to:** T27
stage 2 (contextual toolbar), the R7 z-order design note in `object.proto`.

VLL asked (chat, 2026-07-08) to **build full z-order now** as part of T27 stage 2's
selection toolbar (color / **z-order** / duplicate / delete). Investigating, z-order is
not UI-only: it's a proto + data-model change that brushes the documented R7 decision
and the LWW conflict model — so per the workflow (proto/data-model → arch), it's raised
here before I build.

## The gap

- `Object` (object.proto) has **no order field**. The viewer renders objects
  **layer-major** (`sortedLayers` z-rank), and *within* a layer in **insertion/array
  order** (a stable sort keyed only on `layerId`). So "bring to front / send to back"
  for a single object is not representable, and there is **no reorder mutation**
  (mutation kinds: create / move / resize / setStyle / delete).
- **R7** (object.proto `LayerZone` note): the per-viewer stack is zone-major and
  "removes all global z-order contention: each owner orders only WITHIN their own zone."
  R7 governs **layer** stacking across zones/owners. It does **not** address ordering
  **within a single layer** — which is what a selection toolbar's z-order needs.

## Proposed design (within-layer object order — R7-preserving)

1. **Proto:** add `int32 order = 11;` to `Object` (parallels `Layer.order = 6`).
   Ordering is **within one layer only**; cross-layer/zone stacking stays fixed by
   zone (R7 untouched — no global contention reintroduced).
2. **Render:** within a layer, sort objects by `order` (tiebreak `created_at`, then
   `uuid`) instead of raw array order. Layer-major ordering is unchanged.
3. **Mutation:** a new **`reorder`** mutation kind carrying the object with its new
   `order`, gated exactly like move/resize/setStyle (only the ACTIVE editable layer's
   objects; owner/RW), LWW via the existing `version` bump. Only two ops are exposed in
   the UI — **bring-to-front** (`order = maxSiblingOrder + 1`) and **send-to-back**
   (`order = minSiblingOrder − 1`); computed client-side from current siblings on that
   layer/page. (No arbitrary drag-reorder — keeps the surface small.)
4. **Both repos** (mem + file) persist `order`; the WS snapshot carries it.
5. **Back-compat:** existing objects default `order = 0`; equal-order ties fall back to
   `created_at`/`uuid`, so today's insertion-order rendering is preserved for untouched
   docs.

## Questions for arch

1. **OK to add `Object.order`** (within-layer, owner/RW-gated, LWW via `version`)?
   Confirm this does not offend R7 (I read R7 as *layer/zone* stacking, orthogonal to
   *within-layer* object order).
2. **int + front/back bumps** (my proposal) vs a **fractional/float order** (midpoint
   insert, no renumber storms) — the latter is only worth it if arbitrary reorder is
   ever wanted; for bring-to-front/send-to-back, int is enough. Preference?
3. **New `reorder` mutation kind** vs. carrying `order` on the existing update path —
   I prefer a distinct kind for clarity/telemetry; ok?
4. **Stage-2 sequencing (shift):** the T27 spec's other half — *"style row only when a
   draw tool is active"* — causes the **canvas to shift** in the current *stacked*
   layout (the style block mounts/unmounts). The floating chrome that makes contextual
   show/hide zero-shift is **stage 3** (gated on T15). **Proposal:** stage 2 delivers
   the **floating selection toolbar** (`position:absolute` over the canvas — no shift) +
   z-order + duplicate + color, and the **style-row auto-hide moves to stage 3** (paired
   with the floating layout). Endorse, or accept a transient shift in stage 2?

## What I can build immediately once ruled

Duplicate (client-only: `createObject` a copy on the active layer, small offset) and the
floating selection toolbar shell are unblocked; z-order needs Q1–Q3; the style-row
auto-hide needs the Q4 ruling. Not implementing until the arch decision lands.
