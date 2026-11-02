package bake

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
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
		out[idx] = cropRect{x: r.X, y: r.Y, w: r.W, h: r.H, angle: r.Angle}
	}
	return out
}

// T198 — per-page bake-area (crop) rasterization. A crop renders ONLY a sub-rect of a page, FROM the PDF, at a
// higher effective DPI so the music fills the tablet at real detail (P2: never crop-then-upscale). It rides
// the context (like the cache mode) so the Rasterizer interface is unchanged: an uncropped file renders
// exactly as before, byte-for-byte (P1 — Reset is a byte-identical bake).

// cropRect is one page's bake area in the SOURCE page frame, normalized [0,1], origin top-left. angle (T199,
// degrees, + = counter-clockwise correction) rotates the window about its own centre; 0 ⇒ no rotation and the
// byte-identical T198 path.
type cropRect struct {
	x, y, w, h float64
	angle      float64
}

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

// deskewWindowSpills (T199, B1b) reports whether the rotated crop window reaches past the page edge, given the
// page's pixel size (for its aspect). The rotation is about the window centre in PHYSICAL units: a normalized
// offset (dx,dy) becomes (dx·W, dy·H) before rotating, so a non-square page doesn't shear. The server save
// path has no page dims (it can't run this), and Studio clamps the window client-side, so this fires only for
// a crop that reached the bake un-clamped — e.g. one imported past the dimension-free sanitizer. When it does,
// renderDeskewedCropPage clamps the render to the page and fills the rest with white; the caller warns so the
// clamp is never silent. angle 0 or a degenerate page ⇒ never spills.
func deskewWindowSpills(r cropRect, aspect float64) bool {
	if r.angle == 0 || aspect <= 0 {
		return false
	}
	cx, cy := r.x+r.w/2, r.y+r.h/2
	theta := r.angle * math.Pi / 180
	sin, cos := math.Sin(theta), math.Cos(theta)
	for _, s := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
		dx, dy := s[0]*r.w/2, s[1]*r.h/2
		nx := cx + dx*cos - dy*sin/aspect
		ny := cy + dx*aspect*sin + dy*cos
		if nx < -cropEpsBake || nx > 1+cropEpsBake || ny < -cropEpsBake || ny > 1+cropEpsBake {
			return true
		}
	}
	return false
}

// cropEpsBake mirrors app.cropEps: absorb a float round-trip so a window whose rotated corner lands exactly on
// the page edge isn't reported as spilling by a 1e-15 overshoot.
const cropEpsBake = 1e-6

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

// pagePreviewer renders ONE page of an uploaded PDF — whole, or cropped to a rect — for the T198 "Tablet
// view" preview (U1b). popplerRasterizer implements it; a test can inject a fake. It is page-at-a-time by
// design (pdftoppm -f N -l N), never the whole PDF.
type pagePreviewer interface {
	previewPage(ctx context.Context, pdf []byte, page int, crop *cropRect) ([]byte, error)
}

// previewPage renders page `page` (0-based) of pdf. With crop == nil it is the whole page at base DPI; with a
// crop it is the rect re-rendered from the PDF at the SAME effective DPI the bake uses (effectiveCropDPI +
// the native-ppi cap), so the preview raster matches what bakes. T169 greyscale applies as in the bake.
func (pr popplerRasterizer) previewPage(ctx context.Context, pdf []byte, page int, crop *cropRect) ([]byte, error) {
	dir, err := os.MkdirTemp("", "trouba-preview-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(in, pdf, 0o600); err != nil {
		return nil, err
	}
	base, err := pr.renderPageFull(ctx, in, page, pr.dpi)
	if err != nil {
		return nil, err
	}
	if crop == nil {
		return greyscaleIfMono(base), nil
	}
	cfg, _, derr := image.DecodeConfig(bytes.NewReader(base))
	if derr != nil {
		return nil, fmt.Errorf("decode page %d for preview crop: %w", page, derr)
	}
	native := pr.nativePPIForPage(ctx, in, page, cfg.Width, cfg.Height, pr.dpi)
	eff := effectiveCropDPI(pr.dpi, cfg.Width, cfg.Height, *crop, native)
	out, cerr := pr.renderCropPage(ctx, in, page, *crop, cfg.Width, cfg.Height, eff)
	if cerr != nil {
		return nil, cerr
	}
	return greyscaleIfMono(out), nil
}

