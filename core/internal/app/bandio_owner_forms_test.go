package app_test

import (
	"encoding/json"
	"strings"
	"testing"

	"troubastack/core/internal/domain"
)

// Glossary D14, the `.tband` half — the export's spelling of "shared", pinned so a fourth form cannot
// appear quietly. The other half (domain → baked bundle) is pinned in bake's owner_forms_test.go, and
// both comments carry the same table:
//
//	domain        OwnerID == domain.SharedOwner ("_shared_")
//	baked bundle  LayerImage.Owner == ""              — `owner != ""` IS the personal test
//	.tband export owner: "_shared_"                   — and a PERSONAL owner is a USERNAME here, where
//	                                                    the other two surfaces carry the member id
//
// The username is the part worth pinning hardest: it is a third encoding of the same concept, it is the
// one an importing band has to resolve against ITS members, and nothing else asserts its spelling.

func TestTbandOwnerForms_SharedIsTheWordAndPersonalIsAUsername(t *testing.T) {
	src := newStack()
	admin, member, bandID, _, _, _ := buildSourceBand(t, src)

	zipBytes, _, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatal(err)
	}
	entries := unzip(t, zipBytes)

	var ann struct {
		Layers []struct {
			ID    string `json:"id"`
			Owner string `json:"owner"`
		} `json:"layers"`
	}
	found := false
	for name, data := range entries {
		if !strings.HasPrefix(name, "annotations/") || !strings.HasSuffix(name, ".json") {
			continue
		}
		if err := json.Unmarshal(data, &ann); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		found = true
		// Positive control: an assertion over an empty layer list would pass for the wrong reason.
		if len(ann.Layers) == 0 {
			t.Fatalf("%s carries no layers — the fixture stopped exercising what this guards", name)
		}
	}
	if !found {
		t.Fatal("the export carried no annotations entry at all — nothing was checked")
	}

	byID := map[string]string{}
	for _, l := range ann.Layers {
		byID[l.ID] = l.Owner
	}
	if got := byID["L-shared"]; got != domain.SharedOwner {
		t.Errorf("shared layer exported owner %q, want %q — the .tband spells shared as the WORD, not as \"\"",
			got, domain.SharedOwner)
	}
	if got := byID["L-mine"]; got != member.Username {
		t.Errorf("personal layer exported owner %q, want the username %q — the export resolves ids to usernames so an "+
			"importing band can match its own members; a member id here would be meaningless to them",
			got, member.Username)
	}
	if got := byID["L-mine"]; got == member.ID {
		t.Errorf("personal layer exported the member ID (%q) — that is the domain's spelling, not the export's", got)
	}
}
