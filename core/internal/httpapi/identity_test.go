package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// ---- profile: PATCH /api/me ----

func TestUpdateProfile(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			c.registerLogin("alice", "pw")

			// Update displayName, email, avatarKind.
			resp, body := c.do(http.MethodPatch, "/api/me", map[string]any{
				"displayName": "Alice A.",
				"email":       "alice@example.com",
				"avatarKind":  "woman",
			})
			mustStatus(t, resp, http.StatusOK)
			var u app.PublicUser
			unmarshalField(t, body, "user", &u)
			if u.DisplayName != "Alice A." || u.Email != "alice@example.com" || u.AvatarKind != app.AvatarWoman {
				t.Fatalf("profile not updated: %+v", u)
			}

			// Persists across a fresh GET /api/me.
			resp, body = c.do(http.MethodGet, "/api/me", nil)
			mustStatus(t, resp, http.StatusOK)
			var me app.PublicUser
			unmarshalField(t, body, "user", &me)
			if me.AvatarKind != app.AvatarWoman || me.Email != "alice@example.com" {
				t.Fatalf("profile did not persist: %+v", me)
			}

			// Invalid avatar kind → 400.
			resp, _ = c.do(http.MethodPatch, "/api/me", map[string]any{"avatarKind": "alien"})
			mustStatus(t, resp, http.StatusBadRequest)

			// Empty displayName → 400.
			resp, _ = c.do(http.MethodPatch, "/api/me", map[string]any{"displayName": "   "})
			mustStatus(t, resp, http.StatusBadRequest)

			// Invalid email → 400.
			resp, _ = c.do(http.MethodPatch, "/api/me", map[string]any{"email": "not-an-email"})
			mustStatus(t, resp, http.StatusBadRequest)
		})
	}
}

func TestUpdateProfileEmailUniqueness(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)

			// Bob takes bob@example.com.
			bob := newClient(t, repo)
			bob.registerLogin("bob", "pw")
			resp, _ := bob.do(http.MethodPatch, "/api/me", map[string]any{"email": "bob@example.com"})
			mustStatus(t, resp, http.StatusOK)

			// Alice cannot claim bob's email → 409.
			alice := newClient(t, repo)
			alice.registerLogin("alice", "pw")
			resp, _ = alice.do(http.MethodPatch, "/api/me", map[string]any{"email": "bob@example.com"})
			mustStatus(t, resp, http.StatusConflict)

			// Alice can set a different email, then re-PATCH keeping her own → OK.
			resp, _ = alice.do(http.MethodPatch, "/api/me", map[string]any{"email": "alice@example.com"})
			mustStatus(t, resp, http.StatusOK)
			resp, _ = alice.do(http.MethodPatch, "/api/me", map[string]any{
				"email": "alice@example.com", "displayName": "Alice",
			})
			mustStatus(t, resp, http.StatusOK)
		})
	}
}

// ---- password change: POST /api/me/password ----

func TestChangePassword(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			c.registerLogin("alice", "oldpassword")

			// Wrong current password → 403.
			resp, _ := c.do(http.MethodPost, "/api/me/password", map[string]string{
				"currentPassword": "nope", "newPassword": "newpassword",
			})
			mustStatus(t, resp, http.StatusForbidden)

			// Empty new password → 400 (below the floor).
			resp, _ = c.do(http.MethodPost, "/api/me/password", map[string]string{
				"currentPassword": "oldpassword", "newPassword": "",
			})
			mustStatus(t, resp, http.StatusBadRequest)

			// Correct current → 204.
			resp, _ = c.do(http.MethodPost, "/api/me/password", map[string]string{
				"currentPassword": "oldpassword", "newPassword": "brandnew",
			})
			mustStatus(t, resp, http.StatusNoContent)

			// Old password no longer logs in; new one does.
			c.clearCookies()
			resp, _ = c.do(http.MethodPost, "/api/auth/login", map[string]string{
				"username": "alice", "password": "oldpassword",
			})
			mustStatus(t, resp, http.StatusUnauthorized)
			resp, _ = c.do(http.MethodPost, "/api/auth/login", map[string]string{
				"username": "alice", "password": "brandnew",
			})
			mustStatus(t, resp, http.StatusOK)
		})
	}
}

// ---- invite links ----

// makeBand registers+logs an admin and creates a band, returning its id.
func makeBand(t *testing.T, c *client) string {
	t.Helper()
	resp, body := c.do(http.MethodPost, "/api/bands", map[string]string{"name": "Band"})
	mustStatus(t, resp, http.StatusCreated)
	var b app.Band
	unmarshalField(t, body, "band", &b)
	return b.ID
}

func mintLink(t *testing.T, c *client, bandID string, in map[string]any) (token, id string, body map[string]json.RawMessage) {
	t.Helper()
	resp, b := c.do(http.MethodPost, "/api/bands/"+bandID+"/invite-links", in)
	mustStatus(t, resp, http.StatusCreated)
	_ = json.Unmarshal(b["token"], &token)
	_ = json.Unmarshal(b["id"], &id)
	return token, id, b
}

