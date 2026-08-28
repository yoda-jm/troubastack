package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"troubastack/core/internal/app/memrepo"
)

// T123 — GET /api/version now carries the SERVER IDENTITY fields (a `product` marker + the /api CONTRACT
// version) alongside its build fields, answered WITHOUT a session. The no-session part is load-bearing: a
// phone deciding whether to show a password field for a scanned QR host asks before it trusts the host,
// and there is no session then.
//
// version_test.go already proves the ROUTE is session-free (build fields). These assertions are NEW: they
// prove the IDENTITY FIELDS are present unauthenticated — which is what a client actually reads. A
// teeth-check wrapping /api/version in a.auth reddens THESE (product == "" at 401), not only the
// pre-existing build-field test.
func TestVersionIdentity_NoSessionRequired(t *testing.T) {
	c := newClient(t, memrepo.New())                  // fresh: empty cookie jar, and no users exist
	resp, err := http.Get(c.srv.URL + "/api/version") // NO session cookie, on purpose
	if err != nil {
		t.Fatalf("GET /api/version: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/version without a session = %d, want 200", resp.StatusCode)
	}
	var v map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var product string
	if err := json.Unmarshal(v["product"], &product); err != nil || product != "troubastack" {
		t.Fatalf("product marker = %q (err %v), want \"troubastack\" — the NEW T123 identity assertion", product, err)
	}
	if _, ok := v["apiVersion"]; !ok {
		t.Fatalf("no apiVersion field — the NEW T123 contract-version assertion: %v", v)
	}
	// No box disclosure beyond the pre-existing build diagnostics.
	for _, forbidden := range []string{"goVersion", "host", "hostname", "userCount", "bandCount", "users", "bands"} {
		if _, present := v[forbidden]; present {
			t.Errorf("/api/version discloses %q — identity must not reveal box internals", forbidden)
		}
	}
}

// The identity fields are independent of whether any user exists — a fresh box and a populated one answer
// identically, and neither needs a session (two clients over ONE repo; the second is session-less but a
// user exists).
func TestVersionIdentity_UnaffectedByUsers(t *testing.T) {
	repo := memrepo.New()

	r0, _ := http.Get(newClient(t, repo).srv.URL + "/api/version") // no users, no session
	var v0 map[string]json.RawMessage
	json.NewDecoder(r0.Body).Decode(&v0)
	r0.Body.Close()

	admin := newClient(t, repo)
	admin.registerLogin("alice", "pw") // a user now exists in the shared repo

	r1, err := http.Get(newClient(t, repo).srv.URL + "/api/version") // fresh client: no session, user exists
	if err != nil {
		t.Fatal(err)
	}
	defer r1.Body.Close()
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/version with a user present (no session) = %d, want 200", r1.StatusCode)
	}
	var v1 map[string]json.RawMessage
	json.NewDecoder(r1.Body).Decode(&v1)
	if string(v0["product"]) != string(v1["product"]) || string(v0["apiVersion"]) != string(v1["apiVersion"]) {
		t.Fatalf("identity changed with user existence: %v vs %v", v0, v1)
	}
}
