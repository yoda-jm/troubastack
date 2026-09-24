package bake

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/testutil"
)

// The bake doc (annotations.go) is the FIFTH hand-written mirror of domain.Object/Style, and the only one
// whose output a musician reads directly: the other four lose a field from a wire, this one loses it from
// the page on the stand. It had no field-completeness guard, which is how the three entries in
// objectNotBaked below went unnoticed — each is a field that IS on the object, IS understood by the
// renderer that consumes this doc, and simply never leaves Go.
//
// The check is by FIELD NAME via reflection over the domain types, not against a list written beside them:
// a list beside the type is a second thing to remember, and the fields this guards went missing precisely
// because somebody had to remember.
//
// Values are asserted too. A DTO can declare a field and still not copy it — that is the shape of the
// Anchor and Pressure losses — so presence of the key is necessary and not sufficient.

// Object fields that do not reach the renderer doc. A reason that reads "not needed" is a decision; a
// reason that reads "dropped" is a DEFECT recorded where the next person will see it.
var objectNotBaked = map[string]string{
	"Version":          "server-derived LWW bookkeeping; nothing in a raster depends on it",
	"Deleted":          "tombstones are filtered before the doc is built — a deleted mark is not drawn",
	"OwnerID":          "ownership follows the LAYER; the renderer draws what the layer scoping let through",
	"Scope":            "layer-level, same as OwnerID",
	"Anchor":           "consumed BEFORE this point: T145 reprojection rewrites Points, so the doc carries the resolved coordinates and the anchor has done its work",
	"PointsRenderHash": "same — it says whether Points needed reprojecting, which has already happened",
}

// Point fields that do not reach the doc. Empty since T178 — a point member the renderer understands and
// never receives is a stroke drawn with invented weight.
var pointNotBaked = map[string]string{}

// Style fields that do not reach the doc. Empty today — a style field that does not reach the renderer is
// a style the musician does not see.
var styleNotBaked = map[string]string{}

// Go field name → the doc's JSON key, matched case-INSENSITIVELY: the two spellings differ in case only
// and not always predictably (UUID→"uuid", LayerID→"layerId"), and inventing a rule for that is a rule
// that would itself need maintaining.
func bakedKey(m map[string]any, field string) (any, bool) {
	want := strings.ToLower(field)
	for k, v := range m {
		if strings.ToLower(k) == want {
			return v, true
		}
	}
	return nil, false
}

func bakedObject(t *testing.T, obj domain.Object) map[string]any {
	t.Helper()
	obj.LayerID = "L1"
	obj.Deleted = false
	snap := domain.Snapshot{
		Layers:  []domain.Layer{{ID: "L1"}},
		Objects: []domain.Object{obj},
	}
	doc := snapshotToDoc(snap, "", nil, "")
	if len(doc.Objects) != 1 {
		t.Fatalf("expected the filled object to be baked, got %d objects", len(doc.Objects))
	}
	b, err := json.Marshal(doc.Objects[0])
	if err != nil {
		t.Fatalf("marshal baked object: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal baked object: %v", err)
	}
	return m
}

func TestBakeDoc_CarriesEveryObjectField(t *testing.T) {
	var obj domain.Object
	testutil.Fill(t, &obj, 1)
	obj.Type = domain.TypeLine // enums travel as STRINGS here; a filler int is not a member

	got := bakedObject(t, obj)

	ot := reflect.TypeOf(domain.Object{})
	for i := 0; i < ot.NumField(); i++ {
		name := ot.Field(i).Name
		if _, skipped := objectNotBaked[name]; skipped {
			continue
		}
		if _, ok := bakedKey(got, name); !ok {
			t.Errorf("domain.Object.%s never reaches the bake doc.\n"+
				"Either carry it in snapshotToDoc (and docObject), or add it to objectNotBaked with the reason.", name)
		}
	}
}

func TestBakeDoc_CarriesEveryStyleField(t *testing.T) {
	var obj domain.Object
	testutil.Fill(t, &obj, 2)
	obj.Type = domain.TypeLine

	got := bakedObject(t, obj)
	style, ok := got["style"].(map[string]any)
	if !ok {
		t.Fatalf("baked object has no style object: %#v", got["style"])
	}

	st := reflect.TypeOf(domain.Style{})
	for i := 0; i < st.NumField(); i++ {
		name := st.Field(i).Name
		if _, skipped := styleNotBaked[name]; skipped {
			continue
		}
		if _, ok := bakedKey(style, name); !ok {
			t.Errorf("domain.Style.%s never reaches the bake doc — the renderer cannot draw what it is not sent.\n"+
				"Carry it in snapshotToDoc (and docStyle), or add it to styleNotBaked with the reason.", name)
		}
	}

	// Presence is not enough: a DTO can declare a field and copy nothing into it.
	if style["dash"] != obj.Style.Dash {
		t.Errorf("T177 dash: baked %v, object had %q", style["dash"], obj.Style.Dash)
	}
	ends, ok := style["ends"].(map[string]any)
	if !ok {
		t.Fatalf("T177 ends: the nested record did not reach the doc: %#v", style["ends"])
	}
	if ends["start"] != obj.Style.Ends.Start || ends["end"] != obj.Style.Ends.End {
		t.Errorf("T177 ends: baked %v, object had %+v", ends, *obj.Style.Ends)
	}
}

func TestBakeDoc_CarriesEveryPointField(t *testing.T) {
	var obj domain.Object
	testutil.Fill(t, &obj, 4)
	obj.Type = domain.TypeFreehand

	got := bakedObject(t, obj)
	pts, ok := got["points"].([]any)
	if !ok || len(pts) == 0 {
		t.Fatalf("baked object has no points: %#v", got["points"])
	}
	pt, ok := pts[0].(map[string]any)
	if !ok {
		t.Fatalf("baked point is not an object: %#v", pts[0])
	}

	ptType := reflect.TypeOf(domain.Point{})
	for i := 0; i < ptType.NumField(); i++ {
		name := ptType.Field(i).Name
		if _, skipped := pointNotBaked[name]; skipped {
			continue
		}
		if _, ok := bakedKey(pt, name); !ok {
			t.Errorf("domain.Point.%s never reaches the bake doc.\n"+
				"Carry it in snapshotToDoc (and docPoint), or add it to pointNotBaked with the reason.", name)
		}
	}
}
