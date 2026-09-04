package app_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"troubastack/core/internal/app"
)

// TestImport_PreservesSetlistOrder is the T140 regression: importing a band must keep a setlist's running
// order. The v2 folder expresses order as ARRAY ORDER (v2SetlistItem has no position field), so the reader
// must materialise it as Position — otherwise every imported item is Position 0 and retrieval
// (SortSetlistItems / Setlist) falls back to UUID order and the concert plays scrambled (hit in a real
// rehearsal).
//
// Teeth: 12 items in an order that no sort (alphabetical, insertion, or the random UUID order an all-zero
// set collapses to) would reproduce. Reverting the `Position: idx` assignment in parseV2 turns this red.
func TestImport_PreservesSetlistOrder(t *testing.T) {
	const n = 12
	// a deliberately non-monotonic running order — not sorted, not reverse-sorted.
	order := []int{7, 3, 11, 1, 9, 5, 12, 2, 8, 4, 10, 6}
	if len(order) != n {
		t.Fatalf("test setup: order has %d entries, want %d", len(order), n)
	}

	songs := make([]any, 0, n)
	for i := 1; i <= n; i++ {
		songs = append(songs, map[string]any{"slug": fmt.Sprintf("s%02d", i), "title": fmt.Sprintf("Song %02d", i)})
	}
	items := make([]any, 0, n)
	for _, k := range order {
		items = append(items, map[string]any{"song": fmt.Sprintf("s%02d", k)})
	}
	band, _ := json.Marshal(map[string]any{
		"formatVersion": 2, "name": "Order Band",
		"members": []any{map[string]any{"username": "dana", "displayName": "Dana", "role": "admin"}},
	})
	rep, _ := json.Marshal(map[string]any{"songs": songs})
	setl, _ := json.Marshal(map[string]any{
		"setlists": []any{map[string]any{"name": "Gig", "items": items}},
	})
	zipBytes := rezip(t, map[string][]byte{
		"band.json": band, "repertoire.json": rep, "setlists.json": setl,
	})

	st := newStack()
	importer, _ := st.svc.Register("owner", "Owner", "password123", "")
	report, err := st.svc.ImportBand(importer, st.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	// read the setlist back the way a client (Studio / the baker) does
	setlists, err := st.svc.Setlists(importer, report.Band.ID)
	if err != nil || len(setlists) != 1 {
		t.Fatalf("Setlists: got %d (err %v), want 1", len(setlists), err)
	}
	detail, err := st.svc.Setlist(importer, report.Band.ID, setlists[0].ID)
	if err != nil {
		t.Fatalf("Setlist: %v", err)
	}
	if len(detail.Items) != n {
		t.Fatalf("got %d items, want %d", len(detail.Items), n)
	}
	for k, v := range detail.Items {
		want := fmt.Sprintf("Song %02d", order[k])
		if v.SongTitle != want {
			t.Fatalf("position %d: got %q, want %q — setlist order was scrambled by import\nfull order: %s",
				k, v.SongTitle, want, titles(detail.Items))
		}
	}
}

func titles(items []app.SetlistItemView) string {
	out := make([]string, len(items))
	for i, v := range items {
		out[i] = v.SongTitle
	}
	return fmt.Sprint(out)
}
