package app

import (
	"fmt"

	"troubastack/core/internal/domain"
)

// SetFileCrop saves a per-page bake area on an uploaded PDF (T198). Any band member may set it — the same
// level as uploading or deleting the file, which change the bake far more (Fable). A Generated chart is
// refused (P4: it already lays out to the page). The crop is validated (geometry, canonical page keys), its
// aspect-ratio lock is forced on (policy A), and it is persisted; the caller fires the autobaker so a live
// setlist re-bakes. Passing crop with no pages (or an empty Crop) CLEARS the crop — a Reset.
func (s *Service) SetFileCrop(caller User, bandID, songID, fileID string, crop Crop) (SongFile, error) {
	f, err := s.cropTarget(caller, bandID, songID, fileID)
	if err != nil {
		return SongFile{}, err
	}
	if err := normalizeCrop(&crop); err != nil {
		return SongFile{}, err
	}
	if len(crop.Pages) == 0 && !crop.Auto {
		f.Crop = nil // Reset — back to the whole-page bake, byte-identical (P1)
	} else {
		c := crop
		f.Crop = &c
	}
	if err := s.repo.UpdateSongFile(f); err != nil {
		return SongFile{}, err
	}
	return f, nil
}

// PreviewFileCrop is the crop-save DRY RUN: it resolves and validates a proposed crop EXACTLY as SetFileCrop
// would (same member gate, same P4 refusal, same geometry check) but persists nothing, and returns the live
// marks the crop would hide on the tablet. Because it shares cropTarget + normalizeCrop with the save, a
// preview that returns marks is a crop the save will accept — the warning and the result cannot diverge.
// snap is the song's current annotation HEAD (the caller holds the engine; the service stays engine-free).
func (s *Service) PreviewFileCrop(caller User, bandID, songID, fileID string, crop Crop, snap domain.Snapshot) ([]HiddenMark, error) {
	if _, err := s.cropTarget(caller, bandID, songID, fileID); err != nil {
		return nil, err
	}
	if err := normalizeCrop(&crop); err != nil {
		return nil, err
	}
	return CropHiddenMarks(snap, fileID, crop), nil
}

// CropDetectSource returns an uploaded PDF's bytes for an autodetect proposal, enforcing the SAME gate as a
// crop save (member via cropTarget + P4 refusal of a generated chart). Read-only — the caller (httpapi) runs
// detection through the baker and proposes rects the user then saves through the normal crop save.
func (s *Service) CropDetectSource(caller User, bandID, songID, fileID string) ([]byte, error) {
	f, err := s.cropTarget(caller, bandID, songID, fileID)
	if err != nil {
		return nil, err
	}
	data, err := s.blobs.Get(f.BlobHash)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// cropTarget resolves the uploaded PDF a crop applies to, enforcing the member gate (GetBand: any member may
// crop) and P4 (a generated chart is refused — it already lays out to the page). Shared by the save and its
// dry run so both accept and refuse exactly the same files.
func (s *Service) cropTarget(caller User, bandID, songID, fileID string) (SongFile, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return SongFile{}, err
	}
	f, err := s.repo.GetSongFile(fileID)
	if err != nil || f.BandID != bandID || f.SongID != songID {
		return SongFile{}, ErrNotFound
	}
	if f.Generated {
		return SongFile{}, fmt.Errorf("%w: a generated chart has no crop — it already lays out to the page", ErrInvalidInput)
	}
	return f, nil
}

// normalizeCrop forces policy A (LockAR: a crop is a pure zoom) on and defaults each rect's source to manual
// — autodetect is a separate task, so a manual save never turns Auto on and any rect it carries is manual —
// then validates geometry + canonical page keys. Page-index RANGE is validated at bake (the service has no
// page count without rendering); an out-of-range index is harmlessly ignored by the rasterizer. Mutates in
// place and is shared by the save and its dry run so a preview and a save agree on what is valid.
func normalizeCrop(crop *Crop) error {
	crop.LockAR = true
	crop.Auto = false // a manual save never turns autodetect on; that is a separate task's path.
	for key, r := range crop.Pages {
		if r.Source == "" {
			r.Source = "manual"
			crop.Pages[key] = r
		}
	}
	return ValidateCrop(*crop, 0)
}

// CropHiddenMarks returns the live marks on this file's cropped pages whose extent-aware box falls entirely
// outside the proposed crop — the ones the bake would NOT draw on the tablet, for the crop-save warning. It
// uses the SAME predicate the bake drop uses (CropHidesMark), so the warning can never disagree with the
// result. snap is the song's current annotation HEAD; fileByLayer maps a layer to its file so only marks on
// THIS file's cropped pages are considered.
func CropHiddenMarks(snap domain.Snapshot, fileID string, crop Crop) []HiddenMark {
	fileByLayer := map[string]string{}
	for _, l := range snap.Layers {
		fileByLayer[l.ID] = l.FileID
	}
	var hidden []HiddenMark
	for _, o := range snap.LiveObjects() {
		lf, ok := fileByLayer[o.LayerID]
		if !ok {
			continue
		}
		// A layer with no explicit file belongs to a single-file song's only file; treat it as this file.
		if lf != "" && lf != fileID {
			continue
		}
		rect, cropped := crop.Pages[pageKey(o.Page)]
		if !cropped {
			continue // an uncropped page hides nothing
		}
		pts := make([][2]float64, len(o.Points))
		for i, p := range o.Points {
			pts[i] = [2]float64{p.X, p.Y}
		}
		if CropHidesMark(pts, o.Style.Width, o.Style.FontSize, o.Type == domain.TypeText, o.Text, rect.X, rect.Y, rect.W, rect.H) {
			hidden = append(hidden, HiddenMark{UUID: o.UUID, LayerID: o.LayerID, Page: o.Page})
		}
	}
	return hidden
}

// HiddenMark names one mark a crop would hide on the tablet (no coordinates — just enough for Studio to point
// at it in the layer list).
type HiddenMark struct {
	UUID    string `json:"uuid"`
	LayerID string `json:"layerId"`
	Page    int    `json:"page"`
}

func pageKey(page int) string {
	return fmt.Sprintf("%d", page)
}
