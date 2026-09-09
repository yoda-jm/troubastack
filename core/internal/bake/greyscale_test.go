package bake

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os/exec"
	"testing"
)

// T169 — the page-raster greyscale pass. The measurement that motivates it is in the task; what these
// vectors pin is the RULE: halve a grey page, and leave a coloured one exactly as it was, byte for byte.

// encodePNG is shared with content_bottom_t149_test.go (same package).

// scanLike builds an RGB page of grey content with a little per-channel noise — a scan's own JPEG chroma,
// which is what the real pages carry (worst spread measured: 8/255).
func scanLike(t *testing.T, w, h int) image.Image {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8((x*7 + y*13) % 200)                                       // "ink" and paper
			img.Set(x, y, color.RGBA{R: v, G: v + uint8((x+y)%5), B: v, A: 255}) // spread ≤ 4
		}
	}
	return img
}

func TestGreyscale_MonoPageIsReEncodedAndShrinks(t *testing.T) {
	src := encodePNG(t, scanLike(t, 240, 340))
	out := greyscaleIfMono(src)
	if len(out) >= len(src) {
		t.Fatalf("a grey page must shrink: %d → %d bytes", len(src), len(out))
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("the re-encoded page must still decode: %v", err)
	}
	if _, ok := img.(*image.Gray); !ok {
		t.Fatalf("re-encoded page is %T, want *image.Gray (one channel is the whole win)", img)
	}
	// The pixels a reader sees must not move: compare against the source's own channel mean.
	orig, _ := png.Decode(bytes.NewReader(src))
	for _, p := range [][2]int{{0, 0}, {13, 27}, {239, 339}, {120, 170}} {
		r, g, b, _ := orig.At(p[0], p[1]).RGBA()
		want := uint8(((r + g + b) / 3) >> 8)
		got, _, _, _ := img.At(p[0], p[1]).RGBA()
		if d := int(want) - int(got>>8); d > 1 || d < -1 {
			t.Fatalf("pixel %v moved: %d → %d", p, want, got>>8)
		}
	}
}

func TestGreyscale_ColouredPageIsLeftBYTE_IDENTICAL(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}
	// A real coloured element, the size the task measured on a generated chart: 0.077% of the page,
	// #926b1f, in a band near the foot. Greyscale would DISCARD it, so the page must be left alone.
	for y := 150; y < 154; y++ {
		for x := 20; x < 120; x++ {
			img.Set(x, y, color.RGBA{R: 0x92, G: 0x6b, B: 0x1f, A: 255})
		}
	}
	src := encodePNG(t, img)
	out := greyscaleIfMono(src)
	if !bytes.Equal(src, out) {
		t.Fatalf("a coloured page must be returned untouched (%d → %d bytes)", len(src), len(out))
	}
}

func TestGreyscale_AlreadyGreyIsUntouched(t *testing.T) {
	// Re-encoding a page that is already one channel wins nothing and would churn its content hash —
	// which every device uses to decide whether to re-download the page.
	g := image.NewGray(image.Rect(0, 0, 64, 64))
	for i := range g.Pix {
		g.Pix[i] = uint8(i % 256)
	}
	src := encodePNG(t, g)
	if out := greyscaleIfMono(src); !bytes.Equal(src, out) {
		t.Fatalf("an already-grey page must be returned untouched")
	}
}

func TestGreyscale_GarbageIsPassedThrough(t *testing.T) {
	// A page that cannot be decoded must still bake exactly as it does today: never fail the bake for an
	// optimisation.
	src := []byte("not a png at all")
	if out := greyscaleIfMono(src); !bytes.Equal(src, out) {
		t.Fatalf("undecodable bytes must pass through unchanged")
	}
}

func TestGreyscale_IsDeterministic(t *testing.T) {
	// The bake's byte-reproducibility promise: an identical re-bake produces identical bytes.
	src := encodePNG(t, scanLike(t, 120, 160))
	if !bytes.Equal(greyscaleIfMono(src), greyscaleIfMono(src)) {
		t.Fatalf("the same page must re-encode to the same bytes")
	}
}

