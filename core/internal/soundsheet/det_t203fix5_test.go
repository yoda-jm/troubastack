package soundsheet

import (
	"bytes"
	"testing"
)

// T203-FIX5: the sound sheet PDF must be byte-deterministic between downloads. TWO independent causes:
//  1. SetCatalogSort(true) — without it the font resource objects (regular/bold/oblique/bold-oblique) number in
//     Go map order, which is RANDOMISED per range, so renders vary even in one process (obj 9 = Bold then
//     Oblique). A render-TWICE can pass by luck; render 30× to expose it (≈14/30 differ without the sort).
//  2. Pinned creation/modification dates — fpdf defaults to time.Now(), so two downloads seconds apart differ.
//
// setlistpdf and writeA4PDF already do both; sampleModel exercises all four font variants + the T205 mark.
func TestRenderPDF_Deterministic_T203FIX5(t *testing.T) {
	m := sampleModel(2, 2)
	first, err := RenderPDF(m)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		b, err := RenderPDF(m)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, b) {
			t.Fatalf("sound sheet PDF not deterministic: render %d differs from the first (%d vs %d bytes) — the font objects renumber without SetCatalogSort", i, len(b), len(first))
		}
	}
	// time.Unix(1700000000) = 2023-11-14 UTC. Pinned ⇒ two downloads at different wall-clock times are identical.
	if !bytes.Contains(first, []byte("D:20231114")) {
		i := bytes.Index(first, []byte("CreationDate"))
		got := "none"
		if i >= 0 {
			got = string(first[i:min(i+28, len(first))])
		}
		t.Errorf("creation date is not pinned (two downloads at different times will differ); got %q", got)
	}
}
