package bake

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// T169 — a scanned band's bundle is ~98% page rasters, stored as 8-bit RGB PNG, and every one of those
// pages is grey. Dropping the two redundant channels halves the bundle (measured 1.91× on a 12-page
// sample; ~100 MB → ~52 MB on his real one) with NO quality decision to make: a grey pixel written as one
// channel is the same pixel.
//
// Turning zlib up instead buys nothing (measured: 8.61 MB → 8.63 MB, very slightly WORSE). The encoder is
// already doing its job; the whole lossless win is the channels. Anyone who reaches for "compress harder"
// first will spend a day for 0%.

// greyThreshold is the max per-pixel channel spread a page may have and still count as grey. The scanned
// pages' worst spread is 8/255 (JPEG chroma noise from the source scan, not content), while a genuinely
// coloured element is far above it — a generated chart's ochre is ~#926b1f. 32 sits in that gap with room
// on both sides.
const greyThreshold = 32

// minColouredPixels is how many over-threshold pixels a page needs before it counts as COLOURED. One
// pixel is not colour: measured on his 158 scanned pages, exactly one page carries a single pixel at
// spread 33 (#988777) — scan noise one step over the line — and excluding a whole page for it wins the
// reader nothing and costs ~300 KB. A real coloured element is thousands of pixels (the generated chart's
// ochre is 1671), and the thinnest thing worth protecting — a 1-pixel rule drawn across a page — is over a
// thousand. 32 sits in the wide gap between one noisy pixel and the smallest deliberate mark: about 5 mm
// of a hairline at 150 dpi.
const minColouredPixels = 32

// greyscaleIfMono re-encodes a page raster as 8-bit greyscale when it carries no real colour, and returns
// the ORIGINAL BYTES UNTOUCHED otherwise — including on any decode/encode error, because a page that
// cannot be re-encoded must still bake exactly as it does today.
//
// Per PAGE, never per bundle (T169 R1): a generated chart's pages are 99.92% grey but carry a small ochre
// element, and greyscale would discard it. Those pages are correctly excluded, and that is fine — their
// whole bundle is 5.5 MB. This is a win for SCANNED bands.
func greyscaleIfMono(src []byte) []byte {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return src
	}
	if _, already := img.(*image.Gray); already {
		return src // nothing to win, and re-encoding would only churn the content hash
	}
	if !isMono(img) {
		return src
	}
	b := img.Bounds()
	out := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			// The channels agree to within the threshold; the mean is the honest single value, and it is
			// what a viewer already sees. (color.GrayModel would apply luma weights, which would SHIFT a
			// grey pixel's value for no reason.)
			out.SetGray(x, y, color.Gray{Y: uint8(((r + g + bl) / 3) >> 8)})
		}
	}
	var buf bytes.Buffer
	// DefaultCompression, not BestCompression — measured on 10 of his real pages: identical output size
	// (3.4 MB either way) for 4.6× the time (4.2s vs 19.4s). That is the task's own finding about zlib,
	// which turns out to hold for the greyscale encode too: the win is the channels, never the effort.
	// The cost matters because a cold bake pays it once per page, and this is a 158-page bundle.
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&buf, out); err != nil {
		return src
	}
	if buf.Len() >= len(src) {
		return src // never grow a page: keep whatever already encoded smaller
	}
	return buf.Bytes()
}

// isMono reports whether a page carries no real colour: fewer than minColouredPixels pixels whose channels
// disagree by more than greyThreshold.
//
// Every pixel, not a sample. The first cut probed every 4th pixel in each direction, on the argument that a
// coloured page is coloured at any scale — true for a region, false for a THIN LINE: a 1-pixel horizontal
// rule whose y is not a multiple of the stride is sampled zero times, and the page would then be flattened,
// silently discarding that line's colour (Fable, ⟨GO⟩ a0fa7221). No stride above 1 can fix that shape, so
// the sampling is gone.
//
// It costs nothing to be exact here: the fast path walks the pixel buffer directly and returns on the FIRST
// coloured pixel, which makes a genuinely coloured page cheaper to reject than it was to sample, and a grey
// page's full scan is still far below the encode it guards.
func isMono(img image.Image) bool {
	switch im := img.(type) {
	case *image.RGBA:
		return monoPix(im.Pix, im.Stride, im.Rect, true)
	case *image.NRGBA:
		return monoPix(im.Pix, im.Stride, im.Rect, false)
	}
	// Any other model (paletted, 16-bit, YCbCr): correctness over speed, one At() per pixel.
	b := img.Bounds()
	coloured := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a < 0xffff {
				return false // transparency is not a quantity question: one pixel of it stops the pass
			}
			if spread8(int(r>>8), int(g>>8), int(bl>>8)) > greyThreshold {
				coloured++
				if coloured >= minColouredPixels {
					return false
				}
			}
		}
	}
	return true
}

// monoPix walks a packed 8-bit RGBA/NRGBA buffer. `premul` says whether the samples are alpha-premultiplied
// (image.RGBA); either way an opaque pixel's three channels are directly comparable, and a non-opaque one
// stops the pass (greyscale has no alpha to carry).
func monoPix(pix []uint8, stride int, r image.Rectangle, premul bool) bool {
	_ = premul // the comparison is the same; the distinction matters only for what the values MEAN
	w, h := r.Dx(), r.Dy()
	coloured := 0
	for y := 0; y < h; y++ {
		row := pix[y*stride : y*stride+w*4]
		for x := 0; x < w*4; x += 4 {
			if row[x+3] != 0xff {
				return false // transparency stops the pass outright — there is no alpha to carry
			}
			if spread8(int(row[x]), int(row[x+1]), int(row[x+2])) > greyThreshold {
				coloured++
				if coloured >= minColouredPixels {
					return false
				}
			}
		}
	}
	return true
}

func spread8(r, g, b int) int {
	hi, lo := r, r
	if g > hi {
		hi = g
	}
	if g < lo {
		lo = g
	}
	if b > hi {
		hi = b
	}
	if b < lo {
		lo = b
	}
	return hi - lo
}