func TestIsMono_ThresholdSitsBetweenScanNoiseAndRealColour(t *testing.T) {
	px := func(r, g, b uint8) image.Image {
		i := image.NewRGBA(image.Rect(0, 0, 8, 8))
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				i.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
		return i
	}
	if !isMono(px(128, 136, 130)) { // spread 8 — the worst measured on 158 real scanned pages
		t.Fatalf("scan chroma noise (spread 8) must count as grey, or the whole win is lost")
	}
	if isMono(px(0x92, 0x6b, 0x1f)) { // the generated chart's ochre — spread 0x73
		t.Fatalf("a real colour must NOT count as grey, or it would be discarded")
	}
}

func TestGreyscale_TransparencyIsNeverFlattened(t *testing.T) {
	// Greyscale has no alpha. A page raster is opaque, so this never fires on the real path — but if it
	// ever did, flattening transparency would be a visual change made by a size optimisation, which is
	// exactly the kind of thing nobody would look for.
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.NRGBA{R: 100, G: 100, B: 100, A: 128}) // grey, half transparent
		}
	}
	src := encodePNG(t, img)
	if out := greyscaleIfMono(src); !bytes.Equal(src, out) {
		t.Fatalf("a page with transparency must be returned untouched")
	}
}

// The JOIN, not the function. Removing greyscaleIfMono from the rasterizer leaves every vector above
// green — a component test proves the component, never that anything calls it (the lesson from the ⟨D2⟩
// flag, one day earlier). A blank page out of poppler is grey, so it must come back one-channel.
func TestPopplerRasterizer_AppliesTheGreyscalePass(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm not installed")
	}
	pages, err := popplerRasterizer{bin: "pdftoppm", dpi: 72}.Rasterize(context.Background(), []byte(minimalPDF))
	if err != nil {
		t.Fatalf("rasterize: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pages[0]))
	if err != nil {
		t.Fatalf("page 0 must decode: %v", err)
	}
	if _, ok := img.(*image.Gray); !ok {
		t.Fatalf("a grey page came back as %T — the rasterizer is not applying the T169 pass", img)
	}
}

// The shape the first cut's stride-4 probe could not see (Fable, ⟨GO⟩ a0fa7221): a 1-pixel coloured rule
// whose y is not a multiple of the stride was sampled zero times, and the page was flattened — silently
// discarding the only colour on it. No stride above 1 can catch this, which is why the probe is exact.
func TestGreyscale_ThinColouredRuleIsNotFlattened(t *testing.T) {
	for _, y0 := range []int{1, 3, 5, 7, 41} { // none of them a multiple of 4
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				img.Set(x, y, color.RGBA{R: 250, G: 250, B: 250, A: 255})
			}
		}
		for x := 0; x < 64; x++ { // a 1px ochre rule, the width of the page
			img.Set(x, y0, color.RGBA{R: 0x92, G: 0x6b, B: 0x1f, A: 255})
		}
		src := encodePNG(t, img)
		if out := greyscaleIfMono(src); !bytes.Equal(src, out) {
			t.Fatalf("a 1px coloured rule at y=%d was flattened — its colour is gone", y0)
		}
	}
}

// …and a 1px VERTICAL rule, which the other cheap fix (striding in x only) would still have missed.
func TestGreyscale_ThinVerticalRuleIsNotFlattened(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: 250, G: 250, B: 250, A: 255})
		}
	}
	for y := 0; y < 64; y++ {
		img.Set(7, y, color.RGBA{R: 0x1f, G: 0x6b, B: 0x92, A: 255})
	}
	src := encodePNG(t, img)
	if out := greyscaleIfMono(src); !bytes.Equal(src, out) {
		t.Fatalf("a 1px vertical coloured rule was flattened")
	}
}

// The other half of "is this page coloured": HOW MUCH. Exactly one of his 158 scanned pages carries a
// single pixel at spread 33 — scan noise one step over the line — and the first exact probe excluded the
// whole page for it, winning the reader nothing and costing ~300 KB. A page is coloured when it carries a
// coloured MARK, not a coloured pixel.
func TestGreyscale_ASingleNoisyPixelIsNotColour(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}
	img.Set(17, 43, color.RGBA{R: 0x98, G: 0x87, B: 0x77, A: 255}) // spread 33: his real page's one pixel
	src := encodePNG(t, img)
	out := greyscaleIfMono(src)
	if bytes.Equal(src, out) {
		t.Fatalf("one noisy pixel must not cost the page its re-encode")
	}
	if img2, _ := png.Decode(bytes.NewReader(out)); img2 != nil {
		if _, ok := img2.(*image.Gray); !ok {
			t.Fatalf("page should have been re-encoded grey, got %T", img2)
		}
	}
}
