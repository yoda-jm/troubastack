package soundsheet

import (
	"bytes"
	"strings"
	"testing"

	"troubastack/core/internal/app"
)

func sampleModel(nMus, nSongs int) app.SheetModel {
	m := app.SheetModel{SetlistName: "Test concert", Order: app.SheetOrderStage}
	for i := 0; i < nMus; i++ {
		mid := "m" + string(rune('A'+i))
		m.Subcols = append(m.Subcols,
			app.SheetSubcol{Key: "voice:" + mid, Kind: "voice", OwnerID: mid, OwnerName: mid, Channel: "Voice", Input: app.SheetLabelNotPatched, Amplified: true},
			app.SheetSubcol{Key: "inst:" + mid + ":g", Kind: "inst", OwnerID: mid, OwnerName: mid, Channel: "Guitar", Input: app.SheetLabelNotPatched, Amplified: true},
		)
	}
	m.OwnerSpans = nil
	for i := 0; i < nSongs; i++ {
		cells := map[string]app.SheetCell{}
		for _, sc := range m.Subcols {
			if sc.Kind == "voice" {
				cells[sc.Key] = app.SheetCell{Text: "L"}
			}
		}
		m.Rows = append(m.Rows, app.SheetRow{SongID: "s", Position: i + 1, Title: "Song", Cells: cells, SoundNote: "note", Changes: []string{"x in"}})
	}
	return m
}

func TestRenderPDF_NonEmptyMultiPage(t *testing.T) {
	// 10 musicians × 40 songs — the stress case; must render without error, as a valid multi-page PDF.
	pdf, err := RenderPDF(sampleModel(10, 40))
	if err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("not a PDF (prefix %q)", pdf[:min(8, len(pdf))])
	}
	if n := bytes.Count(pdf, []byte("/Type /Page")); n < 2 {
		t.Fatalf("expected a multi-page PDF, saw %d page objects", n)
	}
}

func TestRenderCSV_OneColumnPerSubcol(t *testing.T) {
	m := sampleModel(2, 3)
	csv := string(RenderCSV(m))
	head := strings.SplitN(csv, "\n", 2)[0]
	// "#","Title", 4 subcols, "Changes","Sound note" = 8 columns
	if got := strings.Count(head, ",") + 1; got != 2+len(m.Subcols)+2 {
		t.Fatalf("CSV header has %d columns, want %d: %q", got, 2+len(m.Subcols)+2, head)
	}
	if !strings.Contains(head, "Voice") || !strings.Contains(head, "Guitar") {
		t.Fatalf("CSV header missing channel labels: %q", head)
	}
	if !strings.Contains(csv, "note") {
		t.Fatalf("CSV missing the sound note")
	}
}
