package app

import (
	"math"
	"testing"
)

// TestImportCrop sanitizes a crop arriving on an imported file: a valid crop on an uploaded file is kept
// verbatim, but a crop on a generated chart (P4), a geometrically invalid crop, or one with a non-canonical
// page key is DROPPED — a hand-edited folder can never inject a crop the save path would refuse.
func TestImportCrop(t *testing.T) {
	// LockAR deliberately false: importCrop must force it on (policy A), even from a hand-edited folder.
	good := &Crop{LockAR: false, Pages: map[string]CropRect{"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Source: "manual"}}}

	if got := importCrop(nil, false); got != nil {
		t.Fatalf("nil crop must stay nil, got %+v", got)
	}
	if got := importCrop(good, false); got == nil || !got.LockAR {
		t.Fatalf("a valid crop on an uploaded file must be kept with LockAR forced on, got %+v", got)
	}
	if got := importCrop(good, true); got != nil {
		t.Fatalf("a crop on a generated chart must be dropped (P4), got %+v", got)
	}
	offPage := &Crop{Pages: map[string]CropRect{"0": {X: 0.9, Y: 0, W: 0.8, H: 0.8}}}
	if got := importCrop(offPage, false); got != nil {
		t.Fatalf("an out-of-page crop must be dropped, got %+v", got)
	}
	badKey := &Crop{Pages: map[string]CropRect{"01": {X: 0.1, Y: 0.1, W: 0.5, H: 0.5}}}
	if got := importCrop(badKey, false); got != nil {
		t.Fatalf("a non-canonical page key must be dropped, got %+v", got)
	}
	// A hand-edited folder with LockAR OFF but a NON-SQUARE rect must be dropped: importCrop forces policy A
	// before validating, so the lockAR square rule rejects it (never stored aspect-locked-but-distorted).
	// This reds if importCrop forces LockAR AFTER validating.
	nonSquare := &Crop{LockAR: false, Pages: map[string]CropRect{"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.5, Source: "manual"}}}
	if got := importCrop(nonSquare, false); got != nil {
		t.Fatalf("a non-square crop must be dropped under forced AR-lock, got %+v", got)
	}

	// T199 B1a: an imported deskew angle is rounded to the 0.05° grid, exactly as a save (NormalizeCrop) does,
	// so an imported angle can never mint a raster hash a save would not have produced. 2.37 → 2.35.
	tilted := &Crop{Pages: map[string]CropRect{"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Angle: 2.37, Source: "manual"}}}
	got := importCrop(tilted, false)
	if got == nil {
		t.Fatal("a valid tilted crop must be kept")
	}
	if a := got.Pages["0"].Angle; math.Abs(a-2.35) > 1e-9 {
		t.Fatalf("imported angle must be rounded to 0.05° (2.37→2.35), got %v", a)
	}
	// An angle past ±10° is DROPPED on import (the dimension-free bound is enforced here).
	wild := &Crop{Pages: map[string]CropRect{"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Angle: 45, Source: "manual"}}}
	if importCrop(wild, false) != nil {
		t.Fatal("a crop with an out-of-range angle must be dropped on import")
	}
}
