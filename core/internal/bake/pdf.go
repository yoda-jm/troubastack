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

	"troubastack/core/internal/pdfmark"

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
	return b.concertPDF(concertID, viewerRole, viewerMemberID, nil)
}

// ConcertPDFWithProgress is ConcertPDF with a per-sheet progress callback for T202's async job: onSheet is
// called once with (0, total) as soon as the sheet count is known, then (done, total) after each sheet
// composes. It is a pure side-channel — the returned bytes are identical to ConcertPDF for the same viewer.
func (b *Baker) ConcertPDFWithProgress(concertID, viewerRole, viewerMemberID string, onSheet func(done, total int)) ([]byte, error) {
	return b.concertPDF(concertID, viewerRole, viewerMemberID, onSheet)
}

// concertPDF is the shared implementation. onSheet, if non-nil, is called once per composed sheet with
// (done, total) for T202's async progress. The sync ConcertPDF passes nil, and onSheet affects nothing in the
// output, so the async job and the sync endpoint produce BYTE-IDENTICAL PDFs for the same viewer.
func (b *Baker) concertPDF(concertID, viewerRole, viewerMemberID string, onSheet func(done, total int)) ([]byte, error) {
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

	// T202: the member's display name for the footer ("<concert> · for <name> · i/total"), from the roster.
	memberName := rosterName(cb.Roster, viewerMemberID)

	// T202: flatten to THIS viewer's own page sequence per song — memberPages[member] else the default else
	// all pages — so the paper matches their tablet (a "bass only" member gets only their pages). pos/seqLen
	// number the page WITHIN the viewer's sequence, as the tablet does.
	type sheet struct {
		song        BakedSong
		pi          int
		pos, seqLen int
	}
	var sheets []sheet
	for _, s := range songs {
		seq := resolvePageSequence(s, viewerMemberID)
		for pos, pi := range seq {
			sheets = append(sheets, sheet{song: s, pi: int(pi), pos: pos, seqLen: len(seq)})
		}
	}
	total := len(sheets)
	if onSheet != nil {
		onSheet(0, total) // publish the sheet count up front, so a poller shows "0 of N" before the first compose
	}

	showMark := !b.svc.BandPdfMarkHidden(cb.BandID) // T205: the band's live setting (bundle carries the id)
	return writeA4PDF(cb.Name, cb.Name, total, showMark, func(i int) (image.Image, string, string, error) {
		sh := sheets[i]
		s := sh.song
		title := s.Title
		if title == "" {
			title = "Song"
		}
		composed, cerr := b.composePage(revDir, s.Pages[sh.pi], viewerRole, viewerMemberID)
		if cerr != nil {
			return nil, "", "", fmt.Errorf("compose song %s page %d: %w", s.SongID, sh.pi, cerr)
		}
		hdr := fmt.Sprintf("%s — page %d/%d", title, sh.pos+1, sh.seqLen)
		if s.OnCall {
			hdr += " · On call"
		}
		ftr := fmt.Sprintf("%s · %d/%d", cb.Name, i+1, total)
		if memberName != "" {
			ftr = fmt.Sprintf("%s · for %s · %d/%d", cb.Name, memberName, i+1, total)
		}
		if onSheet != nil {
			onSheet(i+1, total)
		}
		return composed, hdr, ftr, nil
	})
}

// resolvePageSequence returns the page indices to print for a viewer, in order — matching the tablet
// (StageModel.resolvePageSequence): the member's own selection if present, else the "" default entry, else
// all of the song's pages. Out-of-range indices are dropped defensively.
func resolvePageSequence(song BakedSong, memberID string) []int32 {
	var seq []int32
	foundMember, foundDefault := false, false
	var memberSeq, defaultSeq []int32
	for _, mp := range song.MemberPages {
		switch mp.MemberID {
		case memberID:
			memberSeq, foundMember = mp.Page, true
		case "":
			defaultSeq, foundDefault = mp.Page, true
		}
	}
	switch {
	case foundMember:
		seq = memberSeq
	case foundDefault:
		seq = defaultSeq
	default:
		seq = make([]int32, len(song.Pages))
		for i := range song.Pages {
			seq[i] = int32(i)
		}
	}
	out := make([]int32, 0, len(seq))
	for _, idx := range seq {
		if idx >= 0 && int(idx) < len(song.Pages) {
			out = append(out, idx)
		}
	}
	return out
}

// rosterName is the display name for a member id in a bundle's roster, or "" when absent.
func rosterName(roster []BundleMember, memberID string) string {
	for _, m := range roster {
		if m.MemberID == memberID {
			return m.DisplayName
		}
	}
	return ""
}

// a4Sheet supplies the i-th (0-based) sheet of an A4 print: the composed image and its header/footer text.
type a4Sheet func(i int) (img image.Image, header, footer string, err error)

