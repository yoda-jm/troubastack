package main

import (
	"os/exec"
	"strings"
	"testing"
)

// B13 guard — every text run mkcharts RECORDS as an anchor must actually appear, verbatim, in
// the PDF it drew. This closes the class of bug where the manifest and the page disagree: the
// containment/ink tests are blind to it (they measure geometry and ink density, and mojibake
// has ink), so a label drawn without the cp1252 translator rendered "â€"" while the anchor still
// said "—" and everything passed. Catches any future encoding or draw/record divergence.
//
// Only the mkcharts-generated charts are checked — the engraved PDFs' anchors are hand-calibrated
// boxes, not text runs we drew.
func TestAnchorTextMatchesPDF(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not available")
	}
	for _, base := range []string{
		"open-road-leadsheet", "open-road-guitar", "amazing-grace",
		"house-rising-sun-tab", "house-rising-sun-drums", "blank-chart",
	} {
		pdf := strings.TrimSuffix(anchorsPath(base), ".anchors.json") + ".pdf"
		out, err := exec.Command("pdftotext", "-layout", pdf, "-").Output()
		if err != nil {
			t.Fatalf("pdftotext %s: %v", pdf, err)
		}
		// Normalize whitespace: pdftotext -layout pads columns, and our runs are single lines.
		text := strings.Join(strings.Fields(string(out)), " ")
		for _, a := range mustAnchors(base).boxes {
			want := strings.Join(strings.Fields(a.Text), " ")
			if want == "" {
				continue
			}
			if !strings.Contains(text, want) {
				t.Errorf("%s: anchor text %q is not present in the rendered PDF — the manifest and "+
					"the page disagree (encoding? draw/record mismatch?)", base, a.Text)
			}
		}
	}
}
