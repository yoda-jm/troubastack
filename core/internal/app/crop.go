package app

import (
	"fmt"
	"math"
	"strconv"
)

// T198 — per-page bake-area (crop) rules, kept pure so validation is unit-testable and shared by the
// crop-save endpoint and by import. A crop lives on an uploaded PDF (never a Generated chart, P4); rects are
// in the SOURCE page frame, normalized [0,1], origin top-left — the same frame as annotations.

// cropEps absorbs float round-trips through JSON so a rect that reaches exactly the page edge (x+w == 1) is
// not rejected by a 1e-15 overshoot.
const cropEps = 1e-6

// minCropSide is the smallest side a crop rect may have. A smaller rect is almost surely a mis-drag, not an
// intent, so it is refused rather than baked into a postage-stamp page (spec §Data model).
const minCropSide = 0.2

// ValidateCropRect checks one page's bake area against the geometry rules (spec §Data model):
// 0 ≤ x,y; x+w ≤ 1; y+h ≤ 1; w,h ≥ 0.2. It returns a wrapped ErrInvalidInput naming the broken rule, or nil.
func ValidateCropRect(r CropRect) error {
	switch {
	case isBadFloat(r.X) || isBadFloat(r.Y) || isBadFloat(r.W) || isBadFloat(r.H):
		return fmt.Errorf("%w: crop numbers must be finite (no NaN/Inf)", ErrInvalidInput)
	case r.X < -cropEps || r.Y < -cropEps:
		return fmt.Errorf("%w: crop origin must be inside the page (x,y ≥ 0)", ErrInvalidInput)
	case r.W < minCropSide || r.H < minCropSide:
		return fmt.Errorf("%w: crop is too small — each side must be at least %.0f%% of the page", ErrInvalidInput, minCropSide*100)
	case r.X+r.W > 1+cropEps || r.Y+r.H > 1+cropEps:
		return fmt.Errorf("%w: crop must stay inside the page (x+w ≤ 1, y+h ≤ 1)", ErrInvalidInput)
	case r.Source != "manual" && r.Source != "auto":
		return fmt.Errorf("%w: crop source must be \"manual\" or \"auto\"", ErrInvalidInput)
	}
	return nil
}

// ValidateCrop checks a whole file crop: every page index is in range for a file of pageCount pages, and
// every rect is valid. pageCount ≤ 0 means "unknown" (skip the index-range check — the caller could not
// resolve the page count; the geometry rules still apply). Returns wrapped ErrInvalidInput or nil.
func ValidateCrop(c Crop, pageCount int) error {
	for key, rect := range c.Pages {
		idx, err := parsePageIndex(key)
		if err != nil {
			return err
		}
		if pageCount > 0 && (idx < 0 || idx >= pageCount) {
			return fmt.Errorf("%w: crop names page %d, but the file has %d page(s)", ErrInvalidInput, idx, pageCount)
		}
		if err := ValidateCropRect(rect); err != nil {
			return err
		}
	}
	return nil
}

// isBadFloat reports a non-finite coordinate — a NaN or ±Inf would slip past the ordering comparisons below
// (NaN compares false to everything) and poison the render.
func isBadFloat(f float64) bool { return math.IsNaN(f) || math.IsInf(f, 0) }

// parsePageIndex parses a crop map key to a non-negative page index, accepting ONLY the CANONICAL decimal
// form (strconv.Itoa): "0","1","2",… A non-canonical key ("01", "+1", " 1") is refused, so two keys can never
// name the same page and then merge in map order — the bake must have one rect per page, deterministically.
func parsePageIndex(key string) (int, error) {
	n, err := strconv.Atoi(key)
	if err != nil || n < 0 || strconv.Itoa(n) != key {
		return 0, fmt.Errorf("%w: crop page index %q must be a canonical non-negative integer (0,1,2,…)", ErrInvalidInput, key)
	}
	return n, nil
}
