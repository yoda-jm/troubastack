package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"troubastack/core/internal/app"
)

// sessionCookie is the cookie name carrying the opaque session token.
const sessionCookie = "trouba_session"

// WebAPI is the HTTP edge for the relational ("normal web") domain: auth, bands,
// members, invites, songs. It is a thin adapter — it decodes JSON, resolves the
// session cookie to a user, calls app.Service (which owns ALL policy), and maps
// app sentinel errors to status codes. No business logic lives here (I14).
type WebAPI struct {
	svc    *app.Service
	secure bool // set Secure on the session cookie (true behind TLS)
}

// NewWebAPI builds the relational API adapter over a Service.
func NewWebAPI(svc *app.Service, secureCookies bool) *WebAPI {
	return &WebAPI{svc: svc, secure: secureCookies}
}

// Mount registers the /api/* routes on mux. Go 1.22+ method+wildcard patterns
// give us a real router with no third-party dependency.
func (a *WebAPI) Mount(mux *http.ServeMux) {
	// Public (no session required).
	mux.HandleFunc("POST /api/auth/register", a.register)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/logout", a.logout)

	// Authenticated.
	mux.HandleFunc("GET /api/me", a.auth(a.me))
	mux.HandleFunc("GET /api/bands", a.auth(a.listBands))
	mux.HandleFunc("POST /api/bands", a.auth(a.createBand))
	mux.HandleFunc("GET /api/bands/{bandId}", a.auth(a.getBand))
	mux.HandleFunc("PATCH /api/bands/{bandId}", a.auth(a.updateBand))
	mux.HandleFunc("DELETE /api/bands/{bandId}", a.auth(a.deleteBand))
	mux.HandleFunc("GET /api/bands/{bandId}/members", a.auth(a.listMembers))
	mux.HandleFunc("PATCH /api/bands/{bandId}/members/{userId}", a.auth(a.updateMember))
	mux.HandleFunc("DELETE /api/bands/{bandId}/members/{userId}", a.auth(a.removeMember))
	mux.HandleFunc("POST /api/bands/{bandId}/leave", a.auth(a.leaveBand))
	mux.HandleFunc("POST /api/bands/{bandId}/invites", a.auth(a.createInvite))
	mux.HandleFunc("GET /api/bands/{bandId}/invites", a.auth(a.listBandInvites))
	mux.HandleFunc("DELETE /api/bands/{bandId}/invites/{inviteId}", a.auth(a.revokeInvite))
	mux.HandleFunc("GET /api/bands/{bandId}/songs", a.auth(a.listSongs))
	mux.HandleFunc("POST /api/bands/{bandId}/songs", a.auth(a.createSong))
	mux.HandleFunc("PATCH /api/bands/{bandId}/songs/{songId}", a.auth(a.updateSong))
	mux.HandleFunc("DELETE /api/bands/{bandId}/songs/{songId}", a.auth(a.deleteSong))
	mux.HandleFunc("POST /api/bands/{bandId}/songs/{songId}/files", a.auth(a.uploadFile))
	mux.HandleFunc("GET /api/bands/{bandId}/songs/{songId}/files", a.auth(a.listFiles))
	mux.HandleFunc("PATCH /api/bands/{bandId}/songs/{songId}/files/{fileId}", a.auth(a.updateFile))
	mux.HandleFunc("DELETE /api/bands/{bandId}/songs/{songId}/files/{fileId}", a.auth(a.deleteFile))
	mux.HandleFunc("GET /api/files/{fileId}", a.auth(a.downloadFile))
	mux.HandleFunc("GET /api/bands/{bandId}/setlists", a.auth(a.listSetlists))
	mux.HandleFunc("POST /api/bands/{bandId}/setlists", a.auth(a.createSetlist))
	mux.HandleFunc("GET /api/bands/{bandId}/setlists/{setlistId}", a.auth(a.getSetlist))
	mux.HandleFunc("PATCH /api/bands/{bandId}/setlists/{setlistId}", a.auth(a.updateSetlist))
	mux.HandleFunc("DELETE /api/bands/{bandId}/setlists/{setlistId}", a.auth(a.deleteSetlist))
	mux.HandleFunc("POST /api/bands/{bandId}/setlists/{setlistId}/items", a.auth(a.addSetlistItem))
	mux.HandleFunc("PATCH /api/bands/{bandId}/setlists/{setlistId}/items/{itemId}", a.auth(a.updateSetlistItem))
	mux.HandleFunc("DELETE /api/bands/{bandId}/setlists/{setlistId}/items/{itemId}", a.auth(a.removeSetlistItem))
	mux.HandleFunc("POST /api/bands/{bandId}/setlists/{setlistId}/reorder", a.auth(a.reorderSetlist))
	mux.HandleFunc("GET /api/invites", a.auth(a.listInvites))
	mux.HandleFunc("POST /api/invites/{inviteId}/accept", a.auth(a.acceptInvite))
	mux.HandleFunc("POST /api/invites/{inviteId}/decline", a.auth(a.declineInvite))
}