// writeA4PDF is the ONE A4 page writer behind both prints (T57 concert, FILEPDF single file): one composed
// raster per sheet, fit inside the margins (centered, aspect-preserved) with a header + footer band, pinned
// dates and a sorted catalog so the bytes are deterministic. Sheets are composed lazily, one at a time, so a
// long concert never holds every page in memory. n == 0 still yields a valid one-page PDF headed
// emptyHeading with "(no pages to print)".
func writeA4PDF(docTitle, emptyHeading string, n int, showMark bool, sheet a4Sheet) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetFooterFunc(func() { pdfmark.Draw(pdf, showMark) }) // T205: mark on every page (concert/file print)
	fixed := time.Unix(1700000000, 0).UTC()                   // pinned → deterministic bytes (T19/chartpdf pattern)
	pdf.SetCreationDate(fixed)
	pdf.SetModificationDate(fixed)
	pdf.SetCatalogSort(true)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle(docTitle, true)
	tr := pdf.UnicodeTranslatorFromDescriptor("") // UTF-8 → cp1252 (em-dash etc.)

	availW := pdfPageW - 2*pdfMargin
	imgY := pdfMargin + pdfHeaderH
	availH := pdfPageH - imgY - pdfMargin - pdfFooterH

	for i := 0; i < n; i++ {
		composed, hdr, ftr, cerr := sheet(i)
		if cerr != nil {
			return nil, cerr
		}
		var jbuf bytes.Buffer
		if err := jpeg.Encode(&jbuf, composed, &jpeg.Options{Quality: pdfJPEGQ}); err != nil {
			return nil, err
		}

		pdf.AddPage()

		pdf.SetFont("Helvetica", "", 9)
		pdf.SetXY(pdfMargin, pdfMargin)
		pdf.CellFormat(availW, pdfHeaderH, tr(hdr), "", 0, "L", false, 0, "")

		// Fit the composed image inside the content box, centered, aspect-preserved.
		bnds := composed.Bounds()
		iw, ih := float64(bnds.Dx()), float64(bnds.Dy())
		scale := math.Min(availW/iw, availH/ih)
		w, h := iw*scale, ih*scale
		x := pdfMargin + (availW-w)/2
		y := imgY + (availH-h)/2
		name := "p" + strconv.Itoa(i+1)
		opt := fpdf.ImageOptions{ImageType: "JPG", ReadDpi: false}
		pdf.RegisterImageOptionsReader(name, opt, &jbuf)
		pdf.ImageOptions(name, x, y, w, h, false, opt, 0, "")

		pdf.SetFont("Helvetica", "", 8)
		pdf.SetXY(pdfMargin, pdfPageH-pdfMargin-pdfFooterH)
		pdf.CellFormat(availW, pdfFooterH, tr(ftr), "", 0, "C", false, 0, "")
	}

	if n == 0 {
		// Nothing to print still yields a valid, titled one-page PDF.
		pdf.AddPage()
		pdf.SetFont("Helvetica", "B", 14)
		pdf.SetXY(pdfMargin, pdfMargin)
		pdf.CellFormat(availW, 10, tr(emptyHeading), "", 0, "L", false, 0, "")
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
	layers := make([]composeLayer, 0, len(pg.Overlays))
	for _, ov := range pg.Overlays {
		if ov.ImageRef == "" || !LayerVisible(ov, viewerRole, viewerMemberID) {
			continue
		}
		ref := ov.ImageRef
		layers = append(layers, composeLayer{order: ov.Order, load: func() (image.Image, error) {
			return decodeBlob(filepath.Join(revDir, filepath.FromSlash(ref)))
		}})
	}
	return composeLayers(raster, layers)
}

// composeLayer is one overlay to paint: its z-order and a lazy decoder for its pixels.
type composeLayer struct {
	order int32
	load  func() (image.Image, error)
}

// composeLayers is the ONE flattening both prints share (T57 concert, FILEPDF single file): a white ground
// (JPEG has no alpha), the raster, then the given overlays ascending by z-order — a STABLE sort, so ties keep
// the caller's order. Choosing which layers a viewer sees is the caller's job; everything passed is painted.
func composeLayers(raster image.Image, layers []composeLayer) (image.Image, error) {
	bnds := raster.Bounds()
	canvas := image.NewRGBA(bnds)
	draw.Draw(canvas, bnds, image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(canvas, bnds, raster, bnds.Min, draw.Over)

	ovs := make([]composeLayer, len(layers))
	copy(ovs, layers)
	sort.SliceStable(ovs, func(i, j int) bool { return ovs[i].order < ovs[j].order })
	for _, ov := range ovs {
		olay, derr := ov.load()
		if derr != nil {
			return nil, derr
		}
		// T198: scale an overlay whose size differs from the raster onto the raster bounds. The overlay worker
		// renders every page at ONE width (page 0's), but a cropped page's raster can be a different width
		// (native-ppi floor, or a mix of cropped + uncropped pages) — drawing at the overlay's own bounds then
		// misplaced + mis-scaled the marks in a PRINT (concert T57 + FILEPDF). On the tablet Stage already
		// scales overlay→raster; this makes the print paths (and the T198 preview) match it.
		if ob := olay.Bounds(); ob.Dx() != bnds.Dx() || ob.Dy() != bnds.Dy() {
			olay = resampleBilinear(olay, bnds.Dx(), bnds.Dy())
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
