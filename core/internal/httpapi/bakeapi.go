package httpapi

import (
	"net/http"
	"os"

	"troubastack/core/internal/app"
	"troubastack/core/internal/bake"
)

// BakeAPI is the HTTP edge for bake orchestration (I11): an admin bakes a setlist
// into a downloadable .tstage, and band members list + download baked concerts.
// The actual flattening lives in internal/bake (which shells out to poppler +
// web/bake); this adapter does auth/scoping and streams the result.
//
// A concert's id IS its setlist id (a setlist bakes to one concert, rev-bumped per
// bake). Scoping: bake is admin-only (mirrors T08's import gate); list/download are
// member-only and filtered to setlists that belong to the caller's band.
type BakeAPI struct {
	svc   *app.Service
	baker *bake.Baker
}

func NewBakeAPI(svc *app.Service, baker *bake.Baker) *BakeAPI {
	return &BakeAPI{svc: svc, baker: baker}
}

func (a *BakeAPI) Mount(mux *http.ServeMux, authed func(authedHandler) http.HandlerFunc) {
	mux.HandleFunc("POST /api/bands/{bandId}/setlists/{setlistId}/bake", authed(a.bake))
	mux.HandleFunc("GET /api/bands/{bandId}/concerts", authed(a.listConcerts))
	mux.HandleFunc("GET /api/bands/{bandId}/concerts/{concertId}/bundle", authed(a.downloadBundle))
}

// concertView is the client-facing projection of a baked concert (no page/overlay
// detail — that's inside the .tstage). REST uses plain JSON numbers (unlike the
// container's canonical string-encoded 64-bit ints).
type concertView struct {
	ConcertID   string `json:"concertId"`
	Name        string `json:"name"`
	ConcertRev  uint64 `json:"concertRev"`
	BakedAt     int64  `json:"bakedAt"`
	BakedBy     string `json:"bakedBy"`
	Songs       int    `json:"songs"`
	DownloadURL string `json:"downloadUrl"`
}

func viewOf(bandID string, cb bake.ConcertBundle) concertView {
	return concertView{
		ConcertID:   cb.ConcertID,
		Name:        cb.Name,
		ConcertRev:  cb.ConcertRev,
		BakedAt:     cb.BakedAt,
		BakedBy:     cb.BakedBy,
		Songs:       len(cb.Songs),
		DownloadURL: "/api/bands/" + bandID + "/concerts/" + cb.ConcertID + "/bundle",
	}
}

// bake mints a new revision of a setlist's concert. ADMIN-only (I11).
func (a *BakeAPI) bake(w http.ResponseWriter, r *http.Request, u app.User) {
	if a.baker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "bake not configured"})
		return
	}
	bandID := r.PathValue("bandId")
	// Admin gate, same pattern as T08's annotation import.
	if _, role, err := a.svc.GetBand(u, bandID); err != nil {
		writeErr(w, err)
		return
	} else if role != app.RoleAdmin {
		writeErr(w, app.ErrForbidden)
		return
	}
	cb, err := a.baker.Bake(r.Context(), bandID, r.PathValue("setlistId"), u)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewOf(bandID, cb))
}

// listConcerts returns the baked concerts whose setlist belongs to this band.
func (a *BakeAPI) listConcerts(w http.ResponseWriter, r *http.Request, u app.User) {
	bandID := r.PathValue("bandId")
	setlists, err := a.svc.Setlists(u, bandID) // member-scoped; 403/404 if not our band
	if err != nil {
		writeErr(w, err)
		return
	}
	inBand := make(map[string]bool, len(setlists))
	for _, sl := range setlists {
		inBand[sl.ID] = true
	}
	views := []concertView{}
	if a.baker != nil {
		for _, cb := range a.baker.ListConcerts() {
			if inBand[cb.ConcertID] {
				views = append(views, viewOf(bandID, cb))
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"concerts": views})
}

// downloadBundle streams a concert's latest .tstage. Member-only; the setlist must
// belong to the caller's band (svc.Setlist enforces that scope).
func (a *BakeAPI) downloadBundle(w http.ResponseWriter, r *http.Request, u app.User) {
	bandID := r.PathValue("bandId")
	concertID := r.PathValue("concertId")
	if _, err := a.svc.Setlist(u, bandID, concertID); err != nil {
		writeErr(w, err) // not our band / unknown setlist → 403/404
		return
	}
	if a.baker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "bake not configured"})
		return
	}
	path := a.baker.BundlePath(concertID)
	if path == "" {
		writeErr(w, app.ErrNotFound)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeErr(w, app.ErrNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+concertID+`.tstage"`)
	http.ServeContent(w, r, concertID+".tstage", info.ModTime(), f)
}