// ---- middleware ----

type ctxUserKey struct{}

// authedHandler is a handler that has a resolved user.
type authedHandler func(w http.ResponseWriter, r *http.Request, u app.User)

// auth wraps a handler, requiring a valid session cookie; 401 otherwise.
func (a *WebAPI) auth(h authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeErr(w, app.ErrUnauthorized)
			return
		}
		u, err := a.svc.UserForToken(c.Value)
		if err != nil {
			writeErr(w, app.ErrUnauthorized)
			return
		}
		h(w, r, u)
	}
}

// ---- auth handlers ----

func (a *WebAPI) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
		Email       string `json:"email"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, err := a.svc.Register(in.Username, in.DisplayName, in.Password, in.Email)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": u.Public()})
}

func (a *WebAPI) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, token, err := a.svc.Login(in.Username, in.Password)
	if err != nil {
		writeErr(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})
	writeJSON(w, http.StatusOK, map[string]any{"user": u.Public()})
}

func (a *WebAPI) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = a.svc.Logout(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *WebAPI) me(w http.ResponseWriter, _ *http.Request, u app.User) {
	writeJSON(w, http.StatusOK, map[string]any{"user": u.Public()})
}

// ---- band handlers ----

func (a *WebAPI) listBands(w http.ResponseWriter, _ *http.Request, u app.User) {
	bands, err := a.svc.BandsForUser(u)
	if err != nil {
		writeErr(w, err)
		return
	}
	if bands == nil {
		bands = []app.Band{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"bands": bands})
}

func (a *WebAPI) createBand(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	b, err := a.svc.CreateBand(u, in.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"band": b})
}

func (a *WebAPI) getBand(w http.ResponseWriter, r *http.Request, u app.User) {
	b, role, err := a.svc.GetBand(u, r.PathValue("bandId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"band": b, "myRole": role})
}

func (a *WebAPI) listMembers(w http.ResponseWriter, r *http.Request, u app.User) {
	members, err := a.svc.Members(u, r.PathValue("bandId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if members == nil {
		members = []app.MemberView{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (a *WebAPI) updateBand(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	b, err := a.svc.UpdateBand(u, r.PathValue("bandId"), in.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"band": b})
}

func (a *WebAPI) deleteBand(w http.ResponseWriter, r *http.Request, u app.User) {
	if err := a.svc.DeleteBand(u, r.PathValue("bandId")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *WebAPI) updateMember(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Role string `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	m, err := a.svc.SetMemberRole(u, r.PathValue("bandId"), r.PathValue("userId"), app.Role(in.Role))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"member": m})
}

func (a *WebAPI) removeMember(w http.ResponseWriter, r *http.Request, u app.User) {
	if err := a.svc.RemoveMember(u, r.PathValue("bandId"), r.PathValue("userId")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *WebAPI) leaveBand(w http.ResponseWriter, r *http.Request, u app.User) {
	if err := a.svc.LeaveBand(u, r.PathValue("bandId")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- invite handlers ----

func (a *WebAPI) createInvite(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Identifier string `json:"identifier"`
		Kind       string `json:"kind"`
	}
	if !decode(w, r, &in) {
		return
	}
	inv, err := a.svc.Invite(u, r.PathValue("bandId"), in.Identifier, app.IdentifierKind(in.Kind))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"invite": inv})
}

