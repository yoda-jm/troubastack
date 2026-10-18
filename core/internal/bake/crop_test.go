package bake

import (
	"bytes"
	"context"
	"image"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"troubastack/core/internal/app"
)

// contentPDF is a 200×280pt page with a filled black rectangle covering the CENTRE HALF (frame x 0.25–0.75,
// y 0.25–0.75). So the whole page is ~25% dark, but a crop to the centre half is ~100% dark — which is how a
// crop test proves it rendered the RIGHT region (not just the right size).
const contentPDF = "%PDF-1.4\n" +
	"1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
	"2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
	"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 280]/Contents 4 0 R>>endobj\n" +
	"4 0 obj<</Length 19>>stream\n50 70 100 140 re f\nendstream endobj\n" +
	"trailer<</Root 1 0 R>>\n%%EOF\n"

func darkFraction(t *testing.T, png []byte) float64 {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(png))
	if err != nil {
		t.Fatalf("decode raster: %v", err)
	}
	b := img.Bounds()
	dark, total := 0, 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if (r+g+bl)/3 < 0x8000 {
				dark++
			}
			total++
		}
	}
	return float64(dark) / float64(total)
}

func dims(t *testing.T, png []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(png))
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	return cfg.Width, cfg.Height
}

// TestCropRendersTheRightRegionAtHigherDPI is the P2 integration test on the real pdftoppm: the whole page is
// a quarter dark, but a crop to the centre half fills the frame (≈all dark), at ≈ the same pixel width the
// whole page gets today — real detail from the PDF, not an upscale.
func TestCropRendersTheRightRegionAtHigherDPI(t *testing.T) {
	r := popplerRasterizer{bin: "pdftoppm", imagesBin: "pdfimages", dpi: 72}
	pdf := []byte(contentPDF)

	// Whole page.
	full, err := r.Rasterize(context.Background(), pdf)
	if err != nil {
		t.Fatalf("whole-page rasterize: %v", err)
	}
	if len(full) != 1 {
		t.Fatalf("want 1 page, got %d", len(full))
	}
	fw, fh := dims(t, full[0])
	if fw != 200 || fh != 280 {
		t.Fatalf("whole page dims = %dx%d, want 200x280", fw, fh)
	}
	if df := darkFraction(t, full[0]); df < 0.2 || df > 0.3 {
		t.Fatalf("whole page dark fraction = %.2f, want ~0.25 (the centre-half rect)", df)
	}

	// Crop to the centre half.
	ctx := withPageCrops(context.Background(), pageCrops{0: {x: 0.25, y: 0.25, w: 0.5, h: 0.5}})
	cropped, err := r.Rasterize(ctx, pdf)
	if err != nil {
		t.Fatalf("cropped rasterize: %v", err)
	}
	cw, ch := dims(t, cropped[0])
	// dpi_eff = 72/0.5 = 144 ⇒ the cropped region comes out ≈ the whole-page width (200px), not scaled down.
	if cw < 180 || cw > 220 {
		t.Fatalf("cropped width = %d, want ≈200 (crop region at dpi_eff)", cw)
	}
	_ = ch
	if df := darkFraction(t, cropped[0]); df < 0.95 {
		t.Fatalf("cropped dark fraction = %.2f, want ≈1.0 (the crop is inside the black centre)", df)
	}
}

// TestNoCropIsUnchangedAndDeterministic pins P1: a bake with no crop renders the whole page, and the same
// input renders byte-for-byte the same — so removing a crop (Reset) returns the exact uncropped bake.
func TestNoCropIsUnchangedAndDeterministic(t *testing.T) {
	r := popplerRasterizer{bin: "pdftoppm", imagesBin: "pdfimages", dpi: 72}
	pdf := []byte(contentPDF)
	a, err := r.Rasterize(context.Background(), pdf)
	if err != nil {
		t.Fatal(err)
	}
	// withPageCrops with an empty map is a no-op on the context ⇒ the whole-page path, identical bytes.
	b, err := r.Rasterize(withPageCrops(context.Background(), nil), pdf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a[0], b[0]) {
		t.Fatal("an uncropped render is not byte-identical to itself — Reset would not be byte-stable")
	}
}

