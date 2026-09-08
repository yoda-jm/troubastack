package httpapi

import (
	"testing"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/testutil"
)

// The REST DTO in annotations.go is the same hand-built mirror as the realtime one (Fable, ⟨GO⟩ dd6fed2d):
// domain.Object written and read field by field, Style expanded member by member, the anchor hand-built.
// It is the shape the studio LOADS a song through, so a field missing here is a field the editor never
// knows about — and then writes back as absent the first time the user moves the mark.
var objectNotInTheDTO = map[string]string{
	"Version": "server-derived; the DTO carries no version (the engine owns LWW)",
	"Deleted": "the GET returns live objects; a tombstone is not rendered",
	"OwnerID": "object ownership follows the LAYER in this API",
	"Scope":   "layer-level (largely subsumed by layer role_tag, see domain.Scope)",
}

func TestAnnotationsDTO_CarriesEveryObjectField(t *testing.T) {
	var want domain.Object
	testutil.Fill(t, &want, 0)
	want.Type = domain.TypeFreehand // enums map by STRING here; a filler int would read as a mirror bug
	want.Deleted = false
	want.Version = 0
	want.OwnerID = ""
	want.Scope = domain.ScopeUnspecified

	got := objectFromJSON(objectToJSON(want))

	if diff := testutil.DiffFields(want, got, objectNotInTheDTO); len(diff) > 0 {
		t.Fatalf("domain.Object %v did not survive the REST DTO round-trip.\n  wrote %#v\n  read  %#v\n"+
			"Either carry each in annotations.go (objectToJSON AND objectFromJSON), or add it to objectNotInTheDTO with the reason.",
			diff, want, got)
	}
	if got.Anchor == nil || *got.Anchor != *want.Anchor {
		t.Fatalf("the T145 anchor did not survive: wrote %+v, read %+v", want.Anchor, got.Anchor)
	}
}

func TestAnnotationsDTO_CarriesEveryLayerField(t *testing.T) {
	var want domain.Layer
	testutil.Fill(t, &want, 5)
	want.Zone = domain.ZoneConductor
	want.Access = domain.AccessRO

	got := layerFromJSON(layerToJSON(want))

	if diff := testutil.DiffFields(want, got, map[string]string{}); len(diff) > 0 {
		t.Fatalf("domain.Layer %v did not survive the REST DTO round-trip.\n  wrote %#v\n  read  %#v",
			diff, want, got)
	}
}