func (a *WebAPI) listBandInvites(w http.ResponseWriter, r *http.Request, u app.User) {
	invites, err := a.svc.BandInvites(u, r.PathValue("bandId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if invites == nil {
		invites = []app.Invite{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"invites": invites})
}

func (a *WebAPI) revokeInvite(w http.ResponseWriter, r *http.Request, u app.User) {
	if err := a.svc.RevokeInvite(u, r.PathValue("bandId"), r.PathValue("inviteId")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *WebAPI) listInvites(w http.ResponseWriter, _ *http.Request, u app.User) {
	invites, err := a.svc.PendingInvites(u)
	if err != nil {
		writeErr(w, err)
		return
	}
	if invites == nil {
		invites = []app.Invite{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"invites": invites})
}

func (a *WebAPI) acceptInvite(w http.ResponseWriter, r *http.Request, u app.User) {
	m, err := a.svc.AcceptInvite(u, r.PathValue("inviteId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"membership": m})
}

func (a *WebAPI) declineInvite(w http.ResponseWriter, r *http.Request, u app.User) {
	if err := a.svc.DeclineInvite(u, r.PathValue("inviteId")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": app.InviteDeclined})
}

// ---- song handlers ----

func (a *WebAPI) listSongs(w http.ResponseWriter, r *http.Request, u app.User) {
	songs, err := a.svc.Songs(u, r.PathValue("bandId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if songs == nil {
		songs = []app.Song{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"songs": songs})
}

func (a *WebAPI) createSong(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Title  string `json:"title"`
		Artist string `json:"artist"`
	}
	if !decode(w, r, &in) {
		return
	}
	song, err := a.svc.CreateSong(u, r.PathValue("bandId"), in.Title, in.Artist)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"song": song})
}

func (a *WebAPI) updateSong(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Title  *string   `json:"title"`
		Artist *string   `json:"artist"`
		Key    *string   `json:"key"`
		Tempo  *int      `json:"tempo"`
		Tags   *[]string `json:"tags"`
		Notes  *string   `json:"notes"`
	}
	if !decode(w, r, &in) {
		return
	}
	song, err := a.svc.UpdateSong(u, r.PathValue("bandId"), r.PathValue("songId"), app.SongPatch{
		Title:  in.Title,
		Artist: in.Artist,
		Key:    in.Key,
		Tempo:  in.Tempo,
		Tags:   in.Tags,
		Notes:  in.Notes,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"song": song})
}

func (a *WebAPI) deleteSong(w http.ResponseWriter, r *http.Request, u app.User) {
	if err := a.svc.DeleteSong(u, r.PathValue("bandId"), r.PathValue("songId")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- song file handlers ----

// maxUploadBytes caps a single song-file upload (sheet music PDFs are small).
const maxUploadBytes = 32 << 20 // 32 MiB

// uploadFile handles multipart upload of a song file (field "file"). Member-only;
// Service validates the content type.
func (a *WebAPI) uploadFile(w http.ResponseWriter, r *http.Request, u app.User) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeErr(w, app.ErrInvalidInput)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, app.ErrInvalidInput)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		writeErr(w, app.ErrInvalidInput)
		return
	}
	if len(data) > maxUploadBytes {
		writeErr(w, fmt.Errorf("%w: file too large", app.ErrInvalidInput))
		return
	}
	declared := header.Header.Get("Content-Type")
	f, err := a.svc.UploadSongFile(u, r.PathValue("bandId"), r.PathValue("songId"), header.Filename, declared, data)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"file": f})
}

