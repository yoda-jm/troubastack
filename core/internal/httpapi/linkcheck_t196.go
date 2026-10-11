package httpapi

import (
	"net/http"

	"troubastack/core/internal/app"
	"troubastack/core/internal/domain"
)

// T196 — anchoring stage 3: the PRE-BAKE LIST (critique.md §7 "Stage 3", P29, P34). The tablet cannot show
// a mark's link state, so before a bake Studio lists the songs and files whose marks need a look — Check
// or Lost — with counts. It INFORMS; it never blocks the bake (the bake endpoint does not consult it).
//
// GET /api/bands/{bandId}/setlists/{setlistId}/link-check — member-gated like every read it is made of
// (the setlist, each song's files, each song's annotations). The states are the SERVED ones: every song's
// snapshot goes through projectSnapshot, the same serve projection GET /annotations and the bake use, so a
// mark's state here is the state Studio shows for it. It counts EVERY layer (the bake bakes every layer);
// Studio's per-file counter counts the layers the writer has visible, so it can show fewer. Old-style marks
// (no stage-2 link, shown as Fixed) and marks on uploaded files are never counted; tombstones are not marks.

type linkCheckFileJSON struct {
	FileID   string `json:"fileId"`
	Filename string `json:"filename"`
	Check    int    `json:"check"`
	Lost     int    `json:"lost"`
}

type linkCheckSongJSON struct {
	SongID string              `json:"songId"`
	Title  string              `json:"title"`
	Files  []linkCheckFileJSON `json:"files"`
}

type linkCheckJSON struct {
	// Songs lists ONLY songs with at least one Check or Lost mark, in setlist order; each lists only its
	// files with one. Empty when there is nothing to look at.
	Songs []linkCheckSongJSON `json:"songs"`
	Check int                 `json:"check"` // totals over Songs
	Lost  int                 `json:"lost"`
}

func (a *AnnotationsAPI) linkCheck(w http.ResponseWriter, r *http.Request, u app.User) {
	bandID, setlistID := r.PathValue("bandId"), r.PathValue("setlistId")
	detail, err := a.svc.Setlist(u, bandID, setlistID)
	if err != nil {
		writeErr(w, err)
		return
	}
	out := linkCheckJSON{Songs: []linkCheckSongJSON{}}
	seen := map[string]bool{}
	for _, item := range detail.Items {
		if item.IsIntermission() || item.SongID == "" || seen[item.SongID] {
			continue
		}
		seen[item.SongID] = true
		row, ok := a.songLinkCheck(u, bandID, item.SongID, item.SongTitle)
		if !ok {
			continue
		}
		for _, f := range row.Files {
			out.Check += f.Check
			out.Lost += f.Lost
		}
		out.Songs = append(out.Songs, row)
	}
	writeJSON(w, http.StatusOK, out)
}

// songLinkCheck counts one song's Check and Lost marks per generated file. ok is false when it has none
// (or cannot be read: best-effort, an unreadable song is simply not listed — the list never blocks).
func (a *AnnotationsAPI) songLinkCheck(u app.User, bandID, songID, title string) (linkCheckSongJSON, bool) {
	files, err := a.svc.SongFiles(u, bandID, songID)
	if err != nil {
		return linkCheckSongJSON{}, false
	}
	snap, err := a.eng.Head(songID)
	if err != nil {
		return linkCheckSongJSON{}, false
	}
	served := projectSnapshot(snap, a.chartRenderFor(u, bandID, songID))
	return countLinkStates(songID, title, files, served)
}

// countLinkStates is the pure half of songLinkCheck: per generated file of files, in file order, the live
// marks of served whose link state is check or lost.
func countLinkStates(songID, title string, files []app.SongFile, served domain.Snapshot) (linkCheckSongJSON, bool) {
	fileOfLayer := make(map[string]string, len(served.Layers))
	for _, l := range served.Layers {
		fileOfLayer[l.ID] = l.FileID
	}
	type counts struct{ check, lost int }
	byFile := map[string]*counts{}
	for _, o := range served.Objects {
		if o.Deleted || o.Link == nil {
			continue
		}
		fid := fileOfLayer[o.LayerID]
		if fid == "" {
			continue
		}
		c := byFile[fid]
		if c == nil {
			c = &counts{}
			byFile[fid] = c
		}
		switch o.Link.State {
		case domain.LinkCheck:
			c.check++
		case domain.LinkLost:
			c.lost++
		}
	}
	row := linkCheckSongJSON{SongID: songID, Title: title}
	for _, f := range files {
		c := byFile[f.ID]
		if !f.Generated || c == nil || c.check+c.lost == 0 {
			continue
		}
		row.Files = append(row.Files, linkCheckFileJSON{FileID: f.ID, Filename: f.Filename, Check: c.check, Lost: c.lost})
	}
	return row, len(row.Files) > 0
}
