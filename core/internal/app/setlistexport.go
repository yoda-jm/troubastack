package app

import (
	"troubastack/core/internal/runningorder"
	"troubastack/core/internal/setlistpdf"
)

// ExportSetlistPDF renders a setlist's running order to a printable A4 PDF (T158) and returns it with a
// download filename. Membership-gated like every read (GetBand + Setlist). It is a DOCUMENT — it reads only
// the setlist and its songs and touches no bake, blob, or bundle. The running-order NUMBERS come from the
// one shared rule (internal/runningorder), the same table TroubaStage (Kotlin) and TroubaStudio (TS) run,
// so "7" means the 7th main song on the sheet, on stage, and in the editor alike.
func (s *Service) ExportSetlistPDF(caller User, bandID, setlistID string) ([]byte, string, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return nil, "", err
	}
	det, err := s.Setlist(caller, bandID, setlistID)
	if err != nil {
		return nil, "", err
	}

	pdf, err := setlistpdf.Render(buildSetlistDoc(band.Name, det))
	if err != nil {
		return nil, "", err
	}
	return pdf, sanitizeFilename(band.Name+" - "+det.Setlist.Name) + ".pdf", nil
}

// buildSetlistDoc turns a resolved setlist into the printable Doc: it numbers the
// running order via the ONE shared rule and maps each item to a row. Extracted from
// ExportSetlistPDF as the testable seam T158's review flagged — the vectors prove
// Numbers(), but nothing exercised this mapping, which is where an intermission was
// silently numbered as a song (T153). det.Items is already main-then-bench in
// Position order.
func buildSetlistDoc(bandName string, det SetlistDetail) setlistpdf.Doc {
	entries := make([]runningorder.Entry, len(det.Items))
	for i, it := range det.Items {
		entries[i] = runningorder.Entry{Kind: entryKind(it), OnCall: it.OnCall}
	}
	nums := runningorder.Numbers(entries)

	var main, bench []setlistpdf.Row
	for i, it := range det.Items {
		row := setlistpdf.Row{Number: nums[i], Title: it.SongTitle, Kind: rowKind(it)}
		if it.OnCall {
			bench = append(bench, row)
		} else {
			main = append(main, row)
		}
	}

	return setlistpdf.Doc{
		BandName:    bandName,
		SetlistName: det.Setlist.Name,
		Venue:       det.Setlist.Venue,
		EventDate:   det.Setlist.EventDate,
		Main:        main,
		OnCall:      bench,
	}
}

// entryKind / rowKind map a setlist item to the kind its two consumers understand,
// through the single IsIntermission interpreter so "absent ⇒ song" is stated once and
// the export can never again number a break as a song (T153).
func entryKind(it SetlistItemView) string {
	if it.IsIntermission() {
		return runningorder.KindIntermission
	}
	return runningorder.KindSong
}

func rowKind(it SetlistItemView) string {
	if it.IsIntermission() {
		return setlistpdf.KindIntermission
	}
	return setlistpdf.KindSong
}
