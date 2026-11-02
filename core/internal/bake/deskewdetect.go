package bake

import "math"

// T199 S3 — pure-Go deskew ANGLE autodetect. On the page's ink bitmap, project each dark pixel into the row it
// would occupy if the page were sheared to undo a rotation of angleDeg (row r = y − (x−cx)·tanθ), giving a
// horizontal projection profile; level staff/text lines pile ink into few rows, so the profile's variance
// peaks when θ matches the page's crookedness. Sweep coarse then fine over [−maxDeskewDeg, +maxDeskewDeg] and
// take the argmax. The angle returned is the one to deskew BY (the crop's angle), same sign convention as S1's
// rotateWindow (pinned by the +2.3° vector).
//
// A variance peak alone does NOT mean "a crooked page": a SINGLE strong line (inside ±10°) collapses into one
// row at its own angle and peaks as sharply as many staff lines. So CONFIDENCE requires SEVERAL separated
// peaks in the profile at the refined θ* — many aligned lines, which is what a page actually worth
// straightening has — not one tall peak. No ink ⇒ not confident (never a 0/0).

// deskewSweepStepCoarse/Fine are the coarse and fine angle steps (spec: 0.5° then 0.05°).
const (
	deskewDetectMaxDeg = 10.0 // sweep range ±, mirrors app.maxDeskewDeg (small-skew only)
	deskewSweepCoarse  = 0.5
	deskewSweepFine    = 0.05 // the 0.05° grid angles are rounded to (mirrors app.deskewStepDeg)
	// deskewDeadbandDeg: a detected angle this small is treated as straight (0) — a sub-0.1° tilt is
	// imperceptible and not worth re-rastering, and the fine sweep otherwise lands a true-0 page on ±0.05.
	deskewDeadbandDeg = 0.1
	// deskewMinPeaks is the multi-line confidence band: a page worth straightening shows at least this many
	// separated ink-row peaks at the detected angle. One strong line gives 1; blank gives 0. Measured gap
	// between staff/text pages (many) and a lone line (1) — see TestDeskewDetectVectors.
	deskewMinPeaks = 4
)

// rowProfile projects the grid's ink into sheared rows for a candidate angle and returns the per-row ink
// counts. The profile is padded so a shear never drops edge ink.
func rowProfile(g grid, angleDeg float64) []float64 {
	tan := math.Tan(angleDeg * math.Pi / 180)
	cx := float64(g.w) / 2
	pad := int(math.Ceil(float64(g.w)/2*math.Abs(tan))) + 1
	prof := make([]float64, g.h+2*pad)
	for y := 0; y < g.h; y++ {
		base := y * g.w
		for x := 0; x < g.w; x++ {
			if g.ink[base+x] {
				r := y - int(math.Round((float64(x)-cx)*tan)) + pad
				if r >= 0 && r < len(prof) {
					prof[r]++
				}
			}
		}
	}
	return prof
}

func variance(prof []float64) float64 {
	if len(prof) == 0 {
		return 0
	}
	var sum, sum2 float64
	for _, v := range prof {
		sum += v
		sum2 += v * v
	}
	n := float64(len(prof))
	mean := sum / n
	return sum2/n - mean*mean
}

// countSeparatedPeaks counts the runs of rows whose ink exceeds a prominence threshold (a fraction of the
// profile's max), separated by rows below it — i.e. how many DISTINCT aligned lines the profile holds. Staff
// and text pages give many; a single line gives one; blank gives zero.
func countSeparatedPeaks(prof []float64) int {
	max := 0.0
	for _, v := range prof {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		return 0
	}
	thresh := max * 0.5 // a row counts as "a line" when it holds at least half the strongest row's ink
	peaks, in := 0, false
	for _, v := range prof {
		if v >= thresh {
			if !in {
				peaks++
				in = true
			}
		} else {
			in = false
		}
	}
	return peaks
}

// detectPageAngle returns the deskew angle for a page and whether it is confident enough to apply. It is pure
// (operates on the decoded ink grid) so it is unit-tested directly against synthetic vectors.
func detectPageAngle(g grid) (angleDeg float64, confident bool) {
	if g.w <= 0 || g.h <= 0 {
		return 0, false
	}
	ink := 0
	for _, b := range g.ink {
		if b {
			ink++
		}
	}
	if ink == 0 {
		return 0, false // no ink ⇒ nothing to straighten; never a 0/0 (S3-4)
	}
	best := func(lo, hi, step float64) float64 {
		bestV, bestA := -1.0, 0.0
		for a := lo; a <= hi+1e-9; a += step {
			if v := variance(rowProfile(g, a)); v > bestV {
				bestV, bestA = v, a
			}
		}
		return bestA
	}
	coarse := best(-deskewDetectMaxDeg, deskewDetectMaxDeg, deskewSweepCoarse)
	// Refine on the fine grid around the coarse pick (±one coarse step), clamped to the range (S3-3).
	lo := math.Max(-deskewDetectMaxDeg, coarse-deskewSweepCoarse)
	hi := math.Min(deskewDetectMaxDeg, coarse+deskewSweepCoarse)
	fine := best(lo, hi, deskewSweepFine)
	fine = roundDeskewAngle(fine)
	if math.Abs(fine) < deskewDeadbandDeg {
		return 0, false // effectively straight ⇒ propose no deskew
	}
	// Confidence at the REFINED angle (S3-1, S3-3): several separated line-peaks, not one tall peak.
	peaks := countSeparatedPeaks(rowProfile(g, fine))
	return fine, peaks >= deskewMinPeaks
}

// roundDeskewAngle snaps to the 0.05° grid (mirrors app.roundDeskew, kept local so the bake package has no
// extra import; the service rounds identically on save/import).
func roundDeskewAngle(a float64) float64 {
	if a == 0 {
		return 0
	}
	return math.Round(a/deskewSweepFine) * deskewSweepFine
}
