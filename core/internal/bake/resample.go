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
