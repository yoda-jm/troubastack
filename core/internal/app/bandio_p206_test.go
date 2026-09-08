package app_test

import (
	"strings"
	"testing"

	"troubastack/core/internal/domain"
)

// P206 — a jump is a PAIR of placed landmarks, and the pairing IS the destination object's uuid. The v2
// band folder keeps object uuids verbatim, so the reference survives a round-trip BY CONSTRUCTION — but
// only if the format carries the field at all. `v2Object` is a hand-maintained mirror of `domain.Object`
// (it is built field by field in both directions), and that mirror has now silently dropped a new field
// twice: T86's meter and T152's band identity. This is the same shape, so it gets the same guard.
//
// Red before the fix: the pair round-trips as two unrelated landmarks and the jump is gone, with nothing
// anywhere reporting a loss.
func TestBandExportImport_CarriesJumpPairing_P206(t *testing.T) {
	src := newStack()
	admin, _, bandID, songID, _, _ := buildSourceBand(t, src)

	// A jump pair on the SHARED layer, exactly as the Studio's tool places it: the destination first, then
	// the source carrying jumpTo. Both icons, same glyph — the pair matches by looking the same.
	for _, o := range []domain.Object{
		{UUID: "jump-dest", LayerID: "L-shared", Type: domain.TypeIcon, Page: 1, Version: 1, Text: "segno",
			Points: []domain.Point{{X: 0.40, Y: 0.25}, {X: 0.48, Y: 0.31}}, OwnerID: admin.ID,
			Style: domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.004}},
		{UUID: "jump-src", LayerID: "L-shared", Type: domain.TypeIcon, Page: 0, Version: 1, Text: "segno",
			Points: []domain.Point{{X: 0.10, Y: 0.60}, {X: 0.18, Y: 0.66}}, OwnerID: admin.ID,
			Style: domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.004}, JumpTo: "jump-dest"},
	} {
		obj := o
		if _, err := src.eng.Apply(songID, domain.Mutation{Kind: domain.KindCreate, UUID: obj.UUID, AuthorID: admin.ID, Object: &obj}); err != nil {
			t.Fatalf("seed %s: %v", obj.UUID, err)
		}
	}

	zipBytes, _, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	// The folder is the interchange format a human may read and hand-edit — the pairing must be VISIBLE in
	// it, not implied by something else.
	if ann := string(unzip(t, zipBytes)["annotations/the-open-road.json"]); !strings.Contains(ann, `"jumpTo": "jump-dest"`) {
		t.Fatalf("the exported annotations file must carry the jump's target:\n%s", ann)
	}

	tgt := newStack()
	importer, err := tgt.svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	rep, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	if len(songs) != 1 {
		t.Fatalf("want the one imported song, got %d", len(songs))
	}
	snap, err := tgt.eng.Head(songs[0].ID)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	live := snap.LiveObjects()
	var srcObj, destObj *domain.Object
	for i := range live {
		switch live[i].UUID {
		case "jump-src":
			srcObj = &live[i]
		case "jump-dest":
			destObj = &live[i]
		}
	}
	if srcObj == nil || destObj == nil {
		t.Fatalf("both landmarks must survive the round-trip (src=%v dest=%v)", srcObj != nil, destObj != nil)
	}
	if !srcObj.IsJumpSource() {
		t.Fatalf("the imported source is no longer a jump — the pairing was dropped by the folder format")
	}
	if srcObj.JumpTo != destObj.UUID {
		t.Fatalf("imported jumpTo = %q, want the destination's uuid %q", srcObj.JumpTo, destObj.UUID)
	}
}
