package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/filerepo"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/httpapi"
)

// repoBackend names a Repo impl + a constructor (file backend gets a temp dir).
type repoBackend struct {
	name string
	make func(t *testing.T) app.Repo
}

func backends() []repoBackend {
	return []repoBackend{
		{name: "mem", make: func(*testing.T) app.Repo { return memrepo.New() }},
		{name: "file", make: func(t *testing.T) app.Repo {
			r, err := filerepo.New(t.TempDir())
			if err != nil {
				t.Fatalf("filerepo.New: %v", err)
			}
			return r
		}},
	}
}

// client is a tiny test HTTP client that carries the session cookie like a browser.
type client struct {
	t   *testing.T
	srv *httptest.Server
	jar []*http.Cookie
}

func newClient(t *testing.T, repo app.Repo) *client {
	t.Helper()
	svc := app.NewService(repo)
	h, err := httpapi.Router(svc, false, nil, nil, nil)
	if err != nil {
		t.Fatalf("Router: %v", err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &client{t: t, srv: srv}
}

// do sends a JSON request, storing any Set-Cookie back into the jar.
func (c *client) do(method, path string, body any) (*http.Response, map[string]json.RawMessage) {
	c.t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, c.srv.URL+path, rdr)
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for _, ck := range c.jar {
		req.AddCookie(ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	if cks := resp.Cookies(); len(cks) > 0 {
		c.storeCookies(cks)
	}
	var decoded map[string]json.RawMessage
	if resp.Body != nil {
		defer resp.Body.Close()
		_ = json.NewDecoder(resp.Body).Decode(&decoded)
	}
	return resp, decoded
}

func (c *client) storeCookies(cks []*http.Cookie) {
	for _, ck := range cks {
		replaced := false
		for i, ex := range c.jar {
			if ex.Name == ck.Name {
				if ck.MaxAge < 0 || ck.Value == "" {
					c.jar = append(c.jar[:i], c.jar[i+1:]...)
				} else {
					c.jar[i] = ck
				}
				replaced = true
				break
			}
		}
		if !replaced && ck.Value != "" {
			c.jar = append(c.jar, ck)
		}
	}
}

func (c *client) clearCookies() { c.jar = nil }

func mustStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("%s %s: got status %d, want %d", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, want)
	}
}

// registerLogin registers a user and logs them in, leaving the session cookie set.
func (c *client) registerLogin(username, password string) {
	c.t.Helper()
	resp, _ := c.do(http.MethodPost, "/api/auth/register", map[string]string{
		"username": username, "displayName": username, "password": password,
	})
	mustStatus(c.t, resp, http.StatusCreated)
	resp, _ = c.do(http.MethodPost, "/api/auth/login", map[string]string{
		"username": username, "password": password,
	})
	mustStatus(c.t, resp, http.StatusOK)
}

func unmarshalField(t *testing.T, body map[string]json.RawMessage, key string, dst any) {
	t.Helper()
	raw, ok := body[key]
	if !ok {
		t.Fatalf("response missing field %q (have %v)", key, keysOf(body))
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("unmarshal field %q: %v", key, err)
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// ---- tests (each runs against both backends) ----

func TestAuthFlow(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))

			// register
			resp, body := c.do(http.MethodPost, "/api/auth/register", map[string]string{
				"username": "alice", "displayName": "Alice", "password": "pw-correct",
			})
			mustStatus(t, resp, http.StatusCreated)
			var u app.PublicUser
			unmarshalField(t, body, "user", &u)
			if u.Username != "alice" || u.ID == "" {
				t.Fatalf("bad user in register response: %+v", u)
			}

			// /api/me without cookie → 401
			resp, _ = c.do(http.MethodGet, "/api/me", nil)
			mustStatus(t, resp, http.StatusUnauthorized)

			// wrong password → 401
			resp, _ = c.do(http.MethodPost, "/api/auth/login", map[string]string{
				"username": "alice", "password": "wrong",
			})
			mustStatus(t, resp, http.StatusUnauthorized)

			// correct login → cookie set
			resp, _ = c.do(http.MethodPost, "/api/auth/login", map[string]string{
				"username": "alice", "password": "pw-correct",
			})
			mustStatus(t, resp, http.StatusOK)

			// /api/me with cookie → 200, same user
			resp, body = c.do(http.MethodGet, "/api/me", nil)
			mustStatus(t, resp, http.StatusOK)
			var me app.PublicUser
			unmarshalField(t, body, "user", &me)
			if me.ID != u.ID {
				t.Fatalf("me id %q != registered id %q", me.ID, u.ID)
			}

			// duplicate username → 409
			resp, _ = c.do(http.MethodPost, "/api/auth/register", map[string]string{
				"username": "alice", "displayName": "Other", "password": "x",
			})
			mustStatus(t, resp, http.StatusConflict)

			// logout → 204, then /api/me → 401
			resp, _ = c.do(http.MethodPost, "/api/auth/logout", nil)
			mustStatus(t, resp, http.StatusNoContent)
			resp, _ = c.do(http.MethodGet, "/api/me", nil)
			mustStatus(t, resp, http.StatusUnauthorized)
		})
	}
}

