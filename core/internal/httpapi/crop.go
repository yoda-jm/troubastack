package httpapi

import (
	"net/http"

	"troubastack/core/internal/app"
)

// setCrop saves — or, with dryRun, previews — a per-page bake crop on an uploaded PDF (T198). It is a
// member-level write: any band member may crop, the same bar as uploading or deleting the file (which change
// the bake far more). The crop is non-destructive (P1): the PDF is untouched and an empty crop resets the
// file to a whole-page bake.
//
//	PUT /api/bands/{bandId}/songs/{songId}/files/{fileId}/crop
//	{ "crop": { "lockAR": true, "pages": {"0": {"x":.1,"y":.1,"w":.8,"h":.8}} }, "dryRun": false }
//
// dryRun=true persists nothing and returns {"hiddenMarks": [...]}: the live marks the crop would push off
// the tablet, computed with the SAME predicate the bake drop uses, so Studio can warn before committing.
// A save (dryRun=false) persists via SetFileCrop, fires the autobaker so a live setlist re-bakes, and
// returns {"file": <the updated file>} with its crop. Both paths validate identically (PreviewFileCrop and
// SetFileCrop share the service gate), so a preview that returns marks is a crop the save will accept.
func (a *AnnotationsAPI) setCrop(w http.ResponseWriter, r *http.Request, u app.User) {
	bandID := r.PathValue("bandId")
	song, err := a.svc.SongForMember(u, bandID, r.PathValue("songId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	var in struct {
		Crop   app.Crop `json:"crop"`
		DryRun bool     `json:"dryRun,omitempty"`
	}
	if !decode(w, r, &in) {
		return
	}
	fileID := r.PathValue("fileId")

	if in.DryRun {
		snap, err := a.eng.Head(song.ID)
		if err != nil {
			writeErr(w, err)
			return
		}
		hidden, err := a.svc.PreviewFileCrop(u, bandID, song.ID, fileID, in.Crop, snap)
		if err != nil {
			writeErr(w, err)
			return
		}
		if hidden == nil {
			hidden = []app.HiddenMark{} // a normalized [] on the wire, never null
		}
		writeJSON(w, http.StatusOK, map[string]any{"hiddenMarks": hidden})
		return
	}

	f, err := a.svc.SetFileCrop(u, bandID, song.ID, fileID, in.Crop)
	if err != nil {
		writeErr(w, err)
		return
	}
	if a.notify != nil {
		a.notify(song.ID) // a live setlist containing this song re-bakes with the new crop
	}
	writeJSON(w, http.StatusOK, map[string]any{"file": f})
}