func TestEffectiveCropDPI(t *testing.T) {
	// dpi_eff = baseDPI / w, no cap.
	if got := effectiveCropDPI(150, 1000, 1400, cropRect{w: 0.5, h: 0.5}, 0); got != 300 {
		t.Errorf("dpi_eff for w=0.5 = %d, want 300 (150/0.5)", got)
	}
	// native-ppi cap: a 300-ppi scan is not rendered above 300 even though 150/0.4 = 375.
	if got := effectiveCropDPI(150, 1000, 1400, cropRect{w: 0.4, h: 0.4}, 300); got != 300 {
		t.Errorf("native cap not applied: got %d, want 300", got)
	}
	// FLOOR (the NO-GO fix): a native ppi BELOW the base dpi must NOT make the crop coarser than today's
	// whole-page bake. Old code returned 100 here. This vector fails on the unfixed code.
	if got := effectiveCropDPI(150, 1000, 1400, cropRect{w: 0.8, h: 0.8}, 100); got != 150 {
		t.Errorf("sub-base native ppi: got %d, want 150 (floored at base dpi)", got)
	}
	// pixel budget: a tall thin crop (w small, h large) would exceed 4096px tall ⇒ dpi reduced to fit.
	got := effectiveCropDPI(150, 1000, 1400, cropRect{w: 0.05, h: 0.9}, 0)
	scale := float64(got) / 150.0
	ch := 1400 * scale * 0.9
	if ch > maxCropSidePx+1 {
		t.Errorf("pixel budget not enforced: cropped height ≈ %.0fpx exceeds %d (dpi=%d)", ch, maxCropSidePx, got)
	}
}

func TestCoveringPPI(t *testing.T) {
	// An A4 page at 150 dpi is 1240×1754 px ⇒ 8.27 × 11.69 inches.
	const pageW, pageH = 1240.0 / 150.0, 1754.0 / 150.0
	// A full-page scan image (physical size ≈ the page) bounds the detail ⇒ its ppi caps.
	fullPage := "page num type width height color comp bpc enc interp object ID x-ppi y-ppi size ratio\n" +
		"----\n   1 0 image 1240 1754 gray 1 8 image no 7 0 150 150 — —\n"
	if got := coveringPPI(fullPage, pageW, pageH); got != 150 {
		t.Errorf("full-page scan: coveringPPI = %d, want 150", got)
	}
	// A SMALL logo on a vector page (1in × 0.5in) does NOT cover the page ⇒ no cap (the NO-GO case: the old
	// code capped the whole page at this image's ppi). This vector fails on the unfixed code.
	logo := "page num type width height color comp bpc enc interp object ID x-ppi y-ppi size ratio\n" +
		"----\n   1 0 image 300 150 rgb 3 8 jpeg no 9 0 300 300 — —\n"
	if got := coveringPPI(logo, pageW, pageH); got != 0 {
		t.Errorf("small logo: coveringPPI = %d, want 0 (not a page-covering scan)", got)
	}
	// A vector page with no images ⇒ 0.
	if got := coveringPPI("page num ... x-ppi y-ppi\n----\n", pageW, pageH); got != 0 {
		t.Errorf("vector page: coveringPPI = %d, want 0", got)
	}
}

