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

	// Number the running order via the shared rule (det.Items is already main-then-bench, in Position
	// order). No intermission field on the model yet (T153); every real item is a song — when T153 lands,
	// map its kind here and both the rule and the renderer already handle KindIntermission.
	entries := make([]runningorder.Entry, len(det.Items))
	for i, it := range det.Items {
		entries[i] = runningorder.Entry{Kind: runningorder.KindSong, OnCall: it.OnCall}
	}
	nums := runningorder.Numbers(entries)

	var main, bench []setlistpdf.Row
	for i, it := range det.Items {
		row := setlistpdf.Row{Number: nums[i], Title: it.SongTitle, Kind: setlistpdf.KindSong}
		if it.OnCall {
			bench = append(bench, row)
		} else {
			main = append(main, row)
		}
	}

	pdf, err := setlistpdf.Render(setlistpdf.Doc{
		BandName:    band.Name,
		SetlistName: det.Setlist.Name,
		Venue:       det.Setlist.Venue,
		EventDate:   det.Setlist.EventDate,
		Main:        main,
		OnCall:      bench,
	})
	if err != nil {
		return nil, "", err
	}
	return pdf, sanitizeFilename(band.Name+" - "+det.Setlist.Name) + ".pdf", nil
}
