package chartpdf

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// longBody builds a chart body long enough to need TWO pages in one column at the default size. Invented
// lyrics only — no band data (T146 ⟨R1⟩).
func longBody(nLines int) string {
	var b strings.Builder
	b.WriteString("# A Long Invented Chart\n\n## Verse\n")
	for i := 0; i < nLines; i++ {
		b.WriteString("la la la and the road rolls on\n") // T168: short enough NOT to wrap in a half-width
		// column. With a longer line the wrap doubles the line count and two columns can no longer buy a
		// larger size — which is true, and is the T168 tension, but it is not what THIS test is about.
	}
	return b.String()
}

// TestTwoColumns_FitOnePageAtLargerType_T146 is the ⟨R1⟩ comparison that IS the feature: a chart that needs
// two pages in one column fits ONE page in two columns AT A LARGER TYPE SIZE. A test that only checked "two
// columns appear" would pass a version that just shrank the type — so the assertion is the trade itself.
func TestTwoColumns_FitOnePageAtLargerType_T146(t *testing.T) {
	body := longBody(60)
	oneCol := body
	twoCol := "# A Long Invented Chart\ncolumns: 2\n\n## Verse\n" + strings.SplitN(body, "## Verse\n", 2)[1]

	oneColPages := renderedPageCount(t, oneCol)
	if oneColPages < 2 {
		t.Fatalf("fixture should need >1 page in one column at the default size, got %d", oneColPages)
	}
	twoColPages := renderedPageCount(t, twoCol)
	if twoColPages != 1 {
		t.Fatalf("two columns should fit the chart on ONE page, got %d", twoColPages)
	}

	// ...and at a LARGER type size than the one-column render (which is defaultBodyPt with no directive).
	lines := chartLines(twoCol)
	sub, _, _, _, _, cols, skip := parseHeader(lines)
	if cols != 2 {
		t.Fatalf("columns directive not parsed, cols=%d", cols)
	}
	fitPt := autoFitBodyPt(lines, sub, skip, cols)
	if fitPt <= defaultBodyPt {
		t.Errorf("two-column auto-fit size %.0fpt is not larger than the one-column default %.0fpt", fitPt, defaultBodyPt)
	}
}

// TestTwoColumns_TabChartStaysOneColumn_T146: a chart with a tab block ignores `columns: 2` — a tab stave
// is full-width (T135), so it cannot share a half-width column. Teeth: the directive is present but the
// render must match the same chart without it.
func TestTwoColumns_TabChartStaysOneColumn_T146(t *testing.T) {
	tab := "{sot}\ne|--0--2--3--|\nB|--1--1--1--|\n{eot}\n"
	withDir := "# Riff\ncolumns: 2\n\n" + tab
	without := "# Riff\n\n" + tab
	a, err := Render(withDir)
	if err != nil {
		t.Fatalf("render with directive: %v", err)
	}
	b, err := Render(without)
	if err != nil {
		t.Fatalf("render without: %v", err)
	}
	if hex.EncodeToString(sha256Sum(a)) != hex.EncodeToString(sha256Sum(b)) {
		t.Fatal("a tab chart with columns:2 must render identically to one without — tabs stay one column")
	}
}

// TestTwoColumns_Golden_T146 pins the two-column layout's bytes so a layout change moves it — the T144
// ritual extended to the new path. Deterministic fixture, invented lyrics only.
func TestTwoColumns_Golden_T146(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Golden Two Col\ncolumns: 2\n\n## Verse\n")
	for i := 0; i < 14; i++ {
		b.WriteString("C      G\nshort lyric line\n")
	}
	pdf, err := Render(b.String())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if p := len(goldenPageRe.FindAll(pdf, -1)); p != 1 {
		t.Fatalf("two-col golden page count = %d, want 1", p)
	}
	// Updated DELIBERATELY (T168, 2026-09-08): two-column output changed twice, both intended — body
	// lines now WRAP to the column instead of running off the paper (VLL's ⟨D1⟩), and a grey hairline is
	// drawn down the gutter (his second ask). One-column output is unchanged, byte for byte: the T144
	// goldens and both byte-stability tests stayed green through this change, which is the property
	// stage 2 rests on. Previous hash: 84454e24…e38e9397.
	const wantHash = "c0e6625d8aa6e6185594963e810e2d7ba88b27522ae3d1c3f6909f9c30fae4b7"
	if got := hex.EncodeToString(sha256Sum(pdf)); got != wantHash {
		t.Fatalf("two-col golden hash = %s, want %s (update deliberately if the layout changed)", got, wantHash)
	}
}

func renderedPageCount(t *testing.T, src string) int {
	t.Helper()
	pdf, err := Render(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return len(goldenPageRe.FindAll(pdf, -1))
}

func sha256Sum(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}
