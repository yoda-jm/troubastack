package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"troubastack/core/internal/app"
	"troubastack/core/internal/engine"
)

// BandIOAPI is the HTTP edge for T62 whole-band export/import. It needs both the
// relational Service and the annotation engine (a band export carries annotations),
// so it lives beside the AnnotationsAPI rather than on the plain WebAPI.
type BandIOAPI struct {
	svc *app.Service
	eng *engine.Engine
}

func NewBandIOAPI(svc *app.Service, eng *engine.Engine) *BandIOAPI {
	return &BandIOAPI{svc: svc, eng: eng}
}

func (a *BandIOAPI) Mount(mux *http.ServeMux, authed func(authedHandler) http.HandlerFunc) {
	// Export a whole band as a .tband zip (admin-only, gated in the service).
	mux.HandleFunc("GET /api/bands/{bandId}/export", authed(a.exportBand))
	// Preview a .tband: classify its members (matched vs missing) without writing (T63).
	mux.HandleFunc("POST /api/bands/import:preview", authed(a.previewImport))
	// Import a .tband zip → a NEW band owned by the caller (any authenticated user).
	mux.HandleFunc("POST /api/bands/import", authed(a.importBand))
}

// exportBand streams the band's .tband zip as an attachment.
func (a *BandIOAPI) exportBand(w http.ResponseWriter, r *http.Request, u app.User) {
	data, filename, err := a.svc.ExportBand(u, a.eng, r.PathValue("bandId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition("attachment", filename, "band.tband"))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// readImportZip parses the multipart form and returns the uploaded .tband bytes (the
// "file" field), size-capped. On any error it writes a 400 and returns ok=false.
//
// SECURITY (T63): the account-takeover hold that disabled this route (bandImportDisabled,
// 2026-07-26) is lifted because import is now CONSENT-REQUIRED — a pre-existing account is
// never silently attached (svc.ImportBand invites or skips it), so the manifest-names-a-
// victim → admin-reset chain is closed. See reviews.md.
func readImportZip(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, app.MaxImportBytes+(1<<20))
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeErr(w, app.ErrInvalidInput)
		return nil, false
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, app.ErrInvalidInput)
		return nil, false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, app.MaxImportBytes+1))
	if err != nil || len(data) > app.MaxImportBytes {
		writeErr(w, app.ErrInvalidInput)
		return nil, false
	}
	return data, true
}

// previewImport classifies the manifest's members (matched vs missing) without writing (T63).
func (a *BandIOAPI) previewImport(w http.ResponseWriter, r *http.Request, u app.User) {
	data, ok := readImportZip(w, r)
	if !ok {
		return
	}
	pv, err := a.svc.PreviewImport(u, data)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pv)
}

// importBand accepts a multipart .tband zip (+ an optional per-missing-member disposition
// map, "dispositions": {username: create|invite|skip}) and creates a new band from it.
func (a *BandIOAPI) importBand(w http.ResponseWriter, r *http.Request, u app.User) {
	data, ok := readImportZip(w, r)
	if !ok {
		return
	}
	dispositions := map[string]app.ImportDisposition{}
	if raw := r.FormValue("dispositions"); raw != "" {
		var m map[string]string
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			writeErr(w, app.ErrInvalidInput)
			return
		}
		for k, v := range m {
			dispositions[k] = app.ImportDisposition(v)
		}
	}
	report, err := a.svc.ImportBand(u, a.eng, data, dispositions)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, report)
}
