package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestVersionEndpoint (T29): GET /api/version is unauthenticated (like /healthz)
// and returns the build identity — version, builtAt, and whether a real SPA is
// embedded. Unstamped test builds report "dev"/"unknown"; the test checkout
// carries only the committed placeholder, so spaEmbedded is false here.
func TestVersionEndpoint(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			resp, err := http.Get(c.srv.URL + "/api/version") // NO auth cookie on purpose
			if err != nil {
				t.Fatalf("GET /api/version: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			var v struct {
				Version     string `json:"version"`
				BuiltAt     string `json:"builtAt"`
				SPAEmbedded *bool  `json:"spaEmbedded"`
			}
			if err := json.Unmarshal(body, &v); err != nil {
				t.Fatalf("unmarshal %q: %v", body, err)
			}
			if v.Version == "" {
				t.Fatal("version must never be empty (unstamped = \"dev\")")
			}
			if v.BuiltAt == "" {
				t.Fatal("builtAt must never be empty (unstamped = \"unknown\")")
			}
			if v.SPAEmbedded == nil {
				t.Fatal("spaEmbedded must be present")
			}
			// NOTE: the VALUE is environment-dependent (false on a fresh checkout's
			// placeholder; true if the dev ran `make dist` first, since go:embed bakes
			// whatever is on disk at compile time) — so only presence is asserted.
		})
	}
}
