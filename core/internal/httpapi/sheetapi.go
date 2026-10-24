package httpapi

import (
	"net/http"
	"strconv"

	"troubastack/core/internal/app"
	"troubastack/core/internal/soundsheet"
)

// T203 stage 4 — the sound sheet. Go is authoritative: GET /sheet returns the model as JSON; /sheet.pdf and
// /sheet.csv render the SAME model so screen = PDF = CSV. The per-concert patch is edited here (admin).

func sheetOrder(r *http.Request) app.SheetOrder {
	if r.URL.Query().Get("order") == string(app.SheetOrderPatch) {
		return app.SheetOrderPatch
	}
	return app.SheetOrderStage
}

func (a *WebAPI) soundSheet(w http.ResponseWriter, r *http.Request, u app.User) {
	m, err := a.svc.BuildSheet(u, r.PathValue("bandId"), r.PathValue("setlistId"), sheetOrder(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (a *WebAPI) soundSheetPDF(w http.ResponseWriter, r *http.Request, u app.User) {
	m, err := a.svc.BuildSheet(u, r.PathValue("bandId"), r.PathValue("setlistId"), sheetOrder(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	pdf, err := soundsheet.RenderPDF(m)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", contentDisposition("attachment", m.SetlistName+" - sound sheet.pdf", "sound-sheet.pdf"))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
	_, _ = w.Write(pdf)
}

func (a *WebAPI) soundSheetCSV(w http.ResponseWriter, r *http.Request, u app.User) {
	m, err := a.svc.BuildSheet(u, r.PathValue("bandId"), r.PathValue("setlistId"), sheetOrder(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	csv := soundsheet.RenderCSV(m)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", contentDisposition("attachment", m.SetlistName+" - sound sheet.csv", "sound-sheet.csv"))
	w.Header().Set("Content-Length", strconv.Itoa(len(csv)))
	_, _ = w.Write(csv)
}

func (a *WebAPI) setPatch(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Patch []app.ChannelPatch `json:"patch"`
	}
	if !decode(w, r, &in) {
		return
	}
	sl, err := a.svc.SetPatch(u, r.PathValue("bandId"), r.PathValue("setlistId"), in.Patch)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"patch": sl.Patch})
}

func (a *WebAPI) copyPatch(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		From string `json:"from"`
	}
	if !decode(w, r, &in) {
		return
	}
	sl, err := a.svc.CopyPatchFrom(u, r.PathValue("bandId"), r.PathValue("setlistId"), in.From)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"patch": sl.Patch})
}

func (a *WebAPI) autoNumberPatch(w http.ResponseWriter, r *http.Request, u app.User) {
	sl, err := a.svc.AutoNumber(u, r.PathValue("bandId"), r.PathValue("setlistId"), sheetOrder(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"patch": sl.Patch})
}
