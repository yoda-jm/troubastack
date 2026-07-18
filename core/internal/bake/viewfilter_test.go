package bake

import (
	"encoding/json"
	"os"
	"testing"
)

// TestLayerVisible_Vectors runs the SHARED P205 view-resolution contract
// (testdata/view-resolution.vectors.json) against LayerVisible. The app's Stage-3
// presenter runs the same cases in commonTest, so print (this package) and screen
// (app/shared) can never silently diverge. Editing the rule means editing the
// vectors — the file IS the contract.
func TestLayerVisible_Vectors(t *testing.T) {
	data, err := os.ReadFile("testdata/view-resolution.vectors.json")
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var spec struct {
		Cases []struct {
			Name  string `json:"name"`
			Layer struct {
				Mandatory bool   `json:"mandatory"`
				RoleTag   string `json:"roleTag"`
				Owner     string `json:"owner"`
				DefaultOn *bool  `json:"defaultOn"`
			} `json:"layer"`
			Viewer struct {
				Role     string `json:"role"`
				MemberID string `json:"memberId"`
			} `json:"viewer"`
			ExpectVisible bool `json:"expectVisible"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	if len(spec.Cases) == 0 {
		t.Fatal("no vector cases — the contract file is empty")
	}
	for _, c := range spec.Cases {
		l := LayerImage{
			Mandatory: c.Layer.Mandatory,
			RoleTag:   c.Layer.RoleTag,
			Owner:     c.Layer.Owner,
			DefaultOn: c.Layer.DefaultOn,
		}
		if got := LayerVisible(l, c.Viewer.Role, c.Viewer.MemberID); got != c.ExpectVisible {
			t.Errorf("%s: LayerVisible = %v, want %v", c.Name, got, c.ExpectVisible)
		}
	}
}
