package httpapi

import (
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
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// importBand accepts a multipart .tband zip and creates a new band from it.
// bandImportDisabled is a SECURITY HOLD (2026-07-26, Fable). A deep audit found a
// critical account-takeover chain: import silently attaches an EXISTING account
// (matched by username) to the importer's new band as a member with NO consent, and
// the importer is that band's admin; the admin password-reset (IssuePasswordReset)
// then returns a plaintext reset token for any "member" of a band you admin. So any
// authenticated user can take over any account by importing a manifest naming that
// username. Flip to false only once import requires consent for pre-existing accounts
// (the T63 invite-on-import model). See reviews.md 2026-07-26. (A var, not a const, so
// the real handler body below stays reachable to the compiler/vet.)
var bandImportDisabled = true

func (a *BandIOAPI) importBand(w http.ResponseWriter, r *http.Request, u app.User) {
	if bandImportDisabled {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "band import is temporarily disabled pending a security fix",
		})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, app.MaxImportBytes+(1<<20))
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeErr(w, app.ErrInvalidInput)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, app.ErrInvalidInput)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, app.MaxImportBytes+1))
	if err != nil {
		writeErr(w, app.ErrInvalidInput)
		return
	}
	if len(data) > app.MaxImportBytes {
		writeErr(w, app.ErrInvalidInput)
		return
	}
	report, err := a.svc.ImportBand(u, a.eng, data)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, report)
}
