package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"troubastack/core/internal/app"
)

// condGet does a GET carrying an optional If-None-Match, preserving response headers
// (getRaw/do don't let a test set request headers).
func condGet(t *testing.T, c *client, path, ifNoneMatch string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, c.srv.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for _, ck := range c.jar {
		req.AddCookie(ck)
	}
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	resp.Body.Close()
	return resp
}

// TestDownloadFileCacheFreshness (T67) covers the freshness contract on downloadFile that
// keeps a re-rendered chart from showing stale: a strong ETag (the content-addressed
// blobHash); a bare /api/files/{id} that must revalidate (Cache-Control: no-cache); a
// revision-pinned ?rev= URL that caches immutably; If-None-Match → 304; and — the actual
// bug — re-rendering the chart changes the ETag, so the new ?rev URL serves NEW bytes.
func TestDownloadFileCacheFreshness(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			song := admin.makeSong(band.ID, "Song")
			base := "/api/bands/" + band.ID + "/songs/" + song.ID

			// A generated chart (rev 1).
			resp, body := admin.do(http.MethodPost, base+"/text-charts", map[string]string{"source": chartSrc})
			mustStatus(t, resp, http.StatusCreated)
			var f app.SongFile
			unmarshalField(t, body, "file", &f)

			// Bare URL: a strong ETag + revalidate (never serve stale bytes for an id whose
			// render can change in place).
			bare := condGet(t, admin, "/api/files/"+f.ID, "")
			mustStatus(t, bare, http.StatusOK)
			etag1 := bare.Header.Get("ETag")
			if etag1 == "" || etag1 == `""` {
				t.Fatalf("bare download: ETag = %q, want the blobHash", etag1)
			}
			if cc := bare.Header.Get("Cache-Control"); cc != "no-cache" {
				t.Fatalf("bare download: Cache-Control = %q, want no-cache", cc)
			}

			// Revision-pinned URL: immutable caching (that {id,rev} pair never changes for
			// the client — a re-render is a new rev = a new URL), same ETag.
			pinned := condGet(t, admin, "/api/files/"+f.ID+"?rev=1", "")
			mustStatus(t, pinned, http.StatusOK)
			if cc := pinned.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
				t.Fatalf("?rev download: Cache-Control = %q, want immutable", cc)
			}
			if pinned.Header.Get("ETag") != etag1 {
				t.Fatalf("?rev ETag = %q, bare ETag = %q, want equal", pinned.Header.Get("ETag"), etag1)
			}

			// If-None-Match with the current ETag → 304 (browser revalidation is cheap).
			nm := condGet(t, admin, "/api/files/"+f.ID+"?rev=1", etag1)
			if nm.StatusCode != http.StatusNotModified {
				t.Fatalf("If-None-Match current etag = %d, want 304", nm.StatusCode)
			}

			// THE BUG: save new source → re-render in place (rev 2, new blob). The ETag MUST
			// change, so the new ?rev=2 URL serves the NEW bytes and can't collide with the
			// browser's cached rev-1 copy.
			resp, _ = admin.do(http.MethodPut, base+"/files/"+f.ID+"/chart-source",
				map[string]any{"source": "# My Chart v2\n\n## Chorus\nsing", "baseRevision": 1})
			mustStatus(t, resp, http.StatusOK)

			after := condGet(t, admin, "/api/files/"+f.ID+"?rev=2", "")
			mustStatus(t, after, http.StatusOK)
			etag2 := after.Header.Get("ETag")
			if etag2 == etag1 {
				t.Fatalf("after re-render ETag unchanged (%q) — a new render must be new bytes", etag2)
			}

			// Presenting the OLD etag against the current file yields fresh bytes (200), not a
			// stale 304 — the exact "F5 still stale" failure this fixes.
			stale := condGet(t, admin, "/api/files/"+f.ID+"?rev=2", etag1)
			if stale.StatusCode != http.StatusOK {
				t.Fatalf("stale If-None-Match = %d, want 200 (fresh bytes, not a stale 304)", stale.StatusCode)
			}
		})
	}
}
