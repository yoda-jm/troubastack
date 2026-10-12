package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
)

// The review crops: the stored chart page rasterised by pdftoppm, cut around the mark, with the mark, its
// proposed line and its old anchor's projection drawn over it. Standard library only.

const rasterDPI = 110

var (
	colRed    = color.RGBA{220, 20, 40, 255}
	colBlue   = color.RGBA{20, 90, 230, 255}
	colOrange = color.RGBA{245, 140, 0, 255}
)

// box is a [0,1] page-fraction rectangle.
type box struct{ X0, Y0, X1, Y1 float64 }

func (b box) union(o box) box {
	return box{math.Min(b.X0, o.X0), math.Min(b.Y0, o.Y0), math.Max(b.X1, o.X1), math.Max(b.Y1, o.Y1)}
}

// pageRaster rasterises page `page` (0-based) of a PDF at rasterDPI, caching by (pdf path, page).
type pageRaster struct {
	tmp   string
	cache map[string]image.Image
}

func newPageRaster(tmp string) *pageRaster {
	return &pageRaster{tmp: tmp, cache: map[string]image.Image{}}
}

func (pr *pageRaster) page(pdfPath string, page int) (image.Image, error) {
	key := fmt.Sprintf("%s#%d", pdfPath, page)
	if img, ok := pr.cache[key]; ok {
		return img, nil
	}
	prefix := filepath.Join(pr.tmp, fmt.Sprintf("p%d", len(pr.cache)))
	n := fmt.Sprint(page + 1)
	cmd := exec.Command("pdftoppm", "-r", fmt.Sprint(rasterDPI), "-f", n, "-l", n, "-singlefile", "-png", pdfPath, prefix)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftoppm page %d: %v: %s", page+1, err, stderr.String())
	}
	f, err := os.Open(prefix + ".png")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(prefix + ".png")
	pr.cache[key] = img
	return img, nil
}

// overlay is what one crop shows.
type overlay struct {
	mark    box  // red
	run     *box // blue: the proposed line's run box (linked only)
	refX    float64
	refY    float64 // the run's reference point, the blue tie line's start (linked only)
	tie     bool
	old     *box // dashed orange: the old anchor's projection (old-style, same page only)
	lineH   float64
	minCrop int
}

// renderCrop draws ov over the page image and returns the crop: the boxes shown plus ~3 line-heights of
// margin, at least minCrop px wide (or the page width), clamped to the page.
func renderCrop(page image.Image, ov overlay) *image.RGBA {
	b := page.Bounds()
	W, H := float64(b.Dx()), float64(b.Dy())
	area := ov.mark
	if ov.run != nil {
		area = area.union(*ov.run)
	}
	if ov.old != nil {
		area = area.union(*ov.old)
	}
	lh := ov.lineH
	if lh <= 0 {
		lh = 0.015
	}
	m := 3 * lh
	x0, x1 := (area.X0-m)*W, (area.X1+m)*W
	y0, y1 := (area.Y0-m)*H, (area.Y1+m)*H
	if minW := float64(ov.minCrop); x1-x0 < minW {
		c := (x0 + x1) / 2
		x0, x1 = c-minW/2, c+minW/2
	}
	clampSpan := func(lo, hi, max float64) (float64, float64) {
		if hi-lo >= max {
			return 0, max
		}
		if lo < 0 {
			hi, lo = hi-lo, 0
		}
		if hi > max {
			lo, hi = lo-(hi-max), max
		}
		return lo, hi
	}
	x0, x1 = clampSpan(x0, x1, W)
	y0, y1 = clampSpan(y0, y1, H)
	r := image.Rect(int(x0), int(y0), int(math.Ceil(x1)), int(math.Ceil(y1))).Add(b.Min).Intersect(b)

	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), page, r.Min, draw.Src)
	px := func(x, y float64) (int, int) {
		return int(math.Round(x*W)) - (r.Min.X - b.Min.X), int(math.Round(y*H)) - (r.Min.Y - b.Min.Y)
	}
	rect := func(bx box, c color.RGBA, thick int, dashed bool) {
		ax, ay := px(bx.X0, bx.Y0)
		bxx, byy := px(bx.X1, bx.Y1)
		// a zero-size box (a text label's point) is drawn as a small square so it is visible
		if bxx-ax < 6 {
			ax, bxx = ax-3, bxx+3
		}
		if byy-ay < 6 {
			ay, byy = ay-3, byy+3
		}
		for _, s := range [][4]int{{ax, ay, bxx, ay}, {bxx, ay, bxx, byy}, {bxx, byy, ax, byy}, {ax, byy, ax, ay}} {
			drawLine(out, s[0], s[1], s[2], s[3], c, thick, dashed)
		}
	}
	if ov.run != nil {
		rect(*ov.run, colBlue, 2, false)
		if ov.tie {
			sx, sy := px(ov.refX, ov.refY)
			ex, ey := px(ov.mark.X0, ov.mark.Y0)
			drawLine(out, sx, sy, ex, ey, colBlue, 1, false)
		}
	}
	rect(ov.mark, colRed, 2, false)
	if ov.old != nil { // last, dashed: where it coincides with the red box, the red shows between the dashes
		rect(*ov.old, colOrange, 2, true)
	}
	return out
}

// renderPage is the whole page with the mark outlined in red: the context for a mark whose crop shows no
// text (a fixed mark far from every line).
func renderPage(page image.Image, mark box) *image.RGBA {
	b := page.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), page, b.Min, draw.Src)
	W, H := float64(b.Dx()), float64(b.Dy())
	ax, ay := int(math.Round(mark.X0*W)), int(math.Round(mark.Y0*H))
	bx, by := int(math.Round(mark.X1*W)), int(math.Round(mark.Y1*H))
	for _, s := range [][4]int{{ax, ay, bx, ay}, {bx, ay, bx, by}, {bx, by, ax, by}, {ax, by, ax, ay}} {
		drawLine(out, s[0], s[1], s[2], s[3], colRed, 3, false)
	}
	return out
}

// drawLine draws a (thick, optionally dashed) straight line by stepping along its longer axis.
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA, thick int, dashed bool) {
	dx, dy := x1-x0, y1-y0
	steps := int(math.Max(math.Abs(float64(dx)), math.Abs(float64(dy))))
	if steps == 0 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		if dashed && (i/6)%2 == 1 {
			continue
		}
		x := x0 + int(math.Round(float64(dx)*float64(i)/float64(steps)))
		y := y0 + int(math.Round(float64(dy)*float64(i)/float64(steps)))
		for ox := -(thick - 1) / 2; ox <= thick/2; ox++ {
			for oy := -(thick - 1) / 2; oy <= thick/2; oy++ {
				if (image.Point{x + ox, y + oy}).In(img.Bounds()) {
					img.SetRGBA(x+ox, y+oy, c)
				}
			}
		}
	}
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