func TestInviteLinkMintAdminOnly(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			admin.registerLogin("alice", "pw")
			bandID := makeBand(t, admin)

			// admin mints a member link → 201, has token + url.
			token, _, body := mintLink(t, admin, bandID, map[string]any{"role": "member"})
			if token == "" || string(body["url"]) == "" {
				t.Fatalf("missing token/url: %v", body)
			}
			var revoked bool
			_ = json.Unmarshal(body["revoked"], &revoked)
			if revoked {
				t.Fatalf("new link should not be revoked")
			}

			// role=admin via link → rejected 400.
			resp, _ := admin.do(http.MethodPost, "/api/bands/"+bandID+"/invite-links", map[string]any{"role": "admin"})
			mustStatus(t, resp, http.StatusBadRequest)

			// invalid role → rejected.
			resp, _ = admin.do(http.MethodPost, "/api/bands/"+bandID+"/invite-links", map[string]any{"role": "wizard"})
			mustStatus(t, resp, http.StatusBadRequest)

			// non-admin cannot mint.
			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")
			resp, _ = outsider.do(http.MethodPost, "/api/bands/"+bandID+"/invite-links", map[string]any{"role": "member"})
			if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
				t.Fatalf("non-admin mint: got %d, want 403/404", resp.StatusCode)
			}
		})
	}
}

func TestInviteLinkAcceptAndIdempotent(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			admin.registerLogin("alice", "pw")
			bandID := makeBand(t, admin)

			token, _, _ := mintLink(t, admin, bandID, map[string]any{"role": "conductor"})

			// Bob previews then accepts.
			bob := newClient(t, repo)
			bob.registerLogin("bob", "pw")
			resp, pbody := bob.do(http.MethodGet, "/api/invite-links/"+token, nil)
			mustStatus(t, resp, http.StatusOK)
			var valid bool
			_ = json.Unmarshal(pbody["valid"], &valid)
			if !valid {
				t.Fatalf("preview should be valid: %v", pbody)
			}

			resp, _ = bob.do(http.MethodPost, "/api/invite-links/"+token+"/accept", nil)
			mustStatus(t, resp, http.StatusOK)

			// Bob is now a member — appears in /api/bands.
			resp, lbody := bob.do(http.MethodGet, "/api/bands", nil)
			mustStatus(t, resp, http.StatusOK)
			var bands []app.Band
			_ = json.Unmarshal(lbody["bands"], &bands)
			if len(bands) != 1 || bands[0].ID != bandID {
				t.Fatalf("bob not joined: %v", bands)
			}

			// Accept again → idempotent 200, Uses NOT incremented past 1.
			resp, _ = bob.do(http.MethodPost, "/api/invite-links/"+token+"/accept", nil)
			mustStatus(t, resp, http.StatusOK)

			resp, links := admin.do(http.MethodGet, "/api/bands/"+bandID+"/invite-links", nil)
			mustStatus(t, resp, http.StatusOK)
			var got struct {
				Links []struct {
					Uses int    `json:"uses"`
					Role string `json:"role"`
				} `json:"links"`
			}
			raw, _ := json.Marshal(map[string]json.RawMessage{"links": links["links"]})
			_ = json.Unmarshal(raw, &got)
			if len(got.Links) != 1 || got.Links[0].Uses != 1 {
				t.Fatalf("uses should be 1 after idempotent accept: %+v", got.Links)
			}
			if got.Links[0].Role != "conductor" {
				t.Fatalf("link role = %q, want conductor", got.Links[0].Role)
			}
		})
	}
}

func TestInviteLinkMaxUsesExhausted(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			admin.registerLogin("alice", "pw")
			bandID := makeBand(t, admin)

			token, _, _ := mintLink(t, admin, bandID, map[string]any{"role": "member", "maxUses": 1})

			// First joiner consumes the single use.
			bob := newClient(t, repo)
			bob.registerLogin("bob", "pw")
			resp, _ := bob.do(http.MethodPost, "/api/invite-links/"+token+"/accept", nil)
			mustStatus(t, resp, http.StatusOK)

			// Second joiner is rejected with 410 (exhausted).
			carol := newClient(t, repo)
			carol.registerLogin("carol", "pw")
			resp, _ = carol.do(http.MethodPost, "/api/invite-links/"+token+"/accept", nil)
			mustStatus(t, resp, http.StatusGone)

			// Preview shows valid=false reason=exhausted.
			resp, pbody := carol.do(http.MethodGet, "/api/invite-links/"+token, nil)
			mustStatus(t, resp, http.StatusOK)
			var valid bool
			var reason string
			_ = json.Unmarshal(pbody["valid"], &valid)
			_ = json.Unmarshal(pbody["reason"], &reason)
			if valid || reason != "exhausted" {
				t.Fatalf("expected exhausted preview, got valid=%v reason=%q", valid, reason)
			}
		})
	}
}

func TestInviteLinkRevoke(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			admin.registerLogin("alice", "pw")
			bandID := makeBand(t, admin)

			token, id, _ := mintLink(t, admin, bandID, map[string]any{"role": "member"})

			// Revoke it.
			resp, _ := admin.do(http.MethodDelete, "/api/bands/"+bandID+"/invite-links/"+id, nil)
			mustStatus(t, resp, http.StatusNoContent)

			// Accept on a revoked link → 410.
			bob := newClient(t, repo)
			bob.registerLogin("bob", "pw")
			resp, _ = bob.do(http.MethodPost, "/api/invite-links/"+token+"/accept", nil)
			mustStatus(t, resp, http.StatusGone)

			// Preview still returns the band + reason=revoked.
			resp, pbody := bob.do(http.MethodGet, "/api/invite-links/"+token, nil)
			mustStatus(t, resp, http.StatusOK)
			var reason string
			_ = json.Unmarshal(pbody["reason"], &reason)
			if reason != "revoked" {
				t.Fatalf("expected reason=revoked, got %q", reason)
			}
		})
	}
}