// directCrop renders a crop window the "correct" (P2) way — straight from the PDF with pdftoppm's
// -x/-y/-W/-H at effDPI — so a test can assert the production path produces exactly this, never a
// crop-then-upscale of the low-dpi raster.
func directCrop(t *testing.T, pdf []byte, effDPI, x, y, w, h int) []byte {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(in, pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(dir, "d")
	cmd := exec.CommandContext(context.Background(), "pdftoppm",
		"-png", "-f", "1", "-l", "1", "-singlefile", "-r", strconv.Itoa(effDPI),
		"-x", strconv.Itoa(x), "-y", strconv.Itoa(y), "-W", strconv.Itoa(w), "-H", strconv.Itoa(h), in, prefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("direct pdftoppm: %v: %s", err, out)
	}
	b, err := os.ReadFile(prefix + ".png")
	if err != nil {
		t.Fatal(err)
	}
	return greyscaleIfMono(b)
}

// TestCropOffCentreMatchesDirectRender is the P2 guard (NO-GO fix): an OFF-CENTRE, NON-SQUARE crop is
// rendered FROM the PDF at the effective DPI, byte-for-byte the same as a direct pdftoppm -r eff -x -y -W -H
// render — so a crop-then-upscale of the 72-dpi raster, or an x/y swap, would diverge and fail here.
func TestCropOffCentreMatchesDirectRender(t *testing.T) {
	r := popplerRasterizer{bin: "pdftoppm", imagesBin: "pdfimages", dpi: 72}
	pdf := []byte(contentPDF)
	full, err := r.Rasterize(context.Background(), pdf)
	if err != nil {
		t.Fatal(err)
	}
	bw, bh := dims(t, full[0])

	rect := cropRect{x: 0.4, y: 0.1, w: 0.5, h: 0.35} // off-centre AND non-square, straddling the rect edges
	eff := effectiveCropDPI(72, bw, bh, rect, 0)      // vector page ⇒ no native cap

	got, err := r.Rasterize(withPageCrops(context.Background(), pageCrops{0: rect}), pdf)
	if err != nil {
		t.Fatal(err)
	}
	x, y, w, h := cropWindowPx(bw, bh, 72, eff, rect)
	ref := directCrop(t, pdf, eff, x, y, w, h)
	if !bytes.Equal(got[0], ref) {
		t.Fatalf("cropped page is not the direct pdftoppm -r %d -x %d -y %d -W %d -H %d render — P2 (render from PDF, not upscale) broken", eff, x, y, w, h)
	}
	if gw, gh := dims(t, got[0]); gw == gh {
		t.Errorf("expected a NON-square crop, got %dx%d", gw, gh)
	}

	// ORACLE independent of cropWindowPx (Fable's GO condition): the expected dark fraction is derived from
	// the fixture's LITERAL geometry, so an x/y swap INSIDE cropWindowPx — which the byte-compare above misses
	// (it uses the same helper for its reference) — changes the captured region and reddens this.
	// Fixture black rect: frame x,y ∈ [0.25,0.75]. Crop {0.4,0.1,0.5,0.35} ⇒ inside the crop frame the rect
	// occupies x ∈ [0, 0.70] and y ∈ [(0.25-0.1)/0.35, 1] = [0.4286, 1]. Dark fraction = 0.70 × (1-0.4286) ≈ 0.40.
	if df := darkFraction(t, got[0]); df < 0.35 || df > 0.45 {
		t.Fatalf("off-centre crop dark fraction = %.3f, want ≈0.40 from the fixture geometry — the crop captured the wrong region (e.g. an x/y swap)", df)
	}
}

func TestRasterKeyIncludesCrop(t *testing.T) {
	pdf := []byte("%PDF-1.4 fixture")
	base := rasterKey(pdf, 150, "poppler-x", nil)
	c1 := rasterKey(pdf, 150, "poppler-x", pageCrops{0: {0.1, 0.1, 0.8, 0.8}})
	c2 := rasterKey(pdf, 150, "poppler-x", pageCrops{0: {0.2, 0.1, 0.8, 0.8}})
	if base == c1 {
		t.Error("a crop must change the raster cache key (else a crop serves stale cached pixels)")
	}
	if c1 == c2 {
		t.Error("different crops must produce different keys")
	}
	// Deterministic: same crop, same key.
	if c1 != rasterKey(pdf, 150, "poppler-x", pageCrops{0: {0.1, 0.1, 0.8, 0.8}}) {
		t.Error("the same crop must hash to the same key")
	}
}

func TestPageCropsFor(t *testing.T) {
	// A Generated chart is never cropped (P4).
	if got := pageCropsFor(app.SongFile{Generated: true, Crop: &app.Crop{Pages: map[string]app.CropRect{"0": {X: 0.1, W: 0.5, Y: 0.1, H: 0.5}}}}); got != nil {
		t.Errorf("generated chart must not crop, got %v", got)
	}
	// No crop ⇒ nil (whole-page fast path).
	if got := pageCropsFor(app.SongFile{}); got != nil {
		t.Errorf("no crop must yield nil, got %v", got)
	}
	// An uploaded PDF with crops ⇒ indexed map.
	f := app.SongFile{Crop: &app.Crop{Pages: map[string]app.CropRect{
		"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Source: "manual"},
		"2": {X: 0.2, Y: 0.2, W: 0.6, H: 0.6, Source: "auto"},
	}}}
	got := pageCropsFor(f)
	if len(got) != 2 || got[0].x != 0.1 || got[2].w != 0.6 {
		t.Errorf("pageCropsFor = %+v, want pages 0 and 2 mapped", got)
	}
}

func TestApplyCropToDoc(t *testing.T) {
	doc := annotationsDoc{Objects: []docObject{
		{UUID: "centre", Page: 0, Points: []docPoint{{X: 0.5, Y: 0.5}}, Style: docStyle{Width: 0.01, FontSize: 0.03}},
		{UUID: "corner", Page: 0, Points: []docPoint{{X: 0.3, Y: 0.3}}, Style: docStyle{Width: 0.01}},
		{UUID: "outside", Page: 0, Points: []docPoint{{X: 0.9, Y: 0.9}}, Style: docStyle{Width: 0.01}},
		{UUID: "otherpage", Page: 1, Points: []docPoint{{X: 0.5, Y: 0.5}}, Style: docStyle{Width: 0.01}},
	}}
	out := applyCropToDoc(doc, pageCrops{0: {x: 0.25, y: 0.25, w: 0.5, h: 0.5}})

	byID := map[string]docObject{}
	for _, o := range out.Objects {
		byID[o.UUID] = o
	}
	if _, ok := byID["outside"]; ok {
		t.Error("a mark fully outside the crop must be dropped")
	}
	// centre of the page is the centre of a centred crop.
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	c := byID["centre"]
	if !near(c.Points[0].X, 0.5) || !near(c.Points[0].Y, 0.5) {
		t.Errorf("centre mark mapped to %v, want (0.5,0.5)", c.Points[0])
	}
	// width (page-width fraction) scales by 1/cw; fontSize (page-height fraction) by 1/ch.
	if !near(c.Style.Width, 0.02) || !near(c.Style.FontSize, 0.06) {
		t.Errorf("centre style = w %.3f fs %.3f, want 0.02 / 0.06", c.Style.Width, c.Style.FontSize)
	}
	corner := byID["corner"]
	if !near(corner.Points[0].X, 0.1) || !near(corner.Points[0].Y, 0.1) {
		t.Errorf("corner mark mapped to %v, want (0.1,0.1)", corner.Points[0])
	}
	// a mark on an uncropped page is untouched.
	op := byID["otherpage"]
	if !near(op.Points[0].X, 0.5) || !near(op.Style.Width, 0.01) {
		t.Errorf("uncropped-page mark was changed: %v", op)
	}
}

// TestCropKeepsHighlightSourceWidth pins VLL's requirement: a wide highlight `line` must cover the SAME
// source region (the same lyric-line height) on the cropped page as on the PDF. The stroke width scales by
// 1/cw, so (cropped width) × (crop width fraction) == the original source width. A transform that does NOT
// scale the width fails this (the teeth below).
func TestCropKeepsHighlightSourceWidth(t *testing.T) {
	const srcWidth = 0.03 // a wide highlighter stroke, as a fraction of page width
	const cw = 0.5
	doc := annotationsDoc{Objects: []docObject{
		{UUID: "hl", Type: "line", Page: 0, Points: []docPoint{{X: 0.3, Y: 0.5}, {X: 0.7, Y: 0.5}}, Style: docStyle{Width: srcWidth}},
	}}
	out := applyCropToDoc(doc, pageCrops{0: {x: 0.25, y: 0.25, w: cw, h: cw}})
	got := out.Objects[0].Style.Width * cw // back to source-width units
	if math.Abs(got-srcWidth) > 1e-9 {
		t.Fatalf("highlight width in source units = %.4f, want %.4f (width must scale by 1/cw)", got, srcWidth)
	}
}

// TestCropStraddlingStrokeIsKept pins the clip-not-drop rule: a highlight whose end falls just past the crop's
// right edge (VLL's lines end within 0.3–1% of the auto-crop edge) straddles the boundary and must be KEPT
// (the renderer clips the overflow); only a mark ENTIRELY outside the crop is dropped.
func TestCropStraddlingStrokeIsKept(t *testing.T) {
	doc := annotationsDoc{Objects: []docObject{
		// crop is x∈[0.25,0.75]; this line ends at 0.76 — 1% past the right edge.
		{UUID: "straddle", Type: "line", Page: 0, Points: []docPoint{{X: 0.3, Y: 0.5}, {X: 0.76, Y: 0.5}}, Style: docStyle{Width: 0.02}},
	}}
	out := applyCropToDoc(doc, pageCrops{0: {x: 0.25, y: 0.25, w: 0.5, h: 0.5}})
	if len(out.Objects) != 1 {
		t.Fatalf("a stroke straddling the crop edge was dropped; want it kept (clip the overflow): %d objects", len(out.Objects))
	}
	// Its far end is past 1.0 (will be clipped), its near end inside — i.e. it was kept, not dropped.
	if maxX := out.Objects[0].Points[1].X; maxX <= 1.0 {
		t.Errorf("straddling stroke's far end = %.3f, expected > 1.0 (past the clip edge)", maxX)
	}
}
