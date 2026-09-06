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

// RenderIntermission draws the T153 separator page: a single A4 page carrying, in
// descending prominence, the LABEL (what a musician reads across a room), the BAND
// NAME (omitted entirely when absent — never "Unknown band", the T143 lesson), and the
// TroubaStage mark (smallest — it identifies the tool, not the message). It renders
// through the same deterministic newDoc/output pipeline as a chart, so the page goes
// down the one PDF→raster path and can be pinned by the T144 golden.
//
// An empty label draws intermissionLabelDefault. The bundle still carries the raw
// (possibly empty) label so the Stage drawer can apply its own default independently.
func RenderIntermission(label, bandName string) ([]byte, error) {
	shown := intermissionShownLabel(label)

	pdf, tr := newDoc(shown)
	pdf.AddPage()

	// Label — largest, centred, sitting a little above the middle so the band name and
	// mark have room beneath it.
	pdf.SetFont("Helvetica", "B", 44)
	pdf.SetXY(0, 108)
	pdf.MultiCell(pageW, 18, tr(shown), "", "C", false)

	// Band name — medium, centred below the label. Absent ⇒ draw nothing (no placeholder).
	if bandName != "" {
		pdf.SetFont("Helvetica", "", 20)
		pdf.SetXY(0, 150)
		pdf.MultiCell(pageW, 10, tr(bandName), "", "C", false)
	}

	// The mark — smallest, centred near the foot of the page.
	const markW = 70.0
	markH := markW / wordmarkAspect
	opt := fpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader("troubastage-wordmark", opt, bytes.NewReader(wordmarkPNG))
	pdf.ImageOptions("troubastage-wordmark", (pageW-markW)/2, pageH-40, markW, markH, false, opt, 0, "")

	if err := pdf.Error(); err != nil {
		return nil, err
	}
	return output(pdf)
}
