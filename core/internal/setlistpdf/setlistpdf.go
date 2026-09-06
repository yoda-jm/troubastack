// Package setlistpdf renders a setlist's running order to a printable A4 PDF — the T158 export sheet that
// gets taped to the floor or handed to a sound engineer. It is a DOCUMENT, not a bundle: it touches no
// bake, no blobs, no bundle field. It only draws; the running-order NUMBERS are computed by the caller via
// internal/runningorder (the one shared rule) and passed in, so this package never re-implements numbering.
package setlistpdf

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

const (
	// Entry kinds, mirroring the shared contract; an intermission row (T153) renders inline + unnumbered.
	KindSong         = "song"
	KindIntermission = "intermission"

	margin = 15.0 // mm — a generous document margin (this is a printed sheet, not a dense chart)
	pageW  = 210.0
)

// Row is one line of the sheet. Number is the running-order number (0 ⇒ unnumbered: an intermission, or a
// bench item). Title is the song title (or, for an intermission, its label).
type Row struct {
	Number int
	Title  string
	Kind   string
}

// Doc is the whole sheet. Venue/EventDate are optional — an empty string omits the line entirely (never a
// bare label, never a zero date). Main is the running order (numbered songs, plus any inline intermission);
// OnCall is the bench (unnumbered), rendered under its own heading only if non-empty.
type Doc struct {
	BandName    string
	SetlistName string
	Venue       string
	EventDate   string
	Main        []Row
	OnCall      []Row
}

// Render draws the sheet and returns the PDF bytes. Auto page-break carries a long running order onto
// further A4 pages.
func Render(d Doc) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	// Deterministic bytes (mirrors chartpdf): a fixed creation/modification date + catalog sort so the same
	// setlist always renders to the same PDF — friendly to caching and to any future golden.
	fixed := time.Unix(1700000000, 0).UTC()
	pdf.SetCreationDate(fixed)
	pdf.SetModificationDate(fixed)
	pdf.SetCatalogSort(true)
	tr := pdf.UnicodeTranslatorFromDescriptor("") // UTF-8 → cp1252 (accented band/song names)
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(true, margin)
	pdf.AddPage()

	body := pageW - 2*margin

	// Header: band · setlist · (venue) · (date). Optional lines are omitted entirely when empty.
	pdf.SetFont("Helvetica", "B", 20)
	pdf.MultiCell(body, 9, tr(d.BandName), "", "L", false)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.MultiCell(body, 7, tr(d.SetlistName), "", "L", false)
	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(90, 90, 90)
	if d.Venue != "" {
		pdf.MultiCell(body, 6, tr(d.Venue), "", "L", false)
	}
	if d.EventDate != "" {
		pdf.MultiCell(body, 6, tr(d.EventDate), "", "L", false)
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(3)
	ry := pdf.GetY()
	pdf.SetLineWidth(0.3)
	pdf.Line(margin, ry, pageW-margin, ry)
	pdf.Ln(4)

	// Running order.
	pdf.SetFont("Helvetica", "", 12)
	for _, r := range d.Main {
		if r.Kind == KindIntermission {
			label := r.Title
			if label == "" {
				label = "Intermission"
			}
			pdf.SetFont("Helvetica", "I", 11)
			pdf.SetTextColor(120, 120, 120)
			pdf.MultiCell(body, 7, tr("— "+label+" —"), "", "C", false)
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Helvetica", "", 12)
			continue
		}
		prefix := ""
		if r.Number > 0 {
			prefix = fmt.Sprintf("%d. ", r.Number)
		}
		pdf.MultiCell(body, 7, tr(prefix+r.Title), "", "L", false)
	}

	// On-call bench — its own heading, unnumbered, only if there is one.
	if len(d.OnCall) > 0 {
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "B", 12)
		pdf.SetTextColor(90, 90, 90)
		pdf.MultiCell(body, 7, tr("On call"), "", "L", false)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Helvetica", "", 12)
		for _, r := range d.OnCall {
			pdf.MultiCell(body, 7, tr(r.Title), "", "L", false)
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
