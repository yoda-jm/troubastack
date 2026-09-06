package runningorder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The SHARED contract every surface must agree with (T158). TroubaCore reads the canonical file itself —
// not a hand-transcribed copy — so a change to the rule's truth reaches this test, and an off-by-one or a
// bench/intermission that shifts the count reddens it. (The mid-list intermission/on-call cases are the
// teeth: a "number every entry" implementation makes the following song read one too high and fails here.)
const contractPath = "../../../docs/contracts/running-order-numbering.vectors.json"

type vectorFile struct {
	Cases []struct {
		Name     string  `json:"name"`
		Entries  []Entry `json:"entries"`
		Expected []*int  `json:"expected"` // null ⇒ no number
	} `json:"cases"`
}

func TestNumbers_ContractVectors(t *testing.T) {
	raw, err := os.ReadFile(filepath.Clean(contractPath))
	if err != nil {
		t.Fatalf("read shared contract %q: %v", contractPath, err)
	}
	var vf vectorFile
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	if len(vf.Cases) == 0 {
		t.Fatal("contract has no cases — refusing to pass vacuously")
	}
	for _, c := range vf.Cases {
		got := Numbers(c.Entries)
		if len(got) != len(c.Expected) {
			t.Errorf("%s: got %d numbers, want %d", c.Name, len(got), len(c.Expected))
			continue
		}
		for i, exp := range c.Expected {
			want := 0 // null ⇒ 0 (unnumbered) in the Go representation
			if exp != nil {
				want = *exp
			}
			if got[i] != want {
				t.Errorf("%s: entry %d (%+v) got number %d, want %d", c.Name, i, c.Entries[i], got[i], want)
			}
		}
	}
}