func (a *WebAPI) listFiles(w http.ResponseWriter, r *http.Request, u app.User) {
	files, err := a.svc.SongFiles(u, r.PathValue("bandId"), r.PathValue("songId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if files == nil {
		files = []app.SongFile{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (a *WebAPI) updateFile(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Filename     *string `json:"filename"`
		DisplayOrder *int    `json:"displayOrder"`
	}
	if !decode(w, r, &in) {
		return
	}
	f, err := a.svc.UpdateSongFile(u, r.PathValue("bandId"), r.PathValue("songId"), r.PathValue("fileId"), app.SongFilePatch{
		Filename:     in.Filename,
		DisplayOrder: in.DisplayOrder,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"file": f})
}

func (a *WebAPI) deleteFile(w http.ResponseWriter, r *http.Request, u app.User) {
	if err := a.svc.DeleteSongFile(u, r.PathValue("bandId"), r.PathValue("songId"), r.PathValue("fileId")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// downloadFile streams a blob with its stored Content-Type. Members of the owning
// band only (Service enforces 403/404).
func (a *WebAPI) downloadFile(w http.ResponseWriter, r *http.Request, u app.User) {
	f, data, err := a.svc.DownloadSongFile(u, r.PathValue("fileId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(f.Size, 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", f.Filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ---- setlist handlers ----

func (a *WebAPI) listSetlists(w http.ResponseWriter, r *http.Request, u app.User) {
	setlists, err := a.svc.Setlists(u, r.PathValue("bandId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if setlists == nil {
		setlists = []app.Setlist{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"setlists": setlists})
}

func (a *WebAPI) createSetlist(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Name      string `json:"name"`
		EventDate string `json:"eventDate"`
		Venue     string `json:"venue"`
		Notes     string `json:"notes"`
	}
	if !decode(w, r, &in) {
		return
	}
	sl, err := a.svc.CreateSetlist(u, r.PathValue("bandId"), in.Name, in.EventDate, in.Venue, in.Notes)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"setlist": sl})
}

func (a *WebAPI) getSetlist(w http.ResponseWriter, r *http.Request, u app.User) {
	detail, err := a.svc.Setlist(u, r.PathValue("bandId"), r.PathValue("setlistId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if detail.Items == nil {
		detail.Items = []app.SetlistItemView{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"setlist": detail.Setlist, "items": detail.Items})
}

func (a *WebAPI) updateSetlist(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		Name      *string `json:"name"`
		EventDate *string `json:"eventDate"`
		Venue     *string `json:"venue"`
		Notes     *string `json:"notes"`
	}
	if !decode(w, r, &in) {
		return
	}
	sl, err := a.svc.UpdateSetlist(u, r.PathValue("bandId"), r.PathValue("setlistId"), app.SetlistInput{
		Name:      in.Name,
		EventDate: in.EventDate,
		Venue:     in.Venue,
		Notes:     in.Notes,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"setlist": sl})
}

func (a *WebAPI) deleteSetlist(w http.ResponseWriter, r *http.Request, u app.User) {
	if err := a.svc.DeleteSetlist(u, r.PathValue("bandId"), r.PathValue("setlistId")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *WebAPI) addSetlistItem(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		SongID string `json:"songId"`
	}
	if !decode(w, r, &in) {
		return
	}
	item, err := a.svc.AddSetlistItem(u, r.PathValue("bandId"), r.PathValue("setlistId"), in.SongID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (a *WebAPI) updateSetlistItem(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		KeyOverride   *string `json:"keyOverride"`
		TempoOverride *int    `json:"tempoOverride"`
		Notes         *string `json:"notes"`
	}
	if !decode(w, r, &in) {
		return
	}
	item, err := a.svc.UpdateSetlistItem(u, r.PathValue("bandId"), r.PathValue("setlistId"), r.PathValue("itemId"), app.SetlistItemPatch{
		KeyOverride:   in.KeyOverride,
		TempoOverride: in.TempoOverride,
		Notes:         in.Notes,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (a *WebAPI) removeSetlistItem(w http.ResponseWriter, r *http.Request, u app.User) {
	if err := a.svc.RemoveSetlistItem(u, r.PathValue("bandId"), r.PathValue("setlistId"), r.PathValue("itemId")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *WebAPI) reorderSetlist(w http.ResponseWriter, r *http.Request, u app.User) {
	var in struct {
		OrderedItemIDs []string `json:"orderedItemIds"`
	}
	if !decode(w, r, &in) {
		return
	}
	items, err := a.svc.ReorderSetlist(u, r.PathValue("bandId"), r.PathValue("setlistId"), in.OrderedItemIDs)
	if err != nil {
		writeErr(w, err)
		return
	}
	if items == nil {
		items = []app.SetlistItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ---- JSON / error plumbing ----

// decode reads a JSON body into dst. On malformed JSON it writes a 400 and
// returns false. An empty body decodes to the zero value (Service validates).
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return true // empty body → zero value; let Service validate
		}
		writeErr(w, app.ErrInvalidInput)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// writeErr maps app sentinel errors to status codes with a {"error": string} body.
func writeErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, app.ErrInvalidInput):
		code = http.StatusBadRequest
	case errors.Is(err, app.ErrUnauthorized):
		code = http.StatusUnauthorized
	case errors.Is(err, app.ErrForbidden):
		code = http.StatusForbidden
	case errors.Is(err, app.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, app.ErrConflict), errors.Is(err, app.ErrInviteResolved):
		code = http.StatusConflict
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
