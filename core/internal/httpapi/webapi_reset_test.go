package httpapi_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
	"time"

	"troubastack/core/internal/app"
)

// hashTok mirrors the Service's token-at-rest hashing (SHA-256 hex). Reset
// tokens are stored hashed; the test uses this both to assert that property and
// to craft an expired grant straight into the repo.
func hashTok(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

// TestPasswordReset covers the full admin-issue → out-of-band link → set flow:
// the token is stored hashed, single-use, and consuming it invalidates the
// user's existing sessions; the old password stops working and the new one logs
// in.
func TestPasswordReset(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)

			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")

			// Bob is a member of the band with an active session.
			bob := newClient(t, repo)
			bob.registerLogin("bob", "oldpass1")
			bobID := bob.meID()
			if err := repo.AddMembership(app.Membership{
				BandID: band.ID, UserID: bobID, Role: app.RoleMember, CreatedAt: time.Now().UTC(),
			}); err != nil {
				t.Fatalf("add membership: %v", err)
			}

			// Admin issues a reset for Bob.
			resp, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/members/"+bobID+"/password-reset", nil)
			mustStatus(t, resp, http.StatusCreated)
			var token, resetPath string
			unmarshalField(t, body, "token", &token)
			unmarshalField(t, body, "resetPath", &resetPath)
			if token == "" {
				t.Fatal("issue returned no token")
			}
			if resetPath != "/reset-password/"+token {
				t.Fatalf("resetPath = %q, want /reset-password/<token>", resetPath)
			}

			// Stored HASHED, not in the clear: the plaintext token is not a key,
			// its hash is.
			if _, err := repo.GetPasswordReset(token); err == nil {
				t.Fatal("reset token stored in the clear (plaintext is a key)")
			}
			if pr, err := repo.GetPasswordReset(hashTok(token)); err != nil {
				t.Fatalf("reset not stored under its hash: %v", err)
			} else if pr.UserID != bobID {
				t.Fatalf("reset bound to %q, want bob %q", pr.UserID, bobID)
			}

			// Anyone holding the token (locked-out Bob, no session) can preview it.
			anon := newClient(t, repo)
			resp, pb := anon.do(http.MethodGet, "/api/password-reset/"+token, nil)
			mustStatus(t, resp, http.StatusOK)
			var who struct {
				Username string `json:"username"`
			}
			unmarshalField(t, pb, "user", &who)
			if who.Username != "bob" {
				t.Fatalf("preview named %q, want bob", who.Username)
			}

			// Set the new password.
			resp, _ = anon.do(http.MethodPost, "/api/password-reset/"+token, map[string]string{"newPassword": "newpass1"})
			mustStatus(t, resp, http.StatusNoContent)

			// Single-use: the token is burned.
			resp, _ = anon.do(http.MethodPost, "/api/password-reset/"+token, map[string]string{"newPassword": "another1"})
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("second consume = %d, want 404 (single-use)", resp.StatusCode)
			}

			// Bob's OLD session is dead.
			resp, _ = bob.do(http.MethodGet, "/api/me", nil)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("old session still valid: GET /me = %d, want 401", resp.StatusCode)
			}

			// Old password no longer logs in; the new one does.
			login := newClient(t, repo)
			resp, _ = login.do(http.MethodPost, "/api/auth/login", map[string]string{"username": "bob", "password": "oldpass1"})
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("old password still works: login = %d, want 401", resp.StatusCode)
			}
			resp, _ = login.do(http.MethodPost, "/api/auth/login", map[string]string{"username": "bob", "password": "newpass1"})
			mustStatus(t, resp, http.StatusOK)
		})
	}
}

// TestPasswordResetAuthz: only a band admin can issue, only for a member of
// THAT band (no cross-band reach), and non-admins are refused.
func TestPasswordResetAuthz(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)

			admin := newClient(t, repo)
			bandA := admin.makeBand("alice", "A")
			aliceID := bandA.OwnerID

			// Bob: a plain member of band A.
			bob := newClient(t, repo)
			bob.registerLogin("bob", "pw123456")
			bobID := bob.meID()
			if err := repo.AddMembership(app.Membership{BandID: bandA.ID, UserID: bobID, Role: app.RoleMember, CreatedAt: time.Now().UTC()}); err != nil {
				t.Fatalf("add membership: %v", err)
			}

			// Carol: admin of a DIFFERENT band B.
			carol := newClient(t, repo)
			bandB := carol.makeBand("carol", "B")

			// A member (bob) cannot issue a reset (needs admin).
			resp, _ := bob.do(http.MethodPost, "/api/bands/"+bandA.ID+"/members/"+aliceID+"/password-reset", nil)
			if resp.StatusCode < 400 {
				t.Fatalf("member issue should be denied, got %d", resp.StatusCode)
			}

			// Alice cannot reset a user who is not a member of her band (carol is in B).
			resp, _ = admin.do(http.MethodPost, "/api/bands/"+bandA.ID+"/members/"+bandB.OwnerID+"/password-reset", nil)
			if resp.StatusCode < 400 {
				t.Fatalf("cross-band target should be denied, got %d", resp.StatusCode)
			}

			// Alice cannot act as admin on a band she is not in (band B).
			resp, _ = admin.do(http.MethodPost, "/api/bands/"+bandB.ID+"/members/"+bandB.OwnerID+"/password-reset", nil)
			if resp.StatusCode < 400 {
				t.Fatalf("non-member admin on foreign band should be denied, got %d", resp.StatusCode)
			}
		})
	}
}

// TestPasswordResetExpiry: an expired token is refused for both preview and set.
// The grant is crafted directly into the repo with a past expiry (the Service
// clock is time.Now here).
func TestPasswordResetExpiry(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)

			bob := newClient(t, repo)
			bob.registerLogin("bob", "oldpass1")
			bobID := bob.meID()

			// Seed two expired grants: previewing one sweeps it (the Service drops
			// expired tokens on read), so the set-path check needs its own.
			seedExpired := func(tok string) {
				if err := repo.CreatePasswordReset(app.PasswordReset{
					TokenHash: hashTok(tok),
					UserID:    bobID,
					CreatedAt: time.Now().Add(-48 * time.Hour).UTC(),
					ExpiresAt: time.Now().Add(-24 * time.Hour).UTC(),
				}); err != nil {
					t.Fatalf("seed expired reset: %v", err)
				}
			}
			seedExpired("expired-preview-fixture-0000000000")
			seedExpired("expired-set-fixture-0000000000000")

			anon := newClient(t, repo)
			resp, _ := anon.do(http.MethodGet, "/api/password-reset/expired-preview-fixture-0000000000", nil)
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("expired preview = %d, want 403", resp.StatusCode)
			}
			resp, _ = anon.do(http.MethodPost, "/api/password-reset/expired-set-fixture-0000000000000", map[string]string{"newPassword": "newpass1"})
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("expired set = %d, want 403", resp.StatusCode)
			}

			// Unknown token → 404.
			resp, _ = anon.do(http.MethodGet, "/api/password-reset/nope", nil)
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("unknown token = %d, want 404", resp.StatusCode)
			}
		})
	}
}
