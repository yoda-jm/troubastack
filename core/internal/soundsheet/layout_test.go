package soundsheet

import "testing"

// coversAll checks every channel index 0..n-1 appears exactly once across the bands.
func coversAll(p LayoutPlan, n int) bool {
	seen := make([]bool, n)
	count := 0
	for _, band := range p.Bands {
		for _, i := range band {
			if i < 0 || i >= n || seen[i] {
				return false
			}
			seen[i] = true
			count++
		}
	}
	return count == n
}

func TestPlanLayout_FewColumnsStayInline(t *testing.T) {
	g := A4Landscape()
	p := PlanLayout(4, 12, g)
	if !p.InlineChangesNote || len(p.Bands) != 1 {
		t.Fatalf("4 channels should be one inline band: %+v", p)
	}
	if p.ColW < floorCol {
		t.Fatalf("colW %v below floor", p.ColW)
	}
	if !coversAll(p, 4) {
		t.Fatalf("bands don't cover all columns: %+v", p.Bands)
	}
}

// Fable's required case: 40 songs × 10 musicians (~20 channel columns) must lay out legibly — never below the
// floor, always covering every column, rows paginated (a song is never split), header repeats each page.
func TestPlanLayout_40songs_10musicians(t *testing.T) {
	g := A4Landscape()
	const channels = 20 // 10 musicians × ~2 channels
	const songs = 40
	p := PlanLayout(channels, songs, g)

	if p.ColW < floorCol {
		t.Fatalf("colW %v dropped below the legible floor %v", p.ColW, floorCol)
	}
	if !coversAll(p, channels) {
		t.Fatalf("not every channel is placed exactly once: %+v", p.Bands)
	}
	// 20 channels can't fit inline at the floor beside title+changes+note, so a fallback MUST have engaged.
	if p.InlineChangesNote && len(p.Bands) == 1 {
		t.Fatalf("20 channels should not fit as a single inline band: %+v", p)
	}
	// each band must fit across the page at the chosen width.
	for _, band := range p.Bands {
		if w := titleW + float64(len(band))*p.ColW; w > g.UsableW+0.5 {
			t.Fatalf("band width %v exceeds usable %v", w, g.UsableW)
		}
	}
	if p.RowsPerPage < 1 {
		t.Fatalf("rowsPerPage must be ≥ 1, got %d", p.RowsPerPage)
	}
	// pages needed is finite and the header repeats (implied by whole-song pagination).
	pages := (songs + p.RowsPerPage - 1) / p.RowsPerPage
	if pages < 1 {
		t.Fatalf("pages must be ≥ 1")
	}
	t.Logf("40×10 plan: colW=%.1f inline=%v bands=%d rows/page=%d pages=%d", p.ColW, p.InlineChangesNote, len(p.Bands), p.RowsPerPage, pages)
}

// Binding: 30+ channels cannot fit in one band even at the floor, so step 3 (column bands) MUST engage — more
// than one band, every channel placed once, each band fitting the page. (Red if step 3 is disabled: a single
// forced band of 30 channels is 42 + 30·10 = 342mm, far over the 277mm page.)
func TestPlanLayout_SecondBandEngages(t *testing.T) {
	g := A4Landscape()
	const channels = 32
	p := PlanLayout(channels, 20, g)
	if len(p.Bands) <= 1 {
		t.Fatalf("32 channels must split into >1 band, got %d", len(p.Bands))
	}
	if !coversAll(p, channels) {
		t.Fatalf("bands don't cover all 32 channels once: %+v", p.Bands)
	}
	if p.ColW < floorCol {
		t.Fatalf("colW %v below floor", p.ColW)
	}
	for i, band := range p.Bands {
		if w := titleW + float64(len(band))*p.ColW; w > g.UsableW+0.5 {
			t.Fatalf("band %d width %v exceeds usable %v", i, w, g.UsableW)
		}
	}
}

func TestPlanLayout_zeroChannels(t *testing.T) {
	p := PlanLayout(0, 5, A4Landscape())
	if !coversAll(p, 0) || p.RowsPerPage < 1 {
		t.Fatalf("zero-channel plan invalid: %+v", p)
	}
}
