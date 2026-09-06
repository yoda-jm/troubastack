package app_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/filerepo"
)

// tokenHash mirrors the service's storage key (SHA-256 hex) so the test can assert
// what is — and is not — on disk without reaching into the package.
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func openFileService(t *testing.T, dir string) *app.Service {
	t.Helper()
	repo, err := filerepo.New(dir)
	if err != nil {
		t.Fatalf("filerepo.New: %v", err)
	}
	return app.NewService(repo)
}

// TestSessionSurvivesRestart is the T160 motivating case: a session minted before a
// restart must still authenticate its user after the store is reloaded from disk.
// Teeth: today UserID is json:"-", so the reloaded record is an empty husk and
// UserForToken fails — this test goes red on current code.
func TestSessionSurvivesRestart(t *testing.T) {
	dir := t.TempDir()

	svc := openFileService(t, dir)
	if _, err := svc.Register("dave", "Dave", "oldpass1", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	u, token, err := svc.Login("dave", "oldpass1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// Simulate a server restart: a fresh repo/service over the same directory.
	svc2 := openFileService(t, dir)
	got, err := svc2.UserForToken(token)
	if err != nil {
		t.Fatalf("after restart UserForToken: %v — the session did not survive", err)
	}
	if got.ID != u.ID {
		t.Fatalf("after restart resolved user %q, want %q", got.ID, u.ID)
	}
}

// TestSessionStoresOnlyHash: the raw bearer token must never touch disk — only its
// SHA-256 hash, the PasswordReset shape. Teeth: today the token is the map key, so
// it is written verbatim and this goes red.
func TestSessionStoresOnlyHash(t *testing.T) {
	dir := t.TempDir()
	svc := openFileService(t, dir)
	if _, err := svc.Register("dave", "Dave", "oldpass1", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	_, token, err := svc.Login("dave", "oldpass1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "app.json"))
	if err != nil {
		t.Fatalf("read app.json: %v", err)
	}
	on := string(b)
	if strings.Contains(on, token) {
		t.Fatal("raw bearer token found in app.json — a leaked dataset would be live credentials")
	}
	if !strings.Contains(on, tokenHash(token)) {
		t.Fatal("token hash not found on disk — the session was not persisted by hash")
	}
}

// TestSessionHusksPrunedOnLoad: the dead {} records the old design accumulated (a
// session that authenticates nobody) must not survive a reload. A valid session
// alongside them is kept. Teeth: without pruning, the husk count stays > 0.
func TestSessionHusksPrunedOnLoad(t *testing.T) {
	dir := t.TempDir()
	svc := openFileService(t, dir)
	if _, err := svc.Register("dave", "Dave", "oldpass1", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	_, token, err := svc.Login("dave", "oldpass1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// Inject two husks (empty records, as the pre-T160 store wrote) beside the valid one.
	path := filepath.Join(dir, "app.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var sessions map[string]json.RawMessage
	if err := json.Unmarshal(raw["sessions"], &sessions); err != nil {
		t.Fatalf("unmarshal sessions: %v", err)
	}
	sessions["husk-token-1"] = json.RawMessage(`{}`)
	sessions["husk-token-2"] = json.RawMessage(`{}`)
	raw["sessions"], _ = json.Marshal(sessions)
	nb, _ := json.Marshal(raw)
	if err := os.WriteFile(path, nb, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Reload — pruning happens on load and rewrites the file clean.
	svc2 := openFileService(t, dir)
	if _, err := svc2.UserForToken(token); err != nil {
		t.Fatalf("valid session must survive the prune: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	var raw2 map[string]json.RawMessage
	if err := json.Unmarshal(after, &raw2); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	var sessions2 map[string]json.RawMessage
	if err := json.Unmarshal(raw2["sessions"], &sessions2); err != nil {
		t.Fatalf("re-unmarshal sessions: %v", err)
	}
	if len(sessions2) != 1 {
		t.Fatalf("after prune: %d sessions on disk, want 1 (husks not removed)", len(sessions2))
	}
	if _, ok := sessions2[tokenHash(token)]; !ok {
		t.Fatal("the valid session (by hash) was not the one kept")
	}
}
