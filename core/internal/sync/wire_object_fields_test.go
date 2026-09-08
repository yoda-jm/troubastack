package sync

import (
	"testing"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/testutil"
)

// `mapping.go` is a hand-maintained mirror of domain.Object, written and read field by field with the Style
// sub-struct expanded member by member and anchorToJSON/anchorFromJSON hand-built — the same construction
// that silently dropped five fields in the band folder (Fable, ⟨GO⟩ dd6fed2d).
//
// And this one matters MORE than the folder it was copied from. A band folder is a backup: a field lost
// there is still on the live server. THIS is the realtime wire that every live edit crosses between two
// musicians, with no second copy — a field dropped here is simply gone, and the symptom is "the thing I
// drew looked different on their screen", which nobody reports as a data bug.
//
// It is complete today. The guard is so that it stays complete without anyone remembering.
var objectNotOnTheWire = map[string]string{
	// Server-derived: the hub stamps the version from HEAD and ignores any client value (apply.go), which
	// is what stops a client claiming a version to win an LWW it should lose.
	"Version": "server-derived per mutation; the wire object deliberately carries none",
	// A tombstone is a `delete` MUTATION, never a flag on the object. An object arriving with Deleted=true
	// would be a client asserting a deletion outside the mutation kind that authorizes it.
	"Deleted": "deletion is a mutation kind, not an object field",
	// Ownership and scope are properties of the LAYER on this wire; the hub stamps the author itself.
	"OwnerID": "the hub stamps the authoritative author; object ownership follows the layer",
	"Scope":   "layer-level on this wire (largely subsumed by layer role_tag, see domain.Scope)",
}

func TestSyncWire_CarriesEveryObjectField(t *testing.T) {
	var want domain.Object
	testutil.Fill(t, &want, 0)
	want.Type = domain.TypeIcon // enums map by STRING on this wire: a filler int would read as a bug
	want.Deleted = false
	want.Version = 0
	want.OwnerID = ""
	want.Scope = domain.ScopeUnspecified

	got := objectFromJSON(objectToJSON(want))

	if diff := testutil.DiffFields(want, got, objectNotOnTheWire); len(diff) > 0 {
		t.Fatalf("domain.Object %v did not survive the realtime wire round-trip.\n  wrote %#v\n  read  %#v\n"+
			"Either carry each in mapping.go (objectToJSON AND objectFromJSON), or add it to objectNotOnTheWire with the reason.",
			diff, want, got)
	}
	// The anchor is a POINTER built by hand on both sides: a nil-vs-set mistake would pass a field compare
	// on a fixture that happened to leave it nil, so it is asserted deeply and explicitly.
	if got.Anchor == nil || *got.Anchor != *want.Anchor {
		t.Fatalf("the T145 anchor did not survive: wrote %+v, read %+v", want.Anchor, got.Anchor)
	}
	if got.Anchor == want.Anchor {
		t.Fatalf("the wire returned the SAME anchor pointer — a mutation on one side would alias the other")
	}
}

// The layer mirror on the same wire, for the same reason: a layer's fields are the visibility rules, and
// zone/access are enums that map by string in both directions.
var layerNotOnTheWire = map[string]string{}

func TestSyncWire_CarriesEveryLayerField(t *testing.T) {
	var want domain.Layer
	testutil.Fill(t, &want, 5)
	want.Zone = domain.ZonePersonal // enums, again: real members only
	want.Access = domain.AccessRO   // and RO is the one whose loss is a leak

	got := layerFromJSON(layerToJSON(want))

	if diff := testutil.DiffFields(want, got, layerNotOnTheWire); len(diff) > 0 {
		t.Fatalf("domain.Layer %v did not survive the realtime wire round-trip.\n  wrote %#v\n  read  %#v\n"+
			"Carry each in mapping.go (layerToJSON AND layerFromJSON).", diff, want, got)
	}
}
