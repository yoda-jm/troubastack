package chartpdf

import "troubastack/core/internal/domain"

// anchoringFrozen is the T193 (anchoring stage 0) switch. While it is true:
//   - every serve and bake path hands out each mark's STORED Points and Page, never a projection
//     (ServedObjects);
//   - a new mark is stored with no Anchor and an untouched PointsRenderHash (AnchorNewMark).
//
// Existing Anchor fields are kept, stored, exported and imported exactly as before, so stage 4 can still
// read them. It is a constant on purpose, not a setting: the old text+occurrence projection damages marks
// (docs/design/anchoring/critique.md), and nothing should be able to switch it back on at run time. Stage 2
// replaces the projection and removes this switch.
const anchoringFrozen = true

// AnchoringFrozen reports the T193 switch, for callers that want to skip the render they would only need
// for projecting or anchoring (serve, bake, realtime create).
func AnchoringFrozen() bool { return anchoringFrozen }

// The freeze is applied HERE, in two thin wrappers that every production caller goes through, and not
// inside Reproject / AnchorObject themselves, for two reasons:
//   - Reproject and AnchorObject stay the old mechanism, intact, so the tests that pin it
//     (TestOnRunAnchorIsUnchanged and the others in critique.md §7) keep testing it until stage 2
//     rewrites them;
//   - the one-off cmd/t193-freeze-served needs the UNFROZEN projection to compute what was served before
//     the freeze, so the coordinates VLL sees today can be written back as stored ones.

// ServedObjects is what serve (httpapi) and bake call to turn stored marks into the coordinates they hand
// out. Frozen: objs is returned as is (same slice, nothing re-projected, count 0). Unfrozen: Reproject.
func ServedObjects(objs []domain.Object, anchors []Anchor, renderHash string) ([]domain.Object, int) {
	if anchoringFrozen {
		return objs, 0
	}
	return Reproject(objs, anchors, renderHash)
}

// AnchorNewMark is what create-time anchoring (realtime create, HTTP import) calls. Frozen: the mark is
// returned unchanged, with no new Anchor and no PointsRenderHash change. Unfrozen: AnchorObject.
func AnchorNewMark(o domain.Object, anchors []Anchor, renderHash string) domain.Object {
	if anchoringFrozen {
		return o
	}
	return AnchorObject(o, anchors, renderHash)
}
