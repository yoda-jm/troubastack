package app_test

import (
	"encoding/json"
	"os"
	"testing"

	"troubastack/core/internal/app"
)

// TestDefaultFileVectors runs the shared T138 ⟨R1⟩ contract (docs/contracts/default-file.vectors.json).
// The SAME file is mirrored into web/studio and run by TS; ci.yml diffs the two copies, so a change to
// the rule in one lane that isn't matched in the other fails. Teeth: flip the PDF predicate or drop the
// filename tiebreak in DefaultFile and a case here goes red.
func TestDefaultFileVectors(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/contracts/default-file.vectors.json")
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var doc struct {
		Cases []struct {
			Name     string         `json:"name"`
			Files    []app.SongFile `json:"files"`
			Expected *string        `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	if len(doc.Cases) == 0 {
		t.Fatal("no cases in vectors")
	}
	for _, c := range doc.Cases {
		got, ok := app.DefaultFile(c.Files)
		switch {
		case c.Expected == nil && ok:
			t.Errorf("%s: got default %q, want none", c.Name, got.Filename)
		case c.Expected != nil && !ok:
			t.Errorf("%s: got none, want %q", c.Name, *c.Expected)
		case c.Expected != nil && ok && got.Filename != *c.Expected:
			t.Errorf("%s: got %q, want %q", c.Name, got.Filename, *c.Expected)
		}
	}
}
