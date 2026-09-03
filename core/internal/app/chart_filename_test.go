package app_test

import (
	"testing"

	"troubastack/core/internal/app"
)

// T72: a text chart's default pool name is its title with NO ".pdf"; a user rename survives a
// later source edit (the source-update path must not re-derive the filename). Red on pre-T72 code.
func TestTextChartFilenameSurvivesEdit(t *testing.T) {
	st := newStack()
	admin, err := st.svc.Register("marie", "Marie", "password123", "marie@x.com")
	if err != nil {
		t.Fatal(err)
	}
	band, err := st.svc.CreateBand(admin, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := st.svc.CreateSong(admin, band.ID, "Riverside Waltz", "The Riverside Trio")
	if err != nil {
		t.Fatal(err)
	}

	chart, err := st.svc.CreateTextChart(admin, band.ID, song.ID, "# Riverside Waltz\nThe Riverside Trio\n\n## Verse\nAm\nline\n")
	if err != nil {
		t.Fatal(err)
	}
	if chart.Filename != "Riverside Waltz" {
		t.Fatalf("default filename = %q, want %q (title, no .pdf)", chart.Filename, "Riverside Waltz")
	}

	newName := "Guitar/Bass"
	renamed, err := st.svc.UpdateSongFile(admin, band.ID, song.ID, chart.ID, app.SongFilePatch{Filename: &newName})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Filename != "Guitar/Bass" {
		t.Fatalf("after rename filename = %q, want Guitar/Bass", renamed.Filename)
	}

	edited, err := st.svc.SaveChartSource(admin, band.ID, song.ID, chart.ID, renamed.Revision,
		"# Riverside Waltz\nThe Riverside Trio\n\n## Intro\nAm E7\n\n## Verse\nAm\nline\n")
	if err != nil {
		t.Fatal(err)
	}
	if edited.Filename != "Guitar/Bass" {
		t.Errorf("filename after source edit = %q, want Guitar/Bass — the rename must survive", edited.Filename)
	}
	if edited.BlobHash == renamed.BlobHash {
		t.Errorf("blob hash unchanged after source edit — the chart did not re-render")
	}
	if edited.Revision != renamed.Revision+1 {
		t.Errorf("revision = %d, want %d (bumped once by the edit)", edited.Revision, renamed.Revision+1)
	}
}

// A chart with no `# Title` falls back to "Chart" (via chartpdf.Title), still no ".pdf".
func TestTextChartFilenameNoTitleFallback(t *testing.T) {
	st := newStack()
	admin, _ := st.svc.Register("m", "M", "password123", "m@x.com")
	band, _ := st.svc.CreateBand(admin, "B")
	song, _ := st.svc.CreateSong(admin, band.ID, "S", "A")
	chart, err := st.svc.CreateTextChart(admin, band.ID, song.ID, "no title here\n\njust text\n")
	if err != nil {
		t.Fatal(err)
	}
	if chart.Filename != "Chart" {
		t.Errorf("no-title default = %q, want %q", chart.Filename, "Chart")
	}
}
