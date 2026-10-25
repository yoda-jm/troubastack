package app_test

import (
	"testing"
)

// T205: Band.HidePdfMark survives a .tband export→import round-trip (band.json, additive). A band that never
// set it round-trips to the zero value (shown) — no migration.
func TestBandFolder_RoundTripsHidePdfMark_T205(t *testing.T) {
	src := newStack()
	admin, _, bandID, _, _, _ := buildSourceBand(t, src)

	// Default band (never set) → stays false after a round-trip.
	roundTripHidePdfMark := func(t *testing.T) bool {
		zipBytes, _, err := src.svc.ExportBand(admin, src.eng, bandID)
		if err != nil {
			t.Fatalf("export: %v", err)
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
		got, err := tgt.repo.GetBand(rep.Band.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got.HidePdfMark
	}

	if roundTripHidePdfMark(t) {
		t.Error("a default band round-tripped to HidePdfMark=true; zero value must be false (shown)")
	}

	hide := true
	if _, err := src.svc.UpdateBand(admin, bandID, nil, &hide); err != nil {
		t.Fatalf("set HidePdfMark: %v", err)
	}
	if !roundTripHidePdfMark(t) {
		t.Error("HidePdfMark=true did not survive the .tband round-trip")
	}
}
