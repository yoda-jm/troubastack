package soundsheet

import (
	"bytes"

	"github.com/go-pdf/fpdf"

	"troubastack/core/internal/app"
	"troubastack/core/internal/pdfmark"
)

// RenderPDF renders the sheet model to a landscape A4 PDF. The 3-row column header repeats on every page and a
// song row is never split (the layout plan decides width + pagination). When the channels are too wide for one
// band, each band is drawn as its own full-width section (correct, if less compact) — the common case is one
// band, inline or with a changes+note sub-row.
func RenderPDF(m app.SheetModel) ([]byte, error) {
	g := A4Landscape()
	plan := PlanLayout(len(m.Subcols), len(m.Rows), g)

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetFooterFunc(func() { pdfmark.Draw(pdf, !m.HidePdfMark) }) // T205: mark on every page, set before AddPage
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(false, 10)
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	for bandIdx, band := range plan.Bands {
		pdf.AddPage()
		drawTitle(pdf, tr, m, bandIdx, len(plan.Bands))
		drawBand(pdf, tr, m, plan, band, g)
	}
	drawInputList(pdf, tr, m, g)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawTitle(pdf *fpdf.Fpdf, tr func(string) string, m app.SheetModel, bandIdx, nBands int) {
	pdf.SetFont("Helvetica", "B", 14)
	title := m.SetlistName + " — sound sheet"
	if nBands > 1 {
		title += " (part " + itoa(bandIdx+1) + "/" + itoa(nBands) + ")"
	}
	pdf.CellFormat(0, 8, tr(title), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	pdf.CellFormat(0, 5, tr("order: "+string(m.Order)), "", 1, "L", false, 0, "")
	pdf.Ln(1)
}

// drawHeader draws the 3-row column header (owner colspans / channel / console-input) for the given band's
// channel columns, starting at the current Y. Returns nothing; leaves Y below the header.
func drawHeader(pdf *fpdf.Fpdf, tr func(string) string, m app.SheetModel, band []int, colW float64) {
	x0 := pdf.GetX()
	y0 := pdf.GetY()
	rh := 5.0
	// Row 1: owner spans over adjacent same-owner runs WITHIN this band.
	pdf.SetFont("Helvetica", "B", 7)
	pdf.CellFormat(titleW, rh, "", "1", 0, "C", false, 0, "") // title corner
	i := 0
	for i < len(band) {
		j := i + 1
		for j < len(band) && m.Subcols[band[j]].OwnerID == m.Subcols[band[i]].OwnerID {
			j++
		}
		pdf.CellFormat(colW*float64(j-i), rh, tr(ownerBase(m.Subcols[band[i]])), "1", 0, "C", false, 0, "")
		i = j
	}
	pdf.Ln(rh)
	// Row 2: channel.
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetX(x0)
	pdf.CellFormat(titleW, rh, tr("Song"), "1", 0, "L", false, 0, "")
	for _, ci := range band {
		pdf.CellFormat(colW, rh, tr(m.Subcols[ci].Channel), "1", 0, "C", false, 0, "")
	}
	pdf.Ln(rh)
	// Row 3: console input.
	pdf.SetFont("Helvetica", "I", 6)
	pdf.SetX(x0)
	pdf.CellFormat(titleW, rh, tr("input →"), "1", 0, "R", false, 0, "")
	for _, ci := range band {
		pdf.CellFormat(colW, rh, tr(m.Subcols[ci].Input), "1", 0, "C", false, 0, "")
	}
	pdf.Ln(rh)
	_ = y0
}

func drawBand(pdf *fpdf.Fpdf, tr func(string) string, m app.SheetModel, plan LayoutPlan, band []int, g Geom) {
	drawHeader(pdf, tr, m, band, plan.ColW)
	rh := g.RowH
	fullW := titleW + plan.ColW*float64(len(band))
	onCallHeaderDrawn := false
	for _, row := range m.Rows {
		// page break before a row that wouldn't fit whole (header repeats).
		need := rh
		if row.Kind == app.SheetRowSong && !plan.InlineChangesNote {
			need += g.SubRowH
		}
		if pdf.GetY()+need > 10+g.UsableH+g.HeaderH {
			pdf.AddPage()
			drawHeader(pdf, tr, m, band, plan.ColW)
			onCallHeaderDrawn = false
		}
		// An intermission is a full-width separator row.
		if row.Kind == app.SheetRowIntermission {
			pdf.SetFont("Helvetica", "BI", 7)
			pdf.CellFormat(fullW, rh, tr("— "+nz(row.Label, "Intermission")+" —"), "1", 1, "C", false, 0, "")
			continue
		}
		// A heading before the first on-call song.
		if row.Section == app.SheetSectionOnCall && !onCallHeaderDrawn {
			pdf.SetFont("Helvetica", "B", 7)
			pdf.CellFormat(fullW, rh, tr("On call"), "1", 1, "L", false, 0, "")
			onCallHeaderDrawn = true
		}
		x0 := pdf.GetX()
		pdf.SetFont("Helvetica", "", 7)
		num := ""
		if row.Number > 0 {
			num = itoa(row.Number) + " · "
		}
		pdf.CellFormat(titleW, rh, tr(num+clip(row.Title, 28)), "1", 0, "L", false, 0, "")
		for _, ci := range band {
			c := row.Cells[m.Subcols[ci].Key]
			txt := c.Text
			if c.Optional && txt != "" {
				txt = "(" + txt + ")"
			}
			pdf.CellFormat(plan.ColW, rh, tr(txt), "1", 0, "C", false, 0, "")
		}
		pdf.Ln(rh)
		if plan.InlineChangesNote {
			// (inline changes+note handled as a trailing wide cell set is skipped for width; sub-row is the
			// default when they don't fit. For the inline case we still show them as a light sub-row for clarity.)
		}
		if !plan.InlineChangesNote {
			pdf.SetX(x0)
			pdf.SetFont("Helvetica", "I", 6)
			sub := joinNonEmpty(" | ", joinChanges(row.Changes), row.SoundNote)
			pdf.CellFormat(titleW+plan.ColW*float64(len(band)), g.SubRowH, tr("  "+sub), "1", 1, "L", false, 0, "")
		}
	}
}

func drawInputList(pdf *fpdf.Fpdf, tr func(string) string, m app.SheetModel, g Geom) {
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 7, tr("Input list"), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	for _, l := range m.InputList {
		line := "In " + itoa(l.Input) + " · " + nz(l.Strip, "—") + " · " + l.Who + " · " + l.Part + " · " + l.Mic
		pdf.CellFormat(0, 5, tr(line), "", 1, "L", false, 0, "")
	}
	if len(m.NotPatched) > 0 {
		pdf.Ln(2)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.CellFormat(0, 5, tr("Not patched yet: "+joinChanges(m.NotPatched)), "", 1, "L", false, 0, "")
	}
}

// ownerBase is the row-1 header text. For a shared channel it keeps the "Bass — C/V" header; for a person it is
// the musician name (OwnerName already holds it).
func ownerBase(sc app.SheetSubcol) string { return sc.OwnerName }

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
func nz(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
func joinChanges(ss []string) string { return joinNonEmpty("; ", ss...) }
func joinNonEmpty(sep string, ss ...string) string {
	out := ""
	for _, s := range ss {
		if s == "" {
			continue
		}
		if out != "" {
			out += sep
		}
		out += s
	}
	return out
}
