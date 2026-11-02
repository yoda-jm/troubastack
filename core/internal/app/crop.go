package app

import (
	"fmt"
	"math"
	"strconv"
	"strings"
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

// maxDeskewDeg bounds the T199 per-page deskew: small-skew only. A quarter turn (sideways/upside-down scan) is
// the musician's job (spec §Out of scope), so an angle past ±10° is refused, never auto-rotated.
const maxDeskewDeg = 10.0

// deskewStepDeg is the angle quantum. Rounding to it on save AND import (so the two agree) keeps float noise
// from minting a raster hash that only one path would produce.
const deskewStepDeg = 0.05

// roundDeskew snaps an angle to the 0.05° grid (B1a). Shared by the save (NormalizeCrop) and import so an
// imported angle can never hash to a raster the save path would not have produced.
func roundDeskew(a float64) float64 {
	if a == 0 || isBadFloat(a) {
		return a // 0 stays 0 (omitempty, the T198-identical key); a non-finite angle is caught by validation
	}
	return math.Round(a/deskewStepDeg) * deskewStepDeg
}

// ValidateCropRect checks one page's bake area against the geometry rules (spec §Data model):
// 0 ≤ x,y; x+w ≤ 1; y+h ≤ 1; w,h ≥ 0.2. It returns a wrapped ErrInvalidInput naming the broken rule, or nil.
func ValidateCropRect(r CropRect) error {
	switch {
	case isBadFloat(r.X) || isBadFloat(r.Y) || isBadFloat(r.W) || isBadFloat(r.H) || isBadFloat(r.Angle):
		return fmt.Errorf("%w: crop numbers must be finite (no NaN/Inf)", ErrInvalidInput)
	case r.X < -cropEps || r.Y < -cropEps:
		return fmt.Errorf("%w: crop origin must be inside the page (x,y ≥ 0)", ErrInvalidInput)
	case r.W < minCropSide || r.H < minCropSide:
		return fmt.Errorf("%w: crop is too small — each side must be at least %.0f%% of the page", ErrInvalidInput, minCropSide*100)
	case r.X+r.W > 1+cropEps || r.Y+r.H > 1+cropEps:
		return fmt.Errorf("%w: crop must stay inside the page (x+w ≤ 1, y+h ≤ 1)", ErrInvalidInput)
	// T199: dimension-free angle bound. The rotated-window-inside-the-page check needs the page aspect, which
	// this layer never carries (the service has no page dims without rendering), so it is enforced where the
	// dims exist — Studio clamps it, and the bake clamps defensively — exactly as the page-index RANGE is.
	case math.Abs(r.Angle) > maxDeskewDeg+cropEps:
		return fmt.Errorf("%w: crop deskew angle must be within ±%.0f°", ErrInvalidInput, maxDeskewDeg)
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
		// Policy A (lockAR): a crop is a PURE ZOOM, so each rect must be a square in page-fraction space
		// (w == h) — that is what gives it the page's aspect in pixels and keeps the tablet page shape. The
		// server enforces it so a client that sends a non-square under lockAR is refused, not silently baked
		// into a distorted page. (Studio's crop window keeps w == h; this is the backstop.)
		if c.LockAR && math.Abs(rect.W-rect.H) > 1e-6 {
			return fmt.Errorf("%w: an aspect-locked crop must be square in page fractions (w == h), got w=%.4f h=%.4f", ErrInvalidInput, rect.W, rect.H)
		}
	}
	return nil
}

// importCrop sanitizes a crop arriving on an imported file (T198). An export written by this server is
// always valid, but a hand-edited .tband folder is not trusted: a crop with bad geometry, a non-canonical
// page key, or any crop on a generated chart (P4) is DROPPED rather than imported, so a malformed folder
// can never inject a crop the save path would have refused. The file imports uncropped; its next save
// re-validates. A nil crop (the common case) stays nil.
func importCrop(c *Crop, generated bool) *Crop {
	if c == nil {
		return nil
	}
	if generated {
		return nil // P4: a generated chart lays out to the page; it has no crop.
	}
	// Force policy A (AR-lock) BEFORE validating, so the lockAR square rule is enforced on import too: a
	// hand-edited folder that left LockAR off with a non-square rect is DROPPED, not silently stored
	// aspect-locked-but-distorted.
	c.LockAR = true
	// B1a (T199): round each deskew angle to the 0.05° grid, exactly as NormalizeCrop does on save, so an
	// imported angle can never mint a raster hash a save would not have produced. Neither import NOR save has
	// the page dims, so the rotated-window-inside-the-page check lives only where the dims do — Studio's
	// client-side clamp and the bake's visible defensive clamp; here only the dimension-free bound
	// (|angle| ≤ 10, finite) is enforced, via ValidateCrop.
	for key, r := range c.Pages {
		if a := roundDeskew(r.Angle); a != r.Angle {
			r.Angle = a
			c.Pages[key] = r
		}
	}
	if ValidateCrop(*c, 0) != nil {
		return nil
	}
	return c
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

// CropHidesMark is the ONE predicate (T198) that decides whether a mark is ENTIRELY outside a crop — so the
// bake drops it (markIntersectsCrop) and the crop-save warning lists it (CropHiddenMarks), by exactly the
// same rule. The mark is given by its POINTS (each {x,y}, page-relative [0,1], top-left origin), its stroke
// width (a page-WIDTH fraction) and font size (a page-HEIGHT fraction), and whether it is text (glyphs run
// RIGHT and DOWN from the top-left anchor). The crop rect is (cx,cy,cw,ch). It owns the points-bounding-box
// loop AND the empty-points case so the two callers cannot diverge: a mark with NO points is NEVER hidden
// (it has no extent to place off the page, and the bake keeps it), so the warning must not report it either.
// It is keep-biased: the box is padded by the stroke's round-cap radius and text is extended generously, so
// an over-estimate keeps a mark the renderer then clips, never hides one.
// PageViewTransform (T199) maps a source-frame normalized point (px,py) into the bake's VIEW frame for a
// crop: offset from the WINDOW centre, rotate by the deskew angle in PHYSICAL units (aspect = page W/H, so a
// non-square page doesn't shear), straighten, then crop-normalize to the window. The result is the point's
// position in the upright cropped page — [0,1]² inside the window. It is the ONE transform the bake drop
// (applyCropToDoc) and the crop dryRun (CropHidesMark) share, so they can never disagree about where a mark
// lands or whether it is hidden. At angle 0 it is EXACTLY T198's (px−x)/w, (py−y)/h, and aspect is unused. The
// sign matches bake.rotateWindow (which maps output→source by +angle), pinned by the overlay-dot test.
func PageViewTransform(px, py, x, y, w, h, angleDeg, aspect float64) (ox, oy float64) {
	cx, cy := x+w/2, y+h/2
	dx, dy := px-cx, py-cy
	if angleDeg == 0 {
		return 0.5 + dx/w, 0.5 + dy/h
	}
	th := angleDeg * math.Pi / 180
	sin, cos := math.Sin(th), math.Cos(th)
	ox = 0.5 + (cos*dx+sin*dy/aspect)/w
	oy = 0.5 + (-sin*dx*aspect+cos*dy)/h
	return ox, oy
}

func CropHidesMark(pts [][2]float64, strokeWidth, fontSize float64, isText bool, text string, x, y, w, h, angleDeg, aspect float64) bool {
	if len(pts) == 0 {
		return false // no extent; kept by the bake, so never reported hidden
	}
	x0, y0 := pts[0][0], pts[0][1]
	x1, y1 := x0, y0
	for _, p := range pts[1:] {
		if p[0] < x0 {
			x0 = p[0]
		}
		if p[0] > x1 {
			x1 = p[0]
		}
		if p[1] < y0 {
			y0 = p[1]
		}
		if p[1] > y1 {
			y1 = p[1]
		}
	}
	pad := strokeWidth / 2
	bx0, by0, bx1, by1 := x0-pad, y0-pad, x1+pad, y1+pad
	if isText && text != "" {
		widest := 0
		for _, ln := range strings.Split(text, "\n") {
			if n := len([]rune(ln)); n > widest {
				widest = n
			}
		}
		lines := strings.Count(text, "\n") + 1
		bx1 = math.Max(bx1, x0+float64(widest)*fontSize) // run right from the anchor
		by1 = math.Max(by1, y0+fontSize*1.2*float64(lines))
	}
	// T199: transform the source extent-box's four corners into the view frame and take their AABB, then test
	// against the upright window [0,1]². The AABB of the rotated box OVER-estimates, so the predicate stays
	// keep-biased (a boundary mark is kept, never hidden). At angle 0 this is identical to the T198 test.
	ox0, oy0 := math.Inf(1), math.Inf(1)
	ox1, oy1 := math.Inf(-1), math.Inf(-1)
	for _, c := range [4][2]float64{{bx0, by0}, {bx1, by0}, {bx1, by1}, {bx0, by1}} {
		tx, ty := PageViewTransform(c[0], c[1], x, y, w, h, angleDeg, aspect)
		ox0, ox1 = math.Min(ox0, tx), math.Max(ox1, tx)
		oy0, oy1 = math.Min(oy0, ty), math.Max(oy1, ty)
	}
	intersects := ox1 >= 0 && ox0 <= 1 && oy1 >= 0 && oy0 <= 1
	return !intersects
}
