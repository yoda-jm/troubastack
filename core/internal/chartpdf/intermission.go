package chartpdf

import (
	"bytes"
	_ "embed"

	"github.com/go-pdf/fpdf"
)

// intermissionLabelDefault is what an empty label renders as on the page. The label
// is CONTENT (T153 ⟨D1⟩): the band authors it and a French band types "Entracte". Only
// when it is blank does the renderer supply this word rather than drawing a blank card
// — deliberately NOT the "Song N" fallback (a break is not a song).
const intermissionLabelDefault = "Intermission"

// intermissionShownLabel is the exact text the page draws for a break: the band's own
// label, or the default word when it is blank. This is the "drawn text" T153 ⟨R1⟩ asks
// to assert — kept a pure function so the decision is testable without decompressing a
// PDF, and so it is stated in exactly one place.
func intermissionShownLabel(label string) string {
	if label == "" {
		return intermissionLabelDefault
	}
	return label
}

// wordmarkPNG is the TroubaStage brand mark, embedded in the package that renders it.
// It is the BRAND06 outlined-path wordmark rasterised to a PNG so the page depends on
// neither a font nor an asset outside this package — docs/ is excluded from the Docker
// build context, so reading the SVG from there would render in-tree and fail in the
// image (the build-reads-outside-package lesson). Regenerate with:
//
//	rsvg-convert -w 1200 docs/brand/dist/troubastage-wordmark.svg -o assets/troubastage-wordmark.png
//
//go:embed assets/troubastage-wordmark.png
var wordmarkPNG []byte

// wordmarkAspect is the embedded PNG's width/height (1200x294), used to keep the mark
// undistorted when placed by width.
const wordmarkAspect = 1200.0 / 294.0

// RenderIntermission draws the T153 separator page: a single A4 LANDSCAPE page (T165 ⟨D1⟩) carrying, in
// descending prominence, the LABEL (what a musician reads across a room), the BAND
// NAME (omitted entirely when absent — never "Unknown band", the T143 lesson), and the
// TroubaStage mark (smallest — it identifies the tool, not the message). It renders
// through the same deterministic newDoc/output pipeline as a chart, so the page goes
// down the one PDF→raster path and can be pinned by the T144 golden.
//
// An empty label draws intermissionLabelDefault. The bundle still carries the raw
// (possibly empty) label so the Stage drawer can apply its own default independently.
// intermissionCard is where the three elements sit on a card of a given size. Kept as a pure function of
// the page size for two reasons: the geometry is the thing that actually failed on VLL's stage (the mark
// sat below the fold, so he never saw it), and expressing it in FRACTIONS of the page rather than in magic
// millimetres is what lets the same layout be correct in either orientation instead of silently assuming
// A4 portrait — the assumption that caused T165.
type intermissionCard struct {
	labelY, bandY, markY float64 // top edge of each element
	markW, markH         float64
}

// The proportions are the portrait card's, preserved: label at 36% of the height, band name at 51%, mark
// at 87%. They read the same on any page shape.
const (
	intermissionLabelFrac = 108.0 / pageH
	intermissionBandFrac  = 150.0 / pageH
	intermissionMarkFrac  = (pageH - 40.0) / pageH
	// The mark is absolute, not a fraction of the width: it identifies the tool, not the message, so a
	// wider card should not grow it — on a landscape page it simply sits more modestly.
	intermissionMarkW = 70.0
)

func intermissionLayoutFor(w, h float64) intermissionCard {
	markW := intermissionMarkW
	if markW > w*0.6 {
		markW = w * 0.6 // never let the mark dominate a narrow card
	}
	return intermissionCard{
		labelY: h * intermissionLabelFrac,
		bandY:  h * intermissionBandFrac,
		markY:  h * intermissionMarkFrac,
		markW:  markW,
		markH:  markW / wordmarkAspect,
	}
}

func RenderIntermission(label, bandName string) ([]byte, error) {
	shown := intermissionShownLabel(label)

	pdf, tr := newDoc(shown)
	// T165 ⟨D1⟩ (VLL): the card is baked LANDSCAPE. His tablet is landscape on stage, so a landscape card
	// fills the screen while a portrait one is a narrow strip between two wide empty margins. Stage centres
	// it and letterboxes with the reading scheme when the viewport is portrait (T165 half B). The printed
	// concert PDF composes it aspect-preserved onto its portrait sheet (bake/pdf.go), so it is letterboxed
	// on paper too — never stretched.
	// gofpdf takes the size in PORTRAIT terms and swaps it itself for an "L" page, so passing the already
	// swapped size here silently yields a portrait page again (it swaps back). Pass A4 portrait; read the
	// landscape dimensions back from the page itself rather than assuming the swap happened.
	pdf.AddPageFormat("L", fpdf.SizeType{Wd: pageW, Ht: pageH})
	w, h := pdf.GetPageSize() // A4 landscape: 297 × 210 mm
	card := intermissionLayoutFor(w, h)

	// Label — largest, centred, sitting a little above the middle so the band name and
	// mark have room beneath it.
	pdf.SetFont("Helvetica", "B", 44)
	pdf.SetXY(0, card.labelY)
	pdf.MultiCell(w, 18, tr(shown), "", "C", false)

	// Band name — medium, centred below the label. Absent ⇒ draw nothing (no placeholder).
	if bandName != "" {
		pdf.SetFont("Helvetica", "", 20)
		pdf.SetXY(0, card.bandY)
		pdf.MultiCell(w, 10, tr(bandName), "", "C", false)
	}

	// The mark — smallest, centred near the foot of the page.
	opt := fpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader("troubastage-wordmark", opt, bytes.NewReader(wordmarkPNG))
	pdf.ImageOptions("troubastage-wordmark", (w-card.markW)/2, card.markY, card.markW, card.markH, false, opt, 0, "")

	if err := pdf.Error(); err != nil {
		return nil, err
	}
	return output(pdf)
}
