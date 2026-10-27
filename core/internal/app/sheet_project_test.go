package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The ONE shared projection vector set, loaded by this Go test AND web/studio's lineup-grid.test.ts, so the
// sheet (Go) and the editor (TS) can never disagree on the channel layout (Fable, 2026-10-24).
type sheetVectors struct {
	Cases []struct {
		Name      string        `json:"name"`
		Musicians []projMusJSON `json:"musicians"`
		Parts     []Part        `json:"parts"`
		Songs     []Song        `json:"songs"`
		Channels  []Channel     `json:"channels"`
		Expect    struct {
			Subcols []string                     `json:"subcols"`
			Cells   map[string]map[string]string `json:"cells"`
		} `json:"expect"`
	} `json:"cases"`
}

type projMusJSON struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}

func TestSheetProjection_SharedVectors(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "lineup-sheet-vectors.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var v sheetVectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	if len(v.Cases) == 0 {
		t.Fatal("no vector cases")
	}
	for _, c := range v.Cases {
		t.Run(c.Name, func(t *testing.T) {
			partByID := map[string]Part{}
			for _, p := range c.Parts {
				partByID[p.ID] = p
			}
			mus := make([]projMusician, 0, len(c.Musicians))
			name := map[string]string{}
			for _, m := range c.Musicians {
				mus = append(mus, projMusician{ID: m.ID, Name: m.Name, Order: m.Order})
				name[m.ID] = m.Name
			}
			subcols := projectColumns(c.Songs, mus, c.Parts, c.Channels)
			rank := joinRank(c.Parts)
			gotKeys := make([]string, len(subcols))
			for i, sc := range subcols {
				gotKeys[i] = sc.Key
			}
			if !equalStrings(gotKeys, c.Expect.Subcols) {
				t.Fatalf("subcols = %v, want %v", gotKeys, c.Expect.Subcols)
			}
			byKey := map[string]SheetSubcol{}
			for _, sc := range subcols {
				byKey[sc.Key] = sc
			}
			for _, sg := range c.Songs {
				want, ok := c.Expect.Cells[sg.ID]
				if !ok {
					continue
				}
				byMus := map[string]LineupEntry{}
				for _, e := range sg.Lineup {
					byMus[e.MusicianID] = e
				}
				for key, wantText := range want {
					got := sheetCell(byKey[key], byMus, partByID, name, rank)
					if got.Text != wantText {
						t.Errorf("%s cell[%s] = %q, want %q", sg.ID, key, got.Text, wantText)
					}
				}
			}
		})
	}
}

// A musician split across non-adjacent columns (as patch order can produce) yields TWO owner spans, never one
// spanning across someone else's column (T203 stage 4 review check).
func TestOwnerSpans_PatchSplit(t *testing.T) {
	subcols := []SheetSubcol{
		{Key: "voice:A", OwnerID: "A", OwnerName: "Ann"},
		{Key: "inst:B:g", OwnerID: "B", OwnerName: "Bo"},
		{Key: "inst:A:g", OwnerID: "A", OwnerName: "Ann"},
	}
	spans := ownerSpans(subcols)
	if len(spans) != 3 {
		t.Fatalf("want 3 spans (Ann, Bo, Ann), got %d: %+v", len(spans), spans)
	}
	ann := 0
	for _, s := range spans {
		if s.OwnerName == "Ann" {
			ann++
			if s.Span != 1 {
				t.Errorf("each Ann run spans 1, got %d", s.Span)
			}
		}
	}
	if ann != 2 {
		t.Fatalf("Ann should appear as TWO separate spans, got %d", ann)
	}
}

// T203-FIX2: backfillPartOrders gives dense, non-zero, unique orders in slice order, keeping explicit ones.
func TestBackfillPartOrders(t *testing.T) {
	ps := []Part{{ID: "a"}, {ID: "b", Order: 5}, {ID: "c"}}
	if !backfillPartOrders(ps) {
		t.Fatal("expected a change")
	}
	seen := map[int]bool{}
	for _, p := range ps {
		if p.Order == 0 {
			t.Fatalf("a zero order remains: %+v", ps)
		}
		if seen[p.Order] {
			t.Fatalf("duplicate order %d: %+v", p.Order, ps)
		}
		seen[p.Order] = true
	}
	if ps[1].Order != 5 {
		t.Errorf("explicit order should be kept, got %d", ps[1].Order)
	}
	if backfillPartOrders(ps) {
		t.Error("second backfill should be a no-op")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
