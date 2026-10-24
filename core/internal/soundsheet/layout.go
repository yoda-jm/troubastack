// Package soundsheet renders the T203 sound-sheet model (app.SheetModel) to a landscape PDF and a CSV. The
// layout PLAN is computed separately from drawing so the width/pagination fallbacks are unit-testable without a
// PDF (Fable: "test it on the layout model with 40 songs and 10 musicians").
package soundsheet

import "math"

// Geom is the usable page box + row heights, in millimetres.
type Geom struct {
	UsableW float64 // printable width
	UsableH float64 // printable height below the title block
	HeaderH float64 // the 3-row column header, repeated on every page
	RowH    float64 // one grid row (one band)
	SubRowH float64 // the full-width changes+note sub-row, when not inline
}

// A4Landscape is the default geometry: A4 landscape (297×210) less ~10mm margins, a title block, and sensible
// row heights.
func A4Landscape() Geom {
	return Geom{UsableW: 277, UsableH: 165, HeaderH: 18, RowH: 7, SubRowH: 6}
}

// column width budget (mm)
const (
	titleW   = 42 // "# · Title · key"
	changesW = 38 // the changes column, when inline
	noteW    = 38 // the sound-note column, when inline
	idealCol = 16 // a comfortable channel column
	floorCol = 10 // the legible minimum before we stop shrinking
)

// LayoutPlan is how a sheet of n channel columns × rows is laid out.
type LayoutPlan struct {
	ColW              float64 // channel column width actually used (≥ floor)
	InlineChangesNote bool    // true: changes+note ride each grid row; false: a full-width sub-row per song
	Bands             [][]int // channel-column INDICES per horizontal band (one band = fits across the page once)
	RowsPerPage       int     // whole songs per page (a song is never split; the header repeats)
}

// PlanLayout chooses the width + pagination for n channel columns and rowCount songs. Fallbacks, in order:
// (1) shrink the channel columns toward the floor; (2) move changes+note to a full-width sub-row; (3) split the
// channels into a second (or further) column band, each band repeating the title column.
func PlanLayout(n, rowCount int, g Geom) LayoutPlan {
	if n < 0 {
		n = 0
	}
	avail := g.UsableW - titleW
	all := seq(n)

	// (1) inline changes+note, one band — if the channels fit above the floor in what's left.
	if n == 0 || float64(n)*floorCol <= avail-changesW-noteW {
		colW := float64(idealCol)
		if n > 0 {
			colW = math.Min(idealCol, (avail-changesW-noteW)/float64(n))
		}
		return LayoutPlan{ColW: colW, InlineChangesNote: true, Bands: [][]int{all}, RowsPerPage: rowsPerPage(g, g.RowH, rowCount)}
	}

	// (2) changes+note on a full-width sub-row, one band — if the channels alone fit at the floor.
	if float64(n)*floorCol <= avail {
		colW := math.Min(idealCol, avail/float64(n))
		return LayoutPlan{ColW: colW, InlineChangesNote: false, Bands: [][]int{all}, RowsPerPage: rowsPerPage(g, g.RowH+g.SubRowH, rowCount)}
	}

	// (3) second column band: as many channels per band as fit at the floor, title repeated per band.
	perBand := int(math.Floor(avail / floorCol))
	if perBand < 1 {
		perBand = 1
	}
	var bands [][]int
	for i := 0; i < n; i += perBand {
		bands = append(bands, all[i:min(i+perBand, n)])
	}
	// each song now occupies len(bands) grid rows + one sub-row for changes+note.
	perSong := float64(len(bands))*g.RowH + g.SubRowH
	return LayoutPlan{ColW: floorCol, InlineChangesNote: false, Bands: bands, RowsPerPage: rowsPerPage(g, perSong, rowCount)}
}

func rowsPerPage(g Geom, perSongH float64, rowCount int) int {
	if perSongH <= 0 {
		return max(rowCount, 1)
	}
	n := int(math.Floor((g.UsableH - g.HeaderH) / perSongH))
	if n < 1 {
		n = 1
	}
	return n
}

func seq(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
