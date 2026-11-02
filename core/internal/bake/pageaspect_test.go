package bake

import (
	"math"
	"testing"
)

// TestParsePageAspects pins the page-aspect reader (T199): the aspect is the MediaBox (what pdftoppm renders),
// NOT pdfinfo's "Page size" (which is the CropBox), and a /Rotate of 90/270 swaps W and H. The CropBox case is
// the teeth — a parser that read "Page size" would give the CropBox ratio and reds.
func TestParsePageAspects(t *testing.T) {
	approx := func(got, want float64) bool { return math.Abs(got-want) < 1e-9 }

	// Multi-page: portrait then landscape.
	multi := "Pages:           2\n" +
		"Page    1 size:  216 x 288 pts\nPage    1 rot:   0\n" +
		"Page    2 size:  300 x 200 pts\nPage    2 rot:   0\n" +
		"Page    1 MediaBox:      0.00     0.00   216.00   288.00\n" +
		"Page    2 MediaBox:      0.00     0.00   300.00   200.00\n"
	a, err := parsePageAspects(multi)
	if err != nil {
		t.Fatalf("multi: %v", err)
	}
	if len(a) != 2 || !approx(a[0], 216.0/288) || !approx(a[1], 300.0/200) {
		t.Fatalf("multi aspects = %v, want [%.4f %.4f]", a, 216.0/288, 300.0/200)
	}

	// CropBox ≠ MediaBox (single-page shape): "Page size" is the CropBox (100×300); the MediaBox is 200×300,
	// which is what pdftoppm renders. The aspect MUST be 200/300, never 100/300.
	cb := "Page size:       100 x 300 pts\nPage rot:        0\n" +
		"MediaBox:            0.00     0.00   200.00   300.00\n" +
		"CropBox:             0.00     0.00   100.00   300.00\n"
	ac, err := parsePageAspects(cb)
	if err != nil {
		t.Fatalf("cropbox: %v", err)
	}
	if !approx(ac[0], 200.0/300) {
		t.Fatalf("cropbox aspect = %.4f, want %.4f (the MediaBox, not the CropBox)", ac[0], 200.0/300)
	}
	if approx(ac[0], 100.0/300) {
		t.Fatalf("cropbox aspect read the CropBox (%.4f) — must read the MediaBox", ac[0])
	}

	// /Rotate 90 swaps W and H (pdfinfo reports the size unswapped; pdftoppm renders it rotated).
	rot := "Page size:       216 x 288 pts\nPage rot:        90\n" +
		"MediaBox:            0.00     0.00   216.00   288.00\n"
	ar, err := parsePageAspects(rot)
	if err != nil {
		t.Fatalf("rot90: %v", err)
	}
	if !approx(ar[0], 288.0/216) {
		t.Fatalf("rot90 aspect = %.4f, want %.4f (W/H swapped)", ar[0], 288.0/216)
	}

	// Unreadable ⇒ error, never a silent default (B1).
	if _, err := parsePageAspects("Pages:           1\n"); err == nil {
		t.Error("a listing with no MediaBox must error, not return a default aspect")
	}
}
