package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
	mux.HandleFunc("GET /api/bands/{bandId}/members", a.auth(a.listMembers))
	mux.HandleFunc("POST /api/bands/{bandId}/invites", a.auth(a.createInvite))
	mux.HandleFunc("GET /api/bands/{bandId}/songs", a.auth(a.listSongs))
	mux.HandleFunc("POST /api/bands/{bandId}/songs", a.auth(a.createSong))
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
