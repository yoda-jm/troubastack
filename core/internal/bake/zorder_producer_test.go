package bake

import (
	"encoding/json"
	"sort"
	"testing"

	"troubastack/core/internal/domain"
)

// T178 — the PRODUCER side of z-order, which is where the defect lived and where nothing was looking.
//
// `web/bake/test/zorder.test.mjs` proves the RENDERER honours `order -> createdAt -> uuid`. It is handed a
// doc with those fields already in it, so it stayed green for months while this producer sent neither: both
// read 0 in the comparator, which fell through to UUID, and the baked page stacked overlapping marks by an
// internal id while the screen stacked them by drawing time.
//
// A green consumer test cannot see a silent producer. So this asserts the crossing itself — that what
// snapshotToDoc emits is enough for the renderer's contract to produce DRAWING order — and it is built so
// that it can only pass for the right reason: the fixture's UUID order deliberately CONTRADICTS its
// createdAt order, which is exactly the case the old code got wrong and a same-order fixture cannot detect.

// replayObjectZ is render.ts's objectZ, applied to the doc as it is actually serialised. Keeping it here
// rather than comparing field-by-field is the point: the question is not "did we copy CreatedAt", it is
// "does the renderer, reading what we sent, draw them in the right order".
func replayObjectZ(objs []docObject) []string {
	out := append([]docObject(nil), objs...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt < out[j].CreatedAt
		}
		return out[i].UUID < out[j].UUID
	})
	ids := make([]string, len(out))
	for i, o := range out {
		ids[i] = o.UUID
	}
	return ids
}

func TestBakeDoc_StackingSurvivesToTheRenderer_T178(t *testing.T) {
	// "zzz" was drawn FIRST and "aaa" second, so drawing order is zzz then aaa — the opposite of what
	// sorting by uuid gives. Before T178 the doc carried no createdAt, the comparator fell through to
	// uuid, and the bake drew aaa on top of a mark that was underneath it on screen.
	snap := domain.Snapshot{
		Layers: []domain.Layer{{ID: "L1"}},
		Objects: []domain.Object{
			{UUID: "zzz-drawn-first", LayerID: "L1", Type: domain.TypeRect, CreatedAt: 1000,
				Points: []domain.Point{{X: 0.1, Y: 0.1}, {X: 0.5, Y: 0.5}}},
			{UUID: "aaa-drawn-second", LayerID: "L1", Type: domain.TypeRect, CreatedAt: 2000,
				Points: []domain.Point{{X: 0.3, Y: 0.3}, {X: 0.7, Y: 0.7}}},
		},
	}

	doc := snapshotToDoc(snap, "", nil, "")
	got := replayObjectZ(doc.Objects)
	want := []string{"zzz-drawn-first", "aaa-drawn-second"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the renderer would draw %v; drawing order is %v.\n"+
				"The doc must carry CreatedAt: without it the comparator falls through to UUID and the baked "+
				"page stacks by an internal id instead of by when the musician drew each mark.", got, want)
		}
	}

	// And the fixture has to be one that could FAIL. If uuid order already matched drawing order, the
	// assertion above would pass against a doc carrying nothing at all.
	byUUID := append([]docObject(nil), doc.Objects...)
	sort.SliceStable(byUUID, func(i, j int) bool { return byUUID[i].UUID < byUUID[j].UUID })
	if byUUID[0].UUID == want[0] {
		t.Fatal("fixture is not discriminating: its UUID order already equals its drawing order, so this " +
			"test would pass even with CreatedAt dropped. Pick uuids that contradict the drawing order.")
	}
}

func TestBakeDoc_ExplicitOrderStillOutranksDrawingTime_T178(t *testing.T) {
	// The tiebreak must stay a TIEBREAK: a bring-to-front (Order) has to beat drawing time, or fixing the
	// fallthrough would quietly break the feature the fallthrough was standing in for.
	snap := domain.Snapshot{
		Layers: []domain.Layer{{ID: "L1"}},
		Objects: []domain.Object{
			{UUID: "a-recent-but-sent-back", LayerID: "L1", Type: domain.TypeRect, CreatedAt: 9000, Order: -1,
				Points: []domain.Point{{X: 0.1, Y: 0.1}, {X: 0.5, Y: 0.5}}},
			{UUID: "b-older-but-brought-front", LayerID: "L1", Type: domain.TypeRect, CreatedAt: 1000, Order: 5,
				Points: []domain.Point{{X: 0.3, Y: 0.3}, {X: 0.7, Y: 0.7}}},
		},
	}
	got := replayObjectZ(snapshotToDoc(snap, "", nil, "").Objects)
	if got[len(got)-1] != "b-older-but-brought-front" {
		t.Fatalf("drew %v; an explicit Order must outrank CreatedAt — otherwise bring-to-front is lost", got)
	}
}

func TestBakeDoc_PressureReachesTheStroke_T178(t *testing.T) {
	// Without Pressure on the wire, ink turns SIMULATION on (it simulates when every point lacks it), so a
	// stylus stroke bakes with an invented weight. Nothing in VLL's library carries pressure today, which
	// is why this was invisible; it is a defect regardless of today's data.
	snap := domain.Snapshot{
		Layers: []domain.Layer{{ID: "L1"}},
		Objects: []domain.Object{{
			UUID: "o1", LayerID: "L1", Type: domain.TypeFreehand,
			Points: []domain.Point{{X: 0.1, Y: 0.1, Pressure: 0.25}, {X: 0.4, Y: 0.4, Pressure: 0.9}},
		}},
	}
	doc := snapshotToDoc(snap, "", nil, "")
	b, err := json.Marshal(doc.Objects[0])
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Points []struct {
			Pressure float64 `json:"pressure"`
		} `json:"points"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.Points[0].Pressure != 0.25 || m.Points[1].Pressure != 0.9 {
		t.Fatalf("pressure did not reach the doc: got %v, want 0.25 then 0.9", m.Points)
	}

	// A finger-drawn stroke must stay byte-identical to what it baked before T178 — omitempty means the
	// key is simply absent, not present-and-zero.
	plain := domain.Snapshot{
		Layers:  []domain.Layer{{ID: "L1"}},
		Objects: []domain.Object{{UUID: "o2", LayerID: "L1", Type: domain.TypeFreehand, Points: []domain.Point{{X: 0.1, Y: 0.1}}}},
	}
	pb, _ := json.Marshal(snapshotToDoc(plain, "", nil, "").Objects[0])
	var raw map[string]any
	_ = json.Unmarshal(pb, &raw)
	pts := raw["points"].([]any)
	if _, present := pts[0].(map[string]any)["pressure"]; present {
		t.Error("a point with no pressure emitted a `pressure` key — every finger-drawn stroke in his " +
			"library would change bytes for nothing")
	}
}
