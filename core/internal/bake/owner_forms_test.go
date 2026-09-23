package bake

import (
	"testing"

	"troubastack/core/internal/domain"
)

// Glossary D14 — the shared-owner sentinel has THREE spellings, one per surface, and that is the
// decision rather than the drift (Fable, ⟨GO⟩ d36704c6: document, do not unify). This file pins the two
// halves that live in bake; the `.tband` half is pinned in app's bandio_owner_forms_test.go, and the two
// comments name the same table so a reader arriving at either one sees the whole mapping.
//
// What makes a FOURTH form possible is that nothing ever asserted the mapping — it was one inline `if`
// inside a bake, and a second way to say "shared" could appear beside it without anything going red.

func TestBakedOwner_SharedBecomesEmptyAndNothingElseDoes(t *testing.T) {
	member := "m-3f9c"
	for _, c := range []struct {
		name  string
		layer domain.Layer
		want  string
	}{
		{"the domain sentinel becomes the empty string", domain.Layer{OwnerID: domain.SharedOwner}, ""},
		{"an already-empty owner stays shared", domain.Layer{OwnerID: ""}, ""},
		{"a member id is carried through unchanged", domain.Layer{OwnerID: member}, member},
		// The discriminating case: a value that merely LOOKS like the sentinel is a personal owner. If
		// someone ever adds a second spelling of "shared", this is the row that stops being true.
		{"a near-miss spelling is NOT shared", domain.Layer{OwnerID: "_shared"}, "_shared"},
		{"…nor is the word itself", domain.Layer{OwnerID: "shared"}, "shared"},
	} {
		if got := bakedOwner(c.layer); got != c.want {
			t.Errorf("%s: bakedOwner(%q) = %q, want %q", c.name, c.layer.OwnerID, got, c.want)
		}
	}
}

func TestBakedOwner_MatchesTheTestTheVisibilityRuleApplies(t *testing.T) {
	// The producer and the consumer have to agree, and they are written in different files: this maps
	// the owner, viewfilter.go reads `owner != ""` as "personal". A mapping that stopped producing ""
	// for a shared layer would turn every shared layer into somebody's personal one — on stage, that is
	// a conductor's cues silently disappearing for everyone but their author.
	const viewer = "m-3f9c"
	shared := LayerImage{LayerID: "L1", Owner: bakedOwner(domain.Layer{OwnerID: domain.SharedOwner})}
	mine := LayerImage{LayerID: "L2", Owner: bakedOwner(domain.Layer{OwnerID: viewer})}
	theirs := LayerImage{LayerID: "L3", Owner: bakedOwner(domain.Layer{OwnerID: "m-other"})}

	if !LayerVisible(shared, "", "") {
		t.Error("a shared layer must be visible to a viewer with no role and no identity — it is not anyone's")
	}
	if !LayerVisible(mine, "", viewer) {
		t.Error("my own personal layer must print for me")
	}
	if LayerVisible(theirs, "", viewer) {
		t.Error("someone else's personal layer must not print for me — this is the privacy half of the sentinel")
	}
}
