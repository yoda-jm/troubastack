package httpapi

import (
	"net/http"

	"troubastack/core/internal/app"
)

// T203 — the band's line-up vocabulary: musicians (roster) and parts. Line-up edits themselves ride the song
// PATCH (updateSong). The roster + part metadata are admin; a member may create a part.

func (a *WebAPI) listMusicians(w http.ResponseWriter, r *http.Request, u app.User) {
	ms, err := a.svc.Musicians(u, r.PathValue("bandId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"musicians": ms})
}

func (a *WebAPI) addMusician(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	m, err := a.svc.AddMusician(u, r.PathValue("bandId"), in.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"musician": m})
}

func (a *WebAPI) updateMusician(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Name  *string `json:"name"`
		Order *int    `json:"order"`
	}
	if !decode(w, r, &in) {
		return
	}
	m, err := a.svc.UpdateMusician(u, r.PathValue("bandId"), r.PathValue("musicianId"), app.MusicianPatch{Name: in.Name, Order: in.Order})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"musician": m})
}

func (a *WebAPI) linkMusician(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		UserID string `json:"userId"`
	}
	if !decode(w, r, &in) {
		return
	}
	m, err := a.svc.LinkMusician(u, r.PathValue("bandId"), r.PathValue("musicianId"), in.UserID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"musician": m})
}

func (a *WebAPI) deleteMusician(w http.ResponseWriter, r *http.Request, u app.User) {
	force := r.URL.Query().Get("force") == "1"
	refs, err := a.svc.DeleteMusician(u, r.PathValue("bandId"), r.PathValue("musicianId"), force)
	if err != nil {
		if refs > 0 { // referenced + not forced: report the count so the client can offer "remove from N songs"
			writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "songs": refs})
			return
		}
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *WebAPI) listParts(w http.ResponseWriter, r *http.Request, u app.User) {
	ps, err := a.svc.Parts(u, r.PathValue("bandId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"parts": ps})
}

func (a *WebAPI) createPart(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	p, err := a.svc.CreatePart(u, r.PathValue("bandId"), in.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"part": p})
}

func (a *WebAPI) updatePart(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Name   *string `json:"name"`
		Kind   *string `json:"kind"`
		Short  *string `json:"short"`
		Input  *string `json:"input"`
		Shared *bool   `json:"shared"`
		Order  *int    `json:"order"`
	}
	if !decode(w, r, &in) {
		return
	}
	p, err := a.svc.UpdatePart(u, r.PathValue("bandId"), r.PathValue("partId"), app.PartPatch{
		Name: in.Name, Kind: in.Kind, Short: in.Short, Input: in.Input, Shared: in.Shared, Order: in.Order,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"part": p})
}

func (a *WebAPI) mergeParts(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := a.svc.MergeParts(u, r.PathValue("bandId"), in.From, in.To); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *WebAPI) deletePart(w http.ResponseWriter, r *http.Request, u app.User) {
	force := r.URL.Query().Get("force") == "1"
	refs, err := a.svc.DeletePart(u, r.PathValue("bandId"), r.PathValue("partId"), force)
	if err != nil {
		if refs > 0 {
			writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "songs": refs})
			return
		}
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
