package engine_test

import (
	"testing"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
)

func layer(id, owner string, zone domain.Zone, order int, mandatory bool, roleTag string) domain.Layer {
	return domain.Layer{ID: id, OwnerID: owner, Zone: zone, Order: order, Mandatory: mandatory, RoleTag: roleTag, Access: domain.AccessRW}
}

func TestVisibleStackOrdering(t *testing.T) {
	// Layers across all zones; viewer is "me".
	snap := domain.Snapshot{Layers: []domain.Layer{
		layer("personal-other", "other", domain.ZonePersonal, 0, false, ""),
		layer("personal-mine", "me", domain.ZonePersonal, 0, false, ""),
		layer("shared", domain.SharedOwner, domain.ZoneShared, 0, false, ""),
		layer("conductor", "cond", domain.ZoneConductor, 0, true, ""),
	}}

	full := engine.FullStack(snap, "me", domain.RoleMember)
	gotOrder := make([]string, len(full))
	for i, e := range full {
		gotOrder[i] = e.Layer.ID
	}
	// Bottom→top: conductor < shared < personal(other) < personal(mine on top).
	want := []string{"conductor", "shared", "personal-other", "personal-mine"}
	for i := range want {
		if gotOrder[i] != want[i] {
			t.Fatalf("stack order = %v, want %v", gotOrder, want)
		}
	}
}

func TestVisibleStackDefaults(t *testing.T) {
	snap := domain.Snapshot{Layers: []domain.Layer{
		layer("mandatory-cue", "cond", domain.ZoneConductor, 0, true, ""), // mandatory → always on
		layer("shared", domain.SharedOwner, domain.ZoneShared, 0, false, ""),
		layer("mine", "me", domain.ZonePersonal, 0, false, ""),        // own → on
		layer("others", "you", domain.ZonePersonal, 0, false, ""),     // other's optional → off
		layer("flute", "you", domain.ZonePersonal, 1, false, "flute"), // role_tag mismatch → off
	}}

	visible := engine.VisibleStack(snap, "me", domain.RoleMember)
	on := map[string]bool{}
	for _, l := range visible {
		on[l.ID] = true
	}
	if !on["mandatory-cue"] {
		t.Error("mandatory layer must be on by default")
	}
	if !on["shared"] {
		t.Error("shared layer must be on by default")
	}
	if !on["mine"] {
		t.Error("viewer's own layer must be on by default")
	}
	if on["others"] {
		t.Error("other member's optional layer must be OFF by default")
	}
	if on["flute"] {
		t.Error("non-matching role_tag layer must be OFF by default for a plain member")
	}
}

func TestRoleTagDefaultsOn(t *testing.T) {
	snap := domain.Snapshot{Layers: []domain.Layer{
		layer("cond-cue", "cond", domain.ZoneConductor, 0, false, "conductor"), // optional, role-tagged
	}}
	// A plain member does not see the conductor-tagged optional layer by default.
	if vs := engine.VisibleStack(snap, "me", domain.RoleMember); len(vs) != 0 {
		t.Fatalf("member should not default-see conductor-tagged optional layer: %d", len(vs))
	}
	// A conductor does.
	if vs := engine.VisibleStack(snap, "me", domain.RoleConductor); len(vs) != 1 {
		t.Fatalf("conductor should default-see conductor-tagged layer: %d", len(vs))
	}
}

func TestMandatoryCannotBeHidden(t *testing.T) {
	snap := domain.Snapshot{Layers: []domain.Layer{
		layer("m", "cond", domain.ZoneConductor, 0, true, ""),
		layer("o", "you", domain.ZonePersonal, 0, false, ""),
	}}
	full := engine.FullStack(snap, "me", domain.RoleMember)
	for _, e := range full {
		if e.Layer.ID == "m" && !e.Mandatory {
			t.Error("mandatory flag must be surfaced so the viewer cannot hide it")
		}
	}
}