func TestBandCreateAndList(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			c.registerLogin("alice", "pw")

			resp, body := c.do(http.MethodPost, "/api/bands", map[string]string{"name": "The Troubas"})
			mustStatus(t, resp, http.StatusCreated)
			var band app.Band
			unmarshalField(t, body, "band", &band)
			if band.Name != "The Troubas" || band.ID == "" {
				t.Fatalf("bad band: %+v", band)
			}

			// appears in GET /api/bands
			resp, body = c.do(http.MethodGet, "/api/bands", nil)
			mustStatus(t, resp, http.StatusOK)
			var bands []app.Band
			unmarshalField(t, body, "bands", &bands)
			if len(bands) != 1 || bands[0].ID != band.ID {
				t.Fatalf("bands list wrong: %+v", bands)
			}

			// creator is admin (myRole on GET band)
			resp, body = c.do(http.MethodGet, "/api/bands/"+band.ID, nil)
			mustStatus(t, resp, http.StatusOK)
			var role app.Role
			unmarshalField(t, body, "myRole", &role)
			if role != app.RoleAdmin {
				t.Fatalf("creator role = %q, want admin", role)
			}
		})
	}
}

func TestNonMemberCannotReadBand(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			owner := newClient(t, repo)
			owner.registerLogin("alice", "pw")
			_, body := owner.do(http.MethodPost, "/api/bands", map[string]string{"name": "Band"})
			var band app.Band
			unmarshalField(t, body, "band", &band)

			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")
			resp, _ := outsider.do(http.MethodGet, "/api/bands/"+band.ID, nil)
			mustStatus(t, resp, http.StatusForbidden)
		})
	}
}

func TestInviteAcceptFlow(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)

			admin := newClient(t, repo)
			admin.registerLogin("alice", "pw")
			_, body := admin.do(http.MethodPost, "/api/bands", map[string]string{"name": "Band"})
			var band app.Band
			unmarshalField(t, body, "band", &band)

			// register userB (separate client / session)
			userB := newClient(t, repo)
			userB.registerLogin("bob", "pw")

			// non-admin invite attempt → 403 (bob is not even a member)
			resp, _ := userB.do(http.MethodPost, "/api/bands/"+band.ID+"/invites",
				map[string]string{"identifier": "carol", "kind": "username"})
			mustStatus(t, resp, http.StatusForbidden)

			// admin invites bob by username → 201
			resp, body = admin.do(http.MethodPost, "/api/bands/"+band.ID+"/invites",
				map[string]string{"identifier": "bob", "kind": "username"})
			mustStatus(t, resp, http.StatusCreated)
			var inv app.Invite
			unmarshalField(t, body, "invite", &inv)
			if inv.Status != app.InvitePending {
				t.Fatalf("invite status = %q, want pending", inv.Status)
			}

			// bob sees it in GET /api/invites
			resp, body = userB.do(http.MethodGet, "/api/invites", nil)
			mustStatus(t, resp, http.StatusOK)
			var invites []app.Invite
			unmarshalField(t, body, "invites", &invites)
			if len(invites) != 1 || invites[0].ID != inv.ID {
				t.Fatalf("bob's invites wrong: %+v", invites)
			}

			// before accepting, bob cannot read the band → 403
			resp, _ = userB.do(http.MethodGet, "/api/bands/"+band.ID, nil)
			mustStatus(t, resp, http.StatusForbidden)

			// bob accepts → 200, now a member
			resp, _ = userB.do(http.MethodPost, "/api/invites/"+inv.ID+"/accept", nil)
			mustStatus(t, resp, http.StatusOK)

			// bob can now read the band
			resp, _ = userB.do(http.MethodGet, "/api/bands/"+band.ID, nil)
			mustStatus(t, resp, http.StatusOK)

			// members list shows both alice and bob
			resp, body = admin.do(http.MethodGet, "/api/bands/"+band.ID+"/members", nil)
			mustStatus(t, resp, http.StatusOK)
			var members []app.MemberView
			unmarshalField(t, body, "members", &members)
			if len(members) != 2 {
				t.Fatalf("expected 2 members, got %d: %+v", len(members), members)
			}
			seen := map[string]app.Role{}
			for _, m := range members {
				seen[m.User.Username] = m.Role
			}
			if seen["alice"] != app.RoleAdmin {
				t.Fatalf("alice role = %q, want admin", seen["alice"])
			}
			if seen["bob"] != app.RoleMember {
				t.Fatalf("bob role = %q, want member", seen["bob"])
			}
		})
	}
}

