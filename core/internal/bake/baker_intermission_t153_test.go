package bake

import (
	"context"
	"strings"
	"testing"
)

// T153 slice 1 — the domain can express an intermission, but this build cannot RENDER its separator page.
// The baker must refuse the whole bake rather than skip the entry.
//
// Skipping would be the tempting choice and the wrong one: the bundle would carry fewer entries than the
// running order, so the printed sheet and Stage would disagree about what comes next, and nothing would
// say so — a plausible wrong answer, which is precisely the failure mode T153's spec is written against.
// Refusing cannot corrupt a bundle, and it is removed by the baker slice that renders the page.
//
// Teeth: the same setlist WITHOUT the break must still bake, so the guard rejects the unrenderable entry
// and not the setlist.
func TestBake_RefusesASetlistContainingAnIntermission_T153(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	png := tinyPNG(t, 40, 60)
	newBaker := func() *Baker {
		return &Baker{
			svc:      svc,
			eng:      eng,
			raster:   fakeRaster{pages: 1, png: png},
			overlays: fakeOverlays{png: png},
			bakesDir: t.TempDir(),
			now:      func() int64 { return 1700000000 },
		}
	}

	// Without a break, the seeded setlist bakes — the control arm.
	if _, _, err := newBaker().Bake(context.Background(), bandID, setlistID, u, nil, ""); err != nil {
		t.Fatalf("control: the setlist must bake before a break is added: %v", err)
	}

	if _, err := svc.AddSetlistIntermission(u, bandID, setlistID, "Entracte"); err != nil {
		t.Fatalf("add intermission: %v", err)
	}

	_, _, err := newBaker().Bake(context.Background(), bandID, setlistID, u, nil, "")
	if err == nil {
		t.Fatal("bake accepted a setlist containing an intermission — the bundle would silently carry fewer entries than the running order")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "intermission") {
		t.Errorf("bake refused, but the error does not name the cause: %v", err)
	}
}
