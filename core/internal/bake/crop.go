package bake

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"troubastack/core/internal/app"
)

// pageCropsFor builds the rasterizer's per-page crop input from an uploaded PDF's crop metadata. A Generated
// chart is never cropped (P4), and an absent/empty crop yields nil — the whole-page fast path, byte-identical
// to a bake with no crop at all.
func pageCropsFor(file app.SongFile) pageCrops {
	if file.Generated || file.Crop == nil || len(file.Crop.Pages) == 0 {
		return nil
	}
	out := make(pageCrops, len(file.Crop.Pages))
	for key, r := range file.Crop.Pages {
		idx, err := strconv.Atoi(key)
		if err != nil || idx < 0 {
			continue
		}
		out[idx] = cropRect{x: r.X, y: r.Y, w: r.W, h: r.H}
	}
	return out
}

// T198 — per-page bake-area (crop) rasterization. A crop renders ONLY a sub-rect of a page, FROM the PDF, at a
// higher effective DPI so the music fills the tablet at real detail (P2: never crop-then-upscale). It rides
// the context (like the cache mode) so the Rasterizer interface is unchanged: an uncropped file renders
// exactly as before, byte-for-byte (P1 — Reset is a byte-identical bake).

// cropRect is one page's bake area in the SOURCE page frame, normalized [0,1], origin top-left.
type cropRect struct{ x, y, w, h float64 }

// pageCrops maps a 0-based page index to its bake area. A page absent from the map renders whole.
type pageCrops map[int]cropRect

type pageCropsKey struct{}

// withPageCrops attaches a file's per-page crops to ctx for the next Rasterize call. Empty ⇒ unchanged ctx,
// so the whole-page fast path and its byte output are untouched.
func withPageCrops(ctx context.Context, c pageCrops) context.Context {
	if len(c) == 0 {
		return ctx
	}
	return context.WithValue(ctx, pageCropsKey{}, c)
}

func pageCropsFromContext(ctx context.Context) pageCrops {
	if c, ok := ctx.Value(pageCropsKey{}).(pageCrops); ok {
		return c
	}
	return nil
}

// maxCropSidePx bounds a cropped page's longest side, so a very small crop (large dpi_eff) can't produce a
// giant raster. Applies to vector pages (no native-ppi cap) and as a backstop everywhere.
const maxCropSidePx = 4096

// effectiveCropDPI computes the DPI to render a crop at: bring the cropped region to ≈ the pixel width the
// WHOLE page gets at baseDPI (dpi_eff = baseDPI / w). Then two caps and a FLOOR:
//   - cap at the page's native image ppi, but ONLY when a scan image actually bounds the crop's detail
//     (nativePPI > 0 means a page-covering scan image was found; a vector page or a page with only small
//     embedded images passes nativePPI ≤ 0 and is NOT capped);
//   - FLOOR at baseDPI — a cropped page must NEVER get fewer pixels per source inch than the uncropped bake
//     (so a low-ppi scan, or a stray low-ppi image, cannot starve the render below today's quality);
//   - cap the longest cropped side at maxCropSidePx so a tiny crop can't produce a giant raster.
//
// baseW/baseH are the page's pixels at baseDPI.
func effectiveCropDPI(baseDPI, baseW, baseH int, r cropRect, nativePPI int) int {
	if r.w <= 0 || r.h <= 0 {
		return baseDPI
	}
	dpi := float64(baseDPI) / r.w
	if nativePPI > 0 && float64(nativePPI) < dpi {
		dpi = float64(nativePPI)
	}
	if dpi < float64(baseDPI) { // never coarser than the whole-page bake
		dpi = float64(baseDPI)
	}
	// Pixel budget. Cropped side in px at dpi = (page side px at baseDPI) · (dpi/baseDPI) · (normalized side).
	scale := dpi / float64(baseDPI)
	cw := float64(baseW) * scale * r.w
	ch := float64(baseH) * scale * r.h
	if longest := math.Max(cw, ch); longest > maxCropSidePx {
		dpi *= maxCropSidePx / longest
	}
	// Re-apply the floor AFTER the budget: on an A0-class page the budget reduction could otherwise drop
	// below the base dpi, which the top-of-function promise forbids. A crop at base dpi is never larger than
	// the whole page at base dpi (which is already baked today), so keeping base is safe — that one page's
	// crop simply isn't shrunk to the 4096 budget.
	if dpi < float64(baseDPI) {
		dpi = float64(baseDPI)
	}
	if dpi < 1 {
		dpi = 1
	}
	// Floor (not round) so the pixel budget is never exceeded by a rounding-up; the sub-DPI loss is
	// imperceptible and the cropped region is still ≈ a full page wide.
	return int(dpi)
}

