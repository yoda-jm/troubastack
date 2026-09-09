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

// greyProbeStride subsamples the probe: a coloured page is coloured at ANY scale, so the decision does not
// need every pixel. Reading every 4th pixel in each direction is 16× less work and cannot miss a coloured
// REGION — only a scattering of isolated coloured pixels, which is not what a coloured page looks like.
const greyProbeStride = 4

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

// isMono reports whether every probed pixel's channels agree to within greyThreshold.
func isMono(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += greyProbeStride {
		for x := b.Min.X; x < b.Max.X; x += greyProbeStride {
			r, g, bl, a := img.At(x, y).RGBA()
			if a < 0xffff {
				// Any transparency and we stop: greyscale has no alpha channel, so re-encoding would
				// silently flatten it. A page raster from poppler is opaque, so this costs nothing on the
				// real path — it is here so the pass can never be the thing that made a page opaque.
				return false
			}
			r8, g8, b8 := int(r>>8), int(g>>8), int(bl>>8)
			hi, lo := r8, r8
			for _, v := range [2]int{g8, b8} {
				if v > hi {
					hi = v
				}
				if v < lo {
					lo = v
				}
			}
			if hi-lo > greyThreshold {
				return false
			}
		}
	}
	return true
}