func TestInviteUnknownIdentifierNotDiscoverable(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)

			admin := newClient(t, repo)
			admin.registerLogin("alice", "pw")
			_, body := admin.do(http.MethodPost, "/api/bands", map[string]string{"name": "Band"})
			var band app.Band
			unmarshalField(t, body, "band", &band)

			// invite an identifier that belongs to no user yet → still a valid pending invite
			resp, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/invites",
				map[string]string{"identifier": "ghost", "kind": "username"})
			mustStatus(t, resp, http.StatusCreated)
			var inv app.Invite
			unmarshalField(t, body, "invite", &inv)

			// some other user (not "ghost") cannot accept it — it's not theirs → 404
			intruder := newClient(t, repo)
			intruder.registerLogin("bob", "pw")
			resp, _ = intruder.do(http.MethodPost, "/api/invites/"+inv.ID+"/accept", nil)
			mustStatus(t, resp, http.StatusNotFound)

			// the matching user registers later, sees the invite, and accepts
			ghost := newClient(t, repo)
			ghost.registerLogin("ghost", "pw")
			resp, body = ghost.do(http.MethodGet, "/api/invites", nil)
			mustStatus(t, resp, http.StatusOK)
			var invites []app.Invite
			unmarshalField(t, body, "invites", &invites)
			if len(invites) != 1 {
				t.Fatalf("ghost should see 1 invite, got %d", len(invites))
			}
			resp, _ = ghost.do(http.MethodPost, "/api/invites/"+inv.ID+"/accept", nil)
			mustStatus(t, resp, http.StatusOK)
			resp, _ = ghost.do(http.MethodGet, "/api/bands/"+band.ID, nil)
			mustStatus(t, resp, http.StatusOK)
		})
	}
}

func TestInviteDecline(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			admin.registerLogin("alice", "pw")
			_, body := admin.do(http.MethodPost, "/api/bands", map[string]string{"name": "Band"})
			var band app.Band
			unmarshalField(t, body, "band", &band)

			userB := newClient(t, repo)
			userB.registerLogin("bob", "pw")
			_, body = admin.do(http.MethodPost, "/api/bands/"+band.ID+"/invites",
				map[string]string{"identifier": "bob", "kind": "username"})
			var inv app.Invite
			unmarshalField(t, body, "invite", &inv)

			resp, _ := userB.do(http.MethodPost, "/api/invites/"+inv.ID+"/decline", nil)
			mustStatus(t, resp, http.StatusOK)
			// still not a member
			resp, _ = userB.do(http.MethodGet, "/api/bands/"+band.ID, nil)
			mustStatus(t, resp, http.StatusForbidden)
			// declining again → 409 (already resolved)
			resp, _ = userB.do(http.MethodPost, "/api/invites/"+inv.ID+"/decline", nil)
			mustStatus(t, resp, http.StatusConflict)
		})
	}
}

func TestSongs(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			member := newClient(t, repo)
			member.registerLogin("alice", "pw")
			_, body := member.do(http.MethodPost, "/api/bands", map[string]string{"name": "Band"})
			var band app.Band
			unmarshalField(t, body, "band", &band)

			// member creates a song → 201
			resp, body := member.do(http.MethodPost, "/api/bands/"+band.ID+"/songs",
				map[string]string{"title": "Wonderwall", "artist": "Oasis"})
			mustStatus(t, resp, http.StatusCreated)
			var song app.Song
			unmarshalField(t, body, "song", &song)
			if song.Title != "Wonderwall" || song.BandID != band.ID {
				t.Fatalf("bad song: %+v", song)
			}

			// appears in list
			resp, body = member.do(http.MethodGet, "/api/bands/"+band.ID+"/songs", nil)
			mustStatus(t, resp, http.StatusOK)
			var songs []app.Song
			unmarshalField(t, body, "songs", &songs)
			if len(songs) != 1 || songs[0].ID != song.ID {
				t.Fatalf("songs list wrong: %+v", songs)
			}

			// non-member create → 403
			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")
			resp, _ = outsider.do(http.MethodPost, "/api/bands/"+band.ID+"/songs",
				map[string]string{"title": "Sneaky"})
			mustStatus(t, resp, http.StatusForbidden)
			// non-member list → 403
			resp, _ = outsider.do(http.MethodGet, "/api/bands/"+band.ID+"/songs", nil)
			mustStatus(t, resp, http.StatusForbidden)
		})
	}
}

func TestMissingFieldsAndUnauthenticated(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))

			// register missing password → 400
			resp, _ := c.do(http.MethodPost, "/api/auth/register", map[string]string{"username": "x"})
			mustStatus(t, resp, http.StatusBadRequest)

			// create band while unauthenticated → 401
			resp, _ = c.do(http.MethodPost, "/api/bands", map[string]string{"name": "x"})
			mustStatus(t, resp, http.StatusUnauthorized)

			// authenticated, empty band name → 400
			c.clearCookies()
			c.registerLogin("alice", "pw")
			resp, _ = c.do(http.MethodPost, "/api/bands", map[string]string{"name": "  "})
			mustStatus(t, resp, http.StatusBadRequest)

			// unknown band → 404
			resp, _ = c.do(http.MethodGet, "/api/bands/does-not-exist", nil)
			mustStatus(t, resp, http.StatusNotFound)
		})
	}
}
