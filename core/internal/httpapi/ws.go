package httpapi

import (
	"net/http"

	"troubastack/core/internal/app"
	"troubastack/core/internal/engine"
	syncpkg "troubastack/core/internal/sync"
)

// wsAuth adapts app.Service to sync.Auth so the realtime hub reuses the EXACT
// membership/session checks the annotation REST routes use (one enforcement path,
// I14): a valid session token plus band membership with the song in that band.
type wsAuth struct{ svc *app.Service }

// UserForToken resolves the session-cookie token to the authoritative user id.
func (a wsAuth) UserForToken(token string) (string, error) {
	u, err := a.svc.UserForToken(token)
	if err != nil {
		return "", err
	}
	return u.ID, nil
}

// SongForMember enforces membership + song/band ownership and returns the engine
// songID (the relational Song.ID) plus the caller's band role string. It is the same
// gate getAnnotations uses; the role lets the hub enforce conductor-zone and
// layer-access write rules without importing the app layer.
func (a wsAuth) SongForMember(userID, bandID, songID string) (string, string, error) {
	// SongForMember takes the full user; we only hold the id here, so re-resolve via a
	// lightweight User carrying just the id (the policy keys on caller.ID).
	song, err := a.svc.SongForMember(app.User{ID: userID}, bandID, songID)
	if err != nil {
		return "", "", err
	}
	// Resolve the caller's band role (membership is already proven by SongForMember).
	_, role, err := a.svc.GetBand(app.User{ID: userID}, bandID)
	if err != nil {
		return "", "", err
	}
	return song.ID, string(role), nil
}

// mountWS builds the realtime hub over the SHARED apply engine (same instance backing
// GET …/annotations, so HEAD is consistent) and mounts the per-song WebSocket route.
//
// Auth happens INSIDE the upgrade (hub.Serve): the session cookie is read off the
// request and verified before the socket is accepted, so an unauthenticated or
// non-member upgrade is rejected with 401/403 and never becomes a WebSocket.
func mountWS(mux *http.ServeMux, svc *app.Service, eng *engine.Engine, onCommit func(songID string)) *syncpkg.Hub {
	hub := syncpkg.NewHub(eng, wsAuth{svc: svc})
	if onCommit != nil {
		hub.SetOnCommit(onCommit)
	}
	// T145 forward fix: anchor interactively-drawn marks to their words at create time, so a later chart
	// size/render change re-projects them onto their line instead of orphaning them. Best-effort + nil-safe
	// (see chartAnchorer); a hub without it creates marks exactly as before.
	hub.SetAnchorer(newChartAnchorer(svc, eng))
	mux.HandleFunc("GET /api/bands/{bandId}/songs/{songId}/ws", func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if c, err := r.Cookie(sessionCookie); err == nil {
			token = c.Value
		}
		hub.Serve(w, r, token, r.PathValue("bandId"), r.PathValue("songId"))
	})
	return hub
}
