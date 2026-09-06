package chartpdf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestIntermissionShownLabel asserts the DRAWN text (T153 ⟨R1⟩): a blank label draws
// the default word, a real label draws itself verbatim — a French band's "Entracte"
// is never translated or replaced.
func TestIntermissionShownLabel(t *testing.T) {
	cases := map[string]string{
		"":             "Intermission", // blank ⇒ default, not a blank card
		"Entracte":     "Entracte",     // authored content, verbatim
		"Set break":    "Set break",
		"Intermission": "Intermission",
	}
	for in, want := range cases {
		if got := intermissionShownLabel(in); got != want {
			t.Errorf("intermissionShownLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRenderIntermission_OnePage: the separator is EXACTLY one page — the hard
// requirement (a zero- or multi-page break breaks Stage's page→song resolution).
func TestRenderIntermission_OnePage(t *testing.T) {
	pdf, err := RenderIntermission("Entracte", "The Band")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if n := len(goldenPageRe.FindAll(pdf, -1)); n != 1 {
		t.Fatalf("intermission page count = %d, want exactly 1", n)
	}
}

// TestRenderIntermission_BandLineConditional: the band name is drawn when present and
// OMITTED when absent (no "Unknown band" placeholder — the T143 lesson). Proven
// behaviourally: the with-band render carries content the without-band one does not,
// and neither invents a placeholder.
func TestRenderIntermission_BandLineConditional(t *testing.T) {
	withBand, err := RenderIntermission("Entracte", "The Band")
	if err != nil {
		t.Fatalf("render with band: %v", err)
	}
	noBand, err := RenderIntermission("Entracte", "")
	if err != nil {
		t.Fatalf("render no band: %v", err)
	}
	if bytes.Equal(withBand, noBand) {
		t.Fatal("band name present vs absent produced identical bytes — the band line is not being drawn")
	}
	if len(noBand) >= len(withBand) {
		t.Errorf("no-band render (%d bytes) should be smaller than with-band (%d) — an omitted line, not a placeholder", len(noBand), len(withBand))
	}
}

// TestRenderIntermission_Deterministic: same inputs ⇒ same bytes (newDoc pins the
// dates; the mark is a committed asset), so the golden below is stable run-to-run.
func TestRenderIntermission_Deterministic(t *testing.T) {
	a, err := RenderIntermission("", "")
	if err != nil {
		t.Fatalf("render a: %v", err)
	}
	b, err := RenderIntermission("", "")
	if err != nil {
		t.Fatalf("render b: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("RenderIntermission is not deterministic")
	}
}

// TestRenderIntermission_Golden pins the separator's bytes into the T144 layout guard:
// any change to the card's layout or the embedded mark moves this hash and must be
// updated deliberately in the same commit.
func TestRenderIntermission_Golden(t *testing.T) {
	const wantHash = "ea3c62afb7c5d9b19ef456b70f984604a7db931f155732a6a745ea045ca0032e"
	pdf, err := RenderIntermission("Entracte", "The Band")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	sum := sha256.Sum256(pdf)
	got := hex.EncodeToString(sum[:])
	if got != wantHash {
		t.Fatalf("intermission golden hash = %s, want %s (update deliberately if the card changed)", got, wantHash)
	}
}
