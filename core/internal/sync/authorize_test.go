package sync

import (
	"testing"

	"troubastack/core/internal/domain"
)

// fakeEngine drives authorizeWrite's layer lookups. Only Layer/ObjectLayer matter here; each test row
// configures what the current HEAD would return for its target layer/object.
type fakeEngine struct {
	layer      domain.Layer
	layerFound bool // Layer()'s ok, and ObjectLayer()'s layerFound
	objExists  bool // ObjectLayer()'s objExists
}

func (f fakeEngine) Apply(string, domain.Mutation) (domain.Mutation, error) {
	return domain.Mutation{}, nil
}
func (f fakeEngine) Head(string) (domain.Snapshot, error)      { return domain.Snapshot{}, nil }
func (f fakeEngine) Layer(string, string) (domain.Layer, bool) { return f.layer, f.layerFound }
func (f fakeEngine) ObjectLayer(string, string) (domain.Layer, bool, bool) {
	return f.layer, f.layerFound, f.objExists
}

// TestAuthorizeWrite is the readable statement of the live write-access policy AND its guard. It covers
// the DENY cases for every role, not just the grants — replacing authorizeWrite's body with an
// unconditional `return "", true` reddens this table (T109; teeth-checked, see the task note).
func TestAuthorizeWrite(t *testing.T) {
	const me = "me"
	// A layer owned by someone else, in the shared zone, read-only unless said otherwise.
	other := func(zone domain.Zone, access domain.Access) domain.Layer {
		return domain.Layer{OwnerID: "someone-else", Zone: zone, Access: access}
	}
	mine := func(zone domain.Zone) domain.Layer { return domain.Layer{OwnerID: me, Zone: zone} }

	cases := []struct {
		name       string
		role       string
		kind       domain.Kind
		in         mutationJSON
		eng        fakeEngine
		wantReason string
		wantOK     bool
	}{
		// ---- layerCreate: gated on the SUBMITTED layer (zoneFromString + OwnerID) ----
		{"layerCreate/nil layer is allowed", "member", domain.KindLayerCreate, mutationJSON{}, fakeEngine{}, "", true},
		{"layerCreate/member: own personal layer", "member", domain.KindLayerCreate,
			mutationJSON{Layer: &layerJSON{Zone: "personal", OwnerID: me}}, fakeEngine{}, "", true},
		{"layerCreate/member: personal layer owned by another → DENY", "member", domain.KindLayerCreate,
			mutationJSON{Layer: &layerJSON{Zone: "personal", OwnerID: "someone-else"}}, fakeEngine{}, "forbidden", false},
		{"layerCreate/member: conductor-zone → DENY", "member", domain.KindLayerCreate,
			mutationJSON{Layer: &layerJSON{Zone: "conductor"}}, fakeEngine{}, "forbidden", false},
		{"layerCreate/admin (not conductor): conductor-zone → DENY", "admin", domain.KindLayerCreate,
			mutationJSON{Layer: &layerJSON{Zone: "conductor"}}, fakeEngine{}, "forbidden", false},
		{"layerCreate/conductor: conductor-zone", "conductor", domain.KindLayerCreate,
			mutationJSON{Layer: &layerJSON{Zone: "conductor"}}, fakeEngine{}, "", true},

		// ---- layerUpdate / layerDelete: gated on the RESOLVED layer (owner OR admin; conductor zone → conductor) ----
		{"layerUpdate/owner", "member", domain.KindLayerUpdate,
			mutationJSON{Layer: &layerJSON{ID: "l"}}, fakeEngine{layer: mine(domain.ZoneShared), layerFound: true}, "", true},
		{"layerUpdate/member: another's layer → DENY", "member", domain.KindLayerUpdate,
			mutationJSON{Layer: &layerJSON{ID: "l"}}, fakeEngine{layer: other(domain.ZoneShared, domain.AccessRW), layerFound: true}, "forbidden", false},
		{"layerUpdate/admin: another's layer (override)", "admin", domain.KindLayerUpdate,
			mutationJSON{Layer: &layerJSON{ID: "l"}}, fakeEngine{layer: other(domain.ZoneShared, domain.AccessRW), layerFound: true}, "", true},
		{"layerUpdate/member: conductor-zone → DENY", "member", domain.KindLayerUpdate,
			mutationJSON{Layer: &layerJSON{ID: "l"}}, fakeEngine{layer: other(domain.ZoneConductor, domain.AccessRW), layerFound: true}, "forbidden", false},
		{"layerUpdate/admin: conductor-zone → DENY (admin ≠ conductor)", "admin", domain.KindLayerUpdate,
			mutationJSON{Layer: &layerJSON{ID: "l"}}, fakeEngine{layer: other(domain.ZoneConductor, domain.AccessRW), layerFound: true}, "forbidden", false},
		{"layerUpdate/conductor: OWN conductor-zone layer", "conductor", domain.KindLayerUpdate,
			mutationJSON{Layer: &layerJSON{ID: "l"}}, fakeEngine{layer: mine(domain.ZoneConductor), layerFound: true}, "", true},
		// Asymmetry worth pinning: layerUpdate needs owner-OR-admin ON TOP of the zone gate, so even a
		// conductor cannot update ANOTHER's conductor-zone layer (create/edit's canWriteLayer would allow it).
		{"layerUpdate/conductor: another's conductor-zone layer → DENY", "conductor", domain.KindLayerUpdate,
			mutationJSON{Layer: &layerJSON{ID: "l"}}, fakeEngine{layer: other(domain.ZoneConductor, domain.AccessRW), layerFound: true}, "forbidden", false},
		{"layerUpdate/unknown layer → STALE", "admin", domain.KindLayerUpdate,
			mutationJSON{Layer: &layerJSON{ID: "gone"}}, fakeEngine{layerFound: false}, "stale", false},
		{"layerDelete/member: another's layer → DENY", "member", domain.KindLayerDelete,
			mutationJSON{Layer: &layerJSON{ID: "l"}}, fakeEngine{layer: other(domain.ZoneShared, domain.AccessRW), layerFound: true}, "forbidden", false},
		{"layerDelete/owner", "member", domain.KindLayerDelete,
			mutationJSON{Layer: &layerJSON{ID: "l"}}, fakeEngine{layer: mine(domain.ZoneShared), layerFound: true}, "", true},
		{"layerDelete/unknown layer → STALE", "member", domain.KindLayerDelete,
			mutationJSON{Layer: &layerJSON{ID: "gone"}}, fakeEngine{layerFound: false}, "stale", false},

		// ---- layerReorder: never gated by write-access ----
		{"layerReorder is ungated", "member", domain.KindLayerReorder, mutationJSON{}, fakeEngine{}, "", true},

		// ---- create (object): gated on the TARGET layer via canWriteLayer ----
		{"create/own layer", "member", domain.KindCreate,
			mutationJSON{Object: &objectJSON{LayerID: "l"}}, fakeEngine{layer: mine(domain.ZoneShared), layerFound: true}, "", true},
		{"create/member: another's RO layer → DENY", "member", domain.KindCreate,
			mutationJSON{Object: &objectJSON{LayerID: "l"}}, fakeEngine{layer: other(domain.ZoneShared, domain.AccessRO), layerFound: true}, "forbidden", false},
		{"create/member: another's RW layer", "member", domain.KindCreate,
			mutationJSON{Object: &objectJSON{LayerID: "l"}}, fakeEngine{layer: other(domain.ZoneShared, domain.AccessRW), layerFound: true}, "", true},
		{"create/member: conductor-zone → DENY", "member", domain.KindCreate,
			mutationJSON{Object: &objectJSON{LayerID: "l"}}, fakeEngine{layer: other(domain.ZoneConductor, domain.AccessRW), layerFound: true}, "forbidden", false},
		{"create/conductor: conductor-zone", "conductor", domain.KindCreate,
			mutationJSON{Object: &objectJSON{LayerID: "l"}}, fakeEngine{layer: other(domain.ZoneConductor, domain.AccessRW), layerFound: true}, "", true},
		{"create/unknown target layer is allowed (engine validates)", "member", domain.KindCreate,
			mutationJSON{Object: &objectJSON{LayerID: "new"}}, fakeEngine{layerFound: false}, "", true},

		// ---- edit an existing object (move/resize/setStyle/setText/delete/restore): gated via ObjectLayer ----
		{"edit/own object's layer", "member", domain.KindMove,
			mutationJSON{UUID: "o"}, fakeEngine{layer: mine(domain.ZoneShared), layerFound: true, objExists: true}, "", true},
		{"edit/unknown object → STALE", "member", domain.KindMove,
			mutationJSON{UUID: "ghost"}, fakeEngine{objExists: false}, "stale", false},
		{"edit/object whose layer is unmaterialized is allowed", "member", domain.KindMove,
			mutationJSON{UUID: "o"}, fakeEngine{objExists: true, layerFound: false}, "", true},
		{"edit/member: another's RO layer → DENY", "member", domain.KindSetStyle,
			mutationJSON{UUID: "o"}, fakeEngine{layer: other(domain.ZoneShared, domain.AccessRO), layerFound: true, objExists: true}, "forbidden", false},
		{"edit/member: conductor-zone → DENY", "member", domain.KindDelete,
			mutationJSON{UUID: "o"}, fakeEngine{layer: other(domain.ZoneConductor, domain.AccessRW), layerFound: true, objExists: true}, "forbidden", false},
		{"edit/conductor: conductor-zone", "conductor", domain.KindMove,
			mutationJSON{UUID: "o"}, fakeEngine{layer: other(domain.ZoneConductor, domain.AccessRW), layerFound: true, objExists: true}, "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &conn{authorID: me, role: tc.role, songID: "song", hub: &Hub{eng: tc.eng}}
			reason, ok := c.authorizeWrite(tc.kind, tc.in)
			if ok != tc.wantOK || reason != tc.wantReason {
				t.Fatalf("authorizeWrite = (%q, %v), want (%q, %v)", reason, ok, tc.wantReason, tc.wantOK)
			}
		})
	}
}
