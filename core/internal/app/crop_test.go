package app

import (
	"errors"
	"math"
	"testing"
)

func TestValidateCropRect(t *testing.T) {
	ok := CropRect{X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Source: "manual"}
	if err := ValidateCropRect(ok); err != nil {
		t.Fatalf("valid rect rejected: %v", err)
	}
	// full page is valid (x+w == 1 within eps)
	if err := ValidateCropRect(CropRect{X: 0, Y: 0, W: 1, H: 1, Source: "auto"}); err != nil {
		t.Fatalf("full-page rect rejected: %v", err)
	}
	// T199: a small deskew angle within ±10° is valid, boundaries included.
	for _, a := range []float64{3.25, 10, -10, 0} {
		if err := ValidateCropRect(CropRect{X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Angle: a, Source: "manual"}); err != nil {
			t.Fatalf("valid deskew angle %.2f rejected: %v", a, err)
		}
	}
	inf := math.Inf(1)
	nan := math.NaN()
	bad := map[string]CropRect{
		"negative x":      {X: -0.01, Y: 0.1, W: 0.5, H: 0.5, Source: "manual"},
		"too small w":     {X: 0.1, Y: 0.1, W: 0.19, H: 0.5, Source: "manual"},
		"too small h":     {X: 0.1, Y: 0.1, W: 0.5, H: 0.1, Source: "manual"},
		"x+w over 1":      {X: 0.6, Y: 0.1, W: 0.5, H: 0.5, Source: "manual"},
		"y+h over 1":      {X: 0.1, Y: 0.7, W: 0.5, H: 0.5, Source: "manual"},
		"bad source":      {X: 0.1, Y: 0.1, W: 0.5, H: 0.5, Source: "pink"},
		"NaN x":           {X: nan, Y: 0.1, W: 0.5, H: 0.5, Source: "manual"},
		"Inf w":           {X: 0.1, Y: 0.1, W: inf, H: 0.5, Source: "manual"},
		"-Inf y":          {X: 0.1, Y: math.Inf(-1), W: 0.5, H: 0.5, Source: "manual"},
		"angle over +10":  {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Angle: 10.5, Source: "manual"}, // T199
		"angle under -10": {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Angle: -11, Source: "manual"},
		"NaN angle":       {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Angle: nan, Source: "manual"},
		"Inf angle":       {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Angle: inf, Source: "manual"},
	}
	for name, r := range bad {
		err := ValidateCropRect(r)
		if err == nil {
			t.Errorf("%s: expected rejection, got nil", name)
		} else if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: error does not wrap ErrInvalidInput: %v", name, err)
		}
	}
}

// TestNormalizeCropRoundsAngle guards the SAVE-side 0.05° rounding (B1a's save half): a crop saved with an
// off-grid angle is snapped, so a save and an import of the "same" angle can't differ by float noise. Reds if
// roundDeskew is dropped from NormalizeCrop — which app/httpapi/bake tests otherwise leave green.
func TestNormalizeCropRoundsAngle(t *testing.T) {
	crop := Crop{Pages: map[string]CropRect{"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Angle: 2.37, Source: "manual"}}}
	if err := NormalizeCrop(&crop); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if a := crop.Pages["0"].Angle; math.Abs(a-2.35) > 1e-9 {
		t.Fatalf("save must round the deskew angle to 0.05° (2.37→2.35), got %v", a)
	}
}

func TestValidateCrop_PageIndex(t *testing.T) {
	c := Crop{LockAR: true, Pages: map[string]CropRect{
		"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Source: "manual"},
		"2": {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Source: "manual"},
	}}
	if err := ValidateCrop(c, 3); err != nil {
		t.Fatalf("pages 0,2 on a 3-page file rejected: %v", err)
	}
	if err := ValidateCrop(c, 2); err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("page 2 on a 2-page file must be refused, got: %v", err)
	}
	// unknown page count (0): index check skipped, geometry still applies
	if err := ValidateCrop(c, 0); err != nil {
		t.Fatalf("unknown page count should skip index check: %v", err)
	}
	// non-canonical / non-integer keys refused (else "1" and "01" both validate then merge in map order)
	for _, key := range []string{"1a", "01", "+1", " 1", "-1", ""} {
		c := Crop{Pages: map[string]CropRect{key: {X: 0.1, Y: 0.1, W: 0.5, H: 0.5, Source: "manual"}}}
		if err := ValidateCrop(c, 5); err == nil || !errors.Is(err, ErrInvalidInput) {
			t.Errorf("non-canonical page key %q must be refused, got: %v", key, err)
		}
	}
}

// TestValidateCrop_LockARRequiresSquare pins policy A's backstop: under lockAR a rect must be square in page
// fractions (w == h), so a client cannot slip a distorting non-square crop past the save. Dropping the check
// in ValidateCrop turns this red.
func TestValidateCrop_LockARRequiresSquare(t *testing.T) {
	nonSquare := Crop{LockAR: true, Pages: map[string]CropRect{"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.6, Source: "manual"}}}
	if err := ValidateCrop(nonSquare, 0); err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a non-square aspect-locked crop must be refused, got: %v", err)
	}
	// The same rect is fine when lockAR is off (policy B, a later toggle).
	free := nonSquare
	free.LockAR = false
	if err := ValidateCrop(free, 0); err != nil {
		t.Fatalf("a non-square crop is allowed when not aspect-locked: %v", err)
	}
	// A square under lockAR passes (within the 1e-6 tolerance).
	square := Crop{LockAR: true, Pages: map[string]CropRect{"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Source: "manual"}}}
	if err := ValidateCrop(square, 0); err != nil {
		t.Fatalf("a square aspect-locked crop must pass: %v", err)
	}
}
