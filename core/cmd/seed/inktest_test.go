package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// B13 §5.B.1 — ink-under-mark raster test for the ENGRAVED pages (where no text extraction can
// place music glyphs, so placement is proved on pixels). Rasterize each annotated page and
// check: every "cover" mark (highlighter swipe / ring) sits over real print, and every "clear"
// mark (text label / icon stamp) sits in genuinely empty space (≤ maxClearDark dark). Skips if
// pdftoppm is unavailable.
//
// Cover marks are held to a PER-PLACEMENT golden (docs/demo-charts/ink-golden.json): each mark's
// calibrated ink fraction is recorded once, and a run fails if the mark retains < goldenRatio of
// it — real regression teeth everywhere (a global floor set by the sparsest target, e.g. a
// monospace "D    F" chord cell at ~1.4%, is toothless for the dense marks). Regenerate the
// golden after INTENTIONAL placement changes with TROUBA_INK_GOLDEN=1 (and review the diff).
//
// maxClearDark is a constant — do NOT loosen to pass (the reproduce-before-fixing rule): a clear
// failure means the label/icon is over print, not that the test is wrong.
const (
	inkDPI       = 150
	darkLevel    = 140   // 8-bit luminance below this counts as ink
	maxClearDark = 0.015 // a label/icon must sit in genuinely clear space (spec §5.B.1 constant)
	goldenRatio  = 0.6   // a cover mark must retain ≥60% of its calibrated ink or it has drifted
	goldenFloor  = 0.008 // regen refuses to bless a cover mark below this — it looks misplaced
)

func inkGoldenPath() string {
	return filepath.Join(filepath.Dir(anchorsPath("open-road-leadsheet")), "ink-golden.json")
}

func byUUID(im annotationsImport, uuid string) wireObject {
	for _, o := range im.Objects {
		if o.UUID == uuid {
			return o
		}
	}
	return wireObject{}
}

func rasterPage(t *testing.T, pdf string, page int) image.Image {
	t.Helper()
	dir := t.TempDir()
	base := filepath.Join(dir, "p")
	cmd := exec.Command("pdftoppm", "-png", "-r", strconv.Itoa(inkDPI),
		"-f", strconv.Itoa(page+1), "-l", strconv.Itoa(page+1), pdf, base)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pdftoppm %s p%d: %v\n%s", pdf, page, err, out)
	}
	matches, _ := filepath.Glob(base + "-*.png")
	if len(matches) == 0 {
		t.Fatalf("pdftoppm produced no png for %s p%d", pdf, page)
	}
	f, err := os.Open(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// darkFrac returns the fraction of dark (ink) pixels within box b (in [0,1]² coords).
func darkFrac(img image.Image, b anchorBox) float64 {
	bnd := img.Bounds()
	W, H := bnd.Dx(), bnd.Dy()
	x0, y0 := bnd.Min.X+int(b.X0*float64(W)), bnd.Min.Y+int(b.Y0*float64(H))
	x1, y1 := bnd.Min.X+int(b.X1*float64(W)), bnd.Min.Y+int(b.Y1*float64(H))
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	dark, total := 0, 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			lum := (299*(r>>8) + 587*(g>>8) + 114*(bl>>8)) / 1000
			if lum < darkLevel {
				dark++
			}
			total++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(dark) / float64(total)
}

func TestInkUnderMark_engraved(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm not available")
	}
	u := testUsers()
	charts := []struct {
		base string
		im   annotationsImport
	}{
		{"open-road-leadsheet", buildOpenRoadAnnotations("s0", "f0", u, "u-leo", mustAnchors("open-road-leadsheet"))},
		{"open-road-guitar", buildOpenRoadGuitarAnnotations("s1", "f1", u, "u-leo", mustAnchors("open-road-guitar"))},
		{"house-rising-sun-tab", buildBandChartAnnotations("s3", "f3", "House of the Rising Sun", u, "u-leo", mustAnchors("house-rising-sun-tab"))},
		{"amazing-grace", buildBandChartAnnotations("s4a", "f4a", "Amazing Grace", u, "u-leo", mustAnchors("amazing-grace"))},
		{"greensleeves", buildBandChartAnnotations("s5", "f5", "Greensleeves", u, "u-leo", mustAnchors("greensleeves"))},
		{"ek-violin1", buildEineKleineAnnotations("s6", "f6", u, "u-flora", mustAnchors("ek-violin1"))},
		{"canon-violin1", buildCanonAnnotations("s7", "f7", u, "u-flora", mustAnchors("canon-violin1"))},
		{"ek-score", buildScoreConductorAnnotations("s8", "f8", "ek", "u-flora", mustAnchors("ek-score"))},
		{"canon-score", buildScoreConductorAnnotations("s9", "f9", "canon", "u-flora", mustAnchors("canon-score"))},
		{"house-rising-sun-drums", buildDrumPartAnnotations("s10", "f10", mustAnchors("house-rising-sun-drums"))},
	}
	regen := os.Getenv("TROUBA_INK_GOLDEN") == "1"
	golden := map[string]float64{}
	if !regen {
		raw, err := os.ReadFile(inkGoldenPath())
		if err != nil {
			t.Fatalf("read ink golden (regenerate with TROUBA_INK_GOLDEN=1): %v", err)
		}
		if err := json.Unmarshal(raw, &golden); err != nil {
			t.Fatalf("parse %s: %v", inkGoldenPath(), err)
		}
	}
	measured := map[string]float64{}
	for _, c := range charts {
		pdf := strings.TrimSuffix(anchorsPath(c.base), ".anchors.json") + ".pdf"
		rasters := map[int]image.Image{}
		for _, pl := range c.im.placements {
			img := rasters[pl.page]
			if img == nil {
				img = rasterPage(t, pdf, pl.page)
				rasters[pl.page] = img
			}
			frac := darkFrac(img, pl.tgt)
			o := byUUID(c.im, pl.uuid)
			switch pl.kind {
			case "cover":
				measured[pl.uuid] = frac
				if regen {
					if frac < goldenFloor {
						t.Errorf("REGEN refused: %s %s [%s] p%d only %.1f%% ink — mark looks misplaced, fix before recording golden",
							c.base, pl.uuid, o.Type, pl.page, frac*100)
					}
					continue
				}
				g, ok := golden[pl.uuid]
				if !ok {
					t.Errorf("%s %s [%s]: no ink-golden entry — regenerate with TROUBA_INK_GOLDEN=1", c.base, pl.uuid, o.Type)
					continue
				}
				if frac < goldenRatio*g {
					t.Errorf("%s %s [%s] p%d: cover mark %.1f%% ink < %.0f%% of golden %.1f%% — drifted off print",
						c.base, pl.uuid, o.Type, pl.page, frac*100, goldenRatio*100, g*100)
				}
			case "clear":
				if frac > maxClearDark {
					t.Errorf("%s %s [%s %q] p%d: label/icon %.1f%% ink — not in clear space (want ≤%.1f%%)",
						c.base, pl.uuid, o.Type, o.Text, pl.page, frac*100, maxClearDark*100)
				}
			}
		}
	}
	if regen && !t.Failed() {
		keys := make([]string, 0, len(measured))
		for k := range measured {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var buf bytes.Buffer
		buf.WriteString("{\n")
		for i, k := range keys {
			fmt.Fprintf(&buf, "  %q: %.5f", k, measured[k])
			if i < len(keys)-1 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		buf.WriteString("}\n")
		if err := os.WriteFile(inkGoldenPath(), buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d cover-mark golden fractions to %s", len(measured), inkGoldenPath())
	}
}