// renderPageFull renders page idx (0-based) WHOLE at dpi → PNG. Mirrors renderCropPage's pdftoppm call with
// no crop window; the preview uses it to size a page and to show an uncropped page.
func (pr popplerRasterizer) renderPageFull(ctx context.Context, in string, idx, dpi int) ([]byte, error) {
	dir, err := os.MkdirTemp("", "trouba-page-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	prefix := filepath.Join(dir, "page")
	page := strconv.Itoa(idx + 1) // pdftoppm pages are 1-based
	cmd := exec.CommandContext(ctx, pr.bin, "-png", "-f", page, "-l", page, "-singlefile", "-r", strconv.Itoa(dpi), in, prefix)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftoppm page %d: %w: %s", idx, err, strings.TrimSpace(stderr.String()))
	}
	out, err := os.ReadFile(prefix + ".png")
	if err != nil {
		return nil, fmt.Errorf("page %d produced no output: %w", idx, err)
	}
	return out, nil
}

func (pr popplerRasterizer) renderCropPage(ctx context.Context, in string, idx int, r cropRect, baseW, baseH, effDPI int) ([]byte, error) {
	if r.angle != 0 {
		return pr.renderDeskewedCropPage(ctx, in, idx, r, baseW, baseH, effDPI) // T199
	}
	x, y, w, h := cropWindowPx(baseW, baseH, pr.dpi, effDPI, r)
	if w < 1 || h < 1 {
		return nil, fmt.Errorf("crop of page %d degenerates to %dx%d px", idx, w, h)
	}
	return pr.rasterizeRegion(ctx, in, idx, x, y, w, h, effDPI)
}

