package chartpdf

import (
	"fmt"
	"strings"
	"testing"
)

// T168 — in two columns, nothing may be drawn outside its column.
//
// VLL put `columns: 2` on a real chart and the text ran off the right of the page. Measured before the fix,
// on the fixture below: 26 runs escaped the left column, 14 left the paper, and the worst right edge was
// 2.085 × the page width. The cause was structural — the column's LEFT edge was threaded into the body
// primitives but no width was, and a chart line is never wrapped (wrapping is what these tests now require,
// per VLL's ⟨D1⟩: "passer à la ligne automatiquement avec le mot qui dépasse").
//
// The assertion reads the anchor manifest, which carries every drawn run's box — so "where the ink lands"
// is measurable without rasterising anything. The stage-2 tests asserted the *trade* (fewer pages, larger
// type) and the byte-identical one-column property; neither looked at this.

// columnBounds returns the [0,1] page fractions of the left column's right edge and the page's right edge.
func columnBounds(nCols int) (leftColRight, pageRight float64) {
	colW := (right - leftMargin - float64(nCols-1)*colGutter) / float64(nCols)
	return (leftMargin + colW) / pageW, right / pageW
}

// longLineChart: a body whose lines are longer than half a page, which is the condition that makes two
// columns overflow. Invented words only — no band data.
func longLineChart(twoCol bool, nLines int) string {
	var b strings.Builder
	b.WriteString("# An Invented Chart\n")
	if twoCol {
		b.WriteString("columns: 2\n")
	}
	b.WriteString("\n## Verse\n")
	const long = "a considerably longer lyric line than a half page column can hold without wrapping somewhere"
	for i := 0; i < nLines; i++ {
		if i%4 == 0 {
			b.WriteString("G     D     Am    C     G     D     Am    C     G     D\n")
		}
		fmt.Fprintf(&b, "%s %02d\n", long, i)
	}
	return b.String()
}

// overflowReport counts runs drawn outside their column, and the worst right edge seen.
func overflowReport(t *testing.T, src string, nCols int) (outOfColumn int, offPage int, worst float64) {
	t.Helper()
	_, anchors, err := RenderWithAnchors(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	leftColRight, pageRight := columnBounds(nCols)
	const eps = 1e-6
	for _, a := range anchors {
		if a.X1 > worst {
			worst = a.X1
		}
		if nCols > 1 && a.X0 < leftColRight { // a run in the LEFT column
			if a.X1 > leftColRight+eps {
				outOfColumn++
			}
			continue
		}
		if a.X1 > pageRight+eps {
			offPage++
		}
	}
	return outOfColumn, offPage, worst
}

func TestTwoColumns_NothingIsDrawnOutsideItsColumn_T168(t *testing.T) {
	outOfCol, offPage, worst := overflowReport(t, longLineChart(true, 40), 2)
	leftColRight, pageRight := columnBounds(2)
	if outOfCol > 0 {
		t.Errorf("%d run(s) drawn past the left column's right edge (%.3f) — VLL's \"text overflow on the right\"", outOfCol, leftColRight)
	}
	if offPage > 0 {
		t.Errorf("%d run(s) drawn past the paper's right edge (%.3f) — words printed off the sheet", offPage, pageRight)
	}
	if worst > pageRight+1e-6 {
		t.Errorf("worst right edge %.3f exceeds the page's %.3f", worst, pageRight)
	}
}

// The one-column path must keep drawing inside the page: the fix must not leak into charts that were fine.
func TestOneColumn_NothingIsDrawnOffThePage_T168(t *testing.T) {
	_, offPage, worst := overflowReport(t, longLineChart(false, 40), 1)
	_, pageRight := columnBounds(1)
	if offPage > 0 {
		t.Errorf("%d run(s) off the page in ONE column (worst %.3f, page right %.3f)", offPage, worst, pageRight)
	}
}

// TEETH for the wrap: a chord+lyric pair wraps TOGETHER. Both rows are Courier at the same size, so a
// character index is the same x offset in each — the continuation must keep its chords over its words.
// A wrap that split only the lyric would leave the continuation's chords behind and this fails.
func TestTwoColumns_AWrappedPairKeepsItsChordsOverItsWords_T168(t *testing.T) {
	// One pair, far longer than a column: chords at known character positions over a known lyric.
	const chords = "C                                                            G"
	const lyric = "one two three four five six seven eight nine ten eleven twelve"
	src := "# An Invented Chart\ncolumns: 2\n\n## Verse\n" + chords + "\n" + lyric + "\n"
	_, anchors, err := RenderWithAnchors(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// Every drawn run must sit inside a column.
	leftColRight, pageRight := columnBounds(2)
	for _, a := range anchors {
		limit := pageRight
		if a.X0 < leftColRight {
			limit = leftColRight
		}
		if a.X1 > limit+1e-6 {
			t.Fatalf("run %q spans [%.3f,%.3f], past its column limit %.3f", a.Text, a.X0, a.X1, limit)
		}
	}
	// The trailing chord must still be drawn — a wrap that dropped the tail of the chord row would lose it.
	var sawG bool
	for _, a := range anchors {
		if strings.Contains(a.Text, "G") && !strings.Contains(a.Text, "one") {
			sawG = true
		}
	}
	if !sawG {
		t.Error("the chord that sat over the END of the lyric is gone — the wrap dropped the chord row's tail")
	}
}