// renderCropPage renders page idx (0-based) of in.pdf cropped to r, at effDPI, returning the PNG bytes. The
// crop pixel rect is derived from the page's base pixels (baseW/baseH at r.baseDPI) scaled to effDPI — no
// pdfinfo call needed. pdftoppm's -x/-y/-W/-H are pixels at the -r resolution.
// cropWindowPx is the pdftoppm crop window (-x -y -W -H, pixels at effDPI) for rect r on a page whose base
// pixels are baseW×baseH at baseDPI. Pure, so the render path and the P2 test compute the SAME window.
func cropWindowPx(baseW, baseH, baseDPI, effDPI int, r cropRect) (x, y, w, h int) {
	scale := float64(effDPI) / float64(baseDPI)
	fullW := math.Round(float64(baseW) * scale)
	fullH := math.Round(float64(baseH) * scale)
	x = int(math.Round(r.x * fullW))
	y = int(math.Round(r.y * fullH))
	w = int(math.Round(r.w * fullW))
	h = int(math.Round(r.h * fullH))
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+w > int(fullW) { // rounding can push the window 1px past the edge
		w = int(fullW) - x
	}
	if y+h > int(fullH) {
		h = int(fullH) - y
	}
	return x, y, w, h
}

func (pr popplerRasterizer) renderCropPage(ctx context.Context, in string, idx int, r cropRect, baseW, baseH, effDPI int) ([]byte, error) {
	x, y, w, h := cropWindowPx(baseW, baseH, pr.dpi, effDPI, r)
	if w < 1 || h < 1 {
		return nil, fmt.Errorf("crop of page %d degenerates to %dx%d px", idx, w, h)
	}

	dir, err := os.MkdirTemp("", "trouba-crop-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	prefix := filepath.Join(dir, "crop")
	page := strconv.Itoa(idx + 1) // pdftoppm pages are 1-based
	cmd := exec.CommandContext(ctx, pr.bin,
		"-png", "-f", page, "-l", page, "-singlefile", "-r", strconv.Itoa(effDPI),
		"-x", strconv.Itoa(x), "-y", strconv.Itoa(y), "-W", strconv.Itoa(w), "-H", strconv.Itoa(h),
		in, prefix)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftoppm crop (page %d): %w: %s", idx, err, strings.TrimSpace(stderr.String()))
	}
	out, err := os.ReadFile(prefix + ".png")
	if err != nil {
		return nil, fmt.Errorf("crop of page %d produced no output: %w", idx, err)
	}
	return out, nil
}

// nativePPIForPage reads the native resolution (ppi) of a scan image that COVERS page idx (0-based), via
// `pdfimages -list`, for the native-ppi cap (P2). It returns the ppi only of an image whose physical size is
// ≈ the page's — a full-page scan, which is what actually bounds the page's detail. A small embedded image (a
// logo on a vector page) is ignored, so it can never cap the crop at its own tiny ppi. Returns 0 (no cap) for
// a vector page, a logo-only page, or when pdfimages is unavailable/unparseable (degrade to no cap, never
// fail the bake). baseW/baseH are the page's pixels at baseDPI, which give its physical size. pdfimages is in
// poppler-utils, the same package as pdftoppm.
func (pr popplerRasterizer) nativePPIForPage(ctx context.Context, in string, idx, baseW, baseH, baseDPI int) int {
	if pr.imagesBin == "" || baseDPI <= 0 {
		return 0
	}
	page := strconv.Itoa(idx + 1)
	cmd := exec.CommandContext(ctx, pr.imagesBin, "-list", "-f", page, "-l", page, in)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0
	}
	pageWin := float64(baseW) / float64(baseDPI)
	pageHin := float64(baseH) / float64(baseDPI)
	return coveringPPI(out.String(), pageWin, pageHin)
}

