package annrecover

import "testing"

import "troubastack/core/internal/domain"

func obj(uuid, layer string, deleted bool) domain.Object {
	return domain.Object{UUID: uuid, LayerID: layer, Points: []domain.Point{{X: 0.1, Y: 0.2}}, Deleted: deleted}
}

// TestBuildPlan_CopiesAbsentAndIsIdempotent (⟨R1⟩): a fully-orphaned stream copies every object + its
// layer; feeding the post-copy live state back copies nothing (safe to run twice).
func TestBuildPlan_CopiesAbsentAndIdempotent(t *testing.T) {
	archObjs := []domain.Object{obj("o1", "L1", false), obj("o2", "L1", false)}
	archLayers := []domain.Layer{{ID: "L1", FileID: "f1", Name: "Cues"}}

	p := BuildPlan(archObjs, archLayers, nil, nil)
	if len(p.ObjectsToCopy) != 2 || len(p.LayersToCreate) != 1 {
		t.Fatalf("first pass: got %d objs / %d layers, want 2 / 1", len(p.ObjectsToCopy), len(p.LayersToCreate))
	}
	// Second run against the state AFTER the copy → nothing to do.
	p2 := BuildPlan(archObjs, archLayers, archObjs, archLayers)
	if len(p2.ObjectsToCopy) != 0 || len(p2.LayersToCreate) != 0 {
		t.Fatalf("second pass not idempotent: %d objs / %d layers", len(p2.ObjectsToCopy), len(p2.LayersToCreate))
	}
}

// Teeth: an archived UUID already live must not be duplicated, and its layer (already live) not re-created.
func TestBuildPlan_SkipsAlreadyPresent(t *testing.T) {
	archObjs := []domain.Object{obj("o1", "L1", false), obj("o2", "L1", false)}
	archLayers := []domain.Layer{{ID: "L1"}}
	live := []domain.Object{obj("o1", "L1", false)}
	liveLayers := []domain.Layer{{ID: "L1"}}

	p := BuildPlan(archObjs, archLayers, live, liveLayers)
	if len(p.ObjectsToCopy) != 1 || p.ObjectsToCopy[0].UUID != "o2" {
		t.Fatalf("want only o2 copied, got %+v", p.ObjectsToCopy)
	}
	if len(p.LayersToCreate) != 0 {
		t.Fatalf("layer L1 already live, must not be re-created: %+v", p.LayersToCreate)
	}
}

// A recovered object carries NO Anchor and NO PointsRenderHash — reverting this to "anchor it while we're
// here" must redden (the T159 ⚠ trap).
func TestBuildPlan_NeverAnchors(t *testing.T) {
	archObjs := []domain.Object{obj("o1", "L1", false)}
	p := BuildPlan(archObjs, []domain.Layer{{ID: "L1"}}, nil, nil)
	if len(p.ObjectsToCopy) != 1 {
		t.Fatal("expected one object copied")
	}
	got := p.ObjectsToCopy[0]
	if got.Anchor != nil || got.PointsRenderHash != "" {
		t.Fatalf("recovery must NOT anchor: Anchor=%v PointsRenderHash=%q", got.Anchor, got.PointsRenderHash)
	}
	if len(got.Points) != 1 {
		t.Fatalf("points must be preserved exactly, got %d", len(got.Points))
	}
}

// Archive tombstones are not resurrected.
func TestBuildPlan_SkipsTombstones(t *testing.T) {
	p := BuildPlan([]domain.Object{obj("dead", "L1", true)}, []domain.Layer{{ID: "L1"}}, nil, nil)
	if len(p.ObjectsToCopy) != 0 {
		t.Fatalf("a deleted archived object must not be restored: %+v", p.ObjectsToCopy)
	}
}

func TestMatchTarget(t *testing.T) {
	idx := map[string][]string{
		TargetKey("Good Vibes Only", "Amsterdam"): {"song-a"},
		TargetKey("Good Vibes Only", "Twin"):      {"song-b", "song-c"},
	}
	if id, err := MatchTarget("Good Vibes Only", "Amsterdam", idx); err != nil || id != "song-a" {
		t.Fatalf("unambiguous match: got %q, %v", id, err)
	}
	if _, err := MatchTarget("Good Vibes Only", "Twin", idx); err == nil {
		t.Fatal("ambiguous title must abort the stream")
	}
	if _, err := MatchTarget("Good Vibes Only", "Missing", idx); err == nil {
		t.Fatal("no match must abort the stream")
	}
}
