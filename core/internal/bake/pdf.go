package bake

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/go-pdf/fpdf"
)

// A4 layout (mm) for the printed concert (T57). One baked page per sheet, the
// composed raster fit inside the margins with a header + footer band.
const (
	pdfPageW   = 210.0
	pdfPageH   = 297.0
	pdfMargin  = 12.0
	pdfHeaderH = 8.0
	pdfFooterH = 8.0
	pdfJPEGQ   = 85 // JPEG quality for embedded page images (size vs. fidelity)
)

// ConcertPDF renders a baked concert to a printable A4 PDF — VLL's paper fallback
// "in case of tablet malfunction" (T57). It is pure compositing over an EXISTING
// bundle (no new render pipeline): for each baked page it draws the raster, layers
// the overlays that LayerVisible selects for the chosen viewer, encodes the result
// as JPEG, and places it on one A4 page with a header/footer. Pure Go (image/draw
// + fpdf), no CGo, no shell-outs.
//
// viewerRole is the EXPLICIT print role ("" = a fresh viewer: mandatory + untagged
// shared layers only); viewerMemberID is the identity whose personal layers to
// include. On-call/bench songs (T23) print LAST, marked in the header. The caller
// (httpapi edge) has already gated access to concertID.
//
// The output is deterministic for a given bundle + viewer (pinned PDF dates, sorted
// resource dicts), so a re-print of the same backup is byte-identical.
func (b *Baker) ConcertPDF(concertID, viewerRole, viewerMemberID string) ([]byte, error) {
	rev, ok := b.latestRev(concertID)
	if !ok {
		return nil, os.ErrNotExist
	}
	revDir := filepath.Join(b.bakesDir, concertID, strconv.FormatUint(rev, 10))
	data, err := os.ReadFile(filepath.Join(revDir, "bundle.json"))
	if err != nil {
		return nil, err
	}
	var cb ConcertBundle
	if err := json.Unmarshal(data, &cb); err != nil {
		return nil, err
	}

	// Running order: main songs keep bake order; on-call (bench) songs sink to the
	// end (T23) — a stable sort so ties preserve the baked order.
	songs := make([]BakedSong, len(cb.Songs))
	copy(songs, cb.Songs)
	sort.SliceStable(songs, func(i, j int) bool { return !songs[i].OnCall && songs[j].OnCall })

	total := 0
	for _, s := range songs {
		total += len(s.Pages)
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	fixed := time.Unix(1700000000, 0).UTC() // pinned → deterministic bytes (T19/chartpdf pattern)
	pdf.SetCreationDate(fixed)
	pdf.SetModificationDate(fixed)
	pdf.SetCatalogSort(true)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle(cb.Name, true)
	tr := pdf.UnicodeTranslatorFromDescriptor("") // UTF-8 → cp1252 (em-dash etc.)

	availW := pdfPageW - 2*pdfMargin
	imgY := pdfMargin + pdfHeaderH
	availH := pdfPageH - imgY - pdfMargin - pdfFooterH

	pageNum := 0
	for _, s := range songs {
		title := s.Title
		if title == "" {
			title = "Song"
		}
		for pi, pg := range s.Pages {
			pageNum++
			composed, cerr := b.composePage(revDir, pg, viewerRole, viewerMemberID)
			if cerr != nil {
				return nil, fmt.Errorf("compose song %s page %d: %w", s.SongID, pi, cerr)
			}
			var jbuf bytes.Buffer
			if err := jpeg.Encode(&jbuf, composed, &jpeg.Options{Quality: pdfJPEGQ}); err != nil {
				return nil, err
			}

			pdf.AddPage()

			pdf.SetFont("Helvetica", "", 9)
			hdr := fmt.Sprintf("%s — page %d/%d", title, pi+1, len(s.Pages))
			if s.OnCall {
				hdr += " · On call"
			}
			pdf.SetXY(pdfMargin, pdfMargin)
			pdf.CellFormat(availW, pdfHeaderH, tr(hdr), "", 0, "L", false, 0, "")

			// Fit the composed image inside the content box, centered, aspect-preserved.
			bnds := composed.Bounds()
			iw, ih := float64(bnds.Dx()), float64(bnds.Dy())
			scale := math.Min(availW/iw, availH/ih)
			w, h := iw*scale, ih*scale
			x := pdfMargin + (availW-w)/2
			y := imgY + (availH-h)/2
			name := "p" + strconv.Itoa(pageNum)
			opt := fpdf.ImageOptions{ImageType: "JPG", ReadDpi: false}
			pdf.RegisterImageOptionsReader(name, opt, &jbuf)
			pdf.ImageOptions(name, x, y, w, h, false, opt, 0, "")

			pdf.SetFont("Helvetica", "", 8)
			ftr := fmt.Sprintf("%s · %d/%d", cb.Name, pageNum, total)
			pdf.SetXY(pdfMargin, pdfPageH-pdfMargin-pdfFooterH)
			pdf.CellFormat(availW, pdfFooterH, tr(ftr), "", 0, "C", false, 0, "")
		}
	}

	if pageNum == 0 {
		// A concert with no baked pages still yields a valid, titled one-page PDF.
		pdf.AddPage()
		pdf.SetFont("Helvetica", "B", 14)
		pdf.SetXY(pdfMargin, pdfMargin)
		pdf.CellFormat(availW, 10, tr(cb.Name), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		pdf.SetXY(pdfMargin, pdfMargin+12)
		pdf.CellFormat(availW, 8, tr("(no pages to print)"), "", 0, "L", false, 0, "")
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// composePage flattens one baked page for the viewer: the raster with the visible
// overlays painted over it in z-order, onto a white ground (JPEG has no alpha).
func (b *Baker) composePage(revDir string, pg PageImages, viewerRole, viewerMemberID string) (image.Image, error) {
	raster, err := decodeBlob(filepath.Join(revDir, filepath.FromSlash(pg.PageRasterRef)))
	if err != nil {
		return nil, err
	}
	bnds := raster.Bounds()
	canvas := image.NewRGBA(bnds)
	draw.Draw(canvas, bnds, image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(canvas, bnds, raster, bnds.Min, draw.Over)

	// Overlays ascending by z-order (ties keep bundle order — stable), visible only.
	ovs := make([]LayerImage, len(pg.Overlays))
	copy(ovs, pg.Overlays)
	sort.SliceStable(ovs, func(i, j int) bool { return ovs[i].Order < ovs[j].Order })
	for _, ov := range ovs {
		if ov.ImageRef == "" || !LayerVisible(ov, viewerRole, viewerMemberID) {
			continue
		}
		olay, derr := decodeBlob(filepath.Join(revDir, filepath.FromSlash(ov.ImageRef)))
		if derr != nil {
			return nil, derr
		}
		draw.Draw(canvas, bnds, olay, olay.Bounds().Min, draw.Over)
	}
	return canvas, nil
}

// decodeBlob reads a bundle blob (PNG in practice; image.Decode covers any decoder
// the package registers) into an image.
func decodeBlob(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return img, nil
}
