package httpapi

import (
	"testing"

	"troubastack/core/internal/domain"
)

// TestObjectJSON_AnchorRoundTrip (T145): the source-scoped anchor + the projected-cache render hash must
// survive the domain<->wire mapping, and a mark without an anchor (an uploaded PDF / legacy) must stay
// anchor-less rather than gain an empty one.
func TestObjectJSON_AnchorRoundTrip(t *testing.T) {
	o := domain.Object{
		UUID: "o1", LayerID: "L1", Type: domain.TypeRect, Page: 2,
		Points:           []domain.Point{{X: 0.1, Y: 0.2}, {X: 0.3, Y: 0.4}},
		Anchor:           &domain.SourceAnchor{RunText: "the verse line", Occurrence: 3, CharStart: 2, CharEnd: 9},
		PointsRenderHash: "sha-abc123",
	}
	got := objectFromJSON(objectToJSON(o))
	if got.Anchor == nil {
		t.Fatal("anchor lost in the domain<->wire round-trip")
	}
	if *got.Anchor != *o.Anchor {
		t.Fatalf("anchor changed: %+v vs %+v", *got.Anchor, *o.Anchor)
	}
	if got.PointsRenderHash != o.PointsRenderHash {
		t.Fatalf("PointsRenderHash lost: %q", got.PointsRenderHash)
	}
	if none := objectFromJSON(objectToJSON(domain.Object{UUID: "o2"})); none.Anchor != nil {
		t.Fatalf("a mark with no anchor must round-trip anchor-less, got %+v", none.Anchor)
	}
}
