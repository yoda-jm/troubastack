package httpapi

import (
	"net/http"

	"troubastack/core/internal/app"
)

// T204 — the band's custom channel (console-track) groups. Create/rename/delete are admin; a part's channel is
// set by SetPartChannel (admin), which re-keys + merges per-concert patches and returns any merge notes.

func (a *WebAPI) listChannels(w http.ResponseWriter, r *http.Request, u app.User) {
	cs, err := a.svc.Channels(u, r.PathValue("bandId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": cs})
}

func (a *WebAPI) createChannel(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	c, err := a.svc.CreateChannel(u, r.PathValue("bandId"), in.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"channel": c})
}

func (a *WebAPI) renameChannel(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	c, err := a.svc.RenameChannel(u, r.PathValue("bandId"), r.PathValue("channelId"), in.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channel": c})
}

func (a *WebAPI) deleteChannel(w http.ResponseWriter, r *http.Request, u app.User) {
	force := r.URL.Query().Get("force") == "1"
	refs, notes, err := a.svc.DeleteChannel(u, r.PathValue("bandId"), r.PathValue("channelId"), force)
	if err != nil {
		if refs > 0 {
			writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "parts": refs})
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"merged": notes}) // any patch merges from reverting parts
}

func (a *WebAPI) setPartChannel(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		ChannelID string `json:"channelId"` // "" clears (part reverts to its own track)
	}
	if !decode(w, r, &in) {
		return
	}
	notes, err := a.svc.SetPartChannel(u, r.PathValue("bandId"), r.PathValue("partId"), in.ChannelID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"merged": notes})
}