// rasterizeRegion renders ONE page's pixel sub-window (x,y + w×h, pixels at dpi) via pdftoppm -x -y -W -H,
// returning the PNG. Shared by the axis-aligned crop (angle 0) and the T199 deskew AABB render, so both drive
// pdftoppm identically — the angle-0 crop is byte-for-byte what T198 produced.
func (pr popplerRasterizer) rasterizeRegion(ctx context.Context, in string, idx, x, y, w, h, dpi int) ([]byte, error) {
	dir, err := os.MkdirTemp("", "trouba-crop-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	prefix := filepath.Join(dir, "crop")
	page := strconv.Itoa(idx + 1) // pdftoppm pages are 1-based
	cmd := exec.CommandContext(ctx, pr.bin,
		"-png", "-f", page, "-l", page, "-singlefile", "-r", strconv.Itoa(dpi),
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

// renderDeskewedCropPage (T199) bakes a page whose crop carries an angle: render the axis-aligned bounding box
// of the ROTATED window from the PDF at effDPI (plus a few px of kernel margin), rotate it straight with the
// bicubic resampler, and cut the exact upright window. All geometry is in page pixels at effDPI, the same
// frame cropWindowPx uses. A window whose rotated AABB reaches past the page is clamped to the page here (the
// resampler fills the uncovered area with paper-white); stageFile independently warns when that happens (B1b).
func (pr popplerRasterizer) renderDeskewedCropPage(ctx context.Context, in string, idx int, r cropRect, baseW, baseH, effDPI int) ([]byte, error) {
	scale := float64(effDPI) / float64(pr.dpi)
	fullW := math.Round(float64(baseW) * scale)
	fullH := math.Round(float64(baseH) * scale)
	outW := int(math.Round(r.w * fullW))
	outH := int(math.Round(r.h * fullH))
	if outW < 1 || outH < 1 {
		return nil, fmt.Errorf("crop of page %d degenerates to %dx%d px", idx, outW, outH)
	}
	// Window centre in page px, and the rotated window's AABB half-extents.
	cxF := (r.x + r.w/2) * fullW
	cyF := (r.y + r.h/2) * fullH
	theta := r.angle * math.Pi / 180
	sin, cos := math.Abs(math.Sin(theta)), math.Abs(math.Cos(theta))
	halfW := float64(outW)/2*cos + float64(outH)/2*sin
	halfH := float64(outW)/2*sin + float64(outH)/2*cos
	const margin = 3 // the 4×4 bicubic kernel reads ±2 px around the sample; a little slack on top
	ax := int(math.Floor(cxF - halfW - margin))
	ay := int(math.Floor(cyF - halfH - margin))
	bx := int(math.Ceil(cxF + halfW + margin))
	by := int(math.Ceil(cyF + halfH + margin))
	if ax < 0 {
		ax = 0
	}
	if ay < 0 {
		ay = 0
	}
	if bx > int(fullW) {
		bx = int(fullW)
	}
	if by > int(fullH) {
		by = int(fullH)
	}
	aw, ah := bx-ax, by-ay
	if aw < 1 || ah < 1 {
		return nil, fmt.Errorf("deskew of page %d degenerates to %dx%d px", idx, aw, ah)
	}
	aabbPNG, err := pr.rasterizeRegion(ctx, in, idx, ax, ay, aw, ah, effDPI)
	if err != nil {
		return nil, err
	}
	aabb, err := png.Decode(bytes.NewReader(aabbPNG))
	if err != nil {
		return nil, fmt.Errorf("decode deskew AABB of page %d: %w", idx, err)
	}
	// The AABB raster's (0,0) is page px (ax,ay), so the window centre in AABB coords is (cxF-ax, cyF-ay).
	up := rotateWindow(aabb, outW, outH, cxF-float64(ax), cyF-float64(ay), r.angle)
	var buf bytes.Buffer
	if err := png.Encode(&buf, up); err != nil {
		return nil, fmt.Errorf("encode deskew of page %d: %w", idx, err)
	}
	return buf.Bytes(), nil
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

// markIntersectsCrop reports whether a mark has any pixel inside the crop rect c. It defers to the ONE shared
// predicate app.CropHidesMark (extent-aware: stroke half-width pad + text glyph run, and the empty-points
// case) so the bake drop and the crop-save warning can never disagree about what a crop hides.
func markIntersectsCrop(o docObject, c cropRect, aspect float64) bool {
	pts := make([][2]float64, len(o.Points))
	for i, p := range o.Points {
		pts[i] = [2]float64{p.X, p.Y}
	}
	return !app.CropHidesMark(pts, o.Style.Width, o.Style.FontSize, o.Type == "text", o.Text, c.x, c.y, c.w, c.h, c.angle, aspect)
}

// applyCropToDoc transforms each mark on a cropped page into the CROPPED page frame, so it composites on the
// same music after the crop. The stored coordinates never change (P3) — this is a bake-time VIEW transform
// on the overlay doc only. A point maps x'=(x-cx)/cw, y'=(y-cy)/ch; the stroke width (a page-WIDTH fraction)
// scales by 1/cw and the text size (a page-HEIGHT fraction, matching web/ink's fontPx) by 1/ch. A mark that
// does not intersect the crop (markIntersectsCrop, extent-aware) is dropped — it isn't on the tablet; a
// straddling mark keeps its points and the renderer clips the overflow.
// applyCropToDoc transforms each mark into the cropped VIEW frame (T199: rotate-then-crop via the shared
// app.PageViewTransform), dropping those entirely outside the window. aspects maps a page index to its
// rendered aspect (W/H); only pages carrying an angle need an entry (angle-0 pages ignore it and reduce to
// T198's translate+scale, byte-identically). Stored coordinates never change (P3) — this is a bake-time view.
func applyCropToDoc(doc annotationsDoc, crops pageCrops, aspects map[int]float64) annotationsDoc {
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
		aspect := aspects[o.Page]
		if !markIntersectsCrop(o, c, aspect) {
			continue // entirely outside the cropped page — not drawn on the tablet
		}
		to := o
		to.Points = make([]docPoint, len(o.Points))
		for i, p := range o.Points {
			ox, oy := app.PageViewTransform(p.X, p.Y, c.x, c.y, c.w, c.h, c.angle, aspect)
			to.Points[i] = docPoint{X: ox, Y: oy, Pressure: p.Pressure}
		}
		to.Style = o.Style
		to.Style.Width = o.Style.Width / c.w // a page-width fraction; the rotation preserves it, the crop scales it
		to.Style.FontSize = o.Style.FontSize / c.h
		out = append(out, to)
	}
	doc.Objects = out
	return doc
}
