package app

import (
	"testing"

	"troubastack/core/internal/setlistpdf"
)

// TestBuildSetlistDoc_IntermissionUnnumbered_T153 is the assertion T158's export review
// asked for and T153 makes provable: build the Doc from a setlist of song, song, break,
// song, bench and assert the running-order numbers are 1, 2, null, 3, null — the break
// carries NO number and does NOT shift the song after it.
//
// Teeth: revert buildSetlistDoc to hardcode runningorder.KindSong (its pre-T153 bug) and
// the break becomes #3, the following song #4, and its row Kind is song — this test goes
// red on exactly that line.
func TestBuildSetlistDoc_IntermissionUnnumbered_T153(t *testing.T) {
	det := SetlistDetail{
		Setlist: Setlist{Name: "Night"},
		Items: []SetlistItemView{
			{SetlistItem: SetlistItem{Kind: SetlistKindSong}, SongTitle: "Opener"},
			{SetlistItem: SetlistItem{Kind: SetlistKindSong}, SongTitle: "Second"},
			{SetlistItem: SetlistItem{Kind: SetlistKindIntermission, Label: "Entracte"}, SongTitle: "Entracte"},
			{SetlistItem: SetlistItem{Kind: SetlistKindSong}, SongTitle: "Third"},
			{SetlistItem: SetlistItem{Kind: SetlistKindSong, OnCall: true}, SongTitle: "Bench"},
		},
	}

	doc := buildSetlistDoc("The Band", det)

	if len(doc.Main) != 4 {
		t.Fatalf("main rows = %d, want 4 (three songs + the break)", len(doc.Main))
	}
	if len(doc.OnCall) != 1 {
		t.Fatalf("bench rows = %d, want 1", len(doc.OnCall))
	}

	// The numbers that must hold: 1, 2, null(break), 3 — the break neither takes a number
	// nor pushes "Third" to 4.
	wantMain := []struct {
		title  string
		number int
		kind   string
	}{
		{"Opener", 1, setlistpdf.KindSong},
		{"Second", 2, setlistpdf.KindSong},
		{"Entracte", 0, setlistpdf.KindIntermission}, // 0 ⇒ unnumbered
		{"Third", 3, setlistpdf.KindSong},
	}
	for i, w := range wantMain {
		got := doc.Main[i]
		if got.Title != w.title || got.Number != w.number || got.Kind != w.kind {
			t.Errorf("main[%d] = {title:%q number:%d kind:%q}, want {title:%q number:%d kind:%q}",
				i, got.Title, got.Number, got.Kind, w.title, w.number, w.kind)
		}
	}

	if b := doc.OnCall[0]; b.Number != 0 || b.Kind != setlistpdf.KindSong {
		t.Errorf("bench row = {number:%d kind:%q}, want {number:0 kind:song}", b.Number, b.Kind)
	}
}
