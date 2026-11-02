package bake

import (
	"image"
	"math"
)

// resampleBilinear scales src to exactly w×h with bilinear interpolation — the in-tree interpolating
// resampler (the module is stdlib-only, so no x/image dependency; T199's rotation reuses this). It
// interpolates ALPHA-PREMULTIPLIED components (color.Color.RGBA() is premultiplied, and image.RGBA stores
// premultiplied), so a transparent overlay scales without dark halos at the edges of a mark.
//
// Center-aligned sampling: dst pixel (x,y) maps to src ((x+0.5)·sw/w − 0.5, …), the standard half-pixel
// convention, so a 2× upscale keeps content centred rather than shifted by half a pixel.
func resampleBilinear(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 || w <= 0 || h <= 0 {
		return dst
	}
	clampi := func(v, hi int) int {
		if v < 0 {
			return 0
		}
		if v > hi {
			return hi
		}
		return v
	}
	for y := 0; y < h; y++ {
		fy := (float64(y)+0.5)*float64(sh)/float64(h) - 0.5
		y0 := int(math.Floor(fy))
		wy := fy - float64(y0)
		y0c := clampi(y0, sh-1)
		y1c := clampi(y0+1, sh-1)
		for x := 0; x < w; x++ {
			fx := (float64(x)+0.5)*float64(sw)/float64(w) - 0.5
			x0 := int(math.Floor(fx))
			wx := fx - float64(x0)
			x0c := clampi(x0, sw-1)
			x1c := clampi(x0+1, sw-1)

			// Four source samples (premultiplied, 0..65535), bilinearly weighted.
			r00, g00, b00, a00 := src.At(b.Min.X+x0c, b.Min.Y+y0c).RGBA()
			r10, g10, b10, a10 := src.At(b.Min.X+x1c, b.Min.Y+y0c).RGBA()
			r01, g01, b01, a01 := src.At(b.Min.X+x0c, b.Min.Y+y1c).RGBA()
			r11, g11, b11, a11 := src.At(b.Min.X+x1c, b.Min.Y+y1c).RGBA()
			lerp := func(c00, c10, c01, c11 uint32) uint8 {
				top := float64(c00)*(1-wx) + float64(c10)*wx
				bot := float64(c01)*(1-wx) + float64(c11)*wx
				return uint8((top*(1-wy) + bot*wy) / 256) // 16-bit premultiplied → 8-bit
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i+0] = lerp(r00, r10, r01, r11)
			dst.Pix[i+1] = lerp(g00, g10, g01, g11)
			dst.Pix[i+2] = lerp(b00, b10, b01, b11)
			dst.Pix[i+3] = lerp(a00, a10, a01, a11)
		}
	}
	return dst
}

// cubicCR is the Catmull-Rom (a = −0.5) cubic kernel — the standard interpolating bicubic. Its four taps per
// axis sum to 1 (partition of unity), so a rotated sample needs no weight normalization. It passes through
// its samples (interpolating, not blurring like bilinear) and has a mild negative lobe that keeps a straight
// edge crisp, which is what the T199 deskew quality bar needs.
func cubicCR(x float64) float64 {
	const a = -0.5
	if x < 0 {
		x = -x
	}
	switch {
	case x <= 1:
		return (a+2)*x*x*x - (a+3)*x*x + 1
	case x < 2:
		return a*x*x*x - 5*a*x*x + 8*a*x - 4*a
	default:
		return 0
	}
}

// rotateWindow (T199) straightens a crooked window out of src and returns it upright as an outW×outH RGBA.
// src is the rendered axis-aligned bounding box of the rotated window; (cx,cy) is the window's CENTRE in src
// pixels; angleDeg is the per-page deskew (+ = counter-clockwise correction). For each output pixel it
// inverse-maps — offset from the upright window centre, rotated by the angle about that centre — to a src
// point, and samples with a 4×4 Catmull-Rom bicubic grid (16 taps). A rotation is NOT separable, so this is a
// true 2-D tap grid, not two 1-D passes. Samples that fall off src read WHITE (the paper background), never a
// clamped-edge smear, so the straightened page's margin is paper, not a stretched border. Premultiplied like
// resampleBilinear (pdftoppm output is opaque, so this is a no-op there but stays correct for any alpha).
func rotateWindow(src image.Image, outW, outH int, cx, cy, angleDeg float64) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if outW <= 0 || outH <= 0 || sw <= 0 || sh <= 0 {
		return dst
	}
	theta := angleDeg * math.Pi / 180
	sin, cos := math.Sin(theta), math.Cos(theta)
	ocx := float64(outW) / 2
	ocy := float64(outH) / 2
	for oy := 0; oy < outH; oy++ {
		dy := float64(oy) + 0.5 - ocy
		for ox := 0; ox < outW; ox++ {
			dx := float64(ox) + 0.5 - ocx
			// Rotate the output-centre offset by +theta to find where that pixel came from in the crooked src.
			fx := cx + (cos*dx - sin*dy) - 0.5 // −0.5: sample at pixel centres (half-pixel convention)
			fy := cy + (sin*dx + cos*dy) - 0.5
			ix := int(math.Floor(fx))
			iy := int(math.Floor(fy))
			tx := fx - float64(ix)
			ty := fy - float64(iy)
			var kx, ky [4]float64
			for i := 0; i < 4; i++ {
				kx[i] = cubicCR(tx - float64(i-1))
				ky[i] = cubicCR(ty - float64(i-1))
			}
			var ar, ag, ab, aa float64
			for j := 0; j < 4; j++ {
				sy := iy + j - 1
				for i := 0; i < 4; i++ {
					sx := ix + i - 1
					var r, g, bl, al uint32 = 65535, 65535, 65535, 65535 // off-src ⇒ white paper
					if sx >= 0 && sx < sw && sy >= 0 && sy < sh {
						r, g, bl, al = src.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
					}
					w := kx[i] * ky[j]
					ar += w * float64(r)
					ag += w * float64(g)
					ab += w * float64(bl)
					aa += w * float64(al)
				}
			}
			i := dst.PixOffset(ox, oy)
			dst.Pix[i+0] = clamp8(ar)
			dst.Pix[i+1] = clamp8(ag)
			dst.Pix[i+2] = clamp8(ab)
			dst.Pix[i+3] = clamp8(aa)
		}
	}
	return dst
}

// clamp8 converts a 16-bit value (0..65535) to an 8-bit channel, rounding and clamping the bicubic kernel's
// possible overshoot/undershoot into [0,255]. The divisor is 257, not 256: an 8-bit grey g is stored 16-bit
// as g·257 (0xGG repeated), so v/257 maps every exact grey back to itself — v/256 would lift greys 128–254 by
// one level, re-rendering a page no one deskewed.
func clamp8(v float64) uint8 {
	v = v/257 + 0.5
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