// coverFrac is how much of the page (each dimension) an image must span to count as "covering" — and so to
// bound the crop's native detail. 0.9 admits a scan with a small white border; a logo is far below it.
const coverFrac = 0.9

// coveringPPI parses `pdfimages -list` and returns the max ppi over images that COVER the page (physical size
// ≥ coverFrac of the page in both dimensions). pageWin/pageHin are the page's physical size in inches. An
// image's physical size = width_px / x-ppi (inches). Columns are located by header name so a poppler column
// shift doesn't misread. Returns 0 when no image covers the page (a vector or logo-only page — no cap).
func coveringPPI(listing string, pageWin, pageHin float64) int {
	sc := bufio.NewScanner(strings.NewReader(listing))
	wi, hi, xi, yi := -1, -1, -1, -1
	best := 0
	row := 0
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		row++
		if row == 1 { // header row names the columns
			for i, f := range fields {
				switch f {
				case "width":
					wi = i
				case "height":
					hi = i
				case "x-ppi":
					xi = i
				case "y-ppi":
					yi = i
				}
			}
			continue
		}
		if strings.HasPrefix(fields[0], "---") { // separator row poppler prints under the header
			continue
		}
		wpx := fieldFloat(fields, wi)
		hpx := fieldFloat(fields, hi)
		xppi := fieldFloat(fields, xi)
		yppi := fieldFloat(fields, yi)
		if xppi <= 0 || yppi <= 0 || wpx <= 0 || hpx <= 0 {
			continue
		}
		imgWin := wpx / xppi
		imgHin := hpx / yppi
		if imgWin < coverFrac*pageWin || imgHin < coverFrac*pageHin {
			continue // not a page-covering image — a logo or a thumbnail, never caps the crop
		}
		if p := int(math.Max(xppi, yppi)); p > best {
			best = p
		}
	}
	return best
}

func fieldFloat(fields []string, i int) float64 {
	if i < 0 || i >= len(fields) {
		return 0
	}
	v, err := strconv.ParseFloat(fields[i], 64)
	if err != nil {
		return 0
	}
	return v
}

// applyCropToDoc transforms each mark on a cropped page into the CROPPED page frame, so it composites on the
// same music after the crop. The stored coordinates never change (P3) — this is a bake-time VIEW transform
// on the overlay doc only. A point maps x'=(x-cx)/cw, y'=(y-cy)/ch; the stroke width (a page-WIDTH fraction)
// scales by 1/cw and the text size (a page-HEIGHT fraction, matching web/ink's fontPx) by 1/ch. A mark whose
// transformed box lands entirely outside the crop is dropped — it isn't on the tablet; a straddling mark
// keeps its points and the renderer clips the overflow.
func applyCropToDoc(doc annotationsDoc, crops pageCrops) annotationsDoc {
	if len(crops) == 0 {
		return doc
	}
	out := make([]docObject, 0, len(doc.Objects))
	for _, o := range doc.Objects {
		c, cropped := crops[o.Page]
		if !cropped || c.w <= 0 || c.h <= 0 {
			out = append(out, o)
			continue
		}
		to := o
		to.Points = make([]docPoint, len(o.Points))
		minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for i, p := range o.Points {
			nx := (p.X - c.x) / c.w
			ny := (p.Y - c.y) / c.h
			to.Points[i] = docPoint{X: nx, Y: ny, Pressure: p.Pressure}
			minX, maxX = math.Min(minX, nx), math.Max(maxX, nx)
			minY, maxY = math.Min(minY, ny), math.Max(maxY, ny)
		}
		if len(o.Points) > 0 && (maxX < 0 || minX > 1 || maxY < 0 || minY > 1) {
			continue // entirely outside the cropped page — not drawn on the tablet
		}
		to.Style = o.Style
		to.Style.Width = o.Style.Width / c.w
		to.Style.FontSize = o.Style.FontSize / c.h
		out = append(out, to)
	}
	doc.Objects = out
	return doc
}
