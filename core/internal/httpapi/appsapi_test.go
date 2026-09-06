package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"troubastack/core/internal/httpapi"
)

// appsServer mounts an AppsAPI over the given dir + version on a test server.
func appsServer(t *testing.T, dir, version string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	httpapi.NewAppsAPI(dir, version).Mount(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type appsManifest struct {
	Apps []struct {
		Platform string `json:"platform"`
		Version  string `json:"version"`
		Size     int64  `json:"size"`
		Path     string `json:"path"`
		Filename string `json:"filename"`
	} `json:"apps"`
}

func getManifest(t *testing.T, srv *httptest.Server) appsManifest {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/apps")
	if err != nil {
		t.Fatalf("GET /api/apps: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/apps status = %d, want 200", resp.StatusCode)
	}
	var m appsManifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return m
}

// TestApps_absentDir: no apps dir ⇒ empty manifest (never an error) + 404 download,
// so dev runs and the UI-hides-the-card path both hold.
func TestApps_absentDir(t *testing.T) {
	srv := appsServer(t, "", "v1.0.0")
	if m := getManifest(t, srv); len(m.Apps) != 0 {
		t.Fatalf("manifest with no dir = %+v, want empty", m.Apps)
	}
	resp, err := http.Get(srv.URL + "/apps/troubastage.apk")
	if err != nil {
		t.Fatalf("GET apk: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("download with no dir = %d, want 404", resp.StatusCode)
	}
}

// TestApps_withApk: a dir holding the apk ⇒ manifest lists android with the build
// version + size + path, and the download serves the right MIME + a versioned
// attachment filename.
func TestApps_withApk(t *testing.T) {
	dir := t.TempDir()
	body := []byte("PK\x03\x04 fake apk bytes")
	if err := os.WriteFile(filepath.Join(dir, "troubastage.apk"), body, 0o644); err != nil {
		t.Fatalf("write apk: %v", err)
	}
	srv := appsServer(t, dir, "v2.3.4")

	m := getManifest(t, srv)
	if len(m.Apps) != 1 {
		t.Fatalf("manifest = %+v, want one android entry", m.Apps)
	}
	e := m.Apps[0]
	if e.Platform != "android" || e.Version != "v2.3.4" || e.Size != int64(len(body)) ||
		e.Path != "/apps/troubastage.apk" || e.Filename != "troubastage-v2.3.4.apk" {
		t.Fatalf("manifest entry = %+v, want android/v2.3.4/%d/…", e, len(body))
	}

	resp, err := http.Get(srv.URL + e.Path)
	if err != nil {
		t.Fatalf("GET apk: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/vnd.android.package-archive" {
		t.Fatalf("Content-Type = %q, want the apk MIME", ct)
	}
	// T162: the header now also carries an RFC 5987 filename*; the ASCII fallback (old-client parse) is
	// unchanged and still leads. This name is pure ASCII, so both parts read the same versioned filename.
	if cd := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, `attachment; filename="troubastage-v2.3.4.apk"`) {
		t.Fatalf("Content-Disposition = %q, want versioned filename", cd)
	}
	got, _ := io.ReadAll(resp.Body)
	if string(got) != string(body) {
		t.Fatalf("downloaded bytes differ from the apk")
	}
}

// TestApps_unknownFile: only allow-listed names are served (no traversal surface).
func TestApps_unknownFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("nope"), 0o644)
	srv := appsServer(t, dir, "v1")
	for _, name := range []string{"secret.txt", "troubastage.ipa", "..%2f..%2fetc%2fpasswd"} {
		resp, err := http.Get(srv.URL + "/apps/" + name)
		if err != nil {
			t.Fatalf("GET %s: %v", name, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET /apps/%s = %d, want 404 (only allow-listed names)", name, resp.StatusCode)
		}
	}
}
