package bake

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/runningorder"
	"troubastack/core/internal/setlistpdf"
)

// T153 — "intermission" is written as a Go constant in FOUR packages, and the baker's own comment calls it
// "the ONE string the wire, the baker, and both clients agree on". Nothing enforced that. Four hand-copied
// spellings of one word is three chances to typo it, and every failure is silent: a break would simply read
// as a song everywhere downstream, with no error and no test noticing.
//
// The contract file is the authority — Go and TS already read it for the numbering rule, and the Kotlin
// mirror reads its copy — so this pins the Go constants to it rather than to each other. Pinning them to
// each other would let all four drift together.
//
// Teeth: change any one of the four constants and this reddens naming the package.
func TestWireKindMatchesTheSharedContract_T153(t *testing.T) {
	const contractPath = "../../../docs/contracts/running-order-numbering.vectors.json"

	raw, err := os.ReadFile(filepath.Clean(contractPath))
	if err != nil {
		t.Fatalf("read the running-order contract: %v", err)
	}
	var spec struct {
		Cases []struct {
			Entries []struct {
				Kind string `json:"kind"`
			} `json:"entries"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse the contract: %v", err)
	}

	// The contract's own spelling of a break, taken from the data rather than retyped here — retyping it
	// would reintroduce the very duplication this test exists to remove.
	want := ""
	for _, c := range spec.Cases {
		for _, e := range c.Entries {
			if e.Kind != "" && e.Kind != "song" {
				want = e.Kind
				break
			}
		}
	}
	if want == "" {
		t.Fatal("the contract carries no non-song kind — it cannot pin anything (did the vectors shrink?)")
	}

	for _, c := range []struct {
		where string
		got   string
	}{
		{"bake.bakedKindIntermission (the bundle wire)", bakedKindIntermission},
		{"app.SetlistKindIntermission (the domain)", app.SetlistKindIntermission},
		{"runningorder.KindIntermission (the numbering rule)", runningorder.KindIntermission},
		{"setlistpdf.KindIntermission (the printed sheet)", setlistpdf.KindIntermission},
	} {
		if c.got != want {
			t.Errorf("%s = %q, want %q (the shared contract's spelling) — a break would read as a song here, silently",
				c.where, c.got, want)
		}
	}
}
