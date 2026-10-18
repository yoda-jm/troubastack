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

// detectCrop proposes autodetected crops for an uploaded PDF's pages (T198 4b), READ-ONLY — it writes
// nothing. Same member gate + P4 refusal as the save (via CropDetectSource). It renders the pages through
// the bake path and runs the content detector, then returns the proposed rects AND the marks they would hide
// (PreviewFileCrop), so Studio shows the proposal with its warning. The user commits a proposal through the
// normal crop save with source "auto"; a manual rect is never overwritten here. Detection is never a bake.
//
//	POST /api/bands/{bandId}/songs/{songId}/files/{fileId}/crop-detect
//	{ "pages": [0,2] }  — or  { "all": true }
//	→ { "pages": {"0": {x,y,w,h,source:"auto"}, …}, "hiddenMarks": [...] }
func (a *AnnotationsAPI) detectCrop(w http.ResponseWriter, r *http.Request, u app.User) {
	bandID := r.PathValue("bandId")
	song, err := a.svc.SongForMember(u, bandID, r.PathValue("songId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	fileID := r.PathValue("fileId")
	var in struct {
		Pages []int `json:"pages"`
		All   bool  `json:"all,omitempty"`
	}
	if !decode(w, r, &in) {
		return
	}
	if a.baker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "autodetect needs the bake toolchain, which this server does not have"})
		return
	}
	// Gate (member + P4) and fetch the PDF bytes.
	pdf, err := a.svc.CropDetectSource(u, bandID, song.ID, fileID)
	if err != nil {
		writeErr(w, err)
		return
	}
	want := map[int]bool{}
	for _, p := range in.Pages {
		want[p] = true
	}
	rects, err := a.baker.DetectCropForPages(r.Context(), pdf, want, in.All)
	if err != nil {
		writeErr(w, err)
		return
	}
	// The proposal's hidden marks, by the same predicate the save/warning use. The proposed rects are square
	// (policy A) and ≥ the min side, so PreviewFileCrop's validation accepts them.
	snap, err := a.eng.Head(song.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	hidden, err := a.svc.PreviewFileCrop(u, bandID, song.ID, fileID, app.Crop{LockAR: true, Pages: rects}, snap)
	if err != nil {
		writeErr(w, err)
		return
	}
	if hidden == nil {
		hidden = []app.HiddenMark{}
	}
	if rects == nil {
		rects = map[string]app.CropRect{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": rects, "hiddenMarks": hidden})
}
